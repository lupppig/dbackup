package storage

import (
	"context"
	"io"
)

type Storage interface {
	Save(ctx context.Context, name string, r io.Reader) (string, error)
	Location() string
}
