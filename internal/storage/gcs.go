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
