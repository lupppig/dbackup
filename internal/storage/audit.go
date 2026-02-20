package storage

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"time"
)

type AuditStorage struct {
	inner Storage
}

type AuditEntry struct {
	Timestamp time.Time `json:"timestamp"`
	Operation string    `json:"operation"`
	Path      string    `json:"path"`
	Status    string    `json:"status"`
	Extra     string    `json:"extra,omitempty"`
	PrevHash  string    `json:"prev_hash"`
	Hash      string    `json:"hash"`
}

func NewAuditStorage(inner Storage) *AuditStorage {
	return &AuditStorage{inner: inner}
}

func (s *AuditStorage) log(ctx context.Context, op, path, status, extra string) {
	// 1. Read the previous audit log to get the last hash
	var prevHash string
	data, err := s.inner.GetMetadata(ctx, "audit.jsonl")
	if err == nil && len(data) > 0 {
		// Get last line
		lines := splitLines(data)
		if len(lines) > 0 {
			var lastEntry AuditEntry
			if err := json.Unmarshal([]byte(lines[len(lines)-1]), &lastEntry); err == nil {
				prevHash = lastEntry.Hash
			}
		}
	}

	entry := AuditEntry{
		Timestamp: time.Now(),
		Operation: op,
		Path:      path,
		Status:    status,
		Extra:     extra,
		PrevHash:  prevHash,
	}

	// 2. Calculate current hash
	h := sha256.New()
	h.Write([]byte(entry.Timestamp.String()))
	h.Write([]byte(entry.Operation))
	h.Write([]byte(entry.Path))
	h.Write([]byte(entry.Status))
	h.Write([]byte(entry.Extra))
	h.Write([]byte(entry.PrevHash))
	entry.Hash = hex.EncodeToString(h.Sum(nil))

	// 3. Append to audit log
	entryBytes, _ := json.Marshal(entry)
	newLog := append(data, entryBytes...)
	newLog = append(newLog, '\n')
	_ = s.inner.PutMetadata(ctx, "audit.jsonl", newLog)
}

func splitLines(data []byte) []string {
	var lines []string
	start := 0
	for i, b := range data {
		if b == '\n' {
			if i > start {
				lines = append(lines, string(data[start:i]))
			}
			start = i + 1
		}
	}
	return lines
}

func (s *AuditStorage) Save(ctx context.Context, name string, r io.Reader) (string, error) {
	loc, err := s.inner.Save(ctx, name, r)
	status := "success"
	if err != nil {
		status = "error: " + err.Error()
	}
	s.log(ctx, "SAVE", name, status, "")
	return loc, err
}

func (s *AuditStorage) Open(ctx context.Context, name string) (io.ReadCloser, error) {
	rc, err := s.inner.Open(ctx, name)
	status := "success"
	if err != nil {
		status = "error: " + err.Error()
	}
	s.log(ctx, "OPEN", name, status, "")
	return rc, err
}

func (s *AuditStorage) Exists(ctx context.Context, name string) (bool, error) {
	return s.inner.Exists(ctx, name)
}

func (s *AuditStorage) Delete(ctx context.Context, name string) error {
	err := s.inner.Delete(ctx, name)
	status := "success"
	if err != nil {
		status = "error: " + err.Error()
	}
	s.log(ctx, "DELETE", name, status, "")
	return err
}

func (s *AuditStorage) Location() string {
	return s.inner.Location()
}

func (s *AuditStorage) Close() error {
	return s.inner.Close()
}

func (s *AuditStorage) PutMetadata(ctx context.Context, name string, data []byte) error {
	err := s.inner.PutMetadata(ctx, name, data)
	if name == "audit.jsonl" {
		return err // avoid infinite loop
	}
	status := "success"
	if err != nil {
		status = "error: " + err.Error()
	}
	s.log(ctx, "PUT_METADATA", name, status, "")
	return err
}

func (s *AuditStorage) GetMetadata(ctx context.Context, name string) ([]byte, error) {
	return s.inner.GetMetadata(ctx, name)
}

func (s *AuditStorage) ListMetadata(ctx context.Context, prefix string) ([]string, error) {
	return s.inner.ListMetadata(ctx, prefix)
}
