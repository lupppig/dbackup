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
	if prefix != "" {
		if strings.HasSuffix(prefix, "/") {
			searchDir = filepath.Join(s.baseDir, prefix)
		} else {
			searchDir = filepath.Join(s.baseDir, filepath.Dir(prefix))
		}
	}

	var files []string
	err := filepath.WalkDir(searchDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			if os.IsNotExist(err) && path == searchDir {
				return nil
			}
			return err
		}

		if d.IsDir() {
			// Skip chunks folder unless specifically searching it
			if d.Name() == "chunks" && !strings.Contains(prefix, "chunks") {
				return filepath.SkipDir
			}
			return nil
		}

		rel, err := filepath.Rel(s.baseDir, path)
		if err != nil {
			return nil
		}

		// Normalize to forward slashes for cross-platform consistency
		rel = filepath.ToSlash(rel)

		// Apply prefix filter on base name if prefix is not a directory
		if prefix != "" && !strings.HasSuffix(prefix, "/") {
			if !strings.HasPrefix(rel, prefix) {
				return nil
			}
		}

		files = append(files, rel)
		return nil
	})

	return files, err
}
