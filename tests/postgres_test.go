package tests

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/lupppig/dbackup/internal/backup"
	"github.com/lupppig/dbackup/internal/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

func TestPostgresAdapter_Name(t *testing.T) {
	pa := &db.PostgresAdapter{}
	assert.Equal(t, "postgres", pa.Name())
}

func TestPostgresAdapter_BuildConnection(t *testing.T) {
	pa := &db.PostgresAdapter{}
	ctx := context.Background()

	tests := []struct {
		name    string
		params  db.ConnectionParams
		want    string
		wantErr bool
	}{
		{
			name: "With DBUri",
			params: db.ConnectionParams{
				DBUri: "postgres://user:pass@host:5432/dbname",
			},
			want:    "postgres://user:pass@host:5432/dbname",
			wantErr: false,
		},
		{
			name: "With Individual Flags",
			params: db.ConnectionParams{
				Host:     "localhost",
				User:     "testuser",
				Password: "testpassword",
				DBName:   "testdb",
				Port:     5432,
			},
			want:    "postgres://testuser:testpassword@localhost:5432/testdb?sslmode=disable",
			wantErr: false,
		},
		{
			name: "Missing Required Fields",
			params: db.ConnectionParams{
				Host: "localhost",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := pa.BuildConnection(ctx, tt.params)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestPostgresIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	ctx := context.Background()

	dbName := "testdb"
	dbUser := "postgres"
	dbPassword := "password"

	postgresContainer, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image: "postgres:17-alpine",
			Env: map[string]string{
				"POSTGRES_DB":               dbName,
				"POSTGRES_USER":             dbUser,
				"POSTGRES_PASSWORD":         dbPassword,
				"POSTGRES_HOST_AUTH_METHOD": "trust",
			},
			ExposedPorts: []string{"5432/tcp"},
			Cmd:          []string{"postgres", "-c", "max_wal_senders=10", "-c", "max_replication_slots=10", "-c", "wal_level=replica"},
			WaitingFor: wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(30 * time.Second),
		},
		Started: true,
	})
	require.NoError(t, err)
	defer postgresContainer.Terminate(ctx)

	connHost, err := postgresContainer.Host(ctx)
	require.NoError(t, err)

	connPort, err := postgresContainer.MappedPort(ctx, "5432")
	require.NoError(t, err)

	pa := &db.PostgresAdapter{}
	connParams := db.ConnectionParams{
		Host:     connHost,
		Port:     connPort.Int(),
		User:     dbUser,
		Password: dbPassword,
		DBName:   dbName,
	}

	t.Run("TestConnection", func(t *testing.T) {
		err := pa.TestConnection(ctx, connParams)
		assert.NoError(t, err)
	})

	t.Run("RunLogicalBackup", func(t *testing.T) {
		tempDir, err := os.MkdirTemp("", "dbackup-logical-*")
		require.NoError(t, err)
		defer os.RemoveAll(tempDir)

		opts := backup.BackupOptions{
			DBType:     "postgres",
			OutputDir:  tempDir,
			FileName:   "test_logical.sql",
			Compress:   false,
			BackupType: "logical",
		}

		mgr, err := backup.NewBackupManager(opts)
		require.NoError(t, err)

		err = mgr.Run(ctx, pa, connParams)
		assert.NoError(t, err)

		content, err := os.ReadFile(filepath.Join(tempDir, opts.FileName))
		require.NoError(t, err)
		assert.Contains(t, string(content), "PostgreSQL database dump")
	})

	t.Run("RunPhysicalBackupAndManifest", func(t *testing.T) {
		tempDir, err := os.MkdirTemp("", "dbackup-physical-*")
		require.NoError(t, err)
		defer os.RemoveAll(tempDir)

		stateDir := filepath.Join(tempDir, "state")
		err = os.MkdirAll(stateDir, 0755)
		require.NoError(t, err)

		opts := backup.BackupOptions{
			DBType:     "postgres",
			OutputDir:  tempDir,
			FileName:   "test_base.tar",
			Compress:   false,
			BackupType: "full",
		}

		mgr, err := backup.NewBackupManager(opts)
		require.NoError(t, err)

		// Create a specific connection param with StateDir
		physicalConn := connParams
		physicalConn.StateDir = stateDir

		err = mgr.Run(ctx, pa, physicalConn)
		assert.NoError(t, err)

		// 1. Verify TAR file was created
		backupFile := filepath.Join(tempDir, opts.FileName)
		_, err = os.Stat(backupFile)
		assert.NoError(t, err)

		// 2. Verify backup_manifest was extracted to StateDir
		manifestFile := filepath.Join(stateDir, "backup_manifest")
		manifestContent, err := os.ReadFile(manifestFile)
		assert.NoError(t, err)
		assert.Contains(t, string(manifestContent), "PostgreSQL-Backup-Manifest-Version")

		// 3. Verify Incremental Backup (Auto-mode)
		incrementalOpts := opts
		incrementalOpts.FileName = "test_inc.tar"
		incrementalOpts.BackupType = "auto"
		mgrInc, err := backup.NewBackupManager(incrementalOpts)
		require.NoError(t, err)

		err = mgrInc.Run(ctx, pa, physicalConn)
		assert.NoError(t, err)

		incFile := filepath.Join(tempDir, "test_inc.tar")
		_, err = os.Stat(incFile)
		assert.NoError(t, err)
	})
}
