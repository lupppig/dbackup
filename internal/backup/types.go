package backup

import (
	"context"

	"github.com/lupppig/dbackup/internal/logger"
	"github.com/lupppig/dbackup/internal/notify"
)

type BackupOptions struct {
	DBType        string
	DBName        string
	StorageURI    string // Unified targeting URI
	Compress      bool
	Algorithm     string
	FileName      string
	RemoteExec    bool // Force remote execution if storage is remote
	AllowInsecure bool // Allow insecure protocols

	// Encryption
	Encrypt              bool
	EncryptionKeyFile    string
	EncryptionPassphrase string

	ConfirmRestore bool // Explicitly confirm destructive restore

	Logger   *logger.Logger
	Notifier notify.Notifier
}

type BackupProcess interface {
	Run(ctx context.Context) error
}
