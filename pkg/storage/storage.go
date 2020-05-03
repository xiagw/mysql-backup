package storage

import (
	"net/url"

	"github.com/databacker/mysql-backup/pkg/storage/credentials"
)

type Storage interface {
	Push(creds credentials.Creds, u url.URL, target, source string) (int64, error)
	Pull(creds credentials.Creds, u url.URL, target string) (int64, error)
}
