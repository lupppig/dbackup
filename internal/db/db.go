package database

import (
	"context"
	"fmt"
	"time"

	"github.com/lupppig/dbackup/internal/backup"
	"github.com/lupppig/dbackup/internal/logger"
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
	Storage    string
	Compress   bool
	FileName   string // file name string
	BackupType string // incremental or differential
	OutputDir  string
}
type DBAdapter interface {
	Name() string
	TestConnection(ctx context.Context, conn ConnectionParams) error
	BuildConnection(ctx context.Context, conn ConnectionParams) (string, error)
	RunBackup(ctx context.Context, connStr string, backupOptions BackUpOptions) error
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

func buildWriter(opts BackUpOptions) (backup.BackupWriter, error) {
	name := opts.FileName
	if name == "" {
		name = fmt.Sprintf("backup_%d.sql", time.Now().Unix())
	}
	fileWriter, err := backup.NewFileWriter(opts.OutputDir, name)
	if err != nil {
		return nil, err
	}

	if opts.Compress {
		return backup.NewGzipWriter(fileWriter), nil
	}

	return fileWriter, nil
}
