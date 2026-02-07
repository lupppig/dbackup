package storage

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
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

func (s *LocalStorage) Open(ctx context.Context, name string) (io.ReadCloser, error) {
	path := filepath.Join(s.baseDir, name)
	return os.Open(path)
}

func (s *LocalStorage) Delete(ctx context.Context, name string) error {
	path := filepath.Join(s.baseDir, name)
	return os.Remove(path)
}

func (s *LocalStorage) Location() string {
	return s.baseDir
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

func (s *LocalStorage) ListMetadata(ctx context.Context, prefix string) ([]string, error) {
	searchDir := s.baseDir
	basePrefix := prefix

	if strings.Contains(prefix, "/") {
		if strings.HasSuffix(prefix, "/") {
			searchDir = filepath.Join(s.baseDir, prefix)
			basePrefix = ""
		} else {
			searchDir = filepath.Join(s.baseDir, filepath.Dir(prefix))
			basePrefix = filepath.Base(prefix)
		}
	}

	entries, err := os.ReadDir(searchDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var files []string
	for _, entry := range entries {
		if !entry.IsDir() && (basePrefix == "" || strings.HasPrefix(entry.Name(), basePrefix)) {
			relDir := ""
			if strings.Contains(prefix, "/") {
				if strings.HasSuffix(prefix, "/") {
					relDir = prefix
				} else {
					relDir = filepath.Dir(prefix) + "/"
				}
			}
			files = append(files, relDir+entry.Name())
		}
	}
	return files, nil
}
