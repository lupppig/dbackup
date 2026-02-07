package manifest

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"time"
)

type Manifest struct {
	ID          string    `json:"id"`
	ParentID    string    `json:"parent_id,omitempty"`
	Engine      string    `json:"engine"`
	DBName      string    `json:"dbname,omitempty"`
	Timestamp   string    `json:"timestamp,omitempty"`
	Version     string    `json:"version"`
	Checksum    string    `json:"checksum,omitempty"` // SHA-256 of the stored blob
	Compression string    `json:"compression,omitempty"`
	Encryption  string    `json:"encryption,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	Size        int64     `json:"size,omitempty"`   // Total size of the backup blob
	Chunks      []string  `json:"chunks,omitempty"` // SHA-256 hashes for dedupe
}

func New(id, engine, compression, encryption string) *Manifest {
	return &Manifest{
		ID:          id,
		Engine:      engine,
		Compression: compression,
		Encryption:  encryption,
		CreatedAt:   time.Now(),
	}
}

func (m *Manifest) Serialize() ([]byte, error) {
	return json.MarshalIndent(m, "", "  ")
}

func Deserialize(data []byte) (*Manifest, error) {
	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	return &m, nil
}

func CalculateChecksum(r io.Reader) (string, error) {
	h := sha256.New()
	if _, err := io.Copy(h, r); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
