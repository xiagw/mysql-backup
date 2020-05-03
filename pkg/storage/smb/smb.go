package smb

import (
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/databacker/mysql-backup/pkg/storage/credentials"
	"github.com/hirochachacha/go-smb2"
)

type SMB struct{}

func New() *SMB {
	return &SMB{}
}

func (s *SMB) Pull(creds credentials.Creds, u url.URL, target string) (int64, error) {
	return smbCommand(false, creds, u, "", target)
}

func (s *SMB) Push(creds credentials.Creds, u url.URL, target, source string) (int64, error) {
	return smbCommand(true, creds, u, target, source)
}

func smbCommand(push bool, creds credentials.Creds, u url.URL, remoteFilename, filename string) (int64, error) {
	var (
		username, password string
	)

	hostname, path := u.Hostname(), u.Path
	share, sharepath := parseSMBPath(path)
	// get username and password as passed to us
	smbCreds := creds.SMBCredentials
	if smbCreds != "" {
		parts := strings.SplitN(smbCreds, "%", 2)
		switch len(parts) {
		case 1:
			username = parts[0]
		case 2:
			username, password = parts[0], parts[1]
		}
	}
	// if it was not passed to us, try from URL
	if username == "" && u.User != nil {
		username = u.User.Username()
		password, _ = u.User.Password()
	}

	conn, err := net.Dial("tcp", hostname)
	if err != nil {
		return 0, err
	}
	defer conn.Close()

	d := &smb2.Dialer{
		Initiator: &smb2.NTLMInitiator{
			User:     username, // THIS IS MISSING userdomain
			Password: password,
		},
	}

	s, err := d.Dial(conn)
	if err != nil {
		return 0, err
	}
	defer func() {
		_ = s.Logoff()
	}()

	fs, err := s.Mount(share)
	if err != nil {
		return 0, err
	}
	defer func() {
		_ = fs.Umount()
	}()

	var (
		from io.ReadCloser
		to   io.WriteCloser
	)
	if push {
		from, err = os.Open(filename)
		if err != nil {
			return 0, err
		}
		defer from.Close()
		to, err = fs.Create(fmt.Sprintf("%s%c%s", sharepath, smb2.PathSeparator, filepath.Base(strings.ReplaceAll(remoteFilename, ":", "-"))))
		if err != nil {
			return 0, err
		}
		defer to.Close()
	} else {
		to, err = os.Create(filename)
		if err != nil {
			return 0, err
		}
		defer to.Close()
		from, err = fs.Open(sharepath)
		if err != nil {
			return 0, err
		}
		defer from.Close()
	}
	return io.Copy(to, from)
}

// parseSMBDomain parse a username to get an SMB domain
// nolint: unused
func parseSMBDomain(username string) (user, domain string) {
	parts := strings.SplitN(username, ";", 2)
	if len(parts) < 2 {
		return username, ""
	}
	// if we reached this point, we have a username that has a domain in it
	return parts[1], parts[0]
}

// parseSMBPath parse an smb path into its constituent parts
func parseSMBPath(path string) (share, sharepath string) {
	parts := strings.SplitN(path, "/", 2)
	if len(parts) <= 1 {
		return path, ""
	}
	// need to put back the / as it is part of the actual sharepath
	return parts[0], fmt.Sprintf("/%s", parts[1])
}
