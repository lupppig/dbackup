package database

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPostgresAdapter_Name(t *testing.T) {
	pa := PostgresAdapter{}
	assert.Equal(t, "postgres", pa.Name())
}

func TestPostgresAdapter_BuildConnection(t *testing.T) {
	pa := PostgresAdapter{}
	ctx := context.Background()

	tests := []struct {
		name    string
		params  ConnectionParams
		want    string
		wantErr bool
	}{
		{
			name: "With DBUri",
			params: ConnectionParams{
				DBUri: "postgres://user:pass@host:5432/dbname",
			},
			want:    "postgres://user:pass@host:5432/dbname",
			wantErr: false,
		},
		{
			name: "With Individual Flags",
			params: ConnectionParams{
				Host:     "localhost",
				User:     "testuser",
				Password: "testpassword",
				DBName:   "testdb",
				Port:     5432,
			},
			want:    "postgres://testuser:testpassword@localhost:5432/testdb?sslmode=disable",
			wantErr: false,
		},
		{
			name: "Missing Required Fields",
			params: ConnectionParams{
				Host: "localhost",
			},
			wantErr: true,
		},
		{
			name: "With TLS Enabled (Default Mode)",
			params: ConnectionParams{
				Host:     "localhost",
				User:     "testuser",
				Password: "testpassword",
				DBName:   "testdb",
				TLS: TLSConfig{
					Enabled: true,
				},
			},
			want:    "postgres://testuser:testpassword@localhost:5432/testdb?sslmode=require",
			wantErr: false,
		},
		{
			name: "With TLS Enabled (Custom Mode)",
			params: ConnectionParams{
				Host:     "localhost",
				User:     "testuser",
				Password: "testpassword",
				DBName:   "testdb",
				TLS: TLSConfig{
					Enabled: true,
					Mode:    "verify-full",
				},
			},
			want:    "postgres://testuser:testpassword@localhost:5432/testdb?sslmode=verify-full",
			wantErr: false,
		},
		{
			name: "With Root CA Certificate",
			params: ConnectionParams{
				Host:     "localhost",
				User:     "testuser",
				Password: "testpassword",
				DBName:   "testdb",
				TLS: TLSConfig{
					Enabled: true,
					Mode:    "verify-ca",
					CACert:  "/path/to/ca.pem",
				},
			},
			want:    "postgres://testuser:testpassword@localhost:5432/testdb?sslmode=verify-ca&sslrootcert=%2Fpath%2Fto%2Fca.pem",
			wantErr: false,
		},
		{
			name: "With mTLS (Client Cert and Key)",
			params: ConnectionParams{
				Host:     "localhost",
				User:     "testuser",
				Password: "testpassword",
				DBName:   "testdb",
				TLS: TLSConfig{
					Enabled:    true,
					Mode:       "verify-full",
					ClientCert: "/path/to/client.crt",
					ClientKey:  "/path/to/client.key",
				},
			},
			want:    "postgres://testuser:testpassword@localhost:5432/testdb?sslcert=%2Fpath%2Fto%2Fclient.crt&sslkey=%2Fpath%2Fto%2Fclient.key&sslmode=verify-full",
			wantErr: false,
		},
		{
			name: "mTLS Error (Missing Client Key)",
			params: ConnectionParams{
				Host:     "localhost",
				User:     "testuser",
				Password: "testpassword",
				DBName:   "testdb",
				TLS: TLSConfig{
					Enabled:    true,
					ClientCert: "/path/to/client.crt",
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := pa.BuildConnection(ctx, tt.params)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}
