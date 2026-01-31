package tests

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/lupppig/dbackup/internal/backup"
	"github.com/lupppig/dbackup/internal/compress"
	"github.com/lupppig/dbackup/internal/db"
	"github.com/lupppig/dbackup/internal/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockAdapter captures the restored data
type MockAdapter struct {
	db.DBAdapter
	RestoredData []byte
}

func (m *MockAdapter) Name() string { return "mock" }
func (m *MockAdapter) RunRestore(ctx context.Context, conn db.ConnectionParams, runner db.Runner, r io.Reader) error {
	data, err := io.ReadAll(r)
	if err != nil {
		return err
	}
	m.RestoredData = data
	return nil
}
func (m *MockAdapter) BuildConnection(ctx context.Context, conn db.ConnectionParams) (string, error) {
	return "mock://", nil
}
func (m *MockAdapter) SetLogger(l *logger.Logger) {}
func (m *MockAdapter) TestConnection(ctx context.Context, conn db.ConnectionParams, runner db.Runner) error {
	return nil
}

func TestAutoDecompression(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "dbackup-auto-decomp-*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	rawData := []byte("SELECT * FROM users; -- raw sql data")

	// Prepare a compressed file manually
	gzFile := filepath.Join(tempDir, "backup.sql.gz")
	f, err := os.Create(gzFile)
	require.NoError(t, err)

	c, err := compress.New(f, compress.Gzip)
	require.NoError(t, err)
	_, err = c.Write(rawData)
	require.NoError(t, err)
	c.Close()
	f.Close()

	// Now try to restore it without specifying the algorithm
	mock := &MockAdapter{}

	opts := backup.BackupOptions{
		StorageURI:     "local://" + tempDir,
		FileName:       "backup.sql.gz", // No Algorithm or Compress flag set
		ConfirmRestore: true,
	}

	rmgr, err := backup.NewRestoreManager(opts)
	require.NoError(t, err)

	err = rmgr.Run(context.Background(), mock, db.ConnectionParams{DBType: "mock"})
	assert.NoError(t, err)

	assert.Equal(t, rawData, mock.RestoredData, "Restored data should match raw data after auto-decompression")
}
