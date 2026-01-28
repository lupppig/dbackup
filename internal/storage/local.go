package storage

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

type LocalStorage struct {
	baseDir string
}

func NewLocalStorage(baseDir string) *LocalStorage {
	if baseDir == "" {
		baseDir = "./"
	}
	return &LocalStorage{baseDir: baseDir}
}

func (s *LocalStorage) Save(ctx context.Context, name string, r io.Reader) (string, error) {
	if err := os.MkdirAll(s.baseDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create directory: %w", err)
	}

	path := filepath.Join(s.baseDir, name)
	f, err := os.Create(path)
	if err != nil {
		return "", fmt.Errorf("failed to create file: %w", err)
	}
	defer f.Close()

	if _, err := io.Copy(f, r); err != nil {
		return "", fmt.Errorf("failed to write data: %w", err)
	}

	return path, nil
}

func (s *LocalStorage) Location() string {
	return s.baseDir
}
