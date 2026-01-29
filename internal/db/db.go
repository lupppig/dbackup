package db

import (
	"context"
	"fmt"
	"io"

	"github.com/lupppig/dbackup/internal/logger"
)

type TLSConfig struct {
	Enabled    bool
	Mode       string
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

	BackupType string // "full" or "incremental"
	StateDir   string // Directory to share/store state between runs
}

type BackUpOptions struct {
	Storage    string
	Compress   bool
	Algorithm  string
	FileName   string
	BackupType string
	OutputDir  string
}

type DBAdapter interface {
	Name() string
	TestConnection(ctx context.Context, conn ConnectionParams) error
	BuildConnection(ctx context.Context, conn ConnectionParams) (string, error)
	RunBackup(ctx context.Context, conn ConnectionParams, w io.Writer) error
	RunRestore(ctx context.Context, conn ConnectionParams, r io.Reader) error
	SetLogger(l *logger.Logger)
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
