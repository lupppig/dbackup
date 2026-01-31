package tests

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/lupppig/dbackup/internal/backup"
	"github.com/lupppig/dbackup/internal/db"
	"github.com/lupppig/dbackup/internal/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEncryptedBackupAndRestore(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "dbackup-encrypt-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	passphrase := "test-secret-key"
	rawData := []byte("CREATE TABLE users (id int); INSERT INTO users VALUES (1);")

	opts := backup.BackupOptions{
		StorageURI:           "local://" + tempDir,
		FileName:             "encrypted.sql",
		Encrypt:              true,
		EncryptionPassphrase: passphrase,
		ConfirmRestore:       true,
		Logger:               logger.New(logger.Config{}),
	}

	mgr, err := backup.NewBackupManager(opts)
	require.NoError(t, err)

	// Run backup
	// A dummy Runner is used to write rawData
	err = mgr.Run(context.Background(), &DummyBackupAdapter{Data: rawData}, db.ConnectionParams{DBType: "mock"})
	assert.NoError(t, err)

	// Check that the file is actually encrypted
	encryptedFile := filepath.Join(tempDir, "encrypted.sql")
	content, err := os.ReadFile(encryptedFile)
	require.NoError(t, err)
	assert.NotContains(t, string(content), "CREATE TABLE", "File should be encrypted")
	assert.Contains(t, string(content), "DBKP", "File should have security magic")

	// Now Restore
	rmgr, err := backup.NewRestoreManager(opts)
	require.NoError(t, err)

	restoredMock := &MockAdapter{}
	err = rmgr.Run(context.Background(), restoredMock, db.ConnectionParams{DBType: "mock"})
	assert.NoError(t, err)

	assert.Equal(t, rawData, restoredMock.RestoredData, "Restored data should match raw data")
}

type DummyBackupAdapter struct {
	db.DBAdapter
	Data []byte
}

func (d *DummyBackupAdapter) Name() string { return "dummy" }
func (d *DummyBackupAdapter) RunBackup(ctx context.Context, conn db.ConnectionParams, runner db.Runner, w io.Writer) error {
	_, err := w.Write(d.Data)
	return err
}
func (d *DummyBackupAdapter) SetLogger(l *logger.Logger) {}
