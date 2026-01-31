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
	Version     string    `json:"version"`
	Checksum    string    `json:"checksum"` // SHA-256 of the stored blob
	Compression string    `json:"compression"`
	Encryption  string    `json:"encryption"`
	CreatedAt   time.Time `json:"created_at"`
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

// CalculateChecksum computes the SHA-256 of a stream
func CalculateChecksum(r io.Reader) (string, error) {
	h := sha256.New()
	if _, err := io.Copy(h, r); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
