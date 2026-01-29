package db

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"os"

	"github.com/lupppig/dbackup/internal/logger"
	_ "github.com/mattn/go-sqlite3"
)

type SqliteAdapter struct {
	Logger *logger.Logger
}

func (sq *SqliteAdapter) Name() string {
	return "sqlite"
}

func (sq *SqliteAdapter) SetLogger(l *logger.Logger) {
	sq.Logger = l
}

func (sq *SqliteAdapter) TestConnection(ctx context.Context, connParams ConnectionParams) error {
	sq.Logger.Info("connecting to sqlite Database...")
	db, err := sql.Open("sqlite3", connParams.DBUri)
	if err != nil {
		return fmt.Errorf("failed to open SQLite DB: %w", err)
	}
	defer db.Close()

	if err := db.PingContext(ctx); err != nil {
		return fmt.Errorf("failed to ping SQLite DB: %w", err)
	}
	sq.Logger.Info("Database connection Successful...")
	return nil
}

func (sq *SqliteAdapter) BuildConnection(_ context.Context, connParams ConnectionParams) (string, error) {
	if connParams.DBUri == "" {
		return "", fmt.Errorf("sqlite DB URI is empty")
	}
	return connParams.DBUri, nil
}

func (sq *SqliteAdapter) RunBackup(ctx context.Context, conn ConnectionParams, w io.Writer) error {
	if conn.BackupType == "" || conn.BackupType == "auto" {
		conn.BackupType = "full"
	}

	if sq.Logger != nil {
		sq.Logger.Info("Starting SQLite backup...", "path", conn.DBUri, "type", conn.BackupType)
	}

	srcFile, err := os.Open(conn.DBUri)
	if err != nil {
		return fmt.Errorf("failed to open source DB: %w", err)
	}
	defer srcFile.Close()

	written, err := io.Copy(w, srcFile)
	if err != nil {
		return fmt.Errorf("failed to stream DB file: %w", err)
	}

	if sq.Logger != nil {
		sq.Logger.Debug("SQLite backup complete", "bytes", written)
	}

	return nil
}

func (sq *SqliteAdapter) RunRestore(ctx context.Context, conn ConnectionParams, r io.Reader) error {
	sq.Logger.Info("restoring sqlite database...")

	dstFile, err := os.Create(conn.DBUri)
	if err != nil {
		return fmt.Errorf("failed to create destination DB: %w", err)
	}
	defer dstFile.Close()

	if _, err := io.Copy(dstFile, r); err != nil {
		return fmt.Errorf("failed to write restored DB: %w", err)
	}

	return nil
}
