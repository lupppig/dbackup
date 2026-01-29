package backup

import (
	"context"
	"fmt"
	"strings"

	"github.com/lupppig/dbackup/internal/compress"
	database "github.com/lupppig/dbackup/internal/db"
	"github.com/lupppig/dbackup/internal/storage"
)

type RestoreManager struct {
	Options BackupOptions
	storage storage.Storage
}

func NewRestoreManager(opts BackupOptions) (*RestoreManager, error) {
	var s storage.Storage
	switch strings.ToLower(opts.Storage) {
	case "local", "":
		s = storage.NewLocalStorage(opts.OutputDir)
	default:
		return nil, fmt.Errorf("unsupported storage type: %s", opts.Storage)
	}

	return &RestoreManager{
		Options: opts,
		storage: s,
	}, nil
}

func (m *RestoreManager) Run(ctx context.Context, adapter database.DBAdapter, conn database.ConnectionParams) error {
	name := m.Options.FileName
	if name == "" {
		return fmt.Errorf("file name is required for restore")
	}

	// We open the stream from storage
	r, err := m.storage.Open(ctx, name)
	if err != nil {
		return fmt.Errorf("failed to open backup for restore: %w", err)
	}
	defer r.Close()

	var finalReader = r

	// Handle decompression if requested/detected
	algo := compress.Algorithm(m.Options.Algorithm)
	if m.Options.Compress || (algo != "" && algo != compress.None) {
		if algo == "" {
			algo = compress.Lz4 // Default
		}

		c, err := compress.NewReader(r, algo)
		if err != nil {
			return fmt.Errorf("failed to create decompression reader: %w", err)
		}
		defer c.Close()
		finalReader = c
	}

	if err := adapter.RunRestore(ctx, conn, finalReader); err != nil {
		return fmt.Errorf("restore failed: %w", err)
	}

	if m.Options.Logger != nil {
		m.Options.Logger.Info("Restore completed successfully")
	}

	return nil
}
