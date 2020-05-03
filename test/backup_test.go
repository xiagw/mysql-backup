//go:build integration

package test

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/databacker/mysql-backup/pkg/compression"
	"github.com/databacker/mysql-backup/pkg/core"
	"github.com/databacker/mysql-backup/pkg/database"
	"github.com/databacker/mysql-backup/pkg/storage/credentials"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/docker/go-connections/nat"
	"github.com/johannesboyne/gofakes3"
	"github.com/johannesboyne/gofakes3/backend/s3mem"
	"github.com/moby/moby/pkg/archive"
	log "github.com/sirupsen/logrus"
)

const (
	mysqlUser     = "user"
	mysqlPass     = "abcdefg"
	mysqlRootUser = "root"
	mysqlRootPass = "root"
	smbImage      = "mysqlbackup_smb_test:latest"
	mysqlImage    = "mysql:8.0"
	s3Image       = "lphoward/fake-s3"
)

var dumpFilterRegex = regexp.MustCompile(`s/^(.*SET character_set_client.*|s/^\/\*![0-9]\{5\}.*\/;$//g)$//g`)

type containerPort struct {
	name string
	id   string
	port int
}
type dockerContext struct {
	cli *client.Client
}

type backupTarget struct {
	s     string
	id    string
	subid string
}

func (t backupTarget) String() string {
	return t.s
}
func (t backupTarget) WithPrefix(prefix string) string {
	// prepend the prefix to the path, but only to the path
	u, err := url.Parse(t.s)
	if err != nil {
		return ""
	}
	u.Path = filepath.Join(prefix, u.Path)
	return u.String()
}

func (t backupTarget) Scheme() string {
	u, err := url.Parse(t.s)
	if err != nil {
		return ""
	}
	return u.Scheme
}
func (t backupTarget) Host() string {
	u, err := url.Parse(t.s)
	if err != nil {
		return ""
	}
	return u.Host
}
func (t backupTarget) Path() string {
	u, err := url.Parse(t.s)
	if err != nil {
		return ""
	}
	return u.Path
}

// uniquely generated ID of the target. Shared across multiple targets that are part of the same
// backup set, e.g. "file:///backups/ smb://smb/path", where each sub has its own subid
func (t backupTarget) ID() string {
	return t.id
}
func (t backupTarget) SubID() string {
	return t.subid
}

// getDockerContext retrieves a Docker context with a prepared client handle
func getDockerContext() (*dockerContext, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, err
	}
	return &dockerContext{cli}, nil
}

func (d *dockerContext) execInContainer(ctx context.Context, cid string, cmd []string) (types.HijackedResponse, int, error) {
	execConfig := types.ExecConfig{
		AttachStdout: true,
		AttachStderr: true,
		Cmd:          cmd,
	}
	execResp, err := d.cli.ContainerExecCreate(ctx, cid, execConfig)
	if err != nil {
		return types.HijackedResponse{}, 0, fmt.Errorf("failed to create exec: %w", err)
	}
	var execStartCheck types.ExecStartCheck
	attachResp, err := d.cli.ContainerExecAttach(ctx, execResp.ID, execStartCheck)
	if err != nil {
		return attachResp, 0, fmt.Errorf("failed to attach to exec: %w", err)
	}
	var (
		retryMax   = 20
		retrySleep = 1
		success    bool
		inspect    types.ContainerExecInspect
	)
	for i := 0; i < retryMax; i++ {
		inspect, err = d.cli.ContainerExecInspect(ctx, execResp.ID)
		if err != nil {
			return attachResp, 0, fmt.Errorf("failed to inspect exec: %w", err)
		}
		if !inspect.Running {
			success = true
			break
		}
		time.Sleep(time.Duration(retrySleep) * time.Second)
	}
	if !success {
		return attachResp, 0, fmt.Errorf("failed to wait for exec to finish")
	}
	return attachResp, inspect.ExitCode, nil
}
func (d *dockerContext) waitForDBConnectionAndGrantPrivileges(mysqlCID, dbuser, dbpass string) error {
	ctx := context.Background()

	// Allow up to 20 seconds for the mysql database to be ready
	retryMax := 20
	retrySleep := 1
	success := false

	for i := 0; i < retryMax; i++ {
		// Check database connectivity
		dbValidate := []string{"mysql", fmt.Sprintf("-u%s", dbuser), fmt.Sprintf("-p%s", dbpass), "--protocol=tcp", "-h127.0.0.1", "--wait", "--connect_timeout=20", "tester", "-e", "select 1;"}
		attachResp, exitCode, err := d.execInContainer(ctx, mysqlCID, dbValidate)
		if err != nil {
			return fmt.Errorf("failed to attach to exec: %w", err)
		}
		defer attachResp.Close()
		if exitCode == 0 {
			success = true
			break
		}

		time.Sleep(time.Duration(retrySleep) * time.Second)
	}

	if !success {
		return fmt.Errorf("failed to connect to database after %d tries", retryMax)
	}

	// Ensure the user has the right privileges
	dbGrant := []string{"mysql", fmt.Sprintf("-u%s", dbpass), fmt.Sprintf("-p%s", dbpass), "--protocol=tcp", "-h127.0.0.1", "-e", "grant process on *.* to user;"}
	attachResp, exitCode, err := d.execInContainer(ctx, mysqlCID, dbGrant)
	if err != nil {
		return fmt.Errorf("failed to attach to exec: %w", err)
	}
	defer attachResp.Close()
	var bufo, bufe bytes.Buffer
	_, _ = stdcopy.StdCopy(&bufo, &bufe, attachResp.Reader)
	if exitCode != 0 {
		return fmt.Errorf("failed to grant privileges to user: %s", bufe.String())
	}

	return nil
}

func (d *dockerContext) startSMBContainer(image, name, base string) (cid string, port int, err error) {
	return d.startContainer(image, name, "445/tcp", []string{fmt.Sprintf("%s:/share/backups", base)}, nil, nil)
}

func (d *dockerContext) startContainer(image, name, portMap string, binds []string, cmd []string, env []string) (cid string, port int, err error) {
	ctx := context.Background()

	// Start the SMB container
	containerConfig := &container.Config{
		Image: image,
		Cmd:   cmd,
		Labels: map[string]string{
			"mysqltest": "",
		},
		Env: env,
	}
	hostConfig := &container.HostConfig{
		Binds: binds,
	}
	var containerPort nat.Port
	if portMap != "" {
		containerPort = nat.Port(portMap)
		containerConfig.ExposedPorts = nat.PortSet{
			containerPort: struct{}{},
		}
		hostConfig.PortBindings = nat.PortMap{
			containerPort: []nat.PortBinding{{HostIP: "0.0.0.0"}},
		}
	}
	resp, err := d.cli.ContainerCreate(ctx, containerConfig, hostConfig, nil, nil, name)
	if err != nil {
		return
	}
	cid = resp.ID
	err = d.cli.ContainerStart(ctx, cid, types.ContainerStartOptions{})
	if err != nil {
		return
	}

	// Retrieve the randomly assigned port
	if portMap == "" {
		return
	}
	inspect, err := d.cli.ContainerInspect(ctx, cid)
	if err != nil {
		return
	}
	portStr := inspect.NetworkSettings.Ports[containerPort][0].HostPort
	port, err = strconv.Atoi(portStr)

	return
}

func (d *dockerContext) makeSMB(smbImage string) error {
	ctx := context.Background()

	// Build the smbImage
	buildSMBImageOpts := types.ImageBuildOptions{
		Context: nil,
		Tags:    []string{smbImage},
		Remove:  true,
	}

	tar, err := archive.TarWithOptions("ctr/", &archive.TarOptions{})
	if err != nil {
		return fmt.Errorf("failed to create tar archive: %w", err)
	}
	buildSMBImageOpts.Context = io.NopCloser(tar)

	if _, err = d.cli.ImageBuild(ctx, buildSMBImageOpts.Context, buildSMBImageOpts); err != nil {
		return fmt.Errorf("failed to build smb image: %w", err)
	}

	return nil
}

func (d *dockerContext) createBackupFile(mysqlCID, mysqlUser, mysqlPass, outfile string) error {
	ctx := context.Background()

	// Create and populate the table
	mysqlCreateCmd := []string{"mysql", "-hlocalhost", "--protocol=tcp", fmt.Sprintf("-u%s", mysqlUser), fmt.Sprintf("-p%s", mysqlPass), "-e", `use tester; create table t1 (id INT, name VARCHAR(20)); INSERT INTO t1 (id,name) VALUES (1, "John"), (2, "Jill"), (3, "Sam"), (4, "Sarah");`}
	attachResp, exitCode, err := d.execInContainer(ctx, mysqlCID, mysqlCreateCmd)
	if err != nil {
		return fmt.Errorf("failed to attach to exec: %w", err)
	}
	defer attachResp.Close()
	if exitCode != 0 {
		return fmt.Errorf("failed to create table: %w", err)
	}
	var bufo, bufe bytes.Buffer
	_, _ = stdcopy.StdCopy(&bufo, &bufe, attachResp.Reader)

	// Dump the database
	mysqlDumpCmd := []string{"mysqldump", "-hlocalhost", "--protocol=tcp", fmt.Sprintf("-u%s", mysqlUser), fmt.Sprintf("-p%s", mysqlPass), "--compact", "--databases", "tester"}
	attachResp, exitCode, err = d.execInContainer(ctx, mysqlCID, mysqlDumpCmd)
	if err != nil {
		return fmt.Errorf("failed to attach to exec: %w", err)
	}
	defer attachResp.Close()
	if exitCode != 0 {
		return fmt.Errorf("failed to dump database: %w", err)
	}

	f, err := os.Create(outfile)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = io.Copy(f, attachResp.Reader)
	return err
}

func (d *dockerContext) logContainers(cids ...string) error {
	ctx := context.Background()
	for _, cid := range cids {
		logOptions := types.ContainerLogsOptions{
			ShowStdout: true,
			ShowStderr: true,
		}
		logs, err := d.cli.ContainerLogs(ctx, cid, logOptions)
		if err != nil {
			return fmt.Errorf("failed to get logs for container %s: %w", cid, err)
		}
		defer logs.Close()

		if _, err := io.Copy(os.Stdout, logs); err != nil {
			return fmt.Errorf("failed to stream logs for container %s: %w", cid, err)
		}
	}
	return nil
}

func (d *dockerContext) rmContainers(cids ...string) error {
	ctx := context.Background()
	for _, cid := range cids {
		if err := d.cli.ContainerKill(ctx, cid, "SIGKILL"); err != nil {
			return fmt.Errorf("failed to kill container %s: %w", cid, err)
		}

		rmOpts := types.ContainerRemoveOptions{
			RemoveVolumes: true,
			Force:         true,
		}
		if err := d.cli.ContainerRemove(ctx, cid, rmOpts); err != nil {
			return fmt.Errorf("failed to remove container %s: %w", cid, err)
		}
	}
	return nil
}

// we need to run through each each target and test the backup.
// before the first run, we:
// - start the sql database
// - populate it with a few inserts/creates
// - run a single clear backup
// for each stage, we:
// - clear the target
// - run the backup
// - check that the backup now is there in the right format
// - clear the target

func runDumpTest(dc *dockerContext, base string, targets []backupTarget, sequence int, smb, mysql containerPort, s3 string) error {
	dbconn := database.Connection{
		User: mysqlUser,
		Pass: mysqlPass,
		Host: "localhost",
		Port: mysql.port,
	}
	var stringTargets []string
	// all targets should have the same sequence, with varying subsequence, so take any one
	var id string
	for _, target := range targets {
		t := target.String()
		id = target.ID()
		if target.Scheme() == "file" || target.Scheme() == "" {
			t = target.WithPrefix(base)
		}
		stringTargets = append(stringTargets, t)
	}
	dumpOpts := core.DumpOptions{
		Targets: stringTargets,
		DBConn:  dbconn,
		Creds: credentials.Creds{
			AWSEndpoint: s3,
		},
		Compressor:        &compression.GzipCompressor{},
		PreBackupScripts:  filepath.Join(base, "backups", id, "pre-backup"),
		PostBackupScripts: filepath.Join(base, "backups", id, "post-backup"),
	}
	timerOpts := core.TimerOptions{
		Once: true,
	}
	return core.TimerDump(dumpOpts, timerOpts)

	/*
		not sure what these were for in the old scripted backup test, so commenting out for now

				linkfile := /tmp/link.$$
			        ln -s /backups/$sequence ${linkfile}
			        docker cp ${linkfile} $cid:/scripts.d
			        rm ${linkfile}
	*/
}

func setup(dc *dockerContext, base, backupFile string) (mysql, smb containerPort, s3url string, s3backend gofakes3.Backend, err error) {
	if err := dc.makeSMB(smbImage); err != nil {
		return mysql, smb, s3url, s3backend, fmt.Errorf("failed to build smb image: %v", err)
	}

	// start up the various containers
	smbCID, smbPort, err := dc.startSMBContainer(smbImage, "smb", base)
	if err != nil {
		return
	}
	smb = containerPort{name: "smb", id: smbCID, port: smbPort}

	// start the s3 container
	s3backend = s3mem.New()
	s3 := gofakes3.New(s3backend)
	s3server := httptest.NewServer(s3.Server())
	s3url = s3server.URL

	// start the mysql container; configure it for lots of debug logging, in case we need it
	mysqlConf := `
[mysqld]
log_error       =/var/log/mysql/mysql_error.log
general_log_file=/var/log/mysql/mysql.log
general_log     =1
slow_query_log  =1
slow_query_log_file=/var/log/mysql/mysql_slow.log
long_query_time =2
log_queries_not_using_indexes = 1
`
	confFile := filepath.Join(base, "log.cnf")
	if err := os.WriteFile(confFile, []byte(mysqlConf), 0644); err != nil {
		return mysql, smb, s3url, s3backend, fmt.Errorf("failed to write mysql config file: %v", err)
	}
	logDir := filepath.Join(base, "mysql_logs")
	if err := os.Mkdir(logDir, 0755); err != nil {
		return mysql, smb, s3url, s3backend, fmt.Errorf("failed to create mysql log directory: %v", err)
	}
	mysqlCID, mysqlPort, err := dc.startContainer(mysqlImage, "mysql", "3306/tcp", []string{fmt.Sprintf("%s:/etc/mysql/conf.d/log.conf:ro", confFile), fmt.Sprintf("%s:/var/log/mysql", logDir)}, nil, []string{
		fmt.Sprintf("MYSQL_ROOT_PASSWORD=%s", mysqlRootPass),
		"MYSQL_DATABASE=tester",
		fmt.Sprintf("MYSQL_USER=%s", mysqlUser),
		fmt.Sprintf("MYSQL_PASSWORD=%s", mysqlPass),
	})
	if err != nil {
		return
	}
	mysql = containerPort{name: "mysql", id: mysqlCID, port: mysqlPort}

	if err = dc.waitForDBConnectionAndGrantPrivileges(mysqlCID, mysqlRootUser, mysqlRootPass); err != nil {
		return
	}

	// create the backup file
	log.Debugf("Creating backup file")
	if err := dc.createBackupFile(mysql.id, mysqlUser, mysqlPass, backupFile); err != nil {
		return mysql, smb, s3url, s3backend, fmt.Errorf("failed to create backup file: %v", err)
	}
	return
}

func targetToTargets(target string, sequence int, smb containerPort) ([]backupTarget, error) {
	var (
		targets    = strings.Fields(target)
		allTargets []backupTarget
	)
	id := fmt.Sprintf("%05d", rand.Intn(10000))
	for i, t := range targets {
		subid := fmt.Sprintf("%02d", i)
		// parse the URL, taking any smb protocol and replacing the host:port with our local host:port
		u, err := url.Parse(t)
		if err != nil {
			return nil, err
		}
		if u.Scheme == "smb" {
			u.Host = fmt.Sprintf("localhost:%d", smb.port)
		}
		u.Path = filepath.Join(u.Path, id, subid, "data")
		finalTarget := u.String()
		// keep the original target in case of file
		if strings.HasPrefix(t, "/") {
			finalTarget = strings.TrimPrefix(finalTarget, "file://")
		}
		allTargets = append(allTargets, backupTarget{s: finalTarget, id: id, subid: subid})
	}
	// Configure the container
	if len(allTargets) == 0 {
		return nil, errors.New("must provide at least one target")
	}
	return allTargets, nil
}

type checkCommand func(base string, validBackup []byte, s3backend gofakes3.Backend, targets []backupTarget) (pass, fail []string, err error)

func runTest(t *testing.T, dc *dockerContext, targets []string, base string, prePost bool, backupData []byte, mysql, smb containerPort, s3 string, s3backend gofakes3.Backend, checkCommand checkCommand) {
	// run backups for each target
	var (
		passes, fails []string
	)
	for i, target := range targets {
		t.Run(target, func(t *testing.T) {
			// should add t.Parallel() here for parallel execution, but later
			log.Debugf("Running test for target '%s'", target)
			allTargets, err := targetToTargets(target, i, smb)
			if err != nil {
				t.Fatalf("failed to parse target: %v", err)
			}
			log.Debugf("Populating data for target %s", target)
			if err := populateVol(base, allTargets); err != nil {
				t.Fatalf("failed to populate volume for target %s: %v", target, err)
			}
			if err := populatePrePost(base, allTargets); err != nil {
				t.Fatalf("failed to populate pre-post for target %s: %v", target, err)
			}
			log.Debugf("Running backup for target %s", target)
			if err := runDumpTest(dc, base, allTargets, i, smb, mysql, s3); err != nil {
				t.Fatalf("failed to run dump test: %v", err)
			}

			pass, fail, err := checkCommand(base, backupData, s3backend, allTargets)
			if err != nil {
				t.Fatalf("failed to check backup: %v", err)
			}
			passes = append(passes, pass...)
			fails = append(fails, fail...)
			if len(fail) > 0 {
				t.Errorf("failed tests: %v", fail)
			}
		})
	}

	// report results - this should not be necessary, when we run within the go testing structure
	log.Printf("PASS: %d", len(passes))
	log.Printf("FAIL: %d", len(fails))
	if len(fails) > 0 {
		for _, fail := range fails {
			log.Printf("%s", fail)
		}
		t.Fatalf("failed to pass all tests")
	}
}

func checkDumpTest(base string, expected []byte, s3backend gofakes3.Backend, targets []backupTarget) (pass, fail []string, err error) {
	// all of it is in the volume we created, so check from there
	var (
		backupDataReader io.Reader
	)
	// we might have multiple targets
	for _, target := range targets {
		// check that the expected backups are in the right place
		var (
			id                = target.ID()
			subid             = target.SubID()
			scheme            = target.Scheme()
			postBackupOutFile = fmt.Sprintf("%s/backups/%s/post-backup/post-backup.txt", base, id)
			preBackupOutFile  = fmt.Sprintf("%s/backups/%s/pre-backup/pre-backup.txt", base, id)
			// useful for restore tests, which are disabled for now, so commented out
			//postRestoreFile   = fmt.Sprintf("%s/backups/%s/post-restore/post-restore.txt", base, sequence)
			//preRestoreFile    = fmt.Sprintf("%s/backups/%s/pre-restore/pre-restore.txt", base, sequence)
		)
		msg := fmt.Sprintf("%s %s post-backup", id, target.String())
		if _, err := os.Stat(postBackupOutFile); err == nil {
			pass = append(pass, msg)
		} else {
			fail = append(fail, fmt.Sprintf("%s script didn't run, output file doesn't exist", msg))
		}
		os.RemoveAll(postBackupOutFile)

		msg = fmt.Sprintf("%s %s pre-backup", id, target.String())
		if _, err := os.Stat(preBackupOutFile); err == nil {
			pass = append(pass, msg)
		} else {
			fail = append(fail, fmt.Sprintf("%s script didn't run, output file doesn't exist", msg))
		}
		os.RemoveAll(preBackupOutFile)

		switch scheme {
		case "s3":
			obj, err := s3backend.GetObject("backups", filepath.Join(id, subid, "data"), nil)
			if err != nil {
				return pass, fail, fmt.Errorf("failed to get backup object from s3: %v", err)
			}
			backupDataReader = obj.Contents
		default:
			bdir := filepath.Join(base, "backups", id, subid, "data")

			var backupFile string
			entries, err := os.ReadDir(bdir)
			if err != nil {
				return pass, fail, fmt.Errorf("failed to read backup directory %s: %v", bdir, err)
			}
			for _, entry := range entries {
				if strings.HasSuffix(entry.Name(), ".tgz") {
					backupFile = entry.Name()
					break
				}
			}
			if backupFile == "" {
				fail = append(fail, fmt.Sprintf("%s missing backup tgz file", id))
				continue
			}
			backupFile = filepath.Join(bdir, backupFile)
			backupDataReader, err = os.Open(backupFile)
			if err != nil {
				return pass, fail, fmt.Errorf("failed to read backup file %s: %v", backupFile, err)
			}
		}

		// extract the actual data, but filter out lines we do not care about
		b, err := gunzipScanFilter(backupDataReader)
		if err != nil {
			return pass, fail, err
		}

		if bytes.Equal(b, expected) {
			pass = append(pass, fmt.Sprintf("%s dump-contents", id))
		} else {
			fail = append(fail, fmt.Sprintf("%s tar contents do not match actual dump", id))
		}
	}

	return
}

func gunzipScanFilter(r io.Reader) (b []byte, err error) {
	gr, err := gzip.NewReader(r)
	if err != nil {
		return nil, err
	}
	defer gr.Close()
	scanner := bufio.NewScanner(gr)
	for scanner.Scan() {
		line := scanner.Text()
		if dumpFilterRegex.Match([]byte(line)) {
			continue
		}
		b = append(b, line...)
	}
	return b, nil
}

func populateVol(base string, targets []backupTarget) (err error) {
	workdir := filepath.Join(base, "backups")
	for _, target := range targets {
		dataDir := filepath.Join(base, target.Path())
		if err := os.MkdirAll(dataDir, 0777); err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(dataDir, "list"), []byte(fmt.Sprintf("target: %s\n", target)), 0666); err != nil {
			return err
		}

		// are we working with nopath?
		if target.Path() == "nopath" {
			if err := os.RemoveAll(filepath.Join(workdir, "nopath")); err != nil {
				return err
			}
			if err := os.Symlink(dataDir, filepath.Join(workdir, "nopath")); err != nil {
				return err
			}
		}
	}
	return
}

func populatePrePost(base string, targets []backupTarget) (err error) {
	// Create a test script for the post backup processing test
	if len(targets) == 0 {
		return fmt.Errorf("no targets specified")
	}
	id := targets[0].ID()
	workdir := filepath.Join(base, "backups", id)
	for _, dir := range []string{"pre-backup", "post-backup", "pre-restore", "post-restore"} {
		if err := os.MkdirAll(filepath.Join(workdir, dir), 0777); err != nil {
			return err
		}
		if err := os.WriteFile(
			filepath.Join(workdir, dir, "test.sh"),
			[]byte(fmt.Sprintf("#!/bin/bash\necho touch %s.txt", filepath.Join(workdir, dir, dir))),
			0777); err != nil {
			return err
		}
		// test.sh files need to be executable, but we already set them
		// might need to do this later
		// chmod -R 0777 /backups/${sequence}
		// chmod 755 /backups/${sequence}/*/test.sh
	}

	return nil
}

func TestIntegration(t *testing.T) {
	var (
		err        error
		smb, mysql containerPort
		s3         string
		s3backend  gofakes3.Backend
	)
	// temporary working directory
	base, err := os.MkdirTemp("", "backup-test-")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	dc, err := getDockerContext()
	if err != nil {
		t.Fatalf("failed to get docker client: %v", err)
	}
	backupFile := filepath.Join(base, "backup.sql")
	if mysql, smb, s3, s3backend, err = setup(dc, base, backupFile); err != nil {
		t.Fatalf("failed to setup test: %v", err)
	}
	backupData, err := os.ReadFile(backupFile)
	if err != nil {
		t.Fatalf("failed to read backup file %s: %v", backupFile, err)
	}
	defer func() {
		// log the results before tearing down, if requested
		if err := logContainers(dc, smb.id, mysql.id); err != nil {
			log.Errorf("failed to get logs from service containers: %v", err)
		}

		// tear everything down
		if err := teardown(dc, smb.id, mysql.id); err != nil {
			log.Errorf("failed to teardown test: %v", err)
		}
	}()

	t.Run("Dump", func(t *testing.T) {
		runTest(t, dc, []string{
			"/backups/",
			"file:///backups/",
			"smb://smb/noauth/",
			"smb://smb/nopath",
			"smb://user:pass@smb/auth",
			"smb://CONF;user:pass@smb/auth",
			"s3://mybucket/",
			"file:///backups/ file:///backups/",
		}, base, true, backupData, mysql, smb, s3, s3backend, checkDumpTest)
	})
}
