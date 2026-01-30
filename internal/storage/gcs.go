package storage

import (
	"context"
	"io"
)

type GCSStorage struct {
	Bucket string
}

func (s *GCSStorage) Save(ctx context.Context, name string, r io.Reader) (string, error) {
	return "gs://" + s.Bucket + "/" + name, nil
}

func (s *GCSStorage) Location() string {
	return "gs://" + s.Bucket
}

func (s *GCSStorage) PutMetadata(ctx context.Context, name string, data []byte) error {
	return nil
}

func (s *GCSStorage) GetMetadata(ctx context.Context, name string) ([]byte, error) {
	return nil, nil
}

func (s *GCSStorage) Open(ctx context.Context, name string) (io.ReadCloser, error) {
	return nil, nil
}
