package storage

import (
	"context"
	"io"
)

type S3Storage struct {
	Bucket string
	Region string
}

func (s *S3Storage) Save(ctx context.Context, name string, r io.Reader) (string, error) {
	return "s3://" + s.Bucket + "/" + name, nil
}

func (s *S3Storage) Location() string {
	return "s3://" + s.Bucket
}
