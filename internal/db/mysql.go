package db

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"database/sql"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/go-sql-driver/mysql"
	"github.com/lupppig/dbackup/internal/logger"
)

type MysqlAdapter struct {
	logger *logger.Logger
}

/*
MYSQL BACKUP ARCHITECTURE & TRADEOFFS:

1. LOGICAL BACKUP (mysqldump)
   - Strategy: SQL-level exports.
   - Streaming: YES (stdout).
   - Incremental: YES (via Binlogs).
   - Speed: Moderate for small/medium DBs; slow for 100GB+.
   - Restore: High compatibility; slow (executes SQL).

2. PHYSICAL BACKUP (xtrabackup / mariadb-backup)
   - Strategy: Block-level copy of data files.
   - Streaming: YES (via --stream=xbstream to stdout).
   - Incremental: YES (LSN-based).
   - Speed: VERY FAST for large datasets (multi-threaded block copy).
   - Restore: FAST (data file copy); Requires local tool for 'prepare' phase.
   - Limitation: Requires local filesystem access to MySQL datadir (host/container).

RECOMMENDED STRATEGY:
- Use 'auto' or 'physical' for large datasets where downtime/restore speed is critical.
- Use 'logical' for portability or when block-level access is unavailable.
*/

func (ma *MysqlAdapter) SetLogger(l *logger.Logger) {
	ma.logger = l
}

func (ma *MysqlAdapter) Name() string {
	return "mysql"
}

func (ma *MysqlAdapter) TestConnection(ctx context.Context, conn ConnectionParams, runner Runner) error {
	if ma.logger != nil {
		ma.logger.Info("Testing database connection...", "host", conn.Host, "db", conn.DBName)
	}
	dsn, err := ma.BuildConnection(ctx, conn)
	if err != nil {
		return err
	}

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return fmt.Errorf("failed to open connection: %w", err)
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		return fmt.Errorf("failed to ping database: %w", err)
	}
	return nil
}

func (ma *MysqlAdapter) BuildConnection(ctx context.Context, conn ConnectionParams) (string, error) {
	if conn.DBUri != "" {
		return conn.DBUri, nil
	}

	if conn.Host == "" || conn.User == "" || conn.DBName == "" {
		return "", fmt.Errorf("missing required MySQL connection fields")
	}

	if conn.Port == 0 {
		conn.Port = 3306
	}

	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s", conn.User, conn.Password, conn.Host, conn.Port, conn.DBName)

	if conn.TLS.Enabled {
		tlsName, err := ma.ensureTLSConfig(conn.TLS)
		if err != nil {
			return "", err
		}
		dsn += "?tls=" + tlsName
	}

	return dsn, nil
}

func (ma *MysqlAdapter) ensureTLSConfig(cfg TLSConfig) (string, error) {
	if cfg.CACert == "" && cfg.ClientCert == "" && (cfg.Mode == "" || cfg.Mode == "disable" || cfg.Mode == "require") {
		return "true", nil // Default to basic TLS
	}

	tlsConfig := &tls.Config{
		MinVersion: tls.VersionTLS12,
	}

	if cfg.CACert != "" {
		rootCertPool := x509.NewCertPool()
		pem, err := os.ReadFile(cfg.CACert)
		if err != nil {
			return "", fmt.Errorf("failed to read CA cert: %w", err)
		}
		if ok := rootCertPool.AppendCertsFromPEM(pem); !ok {
			return "", fmt.Errorf("failed to append CA cert")
		}
		tlsConfig.RootCAs = rootCertPool
	}

	if cfg.ClientCert != "" && cfg.ClientKey != "" {
		clientCert := make([]tls.Certificate, 0, 1)
		certs, err := tls.LoadX509KeyPair(cfg.ClientCert, cfg.ClientKey)
		if err != nil {
			return "", fmt.Errorf("failed to load client cert/key: %w", err)
		}
		clientCert = append(clientCert, certs)
		tlsConfig.Certificates = clientCert
	}

	switch cfg.Mode {
	case "skip-verify":
		tlsConfig.InsecureSkipVerify = true
	case "verify-ca", "verify-full":
		tlsConfig.InsecureSkipVerify = false
	}

	configName := fmt.Sprintf("custom_%t_%t", cfg.CACert != "", cfg.ClientCert != "")
	mysql.RegisterTLSConfig(configName, tlsConfig)
	return configName, nil
}

func (ma *MysqlAdapter) RunBackup(ctx context.Context, conn ConnectionParams, runner Runner, w io.Writer) error {
	// 1. Resolve Backup Mode (Logical vs Physical)
	// We default to 'logical' for MySQL unless physical is requested.
	mode := "logical"

	if conn.Port == 0 {
		conn.Port = 3306
	}

	if ma.logger != nil {
		ma.logger.Info("Starting MySQL backup...", "engine", ma.Name(), "mode", mode)
	}

	switch mode {
	case "logical":
		return ma.runLogicalFull(ctx, conn, runner, w)
	case "physical":
		return ma.runPhysicalFull(ctx, conn, runner, w)
	default:
		return fmt.Errorf("unsupported MySQL backup mode: %s", mode)
	}
}

func (ma *MysqlAdapter) runLogicalFull(ctx context.Context, conn ConnectionParams, runner Runner, w io.Writer) error {
	if ma.logger != nil {
		ma.logger.Info("Executing logical full backup (mysqldump)...")
	}

	args := []string{
		fmt.Sprintf("--host=%s", conn.Host),
		fmt.Sprintf("--port=%d", conn.Port),
		fmt.Sprintf("--user=%s", conn.User),
		fmt.Sprintf("--password=%s", conn.Password),
		"--single-transaction",
		"--quick",
		"--skip-lock-tables",
	}

	if conn.TLS.Enabled {
		if conn.TLS.CACert != "" {
			args = append(args, fmt.Sprintf("--ssl-ca=%s", conn.TLS.CACert))
		}
	} else {
		args = append(args, "--ssl=OFF")
	}

	args = append(args, conn.DBName)

	if err := runner.Run(ctx, "mysqldump", args, w); err != nil {
		return fmt.Errorf("mysqldump execution failed: %w", err)
	}

	return nil
}

func (ma *MysqlAdapter) runPhysicalFull(ctx context.Context, conn ConnectionParams, runner Runner, w io.Writer) error {
	// PHYSICAL BACKUP via xtrabackup (Industry Standard)
	// Note: xtrabackup MUST be on the same host as the MySQL data files.
	if ma.logger != nil {
		ma.logger.Info("Executing physical full backup (xtrabackup)...")
	}

	args := []string{
		"--backup",
		"--stream=xbstream",
		fmt.Sprintf("--host=%s", conn.Host),
		fmt.Sprintf("--user=%s", conn.User),
		fmt.Sprintf("--password=%s", conn.Password),
	}

	// XtraBackup streams the entire database instance to stdout in xbstream format.
	if err := runner.Run(ctx, "xtrabackup", args, w); err != nil {
		return fmt.Errorf("xtrabackup physical backup failed: %w", err)
	}

	return nil
}

func (ma *MysqlAdapter) RunRestore(ctx context.Context, conn ConnectionParams, runner Runner, r io.Reader) error {
	if ma.logger != nil {
		ma.logger.Info("Restoring database...", "engine", ma.Name())
	}

	if conn.Port == 0 {
		conn.Port = 3306
	}

	args := []string{
		fmt.Sprintf("--host=%s", conn.Host),
		fmt.Sprintf("--port=%d", conn.Port),
		fmt.Sprintf("--user=%s", conn.User),
		fmt.Sprintf("--password=%s", conn.Password),
		conn.DBName,
	}

	if err := runner.RunWithIO(ctx, "mysql", args, r, nil); err != nil {
		return fmt.Errorf("mysql restore failed: %w", err)
	}
	return nil
}
