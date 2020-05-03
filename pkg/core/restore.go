package core

import (
	"fmt"
	"io"
	"os"
	"path"

	log "github.com/sirupsen/logrus"

	"github.com/databacker/mysql-backup/pkg/archive"
	"github.com/databacker/mysql-backup/pkg/compression"
	"github.com/databacker/mysql-backup/pkg/database"
	"github.com/databacker/mysql-backup/pkg/storage"
	"github.com/databacker/mysql-backup/pkg/storage/credentials"
	"github.com/databacker/mysql-backup/pkg/storage/file"
	"github.com/databacker/mysql-backup/pkg/storage/s3"
	"github.com/databacker/mysql-backup/pkg/storage/smb"
)

const (
	preRestoreDir  = "/scripts.d/pre-restore"
	postRestoreDir = "/scripts.d/post-restore"
	tmpRestoreFile = "/tmp/restorefile"
)

// Restore restore a specific backup into the database
func Restore(target string, dbconn database.Connection, creds credentials.Creds, compressor compression.Compressor) error {
	log.Info("beginning restore")
	// execute pre-restore scripts if any
	if err := preRestore(target); err != nil {
		return fmt.Errorf("error running pre-restore: %v", err)
	}

	// parse the target URL
	u, err := smartParse(target)
	if err != nil {
		return fmt.Errorf("invalid target url: %v", err)
	}
	log.Debugf("restore target: %#v", u)

	// do the restore
	var store storage.Storage
	switch u.Scheme {
	case "file":
		log.Debugf("restoring via file protocol, temporary file location %s", tmpRestoreFile)
		store = file.New()
	case "smb":
		log.Debugf("restoring via smb protocol, temporary file location %s", tmpRestoreFile)
		store = smb.New()
	case "s3":
		log.Debugf("restoring via s3 protocol, temporary file location %s", tmpRestoreFile)
		store = s3.New()
	default:
		return fmt.Errorf("unknown url protocol: %s", u.Scheme)
	}
	copied, err := store.Pull(creds, *u, tmpRestoreFile)
	if err != nil {
		return fmt.Errorf("failed to pull target %s: %v", target, err)
	}
	log.Debugf("completed copying %d bytes", copied)

	// successfully download file, now restore it
	tmpdir, err := os.MkdirTemp("", "restore")
	if err != nil {
		return fmt.Errorf("unable to create temporary working directory: %v", err)
	}
	defer os.RemoveAll(tmpdir)
	f, err := os.Open(tmpRestoreFile)
	if f == nil {
		return fmt.Errorf("unable to read the temporary download file: %v", err)
	}
	defer f.Close()
	os.Remove(tmpRestoreFile)

	// create my tar reader to put the files in the directory
	cr, err := compressor.Uncompress(f)
	if err != nil {
		return fmt.Errorf("unable to create an uncompressor: %v", err)
	}
	if err := archive.Untar(cr, tmpdir); err != nil {
		return fmt.Errorf("error extracting the file: %v", err)
	}

	// run through each file and apply it
	files, err := os.ReadDir(tmpdir)
	if err != nil {
		return fmt.Errorf("failed to find extracted files to restore: %v", err)
	}
	readers := make([]io.Reader, 0)
	for _, f := range files {
		// ignore directories
		if f.IsDir() {
			continue
		}
		file, err := os.Open(path.Join(tmpdir, f.Name()))
		if err != nil {
			continue
		}
		defer file.Close()
		readers = append(readers, file)
	}
	if err := database.Restore(dbconn, readers); err != nil {
		return fmt.Errorf("failed to restore database: %v", err)
	}

	// execute post-restore scripts if any
	if err := postRestore(target); err != nil {
		return fmt.Errorf("error running post-restove: %v", err)
	}
	return nil
}

// run pre-restore scripts, if they exist
func preRestore(target string) error {
	// construct any additional environment
	env := map[string]string{
		"DB_RESTORE_TARGET": target,
	}
	return runScripts(preRestoreDir, env)
}

func postRestore(target string) error {
	// construct any additional environment
	env := map[string]string{
		"DB_RESTORE_TARGET": target,
	}
	return runScripts(postRestoreDir, env)
}
