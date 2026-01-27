package database

import (
	"context"
	"testing"
)

func TestPostgresAdapter_Name(t *testing.T) {
	pa := PostgresAdapter{}
	if pa.Name() != "postgres" {
		t.Errorf("expected postgres, got %s", pa.Name())
	}
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := pa.BuildConnection(ctx, tt.params)
			if (err != nil) != tt.wantErr {
				t.Errorf("BuildConnection() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("BuildConnection() got = %v, want %v", got, tt.want)
			}
		})
	}
}
