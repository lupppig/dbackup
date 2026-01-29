package tests

import (
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/lupppig/dbackup/internal/compress"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockLocationWriter struct {
	io.Writer
	location string
}

func (m *mockLocationWriter) Location() string {
	return m.location
}

func (m *mockLocationWriter) Close() error {
	if c, ok := m.Writer.(io.Closer); ok {
		return c.Close()
	}
	return nil
}

func TestCompressionIntegration(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "dbackup-compression-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	testData := []byte("hello compression world")

	t.Run("Lz4Streaming", func(t *testing.T) {
		fullFileName := "backup.sql.lz4"
		path := filepath.Join(tempDir, fullFileName)
		f, err := os.Create(path)
		require.NoError(t, err)

		mw := &mockLocationWriter{Writer: f, location: path}
		c, err := compress.New(mw, compress.Lz4)
		require.NoError(t, err)

		_, err = c.Write(testData)
		require.NoError(t, err)

		err = c.Close()
		assert.NoError(t, err)

		// Verify it's compressed
		content, err := os.ReadFile(path)
		require.NoError(t, err)
		assert.NotEqual(t, testData, content)

		// Verify we can decompress it
		f2, err := os.Open(path)
		require.NoError(t, err)
		defer f2.Close()

		cr, err := compress.NewReader(f2, compress.Lz4)
		// Wait, my NewReader assumes TAR wrapper!
		// I need to update NewReader to support raw streaming too.
		require.NoError(t, err)
		defer cr.Close()

		decompressed, err := io.ReadAll(cr)
		require.NoError(t, err)
		assert.Equal(t, testData, decompressed)
	})

	t.Run("ZstdStreaming", func(t *testing.T) {
		fullFileName := "archive.sql.zstd"
		path := filepath.Join(tempDir, fullFileName)
		f, err := os.Create(path)
		require.NoError(t, err)

		mw := &mockLocationWriter{Writer: f, location: path}
		c, err := compress.New(mw, compress.Zstd)
		require.NoError(t, err)

		_, err = c.Write(testData)
		require.NoError(t, err)

		err = c.Close()
		assert.NoError(t, err)

		content, err := os.ReadFile(path)
		require.NoError(t, err)
		assert.NotEqual(t, testData, content)
	})
}
