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
	path := filepath.Join(s.baseDir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return "", fmt.Errorf("failed to create directory: %w", err)
	}

	tmpPath := path + ".tmp"
	f, err := os.Create(tmpPath)
	if err != nil {
		return "", fmt.Errorf("failed to create temp file: %w", err)
	}
	defer os.Remove(tmpPath) // Cleanup if we fail

	if _, err := io.Copy(f, r); err != nil {
		f.Close()
		return "", fmt.Errorf("failed to write data: %w", err)
	}
	f.Close()

	if err := os.Rename(tmpPath, path); err != nil {
		return "", fmt.Errorf("failed to finalize file (rename): %w", err)
	}

	return path, nil
}

func (s *LocalStorage) PutMetadata(ctx context.Context, name string, data []byte) error {
	path := filepath.Join(s.baseDir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

func (s *LocalStorage) GetMetadata(ctx context.Context, name string) ([]byte, error) {
	path := filepath.Join(s.baseDir, name)
	return os.ReadFile(path)
}

func (s *LocalStorage) Location() string {
	return s.baseDir
}

func (s *LocalStorage) Open(ctx context.Context, name string) (io.ReadCloser, error) {
	path := filepath.Join(s.baseDir, name)
	return os.Open(path)
}
