package db

import (
	"context"
	"database/sql"
	"io"
	"os"

	apperrors "github.com/lupppig/dbackup/internal/errors"
	"github.com/lupppig/dbackup/internal/logger"
	_ "github.com/mattn/go-sqlite3"
)

func init() {
	RegisterAdapter(&SqliteAdapter{})
}

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
	if sq.Logger != nil {
		sq.Logger.Info("connecting to sqlite Database...", "path", connParams.DBName)
	}
	db, err := sql.Open("sqlite3", connParams.DBName)
	if err != nil {
		return apperrors.Wrap(err, apperrors.TypeConfig, "failed to open SQLite DB", "Verify the file path and permissions.")
	}
	defer db.Close()

	if err := db.PingContext(ctx); err != nil {
		return apperrors.Wrap(err, apperrors.TypeResource, "failed to ping SQLite DB", "Ensure the file is a valid SQLite database.")
	}
	if sq.Logger != nil {
		sq.Logger.Info("Database connection Successful...")
	}
	return nil
}

func (sq *SqliteAdapter) BuildConnection(ctx context.Context, connParams ConnectionParams) (string, error) {
	path := connParams.DBName
	if path == "" && connParams.DBUri != "" {
		path = connParams.DBUri
	}

	if path == "" {
		return "", apperrors.New(apperrors.TypeConfig, "sqlite DB path is empty", "Provide a database file path via --db.")
	}
	return path, nil
}

func (sq *SqliteAdapter) RunBackup(ctx context.Context, conn ConnectionParams, runner Runner, w io.Writer) error {
	path, err := sq.BuildConnection(ctx, conn)
	if err != nil {
		return err
	}
	if sq.Logger != nil {
		sq.Logger.Info("Starting SQLite backup...", "path", path)
	}

	return sq.runFullBackup(ctx, path, runner, w)
}

func (sq *SqliteAdapter) runFullBackup(ctx context.Context, path string, runner Runner, w io.Writer) error {
	if _, ok := runner.(*LocalRunner); !ok && runner != nil {
		return runner.Run(ctx, "cat", []string{path}, w)
	}

	srcFile, err := os.Open(path)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	_, err = io.Copy(w, srcFile)
	return err
}

func (sq *SqliteAdapter) RunRestore(ctx context.Context, conn ConnectionParams, runner Runner, r io.Reader) error {
	path, err := sq.BuildConnection(ctx, conn)
	if err != nil {
		return err
	}
	if sq.Logger != nil {
		sq.Logger.Info("restoring sqlite database...", "path", path)
	}
	return sq.runFullRestore(ctx, path, r)
}

func (sq *SqliteAdapter) runFullRestore(ctx context.Context, path string, r io.Reader) error {
	dstFile, err := os.Create(path)
	if err != nil {
		return err
	}
	defer dstFile.Close()
	_, err = io.Copy(dstFile, r)
	return err
}
