package db

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/go-sql-driver/mysql"
	"github.com/lupppig/dbackup/internal/logger"
)

type MysqlAdapter struct {
	logger        *logger.Logger
	DumpCommand   string
	BinlogCommand string
}

type mysqlState struct {
	LastBinlog string `json:"last_binlog"`
	LastPos    int    `json:"last_pos"`
}

func (ma *MysqlAdapter) SetLogger(l *logger.Logger) {
	ma.logger = l
}

func (ma MysqlAdapter) Name() string {
	return "mysql"
}

func (ma *MysqlAdapter) TestConnection(ctx context.Context, conn ConnectionParams) error {
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

func (ma MysqlAdapter) BuildConnection(ctx context.Context, conn ConnectionParams) (string, error) {
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

func (ma MysqlAdapter) ensureTLSConfig(cfg TLSConfig) (string, error) {
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

func (ma MysqlAdapter) RunBackup(ctx context.Context, conn ConnectionParams, w io.Writer) error {
	backupType := conn.BackupType
	if backupType == "auto" || backupType == "" {
		if conn.StateDir != "" {
			statePath := filepath.Join(conn.StateDir, "mysql_state.json")
			if _, err := os.Stat(statePath); err == nil {
				backupType = "incremental"
			} else {
				backupType = "full"
			}
		} else {
			backupType = "full"
		}
	}

	if backupType == "incremental" {
		return ma.runIncrementalBackup(ctx, conn, w)
	}

	if ma.logger != nil {
		ma.logger.Info("Dumping database...", "engine", ma.Name(), "type", "full")
	}

	if conn.Port == 0 {
		conn.Port = 3306
	}

	args := []string{
		fmt.Sprintf("--host=%s", conn.Host),
		fmt.Sprintf("--port=%d", conn.Port),
		fmt.Sprintf("--user=%s", conn.User),
		fmt.Sprintf("--password=%s", conn.Password),
		"--no-tablespaces",
		"--flush-logs",
		"--source-data=2", // Updated from master-data
	}

	if conn.TLS.Enabled {
		if conn.TLS.CACert != "" {
			args = append(args, fmt.Sprintf("--ssl-ca=%s", conn.TLS.CACert))
		}
		if conn.TLS.ClientCert != "" {
			args = append(args, fmt.Sprintf("--ssl-cert=%s", conn.TLS.ClientCert))
		}
		if conn.TLS.ClientKey != "" {
			args = append(args, fmt.Sprintf("--ssl-key=%s", conn.TLS.ClientKey))
		}

		mode := "ON"
		switch conn.TLS.Mode {
		case "require":
			mode = "REQUIRED"
		case "verify-ca":
			mode = "VERIFY_CA"
		case "verify-full":
			mode = "VERIFY_IDENTITY"
		case "disable":
			mode = "DISABLED"
		}
		args = append(args, fmt.Sprintf("--ssl-mode=%s", mode))
	}

	args = append(args, conn.DBName)

	command := ma.DumpCommand
	if command == "" {
		command = "mysqldump"
	}

	cmd := exec.CommandContext(ctx, command, args...)
	cmd.Stdout = w
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("mysqldump failed: %w", err)
	}

	// After a full backup, we should record the current binlog position
	if conn.StateDir != "" {
		if err := ma.saveCurrentPosition(ctx, conn); err != nil {
			if ma.logger != nil {
				ma.logger.Warn("failed to save binlog position", "error", err)
			}
		}
	}

	return nil
}

func (ma MysqlAdapter) runIncrementalBackup(ctx context.Context, conn ConnectionParams, w io.Writer) error {
	if ma.logger != nil {
		ma.logger.Info("Starting incremental backup...", "engine", ma.Name())
	}

	state, err := ma.loadState(conn)
	if err != nil {
		return fmt.Errorf("failed to load state for incremental backup: %w (did you run a full backup first?)", err)
	}

	if state.LastBinlog == "" {
		return fmt.Errorf("no binlog position found in state; perform a full backup first")
	}

	args := []string{
		fmt.Sprintf("--host=%s", conn.Host),
		fmt.Sprintf("--port=%d", conn.Port),
		fmt.Sprintf("--user=%s", conn.User),
		fmt.Sprintf("--password=%s", conn.Password),
		"--read-from-remote-server",
		fmt.Sprintf("--start-position=%d", state.LastPos),
	}

	if conn.TLS.Enabled {
		// Note: mysqlbinlog SSL flags are similar but slightly different in older versions
		// For simplicity, we assume modern mysqlbinlog
		if conn.TLS.CACert != "" {
			args = append(args, fmt.Sprintf("--ssl-ca=%s", conn.TLS.CACert))
		}
		// ... potentially more SSL flags here if needed
	}

	args = append(args, state.LastBinlog)

	command := ma.BinlogCommand
	if command == "" {
		command = "mysqlbinlog"
	}

	cmd := exec.CommandContext(ctx, command, args...)
	cmd.Stdout = w
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("mysqlbinlog failed: %w", err)
	}

	// Update the state with the new current position
	return ma.saveCurrentPosition(ctx, conn)
}

func (ma MysqlAdapter) saveCurrentPosition(ctx context.Context, conn ConnectionParams) error {
	dsn, err := ma.BuildConnection(ctx, conn)
	if err != nil {
		return err
	}

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return err
	}
	defer db.Close()

	var file string
	var pos int
	var binlogDoDB, binlogIgnoreDB, executedGtidSet sql.NullString

	err = db.QueryRowContext(ctx, "SHOW MASTER STATUS").Scan(&file, &pos, &binlogDoDB, &binlogIgnoreDB, &executedGtidSet)
	if err != nil {
		// Try SHOW BINARY LOG STATUS for MySQL 8.4+
		err = db.QueryRowContext(ctx, "SHOW BINARY LOG STATUS").Scan(&file, &pos, &binlogDoDB, &binlogIgnoreDB, &executedGtidSet)
		if err != nil {
			return fmt.Errorf("failed to get binlog status: %w", err)
		}
	}

	state := mysqlState{
		LastBinlog: file,
		LastPos:    pos,
	}

	return ma.saveState(conn, state)
}

func (ma MysqlAdapter) loadState(conn ConnectionParams) (mysqlState, error) {
	if conn.StateDir == "" {
		return mysqlState{}, fmt.Errorf("StateDir is not configured")
	}

	statePath := filepath.Join(conn.StateDir, "mysql_state.json")
	data, err := os.ReadFile(statePath)
	if err != nil {
		return mysqlState{}, err
	}

	var state mysqlState
	if err := json.Unmarshal(data, &state); err != nil {
		return mysqlState{}, err
	}
	return state, nil
}

func (ma MysqlAdapter) saveState(conn ConnectionParams, state mysqlState) error {
	if conn.StateDir == "" {
		return nil // Nowhere to save
	}

	if err := os.MkdirAll(conn.StateDir, 0755); err != nil {
		return err
	}

	statePath := filepath.Join(conn.StateDir, "mysql_state.json")
	data, err := json.Marshal(state)
	if err != nil {
		return err
	}

	return os.WriteFile(statePath, data, 0644)
}

func (ma MysqlAdapter) RunRestore(ctx context.Context, conn ConnectionParams, r io.Reader) error {
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

	cmd := exec.CommandContext(ctx, "mysql", args...)
	cmd.Stdin = r
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("mysql restore failed: %w", err)
	}
	return nil
}
