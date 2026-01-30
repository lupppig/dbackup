package backup

import (
	"context"
	"fmt"
	"io"

	"github.com/lupppig/dbackup/internal/compress"
	database "github.com/lupppig/dbackup/internal/db"
	"github.com/lupppig/dbackup/internal/storage"
)

type RestoreManager struct {
	Options BackupOptions
	storage storage.Storage
}

func NewRestoreManager(opts BackupOptions) (*RestoreManager, error) {
	s, err := storage.FromURI(opts.StorageURI)
	if err != nil {
		return nil, err
	}

	return &RestoreManager{
		Options: opts,
		storage: s,
	}, nil
}

func (m *RestoreManager) GetStorage() storage.Storage {
	return m.storage
}

func (m *RestoreManager) SetStorage(s storage.Storage) {
	m.storage = s
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

	var finalReader io.Reader = r

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

	var runner database.Runner = &database.LocalRunner{}
	if r, ok := m.storage.(database.Runner); ok {
		runner = r
	}

	if err := adapter.RunRestore(ctx, conn, runner, finalReader); err != nil {
		return fmt.Errorf("restore failed: %w", err)
	}

	if m.Options.Logger != nil {
		m.Options.Logger.Info("Restore completed successfully")
	}

	return nil
}
