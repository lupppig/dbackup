package db

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"net/url"
	"strings"
	"time"

	_ "github.com/lib/pq"
	apperrors "github.com/lupppig/dbackup/internal/errors"
	"github.com/lupppig/dbackup/internal/logger"
)

func init() {
	RegisterAdapter(&PostgresAdapter{})
}

/*
RESTORE SAFETY NOTES (Logical):
1. dbackup currently prioritizes logical dumps via pg_dump for best compatibility.
2. To restore: Use the 'restore' command which pipes the backup into 'psql'.
*/

type PostgresAdapter struct {
	logger *logger.Logger
}

func (pa *PostgresAdapter) SetLogger(l *logger.Logger) {
	pa.logger = l
}

func (pa *PostgresAdapter) Name() string {
	return "postgres"
}

func (pa *PostgresAdapter) TestConnection(ctx context.Context, conn ConnectionParams, runner Runner) error {
	if pa.logger != nil {
		pa.logger.Info("Testing database connection...", "host", conn.Host, "db", conn.DBName)
	}
	dsn, err := pa.BuildConnection(ctx, conn)
	if err != nil {
		return err
	}

	if _, ok := runner.(*LocalRunner); !ok && runner != nil {
		if pa.logger != nil {
			pa.logger.Info("Testing remote connection via psql...", "host", conn.Host, "db", conn.DBName)
		}
		// Remote runner: use psql to test connection
		args := []string{"--dbname", dsn, "-c", "SELECT 1"}
		err := runner.Run(ctx, "psql", args, io.Discard)
		if err != nil {
			if strings.Contains(err.Error(), "status 127") || strings.Contains(err.Error(), "executable file not found") {
				return apperrors.New(apperrors.TypeDependency, "psql client not found", "Please install postgresql-client on the target system to enable connection testing and logical backups.")
			}
			return apperrors.Wrap(err, apperrors.TypeConnection, "failed to connect via psql", "Ensure the database is reachable and the credentials are correct.")
		}
		return nil
	}

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return apperrors.Wrap(err, apperrors.TypeConfig, "failed to open database connection", "Check your connection string and driver availability.")
	}

	defer db.Close()

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		return apperrors.Wrap(err, apperrors.TypeConnection, "failed to ping database", "Verify the database host, port, and credentials.")
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

func (pa *PostgresAdapter) RunBackup(ctx context.Context, conn ConnectionParams, runner Runner, w io.Writer) error {
	// Standard full logical backup
	return pa.runLogicalBackup(ctx, conn, runner, w)
}

func (pa *PostgresAdapter) runLogicalBackup(ctx context.Context, conn ConnectionParams, runner Runner, w io.Writer) error {
	if pa.logger != nil {
		pa.logger.Info("Dumping database...", "engine", pa.Name(), "type", "full (logical)")
	}

	connStr, err := pa.BuildConnection(ctx, conn)
	if err != nil {
		return err
	}

	args := []string{
		"--dbname", connStr,
		"--format=plain",
		"--no-owner",
		"--no-acl",
	}

	if err := runner.Run(ctx, "pg_dump", args, w); err != nil {
		if strings.Contains(err.Error(), "status 127") || strings.Contains(err.Error(), "executable file not found") {
			return apperrors.New(apperrors.TypeDependency, "pg_dump not found", "Please install postgresql-client to enable logical backups.")
		}
		return apperrors.Wrap(err, apperrors.TypeInternal, "pg_dump failed", "Check pg_dump logs or permissions.")
	}

	return nil
}

func (pa *PostgresAdapter) RunRestore(ctx context.Context, conn ConnectionParams, runner Runner, r io.Reader) error {
	if ma := pa.logger; ma != nil {
		ma.Info("Restoring database...", "engine", pa.Name())
	}

	connStr, err := pa.BuildConnection(ctx, conn)
	if err != nil {
		return err
	}

	args := []string{"--dbname", connStr}
	return runner.RunWithIO(ctx, "psql", args, r, nil)
}
