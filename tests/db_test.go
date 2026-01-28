package tests

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lupppig/dbackup/internal/db"
	"github.com/lupppig/dbackup/internal/logger"
	"github.com/lupppig/dbackup/internal/storage"
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

func (m MockAdapter) TestConnection(ctx context.Context, conn db.ConnectionParams) error {
	return nil
}

func (m MockAdapter) BuildConnection(ctx context.Context, conn db.ConnectionParams) (string, error) {
	return "", nil
}

func (m MockAdapter) RunBackup(ctx context.Context, connStr string, w io.Writer) error {
	return nil
}

func (m *MockAdapter) SetLogger(l *logger.Logger) {
	m.logger = l
}

func TestRegisterAndGetAdapter(t *testing.T) {
	name := "mock_db"
	adapter := &MockAdapter{name: name}

	db.RegisterAdapter(adapter)

	got, err := db.GetAdapter(name)
	require.NoError(t, err)
	assert.Equal(t, name, got.Name())
}

func TestGetUnsupportedAdapter(t *testing.T) {
	_, err := db.GetAdapter("non_existent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported database: non_existent")
}

func TestLocalStorage_Save(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "dbackup-storage-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	s := storage.NewLocalStorage(tempDir)
	content := "test content"
	fileName := "test.txt"

	path, err := s.Save(context.Background(), fileName, strings.NewReader(content))
	require.NoError(t, err)

	require.NoError(t, err)
	assert.Equal(t, filepath.Join(tempDir, fileName), path)
	got, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, content, string(got))
}
