package backup

import (
	"context"

	"github.com/lupppig/dbackup/internal/logger"
)

type BackupOptions struct {
	DBType     string
	DBName     string
	Storage    string
	Compress   bool
	Algorithm  string
	FileName   string
	BackupType string
	OutputDir  string
	Logger     *logger.Logger
}

type BackupProcess interface {
	Run(ctx context.Context) error
}
