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

func (s *S3Storage) PutMetadata(ctx context.Context, name string, data []byte) error {
	return nil
}

func (s *S3Storage) GetMetadata(ctx context.Context, name string) ([]byte, error) {
	return nil, nil
}

func (s *S3Storage) Open(ctx context.Context, name string) (io.ReadCloser, error) {
	return nil, nil
}
