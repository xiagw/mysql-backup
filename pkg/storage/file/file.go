package file

import (
	"io"
	"net/url"
	"os"
	"path/filepath"

	"github.com/databacker/mysql-backup/pkg/storage/credentials"
)

type File struct{}

func New() *File {
	return &File{}
}

func (f *File) Pull(creds credentials.Creds, u url.URL, target string) (int64, error) {
	return copyFile(u.Path, target)
}

func (f *File) Push(creds credentials.Creds, u url.URL, target, source string) (int64, error) {
	return copyFile(source, filepath.Join(u.Path, target))
}

// copyFile copy a file from to as efficiently as possible
func copyFile(from, to string) (int64, error) {
	src, err := os.Open(from)
	if err != nil {
		return 0, err
	}
	defer src.Close()

	dst, err := os.Create(to)
	if err != nil {
		return 0, err
	}
	defer dst.Close()
	n, err := io.Copy(dst, src)
	return n, err
}
