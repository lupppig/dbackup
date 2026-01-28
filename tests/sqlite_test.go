package tests

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	database "github.com/lupppig/dbackup/internal/db"
	"github.com/lupppig/dbackup/internal/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSqliteAdapter_Name(t *testing.T) {
	sa := &database.SqliteAdapter{}
	assert.Equal(t, "sqlite", sa.Name())
}

func TestSqliteAdapter_BuildConnection(t *testing.T) {
	sa := &database.SqliteAdapter{}
	ctx := context.Background()

	uri := "test.db"
	got, err := sa.BuildConnection(ctx, database.ConnectionParams{DBUri: uri})
	assert.NoError(t, err)
	assert.Equal(t, uri, got)

	_, err = sa.BuildConnection(ctx, database.ConnectionParams{DBUri: ""})
	assert.Error(t, err)
}

func TestSqliteIntegration(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "dbackup-sqlite-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	dbPath := filepath.Join(tempDir, "source.db")

	// Create and populate source DB
	db, err := sql.Open("sqlite3", dbPath)
	require.NoError(t, err)
	_, err = db.Exec("CREATE TABLE test (id INTEGER PRIMARY KEY, name TEXT)")
	require.NoError(t, err)
	_, err = db.Exec("INSERT INTO test (name) VALUES ('testuser')")
	require.NoError(t, err)
	db.Close()

	l := logger.New(logger.Config{NoColor: true})
	sa := &database.SqliteAdapter{Logger: l}
	ctx := context.Background()
	connParams := database.ConnectionParams{DBUri: dbPath}

	t.Run("TestConnection", func(t *testing.T) {
		err := sa.TestConnection(ctx, connParams)
		assert.NoError(t, err)
	})

	t.Run("RunBackup", func(t *testing.T) {
		backupDir := filepath.Join(tempDir, "backups")
		opts := database.BackUpOptions{
			OutputDir: backupDir,
			FileName:  "test_backup",
		}

		err := sa.RunBackup(ctx, dbPath, opts)
		assert.NoError(t, err)

		// Verify backup file exists (it appends timestamp according to current implementation)
		files, err := os.ReadDir(backupDir)
		require.NoError(t, err)
		assert.Len(t, files, 1)

		backupFile := filepath.Join(backupDir, files[0].Name())
		assert.Contains(t, backupFile, "test_backup")

		// Verify content by opening it as a DB
		copyDB, err := sql.Open("sqlite3", backupFile)
		require.NoError(t, err)
		defer copyDB.Close()

		var name string
		err = copyDB.QueryRow("SELECT name FROM test WHERE id = 1").Scan(&name)
		assert.NoError(t, err)
		assert.Equal(t, "testuser", name)
	})
}
