package db

import (
	"archive/tar"
	"context"
	"database/sql"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	_ "github.com/lib/pq"
	"github.com/lupppig/dbackup/internal/logger"
)

type PostgresAdapter struct {
	logger *logger.Logger
}

func (pa *PostgresAdapter) SetLogger(l *logger.Logger) {
	pa.logger = l
}

func (pa *PostgresAdapter) validateVersion(ctx context.Context, conn ConnectionParams) (int, error) {
	dsn, err := pa.BuildConnection(ctx, conn)
	if err != nil {
		return 0, err
	}
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return 0, err
	}
	defer db.Close()

	var version int
	err = db.QueryRowContext(ctx, "SHOW server_version_num").Scan(&version)
	if err != nil {
		return 0, fmt.Errorf("failed to query postgres version: %w", err)
	}
	return version, nil
}

func (pa *PostgresAdapter) Name() string {
	return "postgres"
}

func (pa *PostgresAdapter) TestConnection(ctx context.Context, conn ConnectionParams) error {
	if pa.logger != nil {
		pa.logger.Info("Testing database connection...", "host", conn.Host, "db", conn.DBName)
	}
	dsn, err := pa.BuildConnection(ctx, conn)
	if err != nil {
		return err
	}

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return fmt.Errorf("failed to open connection: %w", err)
	}

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	defer db.Close()

	if err := db.PingContext(ctx); err != nil {
		return fmt.Errorf("failed to ping database: %w", err)
	}
	return nil
}

func (pa *PostgresAdapter) BuildConnection(ctx context.Context, conn ConnectionParams) (string, error) {
	if conn.DBUri != "" {
		return conn.DBUri, nil
	}

	if conn.Host == "" || conn.User == "" || conn.DBName == "" {
		return "", fmt.Errorf("missing required Postgres connection fields")
	}

	if conn.Port == 0 {
		conn.Port = 5432
	}

	u := &url.URL{
		Scheme: "postgres",
		User:   url.UserPassword(conn.User, conn.Password),
		Host:   fmt.Sprintf("%s:%d", conn.Host, conn.Port),
		Path:   conn.DBName,
	}

	q := u.Query()

	if conn.TLS.Enabled {
		if conn.TLS.Mode == "" {
			conn.TLS.Mode = "require"
		}
		q.Set("sslmode", conn.TLS.Mode)

		if conn.TLS.CACert != "" {
			q.Set("sslrootcert", conn.TLS.CACert)
		}
		if conn.TLS.ClientCert != "" && conn.TLS.ClientKey != "" {
			q.Set("sslcert", conn.TLS.ClientCert)
			q.Set("sslkey", conn.TLS.ClientKey)
		} else if conn.TLS.ClientCert != "" || conn.TLS.ClientKey != "" {
			return "", fmt.Errorf("both TLS ClientCert and ClientKey must be provided for mTLS")
		}
	} else {
		q.Set("sslmode", "disable")
	}

	u.RawQuery = q.Encode()
	return u.String(), nil
}

func (pa *PostgresAdapter) RunBackup(ctx context.Context, conn ConnectionParams, w io.Writer) error {
	// Default to physical (default behavior for large datasets)
	if conn.BackupType == "" || conn.BackupType == "auto" {
		manifestPath := ""
		if conn.StateDir != "" {
			manifestPath = filepath.Join(conn.StateDir, "backup_manifest")
		}

		if manifestPath != "" {
			if _, err := os.Stat(manifestPath); err == nil {
				conn.BackupType = "incremental"
				if pa.logger != nil {
					pa.logger.Info("Previous manifest found, using incremental mode", "manifest", manifestPath)
				}
			} else {
				conn.BackupType = "full"
			}
		} else {
			conn.BackupType = "full"
		}
	}

	if conn.BackupType == "logical" {
		return pa.runLogicalBackup(ctx, conn, w)
	}

	// Physical backup logic (pg_basebackup)
	if pa.logger != nil {
		pa.logger.Info("Starting physical backup...", "engine", pa.Name(), "type", conn.BackupType)
	}

	version, err := pa.validateVersion(ctx, conn)
	if err != nil {
		return fmt.Errorf("version validation failed: %w", err)
	}

	if version < 120000 { // Minimum for modern pg_basebackup streaming reliably
		return fmt.Errorf("PostgreSQL version %d is too old for streaming physical backups (minimum 12+)", version)
	}

	isIncremental := conn.BackupType == "incremental"
	manifestPath := ""
	if conn.StateDir != "" {
		manifestPath = filepath.Join(conn.StateDir, "backup_manifest")
	}

	if isIncremental {
		if version < 150000 {
			return fmt.Errorf("physical incremental backups require PostgreSQL 15+ (detected version %d)", version)
		}
		if version < 170000 && pa.logger != nil {
			pa.logger.Warn("Native pg_basebackup --incremental typically requires PG 17+; ensure your server/client setup supports this feature", "version", version)
		}
		if manifestPath == "" {
			return fmt.Errorf("StateDir is required for incremental backups to find backup_manifest")
		}
		if _, err := os.Stat(manifestPath); err != nil {
			return fmt.Errorf("backup_manifest not found at %s; incremental backup cannot proceed", manifestPath)
		}
	}

	connStr, err := pa.BuildConnection(ctx, conn)
	if err != nil {
		return err
	}

	args := []string{
		"--dbname", connStr,
		"--format=tar",
		"--pgdata", "-",
		"--wal-method=none",
	}

	if isIncremental && manifestPath != "" {
		args = append(args, "--incremental", manifestPath)
	}

	cmd := exec.CommandContext(ctx, "pg_basebackup", args...)

	if conn.StateDir != "" {
		return pa.streamWithManifestExtraction(ctx, cmd, w, conn.StateDir)
	}

	cmd.Stdout = w
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("pg_basebackup failed: %w", err)
	}

	return nil
}

func (pa *PostgresAdapter) runLogicalBackup(ctx context.Context, conn ConnectionParams, w io.Writer) error {
	if pa.logger != nil {
		pa.logger.Info("Dumping database...", "engine", pa.Name(), "type", "full (logical)")
	}

	connStr, err := pa.BuildConnection(ctx, conn)
	if err != nil {
		return err
	}

	cmd := exec.CommandContext(
		ctx,
		"pg_dump",
		"--dbname", connStr,
		"--format=plain",
		"--no-owner",
		"--no-acl",
	)

	cmd.Stdout = w
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("pg_dump failed: %w", err)
	}

	return nil
}

func (pa *PostgresAdapter) streamWithManifestExtraction(ctx context.Context, cmd *exec.Cmd, w io.Writer, stateDir string) error {
	pr, pw := io.Pipe()

	cmd.Stdout = io.MultiWriter(w, pw)
	cmd.Stderr = os.Stderr

	tempManifestPath := filepath.Join(stateDir, "backup_manifest.tmp")
	manifestPath := filepath.Join(stateDir, "backup_manifest")

	type extractionResult struct {
		extracted bool
		err       error
	}
	resultChan := make(chan extractionResult, 1)

	go func() {
		<-ctx.Done()
		pr.CloseWithError(ctx.Err())
	}()

	go func() {
		defer pr.Close()
		extracted := false

		for {
			select {
			case <-ctx.Done():
				resultChan <- extractionResult{err: ctx.Err()}
				return
			default:
			}

			tr := tar.NewReader(pr)
			foundAnyHeaderInArchive := false
			for {
				header, err := tr.Next()
				if err == io.EOF {
					break 
				}
				if err != nil {
					if !foundAnyHeaderInArchive {
						resultChan <- extractionResult{extracted: extracted, err: nil}
						return
					}
					resultChan <- extractionResult{err: fmt.Errorf("tar stream error: %w", err)}
					return
				}

				foundAnyHeaderInArchive = true
				if header.Name == "backup_manifest" {
					if pa.logger != nil {
						pa.logger.Info("Found backup_manifest in stream, extracting...")
					}
					f, err := os.Create(tempManifestPath)
					if err != nil {
						resultChan <- extractionResult{err: fmt.Errorf("failed to create temp manifest: %w", err)}
						return
					}
					written, err := io.Copy(f, tr)
					if err != nil {
						f.Close()
						os.Remove(tempManifestPath)
						resultChan <- extractionResult{err: fmt.Errorf("failed to write temp manifest: %w", err)}
						return
					}
					if err := f.Sync(); err != nil {
						f.Close()
						os.Remove(tempManifestPath)
						resultChan <- extractionResult{err: fmt.Errorf("failed to sync manifest: %w", err)}
						return
					}
					f.Close()
					extracted = true
					if pa.logger != nil {
						pa.logger.Debug("Manifest extraction complete", "bytes", written)
					}
				}
			}
			if !foundAnyHeaderInArchive {
				resultChan <- extractionResult{extracted: extracted, err: nil}
				return
			}
		}
	}()

	cmdErr := cmd.Run()
	pw.CloseWithError(cmdErr) 

	res := <-resultChan
	if cmdErr != nil {
		os.Remove(tempManifestPath) 
		return fmt.Errorf("pg_basebackup failed: %w", cmdErr)
	}

	if res.err != nil {
		os.Remove(tempManifestPath)
		return fmt.Errorf("manifest extraction failed: %w", res.err)
	}

	if res.extracted {
		if err := os.Rename(tempManifestPath, manifestPath); err != nil {
			return fmt.Errorf("failed to finalize manifest: %w", err)
		}
		if pa.logger != nil {
			pa.logger.Info("Successfully rotated backup_manifest atomically", "path", manifestPath)
		}
	}

	return nil
}

func (pa *PostgresAdapter) RunRestore(ctx context.Context, conn ConnectionParams, r io.Reader) error {
	if ma := pa.logger; ma != nil {
		ma.Info("Restoring database...", "engine", pa.Name())
	}

	if conn.BackupType == "physical" || conn.BackupType == "incremental" {
		return fmt.Errorf("physical/incremental restore not yet implemented via streaming reader (requires local extraction)")
	}

	connStr, err := pa.BuildConnection(ctx, conn)
	if err != nil {
		return err
	}

	cmd := exec.CommandContext(ctx, "psql", "--dbname", connStr)
	cmd.Stdin = r
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("psql restore failed: %w", err)
	}
	return nil
}

/*
RESTORE SAFETY NOTES (Physical/Incremental):
1. Physical backups created by pg_basebackup are not typical SQL dumps.
   They are snapshots of the PGDATA directory.
2. To restore:
   a. Stop the Postgres server.
   b. Clear the PGDATA directory.
   c. Extract the TAR stream into PGDATA.
   d. If it's an incremental backup, you must restore the full backup first,
      then apply each incremental backup in sequence using pg_combinebackup (PG 17+).
3. dbackup currently prioritizes streaming extraction. Direct 'restore' to a running
   Postgres instance is only supported for 'logical' backups via psql.

WAL SEMANTICS CAUTION:
1. WAL is NOT currently streamed for backup consistency during the physical backup operation
   due to streaming TAR limitations.
2. Users MUST enable WAL archiving on the PostgreSQL server for Point-In-Time Recovery (PITR)
   or to ensure a consistent physical backup.
3. PITR is NOT claimed or supported without external WAL archiving.
*/
