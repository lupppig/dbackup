package db

import (
	"context"
	"database/sql"
	"io"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lupppig/dbackup/internal/logger"
)

type mockRunner struct {
	lastCmd  string
	lastArgs []string
}

func (m *mockRunner) Run(ctx context.Context, name string, args []string, stdout io.Writer) error {
	m.lastCmd = name
	m.lastArgs = args
	return nil
}

func (m *mockRunner) RunWithIO(ctx context.Context, name string, args []string, stdin io.Reader, stdout io.Writer) error {
	m.lastCmd = name
	m.lastArgs = args
	return nil
}

func TestPostgresPhysicalBackup(t *testing.T) {
	pa := &PostgresAdapter{}
	pa.SetLogger(logger.New(logger.Config{NoColor: true}))

	runner := &mockRunner{}
	conn := ConnectionParams{
		Host:       "localhost",
		User:       "postgres",
		DBName:     "testdb",
		IsPhysical: true,
	}

	err := pa.RunBackup(context.Background(), conn, runner, io.Discard)
	if err != nil {
		t.Fatalf("RunBackup failed: %v", err)
	}

	if runner.lastCmd != "pg_basebackup" {
		t.Errorf("expected pg_basebackup, got %s", runner.lastCmd)
	}

	foundTar := false
	for _, arg := range runner.lastArgs {
		if arg == "--format=tar" {
			foundTar = true
		}
	}
	if !foundTar {
		t.Error("expected --format=tar in pg_basebackup args")
	}
}

func TestMysqlPhysicalRestoreLifecycle(t *testing.T) {
	ma := &MysqlAdapter{}
	ma.SetLogger(logger.New(logger.Config{NoColor: true}))

	runner := &mockRunner{}
	conn := ConnectionParams{
		Host:       "localhost",
		User:       "root",
		DBName:     "testdb",
		IsPhysical: true,
	}

	// Mocking Run to just capture commands
	err := ma.RunRestore(context.Background(), conn, runner, strings.NewReader("fake xbstream"))
	if err != nil {
		t.Fatalf("RunRestore failed: %v", err)
	}

	// In our implementation, RunRestore calls RunWithIO for xbstream, then Run for prepare and copy-back.
	// Our mockRunner.lastCmd will be the last one executed (copy-back).
	if runner.lastCmd != "xtrabackup" {
		t.Errorf("expected xtrabackup, got %s", runner.lastCmd)
	}

	foundCopyBack := false
	for _, arg := range runner.lastArgs {
		if arg == "--copy-back" {
			foundCopyBack = true
		}
	}
	if !foundCopyBack {
		t.Error("expected --copy-back in xtrabackup args")
	}
}

func TestSqliteOnlineBackup(t *testing.T) {
	// Create a dummy source database
	srcPath := filepath.Join(t.TempDir(), "src.sqlite")
	db, err := sql.Open("sqlite3", srcPath)
	if err != nil {
		t.Fatalf("failed to open src db: %v", err)
	}
	_, err = db.Exec("CREATE TABLE test (id INTEGER PRIMARY KEY, val TEXT); INSERT INTO test (val) VALUES ('hello');")
	if err != nil {
		t.Fatalf("failed to setup src db: %v", err)
	}
	db.Close()

	sq := &SqliteAdapter{}
	sq.SetLogger(logger.New(logger.Config{NoColor: true}))

	runner := &LocalRunner{} // Use real local runner for this test
	conn := ConnectionParams{
		DBName: srcPath,
	}

	// We'll capture the output in a buffer
	var buf strings.Builder
	err = sq.RunBackup(context.Background(), conn, runner, &writerWrapper{&buf})
	if err != nil {
		t.Fatalf("RunBackup failed: %v", err)
	}

	if buf.Len() == 0 {
		t.Error("expected non-empty output from backup")
	}
}

type writerWrapper struct {
	w io.Writer
}

func (ww *writerWrapper) Write(p []byte) (n int, err error) {
	return ww.w.Write(p)
}
