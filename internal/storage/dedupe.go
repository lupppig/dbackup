package storage

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"time"

	"github.com/lupppig/dbackup/internal/manifest"
)

type DedupeStorage struct {
	inner Storage
}

func NewDedupeStorage(inner Storage) *DedupeStorage {
	return &DedupeStorage{inner: inner}
}

func (s *DedupeStorage) Save(ctx context.Context, name string, r io.Reader) (string, error) {
	chunker := NewChunker(r)
	var chunkHashes []string

	for {
		data, err := chunker.Next()
		if err != nil {
			if err == io.EOF {
				break
			}
			return "", err
		}

		hash := sha256.Sum256(data)
		hashStr := hex.EncodeToString(hash[:])
		chunkHashes = append(chunkHashes, hashStr)

		chunkPath := "chunks/" + hashStr
		r, err := s.inner.Open(ctx, chunkPath)
		if err == nil {
			// Exists, skip
			r.Close()
		} else {
			// Assume it doesn't exist, save it
			_, err = s.inner.Save(ctx, chunkPath, bytes.NewReader(data))
			if err != nil {
				return "", fmt.Errorf("failed to save chunk %s: %w", hashStr, err)
			}
		}
	}

	baseName := filepath.Base(name)
	engine := strings.Split(baseName, "-")[0]
	dbName := ""
	if strings.Contains(baseName, "-") {
		parts := strings.Split(baseName, "-")
		if len(parts) > 1 {
			dbName = parts[1]
		}
	}

	m := manifest.Manifest{
		ID:        fmt.Sprintf("%x", time.Now().UnixNano()),
		Engine:    engine,
		DBName:    dbName,
		Timestamp: name,
		CreatedAt: time.Now(),
		Chunks:    chunkHashes,
		Version:   "0.1.0",
	}

	manifestData, err := json.Marshal(m)
	if err != nil {
		return "", err
	}

	manifestName := "backups/" + name + ".manifest"
	err = s.inner.PutMetadata(ctx, manifestName, manifestData)
	if err != nil {
		return "", fmt.Errorf("failed to save manifest: %w", err)
	}

	return s.inner.Location() + "/" + manifestName, nil
}

func (s *DedupeStorage) Open(ctx context.Context, name string) (io.ReadCloser, error) {
	manifestName := name
	if !strings.HasSuffix(name, ".manifest") {
		manifestName = "backups/" + name + ".manifest"
	}

	data, err := s.inner.GetMetadata(ctx, manifestName)
	if err != nil {
		return nil, fmt.Errorf("manifest not found: %w", err)
	}

	m, err := manifest.Deserialize(data)
	if err != nil {
		return nil, err
	}

	readers := make([]io.Reader, len(m.Chunks))
	closers := make([]io.Closer, 0, len(m.Chunks))

	for i, hash := range m.Chunks {
		r, err := s.inner.Open(ctx, "chunks/"+hash)
		if err != nil {
			for _, c := range closers {
				c.Close()
			}
			return nil, fmt.Errorf("failed to open chunk %s: %w", hash, err)
		}
		readers[i] = r
		closers = append(closers, r)
	}

	return &multiReadCloser{
		Reader:  io.MultiReader(readers...),
		closers: closers,
	}, nil
}

func (s *DedupeStorage) Location() string {
	return "dedupe://" + s.inner.Location()
}

func (s *DedupeStorage) PutMetadata(ctx context.Context, name string, data []byte) error {
	return s.inner.PutMetadata(ctx, name, data)
}

func (s *DedupeStorage) GetMetadata(ctx context.Context, name string) ([]byte, error) {
	return s.inner.GetMetadata(ctx, name)
}

func (s *DedupeStorage) ListMetadata(ctx context.Context, prefix string) ([]string, error) {
	return s.inner.ListMetadata(ctx, prefix)
}

type multiReadCloser struct {
	io.Reader
	closers []io.Closer
}

func (m *multiReadCloser) Close() error {
	var errs []string
	for _, c := range m.closers {
		if err := c.Close(); err != nil {
			errs = append(errs, err.Error())
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("errors closing chunks: %s", strings.Join(errs, ", "))
	}
	return nil
}
