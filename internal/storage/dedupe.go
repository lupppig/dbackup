package storage

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"
	"path/filepath"
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

	const stripeSize = 10 // Every 10 chunks, we generate a parity chunk
	var stripe [][]byte

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
		exists, err := s.inner.Exists(ctx, chunkPath)
		if err == nil && exists {
			// Exists, skip
		} else {
			// Assume it doesn't exist, save it
			_, err = s.inner.Save(ctx, chunkPath, bytes.NewReader(data))
			if err != nil {
				return "", fmt.Errorf("failed to save chunk %s: %w", hashStr, err)
			}
		}

		// Keep track of data for parity
		stripe = append(stripe, data)
		if len(stripe) == stripeSize {
			if err := s.saveParity(ctx, stripe); err != nil {
				// Don't fail the whole backup for parity failure, but log it if we had a logger here
			}
			stripe = nil
		}
	}

	// Save final incomplete stripe parity
	if len(stripe) > 0 {
		_ = s.saveParity(ctx, stripe)
	}

	return s.inner.Location() + "/" + name, nil
}

func (s *DedupeStorage) saveParity(ctx context.Context, stripe [][]byte) error {
	if len(stripe) == 0 {
		return nil
	}

	maxLen := 0
	for _, b := range stripe {
		if len(b) > maxLen {
			maxLen = len(b)
		}
	}

	// Prepend lengths as a header (4 bytes per chunk)
	header := make([]byte, len(stripe)*4)
	for i, b := range stripe {
		binary.LittleEndian.PutUint32(header[i*4:], uint32(len(b)))
	}

	parity := make([]byte, maxLen)
	for _, b := range stripe {
		for i, v := range b {
			parity[i] ^= v
		}
	}

	h := sha256.New()
	for _, b := range stripe {
		chash := sha256.Sum256(b)
		h.Write([]byte(hex.EncodeToString(chash[:])))
	}
	stripeHash := hex.EncodeToString(h.Sum(nil))

	fullParity := append(header, parity...)
	_, err := s.inner.Save(ctx, "parity/"+stripeHash, bytes.NewReader(fullParity))
	return err
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
		chunkPath := "chunks/" + hash
		exists, _ := s.inner.Exists(ctx, chunkPath)
		if exists {
			r, err := s.inner.Open(ctx, chunkPath)
			if err == nil {
				readers[i] = r
				closers = append(closers, r)
				continue
			}
		}

		// Chunk is missing, try recovery via parity
		recovered, err := s.tryRecoverChunk(ctx, m.Chunks, i)
		if err != nil {
			for _, c := range closers {
				c.Close()
			}
			return nil, fmt.Errorf("failed to open/recover chunk %s: %w", hash, err)
		}
		readers[i] = io.NopCloser(bytes.NewReader(recovered))
	}

	return &multiReadCloser{
		Reader:  io.MultiReader(readers...),
		closers: closers,
	}, nil
}

func (s *DedupeStorage) tryRecoverChunk(ctx context.Context, allChunks []string, missingIndex int) ([]byte, error) {
	const stripeSize = 10
	stripeIdx := (missingIndex / stripeSize) * stripeSize
	stripeEnd := stripeIdx + stripeSize
	if stripeEnd > len(allChunks) {
		stripeEnd = len(allChunks)
	}

	stripeHashes := allChunks[stripeIdx:stripeEnd]
	h := sha256.New()
	for _, hash := range stripeHashes {
		h.Write([]byte(hash))
	}
	stripeHash := hex.EncodeToString(h.Sum(nil))

	fullParity, err := s.inner.GetMetadata(ctx, "parity/"+stripeHash)
	if err != nil {
		return nil, fmt.Errorf("parity chunk not found: %w", err)
	}

	headerLen := len(stripeHashes) * 4
	if len(fullParity) < headerLen {
		return nil, fmt.Errorf("malformed parity chunk")
	}

	header := fullParity[:headerLen]
	parityData := fullParity[headerLen:]

	missingLen := int(binary.LittleEndian.Uint32(header[(missingIndex-stripeIdx)*4:]))
	recovered := make([]byte, missingLen)

	temp := make([]byte, len(parityData))
	copy(temp, parityData)

	for i, hash := range stripeHashes {
		if stripeIdx+i == missingIndex {
			continue
		}
		data, err := s.getChunkData(ctx, hash)
		if err != nil {
			return nil, fmt.Errorf("failed to read sibling %s: %w", hash, err)
		}
		for j, v := range data {
			temp[j] ^= v
		}
	}

	recovered = temp[:missingLen]

	recoveredHash := sha256.Sum256(recovered)
	if hex.EncodeToString(recoveredHash[:]) != allChunks[missingIndex] {
		return nil, fmt.Errorf("recovered chunk hash mismatch")
	}

	return recovered, nil
}

func (s *DedupeStorage) getChunkData(ctx context.Context, hash string) ([]byte, error) {
	r, err := s.inner.Open(ctx, "chunks/"+hash)
	if err != nil {
		return nil, err
	}
	defer r.Close()
	return io.ReadAll(r)
}

func (s *DedupeStorage) Delete(ctx context.Context, name string) error {
	// If it's a regular file (not a manifest), just pass it down.
	// In a dedupe layout, the base backup file doesn't actually exist, but we might pass it.
	if !strings.HasSuffix(name, ".manifest") {
		return s.inner.Delete(ctx, name)
	}

	// 1. Read the manifest we are about to delete to get its chunks
	data, err := s.GetMetadata(ctx, name)
	if err != nil {
		return s.inner.Delete(ctx, name) // just delete it if metadata is missing
	}

	man, err := manifest.Deserialize(data)
	if err != nil || man == nil {
		return s.inner.Delete(ctx, name)
	}

	// 2. Identify candidate chunks to delete
	candidates := make(map[string]bool)
	for _, c := range man.Chunks {
		candidates[c] = true
	}

	// 3. Delete the manifest itself
	if err := s.inner.Delete(ctx, name); err != nil {
		return err
	}

	// 4. Read all remaining manifests to find referenced chunks
	files, err := s.ListMetadata(ctx, "")
	if err != nil {
		return nil // gracefully skip GC if list fails
	}

	for _, f := range files {
		if !strings.HasSuffix(f, ".manifest") || f == name || f == "latest.manifest" {
			continue
		}
		fdata, ferr := s.GetMetadata(ctx, f)
		if ferr != nil {
			continue
		}
		fman, ferr := manifest.Deserialize(fdata)
		if ferr != nil || fman == nil {
			continue
		}
		for _, c := range fman.Chunks {
			if candidates[c] {
				delete(candidates, c) // chunk is still in use, remove from deletion candidates
			}
		}
	}

	// 5. Delete orphaned chunks
	for c := range candidates {
		_ = s.inner.Delete(ctx, "chunks/"+c)
	}

	return nil
}

func (s *DedupeStorage) Exists(ctx context.Context, name string) (bool, error) {
	return s.inner.Exists(ctx, name)
}

func (s *DedupeStorage) Verify(ctx context.Context) ([]string, error) {
	// 1. Get all manifests
	files, err := s.inner.ListMetadata(ctx, "")
	if err != nil {
		return nil, err
	}

	referenced := make(map[string]bool)
	for _, f := range files {
		if !strings.HasSuffix(f, ".manifest") || f == "latest.manifest" {
			continue
		}
		data, err := s.inner.GetMetadata(ctx, f)
		if err != nil {
			continue
		}
		m, err := manifest.Deserialize(data)
		if err != nil {
			continue
		}
		for _, c := range m.Chunks {
			referenced[c] = true
		}
	}

	// 2. Check existence of all referenced chunks
	var missing []string
	for c := range referenced {
		exists, err := s.inner.Exists(ctx, "chunks/"+c)
		if err != nil {
			return nil, err
		}
		if !exists {
			missing = append(missing, c)
		}
	}

	return missing, nil
}

func (s *DedupeStorage) GC(ctx context.Context) (int, error) {
	// 1. Get all manifests and collect all referenced chunks
	files, err := s.inner.ListMetadata(ctx, "")
	if err != nil {
		return 0, err
	}

	referenced := make(map[string]bool)
	for _, f := range files {
		if !strings.HasSuffix(f, ".manifest") || f == "latest.manifest" {
			continue
		}
		data, err := s.inner.GetMetadata(ctx, f)
		if err != nil {
			continue
		}
		m, err := manifest.Deserialize(data)
		if err != nil {
			continue
		}
		for _, c := range m.Chunks {
			referenced[c] = true
		}
	}

	// 2. List all actual chunks in storage
	// We need a way to list chunks. ListMetadata(ctx, "chunks/") should work if implemented.
	actualChunks, err := s.inner.ListMetadata(ctx, "chunks/")
	if err != nil {
		return 0, err
	}

	// 3. Delete orphans
	deletedCount := 0
	for _, chunkPath := range actualChunks {
		// chunkPath might be "chunks/hash" or just "hash" depending on implementation
		hash := filepath.Base(chunkPath)
		if !referenced[hash] {
			if err := s.inner.Delete(ctx, chunkPath); err == nil {
				deletedCount++
			}
		}
	}

	return deletedCount, nil
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
		if strings.HasPrefix(f, "chunks/") && !strings.HasPrefix(prefix, "chunks/") {
			continue
		}
		filtered = append(filtered, f)
	}
	return filtered, nil
}

func (s *DedupeStorage) Close() error {
	return s.inner.Close()
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
