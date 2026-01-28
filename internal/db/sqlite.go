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

func (sq SqliteAdapter) Name() string {
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

func (sq *SqliteAdapter) RunBackup(ctx context.Context, connString string, w io.Writer) error {
	sq.Logger.Info("attempting to backup database...")

	srcFile, err := os.Open(connString)
	if err != nil {
		sq.Logger.Error("failed to open source Database", "error", err)
		return fmt.Errorf("failed to open source DB: %w", err)
	}
	defer srcFile.Close()

	if _, err := io.Copy(w, srcFile); err != nil {
		sq.Logger.Error("failed to stream DB file", "error", err)
		return fmt.Errorf("failed to stream DB file: %w", err)
	}

	return nil
}
