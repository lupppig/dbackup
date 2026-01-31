package tests

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/lupppig/dbackup/internal/backup"
	"github.com/lupppig/dbackup/internal/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

func TestMysqlAdapter_Name(t *testing.T) {
	ma := &db.MysqlAdapter{}
	assert.Equal(t, "mysql", ma.Name())
}

func TestMysqlAdapter_BuildConnection(t *testing.T) {
	ma := &db.MysqlAdapter{}
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
				DBUri: "mysql://user:pass@tcp(host:3306)/dbname",
			},
			want:    "mysql://user:pass@tcp(host:3306)/dbname",
			wantErr: false,
		},
		{
			name: "With Individual Flags",
			params: db.ConnectionParams{
				Host:     "localhost",
				User:     "testuser",
				Password: "testpassword",
				DBName:   "testdb",
				Port:     3306,
			},
			want:    "testuser:testpassword@tcp(localhost:3306)/testdb",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ma.BuildConnection(ctx, tt.params)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestMysqlAdapter_BuildConnection_TLS(t *testing.T) {
	ma := &db.MysqlAdapter{}
	ctx := context.Background()

	// Create dummy cert files for testing
	tempDir, err := os.MkdirTemp("", "tls-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	caFile := filepath.Join(tempDir, "ca.pem")
	certFile := filepath.Join(tempDir, "cert.pem")
	keyFile := filepath.Join(tempDir, "key.pem")

	// Helper to write dummy file
	writeFile := func(path, content string) {
		err := os.WriteFile(path, []byte(content), 0644)
		require.NoError(t, err)
	}

	dummyCert := `-----BEGIN CERTIFICATE-----
MIIC/zCCAeegAwIBAgIUYnyQN54ZnLKsbqhfYNDJ4a2Hbp8wDQYJKoZIhvcNAQEL
BQAwDzENMAsGA1UEAwwEdGVzdDAeFw0yNjAxMjgyMjI2MTlaFw0yNjAxMjkyMjI2
MTlaMA8xDTALBgNVBAMMBHRlc3QwggEiMA0GCSqGSIb3DQEBAQUAA4IBDwAwggEK
AoIBAQC1oyQrTN3KuWAtodDnrwpOvim43pcgmTEY0T9eC/zEwuP5Cbn5ZBd/j3wD
RQ2p0ww1Wt4a5dwre0FvkU/prqToVOCSLBmOa6ZwHADH4EnpJSksBbHHD6pX6tYs
AhPR+43WSq1C55mPwum4HGJsIW/yCm1QRtOaC9/EnowL3AA6KApbphxvTBlD/2zY
dkWq+Le+paS9WSTy/lOaE7WLmOSOSp4ujlED7YX/O/hX9XS+AS5SnOS6D0UIO4L7
NIAiMMzNSRSMw7/ChTr+5HqiJgoRahGiQkK3xamOFkD/f7XVsXNSSIUuWInnGPBB
ZLRSD7fuv2Cjvk9z/Gfey7jQDkTLAgMBAAGjUzBRMB0GA1UdDgQWBBRMC5HdHUYa
xqxaQo37F04XQPqCGjAfBgNVHSMEGDAWgBRMC5HdHUYaxqxaQo37F04XQPqCGjAP
BgNVHRMBAf8EBTADAQH/MA0GCSqGSIb3DQEBCwUAA4IBAQBC2AhTrpMBOQBQbYBv
UfIfvTrc9lK15noP4axdGHcterLT6J2/N/1OIVijctgzOpfW9DL21MxwkMx6aP0X
ev9DcC1YosatS7Iz30NNxp81KfKp6mio5nJ1slw3hauFLccH8TPRFg81ASwHthVq
/2Y8o6CcoCgjtLpD344P5rVcpvlavfzLuPVcJKCixd2Mq4C5rt80e++Od/yQ/Mt7
FqZx1VMt3ihNq0yDyn+IyyWmEfy3bFJy0kiDCTL5hl3luQv0rIrTeoEQ2x9aOP30
7oux80dQ/9Ne8dEaDBnYLD4qyRNTqznMPckmOObTE3ZNweqZgYAbaoVfjH9lU3Hb
wMl9
-----END CERTIFICATE-----`
	writeFile(caFile, dummyCert)

	_ = certFile // avoid unused
	_ = keyFile  // avoid unused

	tests := []struct {
		name    string
		params  db.ConnectionParams
		wantDSN string
		wantErr bool
	}{
		{
			name: "Basic TLS",
			params: db.ConnectionParams{
				Host:   "localhost",
				User:   "user",
				DBName: "db",
				TLS:    db.TLSConfig{Enabled: true},
			},
			wantDSN: "user:@tcp(localhost:3306)/db?tls=true",
		},
		{
			name: "TLS with CA Cert",
			params: db.ConnectionParams{
				Host:   "localhost",
				User:   "user",
				DBName: "db",
				TLS: db.TLSConfig{
					Enabled: true,
					CACert:  caFile,
				},
			},
			wantDSN: "user:@tcp(localhost:3306)/db?tls=custom_true_false",
		},
		{
			name: "TLS with Missing CA Cert",
			params: db.ConnectionParams{
				Host:   "localhost",
				User:   "user",
				DBName: "db",
				TLS: db.TLSConfig{
					Enabled: true,
					CACert:  "non_existent.pem",
				},
			},
			wantErr: true,
		},
		{
			name: "TLS Skip Verify",
			params: db.ConnectionParams{
				Host:   "localhost",
				User:   "user",
				DBName: "db",
				TLS: db.TLSConfig{
					Enabled: true,
					Mode:    "skip-verify",
				},
			},
			wantDSN: "user:@tcp(localhost:3306)/db?tls=custom_false_false",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ma.BuildConnection(ctx, tt.params)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.wantDSN, got)
			}
		})
	}
}

func importStrings(s string) string { return s[1:] }

func TestMysqlIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	ctx := context.Background()

	dbName := "testdb"
	dbUser := "user"
	dbPassword := "password"

	mysqlContainer, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image: "mariadb:latest",
			Env: map[string]string{
				"MARIADB_DATABASE":                  dbName,
				"MARIADB_USER":                      dbUser,
				"MARIADB_PASSWORD":                  dbPassword,
				"MARIADB_ROOT_PASSWORD":             dbPassword,
				"MARIADB_ALLOW_EMPTY_ROOT_PASSWORD": "yes",
			},
			Cmd:          []string{"mariadbd", "--log-bin", "--binlog-format=ROW", "--server-id=1"},
			ExposedPorts: []string{"3306/tcp"},
			WaitingFor: wait.ForLog("mariadbd: ready for connections").
				WithStartupTimeout(60 * time.Second),
		},
		Started: true,
	})
	require.NoError(t, err)
	defer mysqlContainer.Terminate(ctx)

	connHost, err := mysqlContainer.Host(ctx)
	require.NoError(t, err)

	connPort, err := mysqlContainer.MappedPort(ctx, "3306")
	require.NoError(t, err)

	ma := &db.MysqlAdapter{}
	connParams := db.ConnectionParams{
		Host:     connHost,
		Port:     connPort.Int(),
		User:     dbUser,
		Password: dbPassword,
		DBName:   dbName,
	}

	// Create wrappers for mysqldump and mysqlbinlog
	tDir, err := os.MkdirTemp("", "dbackup-mysql-integration-*")
	require.NoError(t, err)
	defer os.RemoveAll(tDir)
	containerName, err := mysqlContainer.Name(ctx)
	require.NoError(t, err)
	containerName = importStrings(containerName)

	// Wait for the server to actually be ready to handle queries
	time.Sleep(2 * time.Second)

	// MariaDB root might be reachable without pass if MARIADB_ALLOW_EMPTY_ROOT_PASSWORD=yes
	dsn := fmt.Sprintf("root@tcp(%s:%d)/%s", connHost, connPort.Int(), dbName)
	if dbPassword != "" {
		dsn = fmt.Sprintf("root:%s@tcp(%s:%d)/%s", dbPassword, connHost, connPort.Int(), dbName)
	}

	dbInstance, err := sql.Open("mysql", dsn)
	require.NoError(t, err)
	defer dbInstance.Close()

	// Ensure we can actually reach it
	require.Eventually(t, func() bool {
		return dbInstance.PingContext(ctx) == nil
	}, 10*time.Second, 500*time.Millisecond)

	_, err = dbInstance.ExecContext(ctx, fmt.Sprintf("GRANT ALL PRIVILEGES ON *.* TO '%s'@'%%'", dbUser))
	require.NoError(t, err)
	_, err = dbInstance.ExecContext(ctx, "FLUSH PRIVILEGES")
	require.NoError(t, err)

	t.Run("TestConnection", func(t *testing.T) {
		err := ma.TestConnection(ctx, connParams, &db.LocalRunner{})
		assert.NoError(t, err)
	})

	t.Run("RunFullBackup", func(t *testing.T) {
		tTempDir, err := os.MkdirTemp("", "dbackup-mysql-test-*")
		require.NoError(t, err)
		defer os.RemoveAll(tTempDir)

		opts := backup.BackupOptions{
			DBType:     "mysql",
			StorageURI: "local://" + tTempDir,
			FileName:   "test_backup.sql",
			Compress:   false,
		}

		mgr, err := backup.NewBackupManager(opts)
		require.NoError(t, err)

		err = mgr.Run(ctx, ma, connParams)
		assert.NoError(t, err)

		backupFile := filepath.Join(tTempDir, opts.FileName)
		_, err = os.Stat(backupFile)
		assert.NoError(t, err)

		f, err := os.Open(backupFile)
		require.NoError(t, err)
		defer f.Close()

		content, err := io.ReadAll(f)
		require.NoError(t, err)
		assert.True(t, strings.Contains(string(content), "MySQL dump") || strings.Contains(string(content), "MariaDB dump"))
	})

}
