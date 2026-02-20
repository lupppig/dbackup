package storage

import (
	"bytes"
	"context"
	"io"
	"testing"

	"github.com/lupppig/dbackup/internal/manifest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDedupeStorage_SaveAndOpen(t *testing.T) {
	ctx := context.Background()
	local := NewLocalStorage(t.TempDir())
	dedupe := NewDedupeStorage(local)

	data := []byte("hello world, this is a test payload for deduplication testing. It needs to be long enough to test chunking properly, but Gear hash chunks are min 32KB. We will just test that it writes the file and chunks.")

	buf := bytes.NewReader(data)
	loc, err := dedupe.Save(ctx, "testbackup", buf)
	require.NoError(t, err)
	assert.Contains(t, loc, "testbackup")

	// Verify chunks are recorded
	chunks := dedupe.LastChunks()
	assert.NotEmpty(t, chunks)

	// Create a dummy manifest to mock how BackupManager works
	man := &manifest.Manifest{
		ID:     "test-save",
		Chunks: chunks,
	}

	manBytes, _ := man.Serialize()
	err = dedupe.PutMetadata(ctx, "testbackup.manifest", manBytes)
	require.NoError(t, err)

	// Now try to open it
	rc, err := dedupe.Open(ctx, "testbackup")
	require.NoError(t, err)
	defer rc.Close()

	readData, err := io.ReadAll(rc)
	require.NoError(t, err)
	assert.Equal(t, data, readData)
}

func TestDedupeStorage_DeduplicationRatio(t *testing.T) {
	ctx := context.Background()
	local := NewLocalStorage(t.TempDir())
	dedupe := NewDedupeStorage(local)

	// Create a large, somewhat random but repetitive dataset to test chunks
	pattern := []byte("repeating pattern for deduplication testing with some random data interspersed ")
	data := make([]byte, 0, len(pattern)*5000) // ~380KB
	for i := 0; i < 5000; i++ {
		data = append(data, pattern...)
	}

	// Save first time
	_, err := dedupe.Save(ctx, "backup1", bytes.NewReader(data))
	require.NoError(t, err)

	files1, _ := local.ListMetadata(ctx, "chunks/")
	chunkCount1 := len(files1)

	// Save exact same data again
	_, err = dedupe.Save(ctx, "backup2", bytes.NewReader(data))
	require.NoError(t, err)

	files2, _ := local.ListMetadata(ctx, "chunks/")
	chunkCount2 := len(files2)

	// Second save should not increase the number of chunks
	assert.Equal(t, chunkCount1, chunkCount2, "Chunks should be fully deduplicated")
	assert.Greater(t, chunkCount1, 0, "Should have created at least one chunk")
}
