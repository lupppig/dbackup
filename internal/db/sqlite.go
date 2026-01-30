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

func (sq *SqliteAdapter) TestConnection(ctx context.Context, connParams ConnectionParams, runner Runner) error {
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

func (sq *SqliteAdapter) BuildConnection(ctx context.Context, connParams ConnectionParams) (string, error) {
	if connParams.DBUri == "" {
		return "", fmt.Errorf("sqlite DB URI is empty")
	}
	return connParams.DBUri, nil
}

func (sq *SqliteAdapter) RunBackup(ctx context.Context, conn ConnectionParams, runner Runner, w io.Writer) error {
	if sq.Logger != nil {
		sq.Logger.Info("Starting SQLite backup...", "path", conn.DBUri)
	}

	return sq.runFullBackup(ctx, conn, runner, w)
}

func (sq *SqliteAdapter) runFullBackup(ctx context.Context, conn ConnectionParams, runner Runner, w io.Writer) error {
	if _, ok := runner.(*LocalRunner); !ok && runner != nil {
		return runner.Run(ctx, "cat", []string{conn.DBUri}, w)
	}

	srcFile, err := os.Open(conn.DBUri)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	_, err = io.Copy(w, srcFile)
	return err
}

func (sq *SqliteAdapter) RunRestore(ctx context.Context, conn ConnectionParams, runner Runner, r io.Reader) error {
	sq.Logger.Info("restoring sqlite database...")
	return sq.runFullRestore(ctx, conn, r)
}

func (sq *SqliteAdapter) runFullRestore(ctx context.Context, conn ConnectionParams, r io.Reader) error {
	dstFile, err := os.Create(conn.DBUri)
	if err != nil {
		return err
	}
	defer dstFile.Close()
	_, err = io.Copy(dstFile, r)
	return err
}
