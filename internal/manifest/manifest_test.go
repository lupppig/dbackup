package manifest

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestManifest_SerializeDeserialize(t *testing.T) {
	now := time.Now().Truncate(time.Millisecond) // JSON marshal truncates precision

	m := &Manifest{
		ID:          "123-abc",
		Engine:      "postgres",
		DBName:      "testdb",
		Compression: "lz4",
		Encryption:  "none",
		Version:     "1.0",
		CreatedAt:   now,
		FileName:    "test.lz4",
		Checksum:    "deadbeef",
		Size:        1024,
		Chunks:      []string{"chunk1", "chunk2"},
	}

	data, err := m.Serialize()
	assert.NoError(t, err)
	assert.NotEmpty(t, data)

	m2, err := Deserialize(data)
	assert.NoError(t, err)

	assert.Equal(t, m.ID, m2.ID)
	assert.Equal(t, m.Engine, m2.Engine)
	assert.Equal(t, m.DBName, m2.DBName)
	assert.Equal(t, m.Compression, m2.Compression)
	assert.Equal(t, m.Encryption, m2.Encryption)
	assert.Equal(t, m.Version, m2.Version)
	assert.Equal(t, m.FileName, m2.FileName)
	assert.Equal(t, m.Checksum, m2.Checksum)
	assert.Equal(t, m.Size, m2.Size)
	assert.Equal(t, m.Chunks, m2.Chunks)

	// Compare times safely due to JSON float differences
	assert.True(t, m.CreatedAt.Equal(m2.CreatedAt), "times should match")
}

func TestManifest_Deserialize_Invalid(t *testing.T) {
	_, err := Deserialize([]byte(`{invalid json`))
	assert.Error(t, err)
}

func TestNewManifest(t *testing.T) {
	m := New("test-id", "mysql", "gzip", "aes-256-gcm")

	assert.Equal(t, "test-id", m.ID)
	assert.Equal(t, "mysql", m.Engine)
	assert.Equal(t, "gzip", m.Compression)
	assert.Equal(t, "aes-256-gcm", m.Encryption)
	assert.WithinDuration(t, time.Now(), m.CreatedAt, 1*time.Second)
}
