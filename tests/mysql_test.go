package tests

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/lupppig/dbackup/internal/backup"
	"github.com/lupppig/dbackup/internal/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/mysql"
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

	mysqlContainer, err := mysql.Run(ctx,
		"mysql:8.0-debian",
		mysql.WithDatabase(dbName),
		mysql.WithUsername(dbUser),
		mysql.WithPassword(dbPassword),
		testcontainers.WithWaitStrategy(
			wait.ForLog("port: 3306  MySQL Community Server").
				WithStartupTimeout(60*time.Second)),
	)
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

	dumpWrapper := filepath.Join(tDir, "mysqldump_wrapper.sh")
	dumpContent := fmt.Sprintf("#!/bin/sh\ndocker exec %s mysqldump \"$@\"\n", containerName)
	err = os.WriteFile(dumpWrapper, []byte(dumpContent), 0755)
	require.NoError(t, err)
	ma.DumpCommand = dumpWrapper

	binlogWrapper := filepath.Join(tDir, "mysqlbinlog_wrapper.sh")
	binlogContent := fmt.Sprintf("#!/bin/sh\ndocker exec %s mysqlbinlog --base64-output=decode-rows -v \"$@\"\n", containerName)
	err = os.WriteFile(binlogWrapper, []byte(binlogContent), 0755)
	require.NoError(t, err)
	ma.BinlogCommand = binlogWrapper

	// Grant necessary privileges for incremental backups
	dbInstance, err := sql.Open("mysql", fmt.Sprintf("root:%s@tcp(%s:%d)/%s", dbPassword, connHost, connPort.Int(), dbName))
	require.NoError(t, err)
	_, err = dbInstance.ExecContext(ctx, fmt.Sprintf("GRANT ALL PRIVILEGES ON *.* TO '%s'@'%%'", dbUser))
	require.NoError(t, err)
	_, err = dbInstance.ExecContext(ctx, "FLUSH PRIVILEGES")
	require.NoError(t, err)
	_ = dbInstance.Close()

	t.Run("TestConnection", func(t *testing.T) {
		err := ma.TestConnection(ctx, connParams)
		assert.NoError(t, err)
	})

	t.Run("RunFullBackup", func(t *testing.T) {
		tTempDir, err := os.MkdirTemp("", "dbackup-mysql-test-*")
		require.NoError(t, err)
		defer os.RemoveAll(tTempDir)

		opts := backup.BackupOptions{
			DBType:    "mysql",
			OutputDir: tTempDir,
			FileName:  "test_backup.sql",
			Compress:  false,
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
		assert.Contains(t, string(content), "MySQL dump")
	})

	t.Run("RunIncrementalBackup", func(t *testing.T) {
		tTempDir, err := os.MkdirTemp("", "dbackup-mysql-inc-test-*")
		require.NoError(t, err)
		defer os.RemoveAll(tTempDir)

		// We need to pass the StateDir
		connParamsWithState := connParams
		connParamsWithState.StateDir = tTempDir
		connParamsWithState.BackupType = "full" // First run full to establish state

		opts := backup.BackupOptions{
			DBType:    "mysql",
			OutputDir: tTempDir,
			FileName:  "full_backup.sql",
			Compress:  false,
		}

		mgr, err := backup.NewBackupManager(opts)
		require.NoError(t, err)

		// 1. Run full backup to record position
		err = mgr.Run(ctx, ma, connParamsWithState)
		assert.NoError(t, err)

		// Check if state file exists
		stateFile := filepath.Join(tTempDir, "mysql_state.json")
		_, err = os.Stat(stateFile)
		assert.NoError(t, err)

		// 2. Insert some data
		dbInstance, err := sql.Open("mysql", fmt.Sprintf("%s:%s@tcp(%s:%d)/%s", dbUser, dbPassword, connHost, connPort.Int(), dbName))
		require.NoError(t, err)
		_, err = dbInstance.ExecContext(ctx, "CREATE TABLE test_inc (id INT PRIMARY KEY, val VARCHAR(255))")
		require.NoError(t, err)
		_, err = dbInstance.ExecContext(ctx, "INSERT INTO test_inc VALUES (1, 'incremental data')")
		require.NoError(t, err)
		_ = dbInstance.Close()

		// 3. Run incremental backup
		connParamsWithState.BackupType = "incremental"
		opts.FileName = "inc_backup.sql"
		mgr, err = backup.NewBackupManager(opts)
		require.NoError(t, err)

		err = mgr.Run(ctx, ma, connParamsWithState)
		assert.NoError(t, err)

		incFile := filepath.Join(tTempDir, "inc_backup.sql")
		f, err := os.Open(incFile)
		require.NoError(t, err)
		content, _ := io.ReadAll(f)
		f.Close()
		assert.Contains(t, string(content), "incremental data")
	})
}
