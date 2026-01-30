package db

import (
	"context"
	"errors"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type MockErrorRunner struct {
	Err error
}

func (r *MockErrorRunner) Run(ctx context.Context, name string, args []string, w io.Writer) error {
	return r.Err
}

func (r *MockErrorRunner) RunWithIO(ctx context.Context, name string, args []string, stdin io.Reader, stdout io.Writer) error {
	return r.Err
}

func TestRegisterAndGetAdapter(t *testing.T) {
	// Simple mock adapter that doesn't need external setup
	// We'll use SQLite as it's already registered
	got, err := GetAdapter("sqlite")
	require.NoError(t, err)
	assert.Equal(t, "sqlite", got.Name())
}

func TestGetUnsupportedAdapter(t *testing.T) {
	_, err := GetAdapter("non_existent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported database: non_existent")
}

func TestPostgresAdapter_ToolFailure(t *testing.T) {
	pa := &PostgresAdapter{}
	ctx := context.Background()
	conn := ConnectionParams{
		DBUri: "postgres://u:p@h:5432/d",
	}
	runner := &MockErrorRunner{Err: errors.New("pg_dump not found")}

	err := pa.RunBackup(ctx, conn, runner, io.Discard)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "pg_dump failed") // PostgresAdapter wraps the error
}

func TestMysqlAdapter_ToolFailure(t *testing.T) {
	ma := &MysqlAdapter{}
	ctx := context.Background()
	conn := ConnectionParams{
		DBUri: "mysql://u:p@tcp(h:3306)/d",
	}
	runner := &MockErrorRunner{Err: errors.New("mysqldump failed")}

	err := ma.RunBackup(ctx, conn, runner, io.Discard)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "mysqldump execution failed") // MysqlAdapter wraps the error
}
