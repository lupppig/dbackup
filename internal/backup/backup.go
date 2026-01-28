package backup

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/lupppig/dbackup/internal/compress"
	database "github.com/lupppig/dbackup/internal/db"
	"github.com/lupppig/dbackup/internal/storage"
)

type BackupManager struct {
	Options BackupOptions
	storage storage.Storage
}

func NewBackupManager(opts BackupOptions) (*BackupManager, error) {
	var s storage.Storage
	switch strings.ToLower(opts.Storage) {
	case "local", "":
		s = storage.NewLocalStorage(opts.OutputDir)
	default:
		return nil, fmt.Errorf("unsupported storage type: %s", opts.Storage)
	}

	return &BackupManager{
		Options: opts,
		storage: s,
	}, nil
}

func (m *BackupManager) Run(ctx context.Context, adapter database.DBAdapter, connStr string) error {
	name := m.Options.FileName
	if name == "" {
		name = fmt.Sprintf("backup_%d.sql", time.Now().Unix())
	}

	algo := compress.Algorithm(m.Options.Algorithm)
	if m.Options.Compress && algo == "" {
		algo = compress.Lz4
	}

	finalName := name
	if m.Options.Compress && algo != compress.None {
		ext := ".tar"
		if !strings.HasSuffix(finalName, ext) {
			finalName += ext
		}
	}

	pr, pw := io.Pipe()

	errChan := make(chan error, 1)
	go func() {
		defer pw.Close()
		var w io.Writer = pw

		if m.Options.Compress {
			c, err := compress.New(pw, algo)
			if err != nil {
				errChan <- err
				return
			}
			if algo == compress.Tar {
				c.SetTarBufferName(name)
			}
			defer c.Close()
			w = c
		}

		if err := adapter.RunBackup(ctx, connStr, w); err != nil {
			errChan <- err
			return
		}
		errChan <- nil
	}()

	location, err := m.storage.Save(ctx, finalName, pr)
	if err != nil {
		pr.CloseWithError(err)
		return fmt.Errorf("storage save failed: %w", err)
	}

	if err := <-errChan; err != nil {
		return err
	}

	if m.Options.Logger != nil {
		m.Options.Logger.Info("Backup saved successfully", "location", location)
	}

	return nil
}
