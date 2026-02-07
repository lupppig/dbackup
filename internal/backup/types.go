package backup

import (
	"context"

	"time"

	"github.com/lupppig/dbackup/internal/logger"
	"github.com/lupppig/dbackup/internal/notify"
	"github.com/vbauerster/mpb/v8"
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
	Dedupe        bool // Enable storage-level deduplication (incremental)

	Retention time.Duration
	Keep      int

	// Encryption
	Encrypt              bool
	EncryptionKeyFile    string
	EncryptionPassphrase string

	ConfirmRestore bool // Explicitly confirm destructive restore
	DryRun         bool // Simulation mode

	Logger   *logger.Logger
	Notifier notify.Notifier
	Progress *mpb.Progress
}

type BackupProcess interface {
	Run(ctx context.Context) error
}
