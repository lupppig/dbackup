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
func TestDedupeStorage_Verify_GC(t *testing.T) {
	ctx := context.Background()
	local := NewLocalStorage(t.TempDir())
	dedupe := NewDedupeStorage(local)

	data := []byte("some test data for verify and gc")
	_, err := dedupe.Save(ctx, "test", bytes.NewReader(data))
	require.NoError(t, err)

	chunks := dedupe.LastChunks()
	man := &manifest.Manifest{Chunks: chunks}
	mb, _ := man.Serialize()
	err = dedupe.PutMetadata(ctx, "test.manifest", mb)
	require.NoError(t, err)

	// Verify should pass
	missing, err := dedupe.Verify(ctx)
	require.NoError(t, err)
	assert.Empty(t, missing)

	// Add an orphan chunk manually
	_, err = local.Save(ctx, "chunks/orphan", bytes.NewReader([]byte("orphan")))
	require.NoError(t, err)

	// GC should remove it
	deleted, err := dedupe.GC(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, deleted)

	// Verify should still pass
	missing, err = dedupe.Verify(ctx)
	require.NoError(t, err)
	assert.Empty(t, missing)

	// Delete a real chunk
	err = local.Delete(ctx, "chunks/"+chunks[0])
	require.NoError(t, err)

	// Verify should now fail (without recovery involved yet)
	missing, err = dedupe.Verify(ctx)
	require.NoError(t, err)
	assert.Len(t, missing, 1)
	assert.Equal(t, chunks[0], missing[0])
}

func TestDedupeStorage_ParityRecovery(t *testing.T) {
	ctx := context.Background()
	local := NewLocalStorage(t.TempDir())
	dedupe := NewDedupeStorage(local)

	// Gear chunking min is 32KB, average is 64KB. 512KB should produce multiple chunks.
	pattern := []byte("this is a repetitive string to test stripe reconstruction via parity ")
	data := make([]byte, 0, 512*1024)
	for len(data) < 512*1024 {
		data = append(data, pattern...)
	}

	_, err := dedupe.Save(ctx, "test", bytes.NewReader(data))
	require.NoError(t, err)

	chunks := dedupe.LastChunks()
	require.Greater(t, len(chunks), 1, "Should have more than one chunk for stripe test")

	man := &manifest.Manifest{Chunks: chunks}
	mb, _ := man.Serialize()
	err = dedupe.PutMetadata(ctx, "test.manifest", mb)
	require.NoError(t, err)

	// Verify it opens normally
	rc, err := dedupe.Open(ctx, "test")
	require.NoError(t, err)
	d, _ := io.ReadAll(rc)
	assert.Equal(t, data, d)
	rc.Close()

	// Corrupt it: delete the first chunk
	err = local.Delete(ctx, "chunks/"+chunks[0])
	require.NoError(t, err)

	// Verify should still report it missing
	missing, _ := dedupe.Verify(ctx)
	assert.Contains(t, missing, chunks[0])

	// But Open should recover it via parity!
	rc, err = dedupe.Open(ctx, "test")
	require.NoError(t, err, "Should recover via parity")
	d, err = io.ReadAll(rc)
	require.NoError(t, err)
	assert.Equal(t, data, d, "Data should be reconstructed exactly")
	rc.Close()
}
