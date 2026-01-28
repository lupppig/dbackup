package database

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

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

func (sq *SqliteAdapter) RunBackup(ctx context.Context, connString string, backupOptions BackUpOptions) error {
	sq.Logger.Info("attempting to backup database...")
	backupDir := backupOptions.OutputDir

	if backupDir == "" {
		backupDir = "./backup"
	}

	if err := os.MkdirAll(backupDir, os.ModePerm); err != nil {
		return fmt.Errorf("failed to create backup directory: %w", err)
	}
	var backupPath string
	if backupOptions.FileName == "" {
		backupPath = filepath.Join(backupDir, fmt.Sprintf("backup_%d.db", time.Now().Unix()))
	} else {
		backupPath = filepath.Join(backupDir, fmt.Sprintf("%s_%d.db", backupOptions.FileName, time.Now().Unix()))
	}

	srcFile, err := os.Open(connString)
	if err != nil {
		sq.Logger.Error("failed to open source Database", "error", err)
		return fmt.Errorf("failed to open source DB: %w", err)
	}
	defer srcFile.Close()

	dstFile, err := os.Create(backupPath)
	if err != nil {
		sq.Logger.Error("failed to create backup file", "error", err)
		return fmt.Errorf("failed to create backup file: %w", err)
	}
	defer dstFile.Close()

	if _, err := dstFile.ReadFrom(srcFile); err != nil {
		sq.Logger.Error("failed to copy DB file", "error", err)
		return fmt.Errorf("failed to copy DB file: %w", err)
	}

	// flush to disk...
	if err := dstFile.Sync(); err != nil {
		sq.Logger.Error("failed to flush backup file to disk", "error", err)
		return fmt.Errorf("failed to flush backup file: %w", err)
	}

	sq.Logger.Info("SQLite full backup created", "path", backupPath)
	return nil
}
