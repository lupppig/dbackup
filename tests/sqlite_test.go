package tests

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"github.com/lupppig/dbackup/internal/backup"
	"github.com/lupppig/dbackup/internal/db"
	"github.com/lupppig/dbackup/internal/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSqliteAdapter_Name(t *testing.T) {
	sa := &db.SqliteAdapter{}
	assert.Equal(t, "sqlite", sa.Name())
}

func TestSqliteAdapter_BuildConnection(t *testing.T) {
	sa := &db.SqliteAdapter{}
	ctx := context.Background()

	uri := "test.db"
	got, err := sa.BuildConnection(ctx, db.ConnectionParams{DBUri: uri})
	assert.NoError(t, err)
	assert.Equal(t, uri, got)

	_, err = sa.BuildConnection(ctx, db.ConnectionParams{DBUri: ""})
	assert.Error(t, err)
}

func TestSqliteIntegration(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "dbackup-sqlite-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	dbPath := filepath.Join(tempDir, "source.db")

	dbConn, err := sql.Open("sqlite3", dbPath)
	require.NoError(t, err)
	_, err = dbConn.Exec("CREATE TABLE test (id INTEGER PRIMARY KEY, name TEXT)")
	require.NoError(t, err)
	_, err = dbConn.Exec("INSERT INTO test (name) VALUES ('testuser')")
	require.NoError(t, err)
	dbConn.Close()

	l := logger.New(logger.Config{NoColor: true})
	sa := &db.SqliteAdapter{Logger: l}
	ctx := context.Background()
	connParams := db.ConnectionParams{DBUri: dbPath}

	t.Run("TestConnection", func(t *testing.T) {
		err := sa.TestConnection(ctx, connParams)
		assert.NoError(t, err)
	})

	t.Run("RunBackupViaManager", func(t *testing.T) {
		backupDir := filepath.Join(tempDir, "backups")
		opts := backup.BackupOptions{
			OutputDir: backupDir,
			FileName:  "test_backup.db",
			Compress:  false,
		}

		mgr, err := backup.NewBackupManager(opts)
		require.NoError(t, err)

		err = mgr.Run(ctx, sa, dbPath)
		assert.NoError(t, err)

		backupFile := filepath.Join(backupDir, opts.FileName)
		_, err = os.Stat(backupFile)
		assert.NoError(t, err)

		copyDB, err := sql.Open("sqlite3", backupFile)
		require.NoError(t, err)
		defer copyDB.Close()

		var name string
		err = copyDB.QueryRow("SELECT name FROM test WHERE id = 1").Scan(&name)
		assert.NoError(t, err)
		assert.Equal(t, "testuser", name)
	})
}
