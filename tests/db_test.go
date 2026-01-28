package tests

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/lupppig/dbackup/internal/backup"
	database "github.com/lupppig/dbackup/internal/db"
	"github.com/lupppig/dbackup/internal/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type MockAdapter struct {
	name   string
	logger *logger.Logger
}

func (m MockAdapter) Name() string {
	return m.name
}

func (m MockAdapter) TestConnection(ctx context.Context, conn database.ConnectionParams) error {
	return nil
}

func (m MockAdapter) BuildConnection(ctx context.Context, conn database.ConnectionParams) (string, error) {
	return "", nil
}

func (m MockAdapter) RunBackup(ctx context.Context, connStr string, backupOption database.BackUpOptions) error {
	return nil
}

func (m *MockAdapter) SetLogger(l *logger.Logger) {
	m.logger = l
}

func TestRegisterAndGetAdapter(t *testing.T) {
	name := "mock_db"
	adapter := &MockAdapter{name: name}

	database.RegisterAdapter(adapter)

	got, err := database.GetAdapter(name)
	require.NoError(t, err)
	assert.Equal(t, name, got.Name())
}

func TestGetUnsupportedAdapter(t *testing.T) {
	_, err := database.GetAdapter("non_existent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported database: non_existent")
}

func TestNewFileWriter_CreatesDirectory(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "dbackup-dir-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	nestedDir := filepath.Join(tempDir, "nested", "dir", "structure")
	fileName := "test.sql"

	// Ensure the nested dir does NOT exist
	_, err = os.Stat(nestedDir)
	assert.True(t, os.IsNotExist(err))

	writer, err := backup.NewFileWriter(nestedDir, fileName)
	require.NoError(t, err)
	defer writer.Close()

	// Verify the directory was created
	_, err = os.Stat(nestedDir)
	assert.NoError(t, err)

	// Verify the file was created
	_, err = os.Stat(filepath.Join(nestedDir, fileName))
	assert.NoError(t, err)
}
