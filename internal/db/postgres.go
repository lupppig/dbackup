package database

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"time"
)

type PostgresAdapter struct{}

func (pa PostgresAdapter) Name() string {
	return "postgres"
}

func (pa PostgresAdapter) TestConnection(ctx context.Context, conn ConnectionParams) error {
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

func (pa PostgresAdapter) BuildConnection(ctx context.Context, conn ConnectionParams) (string, error) {
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
