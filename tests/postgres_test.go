package tests

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	database "github.com/lupppig/dbackup/internal/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

func TestPostgresAdapter_Name(t *testing.T) {
	pa := &database.PostgresAdapter{}
	assert.Equal(t, "postgres", pa.Name())
}

func TestPostgresAdapter_BuildConnection(t *testing.T) {
	pa := &database.PostgresAdapter{}
	ctx := context.Background()

	tests := []struct {
		name    string
		params  database.ConnectionParams
		want    string
		wantErr bool
	}{
		{
			name: "With DBUri",
			params: database.ConnectionParams{
				DBUri: "postgres://user:pass@host:5432/dbname",
			},
			want:    "postgres://user:pass@host:5432/dbname",
			wantErr: false,
		},
		{
			name: "With Individual Flags",
			params: database.ConnectionParams{
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
			params: database.ConnectionParams{
				Host: "localhost",
			},
			wantErr: true,
		},
		{
			name: "With TLS Enabled (Default Mode)",
			params: database.ConnectionParams{
				Host:     "localhost",
				User:     "testuser",
				Password: "testpassword",
				DBName:   "testdb",
				TLS: database.TLSConfig{
					Enabled: true,
				},
			},
			want:    "postgres://testuser:testpassword@localhost:5432/testdb?sslmode=require",
			wantErr: false,
		},
		{
			name: "With TLS Enabled (Custom Mode)",
			params: database.ConnectionParams{
				Host:     "localhost",
				User:     "testuser",
				Password: "testpassword",
				DBName:   "testdb",
				TLS: database.TLSConfig{
					Enabled: true,
					Mode:    "verify-full",
				},
			},
			want:    "postgres://testuser:testpassword@localhost:5432/testdb?sslmode=verify-full",
			wantErr: false,
		},
		{
			name: "With Root CA Certificate",
			params: database.ConnectionParams{
				Host:     "localhost",
				User:     "testuser",
				Password: "testpassword",
				DBName:   "testdb",
				TLS: database.TLSConfig{
					Enabled: true,
					Mode:    "verify-ca",
					CACert:  "/path/to/ca.pem",
				},
			},
			want:    "postgres://testuser:testpassword@localhost:5432/testdb?sslmode=verify-ca&sslrootcert=%2Fpath%2Fto%2Fca.pem",
			wantErr: false,
		},
		{
			name: "With mTLS (Client Cert and Key)",
			params: database.ConnectionParams{
				Host:     "localhost",
				User:     "testuser",
				Password: "testpassword",
				DBName:   "testdb",
				TLS: database.TLSConfig{
					Enabled:    true,
					Mode:       "verify-full",
					ClientCert: "/path/to/client.crt",
					ClientKey:  "/path/to/client.key",
				},
			},
			want:    "postgres://testuser:testpassword@localhost:5432/testdb?sslcert=%2Fpath%2Fto%2Fclient.crt&sslkey=%2Fpath%2Fto%2Fclient.key&sslmode=verify-full",
			wantErr: false,
		},
		{
			name: "mTLS Error (Missing Client Key)",
			params: database.ConnectionParams{
				Host:     "localhost",
				User:     "testuser",
				Password: "testpassword",
				DBName:   "testdb",
				TLS: database.TLSConfig{
					Enabled:    true,
					ClientCert: "/path/to/client.crt",
				},
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
	dbUser := "user"
	dbPassword := "password"

	postgresContainer, err := postgres.Run(ctx,
		"postgres:16-alpine",
		postgres.WithDatabase(dbName),
		postgres.WithUsername(dbUser),
		postgres.WithPassword(dbPassword),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(5*time.Second)),
	)
	require.NoError(t, err)
	defer func() {
		if err := postgresContainer.Terminate(ctx); err != nil {
			t.Fatalf("failed to terminate container: %s", err)
		}
	}()

	connHost, err := postgresContainer.Host(ctx)
	require.NoError(t, err)

	connPort, err := postgresContainer.MappedPort(ctx, "5432")
	require.NoError(t, err)

	pa := &database.PostgresAdapter{}

	connParams := database.ConnectionParams{
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

	t.Run("RunBackup", func(t *testing.T) {
		tempDir, err := os.MkdirTemp("", "dbackup-test-*")
		require.NoError(t, err)
		defer os.RemoveAll(tempDir)

		connStr, err := pa.BuildConnection(ctx, connParams)
		require.NoError(t, err)

		opts := database.BackUpOptions{
			OutputDir: tempDir,
			FileName:  "test_backup.sql",
			Compress:  false,
		}

		err = pa.RunBackup(ctx, connStr, opts)
		assert.NoError(t, err)

		backupFile := filepath.Join(tempDir, opts.FileName)
		_, err = os.Stat(backupFile)
		assert.NoError(t, err)

		// Optionally check content
		f, err := os.Open(backupFile)
		require.NoError(t, err)
		defer f.Close()

		content, err := io.ReadAll(f)
		require.NoError(t, err)
		assert.Contains(t, string(content), "PostgreSQL database dump")
	})
}
