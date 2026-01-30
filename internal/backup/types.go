package backup

import (
	"context"

	"github.com/lupppig/dbackup/internal/logger"
	"github.com/lupppig/dbackup/internal/notify"
)

type BackupOptions struct {
	DBType     string
	DBName     string
	Storage    string // Backend type: local, s3, gs, ssh, ftp, docker
	StorageURI string // Unified targeting URI
	Compress   bool
	Algorithm  string
	FileName   string
	OutputDir  string
	RemoteExec bool // Force remote execution if storage is remote
	Logger     *logger.Logger
	Notifier   notify.Notifier
}

type BackupProcess interface {
	Run(ctx context.Context) error
}
