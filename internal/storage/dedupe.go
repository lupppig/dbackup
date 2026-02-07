package storage

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"strings"

	"github.com/lupppig/dbackup/internal/manifest"
)

type DedupeStorage struct {
	inner      Storage
	lastChunks []string
}

func NewDedupeStorage(inner Storage) *DedupeStorage {
	return &DedupeStorage{inner: inner}
}

func (s *DedupeStorage) LastChunks() []string {
	return s.lastChunks
}

func (s *DedupeStorage) Save(ctx context.Context, name string, r io.Reader) (string, error) {
	chunker := NewChunker(r)
	s.lastChunks = nil

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
		s.lastChunks = append(s.lastChunks, hashStr)

		chunkPath := "chunks/" + hashStr
		chk, err := s.inner.Open(ctx, chunkPath)
		if err == nil {
			// Exists, skip
			chk.Close()
		} else {
			// Assume it doesn't exist, save it
			_, err = s.inner.Save(ctx, chunkPath, bytes.NewReader(data))
			if err != nil {
				return "", fmt.Errorf("failed to save chunk %s: %w", hashStr, err)
			}
		}
	}

	return s.inner.Location() + "/" + name, nil
}

func (s *DedupeStorage) Open(ctx context.Context, name string) (io.ReadCloser, error) {
	manifestName := name
	data, err := s.inner.GetMetadata(ctx, manifestName)
	if err != nil && !strings.HasSuffix(name, ".manifest") {
		manifestName = name + ".manifest"
		data, err = s.inner.GetMetadata(ctx, manifestName)
	}

	if err != nil {
		return s.inner.Open(ctx, name)
	}

	m, err := manifest.Deserialize(data)
	if err != nil || len(m.Chunks) == 0 {
		// Not a dedupe manifest, try as raw file
		return s.inner.Open(ctx, name)
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

func (s *DedupeStorage) Delete(ctx context.Context, name string) error {
	// Try deleting manifest and chunks?
	// For now just delete the manifest/name.
	return s.inner.Delete(ctx, name)
}

func (s *DedupeStorage) Location() string {
	return s.inner.Location()
}

func (s *DedupeStorage) PutMetadata(ctx context.Context, name string, data []byte) error {
	return s.inner.PutMetadata(ctx, name, data)
}

func (s *DedupeStorage) GetMetadata(ctx context.Context, name string) ([]byte, error) {
	return s.inner.GetMetadata(ctx, name)
}

func (s *DedupeStorage) ListMetadata(ctx context.Context, prefix string) ([]string, error) {
	files, err := s.inner.ListMetadata(ctx, prefix)
	if err != nil {
		return nil, err
	}

	var filtered []string
	for _, f := range files {
		// Skip chunks folder for general listings to avoid performance issues
		if strings.HasPrefix(f, "chunks/") && !strings.HasPrefix(prefix, "chunks/") {
			continue
		}
		filtered = append(filtered, f)
	}
	return filtered, nil
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
