package backup

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/lupppig/dbackup/internal/compress"
	"github.com/lupppig/dbackup/internal/crypto"
	database "github.com/lupppig/dbackup/internal/db"
	apperrors "github.com/lupppig/dbackup/internal/errors"
	"github.com/lupppig/dbackup/internal/manifest"
	"github.com/lupppig/dbackup/internal/notify"
	"github.com/lupppig/dbackup/internal/storage"
)

type BackupManager struct {
	Options BackupOptions
	storage storage.Storage
}

func NewBackupManager(opts BackupOptions) (*BackupManager, error) {
	s, err := storage.FromURI(opts.StorageURI, storage.StorageOptions{
		AllowInsecure: opts.AllowInsecure,
	})
	if err != nil {
		return nil, err
	}

	// Wrap with dedupe storage for incremental backups
	if opts.Dedupe {
		s = storage.NewDedupeStorage(s)
	}

	return &BackupManager{
		Options: opts,
		storage: s,
	}, nil
}

func (m *BackupManager) GetStorage() storage.Storage {
	return m.storage
}

func (m *BackupManager) SetStorage(s storage.Storage) {
	m.storage = s
}

func (m *BackupManager) Run(ctx context.Context, adapter database.DBAdapter, conn database.ConnectionParams) (err error) {
	start := time.Now()
	if err := conn.ParseURI(); err != nil {
		if m.Options.Logger != nil {
			m.Options.Logger.Warn("Failed to parse DB URI", "error", err)
		}
	}

	if m.Options.Logger != nil {
		m.Options.Logger.Debug("Backup process started", "engine", conn.DBType)
	}

	name := m.Options.FileName
	if name == "" {
		prefix := strings.ToLower(conn.DBType)
		if prefix == "" {
			prefix = "backup"
		}
		dbPart := ""
		if conn.DBName != "" {
			// Sanitize DBName for filename
			dbPart = strings.ReplaceAll(conn.DBName, "/", "_")
			dbPart = strings.ReplaceAll(dbPart, "\\", "_")
			dbPart = "-" + dbPart
		}
		name = fmt.Sprintf("%s%s-%s.sql", prefix, dbPart, time.Now().Format("20060102-150405.000"))
	}

	algo := compress.Algorithm(m.Options.Algorithm)
	if m.Options.Compress && algo == "" {
		algo = compress.Lz4
	}

	finalName := name
	if m.Options.Compress && algo != compress.None {
		switch algo {
		case compress.Gzip:
			finalName += ".gz"
		case compress.Lz4:
			finalName += ".lz4"
		case compress.Zstd:
			finalName += ".zst"
		case compress.Tar:
			finalName += ".tar"
		}
	}

	// Stats for notification
	defer func() {
		if m.Options.Notifier != nil {
			status := notify.StatusSuccess
			if err != nil {
				status = notify.StatusError
			}
			m.Options.Notifier.Notify(ctx, notify.Stats{
				Status:    status,
				Operation: "Backup",
				Engine:    conn.DBType,
				Database:  conn.DBName,
				FileName:  finalName,
				Duration:  time.Since(start),
				Error:     err,
			})
		}
	}()

	pr, pw := io.Pipe()

	errChan := make(chan error, 1)
	go func() {
		defer pw.Close()
		var w io.Writer = pw

		if m.Options.Encrypt {
			km, err := crypto.NewKeyManager(m.Options.EncryptionPassphrase, m.Options.EncryptionKeyFile)
			if err != nil {
				errChan <- err
				return
			}
			ew, err := crypto.NewEncryptWriter(pw, km)
			if err != nil {
				errChan <- err
				return
			}
			defer ew.Close()
			w = ew
		}

		if m.Options.Compress {
			c, err := compress.New(w, algo)
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

		var r database.Runner = &database.LocalRunner{}
		if m.Options.RemoteExec {
			if runner, ok := m.storage.(database.Runner); ok {
				if m.Options.Logger != nil {
					m.Options.Logger.Info("Using remote runner from storage backend (remote-exec enabled)")
				}
				r = runner
			}
		}

		if err := adapter.RunBackup(ctx, conn, r, w); err != nil {
			errChan <- err
			return
		}
		errChan <- nil
	}()

	// Integrity & Manifesting
	hasher := sha256.New()
	tr := io.TeeReader(pr, hasher)

	location, err := m.storage.Save(ctx, finalName, tr)
	if err != nil {
		return apperrors.Wrap(err, apperrors.TypeResource, "storage save failed", "Check storage permissions and disk space.")
	}

	if err := <-errChan; err != nil {
		return err
	}

	checksum := hex.EncodeToString(hasher.Sum(nil))

	encryption := "none"
	if m.Options.Encrypt {
		encryption = "aes-256-gcm"
	}

	man := manifest.New(
		fmt.Sprintf("%x", time.Now().UnixNano()),
		conn.DBType,
		string(algo),
		encryption,
	)
	man.DBName = conn.DBName
	man.Checksum = checksum
	man.Version = "0.1.0"

	manBytes, err := man.Serialize()
	if err == nil {
		_ = m.storage.PutMetadata(ctx, finalName+".manifest", manBytes)
	}

	if m.Options.Logger != nil {
		m.Options.Logger.Info("Backup saved successfully", "location", location)
	}

	return nil
}
