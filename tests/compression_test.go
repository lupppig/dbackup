package tests

import (
	"archive/tar"
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
	fileName := "backup.sql"

	t.Run("DefaultTar", func(t *testing.T) {
		fullFileName := fileName + ".tar"
		f, err := os.Create(filepath.Join(tempDir, fullFileName))
		require.NoError(t, err)

		mw := &mockLocationWriter{Writer: f, location: filepath.Join(tempDir, fullFileName)}
		c, err := compress.New(mw, compress.Tar)
		require.NoError(t, err)
		c.SetTarBufferName("backup.sql")

		_, err = c.Write(testData)
		require.NoError(t, err)

		loc := c.Location()
		assert.Equal(t, filepath.Join(tempDir, fullFileName), loc)

		err = c.Close()
		assert.NoError(t, err)

		tf, err := os.Open(loc)
		require.NoError(t, err)
		defer tf.Close()

		tr := tar.NewReader(tf)
		hdr, err := tr.Next()
		require.NoError(t, err)
		assert.Equal(t, "backup.sql", hdr.Name)
	})

	t.Run("ZstdMode", func(t *testing.T) {
		fullFileName := "archive.sql.zstd"
		f, err := os.Create(filepath.Join(tempDir, fullFileName))
		require.NoError(t, err)

		mw := &mockLocationWriter{Writer: f, location: filepath.Join(tempDir, fullFileName)}
		c, err := compress.New(mw, compress.Zstd)
		require.NoError(t, err)

		_, err = c.Write(testData)
		require.NoError(t, err)

		loc := c.Location()
		assert.Equal(t, filepath.Join(tempDir, fullFileName), loc)

		assert.NoError(t, err)

		content, err := os.ReadFile(loc)
		require.NoError(t, err)
		assert.NotEqual(t, testData, content)
	})
}
