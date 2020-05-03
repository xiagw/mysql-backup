package core

import (
	"github.com/databacker/mysql-backup/pkg/compression"
	"github.com/databacker/mysql-backup/pkg/database"
	"github.com/databacker/mysql-backup/pkg/storage/credentials"
)

type DumpOptions struct {
	Targets           []string
	Safechars         bool
	BySchema          bool
	KeepPermissions   bool
	DBNames           []string
	DBConn            database.Connection
	Creds             credentials.Creds
	Compressor        compression.Compressor
	Exclude           []string
	PreBackupScripts  string
	PostBackupScripts string
}
