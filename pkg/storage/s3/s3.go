package s3

import (
	"fmt"
	"net/url"
	"os"
	"path"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/databacker/mysql-backup/pkg/storage/credentials"
)

type S3 struct{}

func New() *S3 {
	return &S3{}
}

func (s *S3) Pull(creds credentials.Creds, u url.URL, target string) (int64, error) {
	// TODO: need to find way to include cli opts and cli_s3_cp_opts
	// old was:
	// 		aws ${AWS_CLI_OPTS} s3 cp ${AWS_CLI_S3_CP_OPTS} "${DB_RESTORE_TARGET}" $TMPRESTORE

	bucket, path := u.Hostname(), u.Path
	// The session the S3 Downloader will use
	opts := session.Options{}
	if creds.AWSEndpoint != "" {
		opts.Config = aws.Config{
			Endpoint: aws.String(creds.AWSEndpoint),
		}
	}
	sess, err := session.NewSessionWithOptions(opts)
	if err != nil {
		return 0, fmt.Errorf("failed to create AWS session: %v", err)
	}

	// Create a downloader with the session and default options
	downloader := s3manager.NewDownloader(sess)

	// Create a file to write the S3 Object contents to.
	f, err := os.Create(target)
	if err != nil {
		return 0, fmt.Errorf("failed to create target restore file %q, %v", target, err)
	}
	defer f.Close()

	// Write the contents of S3 Object to the file
	n, err := downloader.Download(f, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(path),
	})
	if err != nil {
		return 0, fmt.Errorf("failed to download file, %v", err)
	}
	return n, nil
}

func (s *S3) Push(creds credentials.Creds, u url.URL, target, source string) (int64, error) {
	// TODO: need to find way to include cli opts and cli_s3_cp_opts
	// old was:
	// 		aws ${AWS_CLI_OPTS} s3 cp ${AWS_CLI_S3_CP_OPTS} "${DB_RESTORE_TARGET}" $TMPRESTORE

	bucket, key := u.Hostname(), u.Path
	// The session the S3 Downloader will use
	opts := session.Options{}
	if creds.AWSEndpoint != "" {
		opts.Config = aws.Config{
			Endpoint: aws.String(creds.AWSEndpoint),
		}
	}
	sess, err := session.NewSessionWithOptions(opts)
	if err != nil {
		return 0, fmt.Errorf("failed to create AWS session: %v", err)
	}

	// Create an uploader with the session and default options
	uploader := s3manager.NewUploader(sess)

	// Create a file to write the S3 Object contents to.
	f, err := os.Open(source)
	if err != nil {
		return 0, fmt.Errorf("failed to read input file %q, %v", source, err)
	}
	defer f.Close()

	// Write the contents of the file to the S3 object
	_, err = uploader.Upload(&s3manager.UploadInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(path.Join(key, target)),
		Body:   f,
	})
	if err != nil {
		return 0, fmt.Errorf("failed to upload file, %v", err)
	}
	return 0, nil
}
