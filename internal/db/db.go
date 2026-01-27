package database

import (
	"context"
	"fmt"
)

type TLSConfig struct {
	Enabled    bool
	Mode       string // disable | require | verify-ca | verify-full
	CACert     string
	ClientCert string
	ClientKey  string
}

type ConnectionParams struct {
	DBtype   string
	DBName   string
	Password string
	User     string
	Host     string
	Port     int
	DBUri    string

	TLS TLSConfig
}

type BackUpOptions struct {
	Compress   bool
	Outputpath string
	Format     string // sql tar or zip, custom etc...
	BackupType string // incremental or differential
}
type DBAdapter interface {
	Name() string
	TestConnection(ctx context.Context, conn ConnectionParams) error
	BuildConnection(ctx context.Context, conn ConnectionParams) (string, error)
}

var adapters = map[string]DBAdapter{}

func RegisterAdapter(adapter DBAdapter) {
	adapters[adapter.Name()] = adapter
}

func GetAdapter(name string) (DBAdapter, error) {
	adapter, ok := adapters[name]
	if !ok {
		return nil, fmt.Errorf("unsupported database: %s", name)
	}
	return adapter, nil
}
