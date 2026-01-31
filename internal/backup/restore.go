package backup

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/lupppig/dbackup/internal/compress"
	"github.com/lupppig/dbackup/internal/crypto"
	database "github.com/lupppig/dbackup/internal/db"
	"github.com/lupppig/dbackup/internal/manifest"
	"github.com/lupppig/dbackup/internal/notify"
	"github.com/lupppig/dbackup/internal/storage"
)

type RestoreManager struct {
	Options BackupOptions
	storage storage.Storage
}

func NewRestoreManager(opts BackupOptions) (*RestoreManager, error) {
	s, err := storage.FromURI(opts.StorageURI, storage.StorageOptions{
		AllowInsecure: opts.AllowInsecure,
	})
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

func (m *RestoreManager) Run(ctx context.Context, adapter database.DBAdapter, conn database.ConnectionParams) (err error) {
	if !m.Options.ConfirmRestore {
		return fmt.Errorf("RESTORE DENIED: Destructive operations require explicit confirmation. Use --confirm-restore to proceed")
	}

	start := time.Now()
	name := m.Options.FileName
	if name == "" {
		return fmt.Errorf("file name is required for restore")
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
				Operation: "Restore",
				Engine:    conn.DBType,
				Database:  conn.DBName,
				FileName:  name,
				Duration:  time.Since(start),
				Error:     err,
			})
		}
	}()

	// Integrity & Manifesting Logic
	// Step 1: Read Manifest
	manBytes, err := m.storage.GetMetadata(ctx, name+".manifest")
	if err != nil {
		if m.Options.Logger != nil {
			m.Options.Logger.Warn("Manifest not found for backup, skipping integrity check", "file", name)
		}
	}

	var man *manifest.Manifest
	if err == nil {
		man, _ = manifest.Deserialize(manBytes)
	}

	// Step 2: Download to temporary workspace for verification
	tmpDir, err := os.MkdirTemp("", "dbackup-restore-*")
	if err != nil {
		return fmt.Errorf("failed to create temporary workspace: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	tmpFile := filepath.Join(tmpDir, name)
	f, err := os.Create(tmpFile)
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}

	r, err := m.storage.Open(ctx, name)
	if err != nil {
		f.Close()
		return fmt.Errorf("failed to open backup for restore: %w", err)
	}

	// Hash while downloading
	hasher := sha256.New()
	tr := io.TeeReader(r, hasher)

	if m.Options.Logger != nil {
		m.Options.Logger.Info("Downloading backup for verification...", "name", name)
	}
	_, err = io.Copy(f, tr)
	r.Close()
	f.Close()
	if err != nil {
		return fmt.Errorf("failed to download backup: %w", err)
	}

	// Step 3: Verify Integrity
	if man != nil {
		actualChecksum := hex.EncodeToString(hasher.Sum(nil))
		if man.Checksum != "" && man.Checksum != actualChecksum {
			return fmt.Errorf("INTEGRITY FAILURE: backup checksum mismatch (expected %s, got %s)", man.Checksum, actualChecksum)
		}
		if m.Options.Logger != nil {
			m.Options.Logger.Info("Integrity verification passed", "checksum", actualChecksum)
		}
	}

	// Step 4: Perform Restoration from temp file
	f, err = os.Open(tmpFile)
	if err != nil {
		return fmt.Errorf("failed to open temp file for reading: %w", err)
	}
	defer f.Close()

	var finalReader io.Reader = f

	if m.Options.Encrypt {
		km, err := crypto.NewKeyManager(m.Options.EncryptionPassphrase, m.Options.EncryptionKeyFile)
		if err != nil {
			return err
		}
		dr := crypto.NewDecryptReader(finalReader, km)
		finalReader = dr
	}

	// Handle decompression if requested/detected
	algo := compress.Algorithm(m.Options.Algorithm)
	if algo == "" || algo == compress.None {
		// Auto-detect from filename
		algo = compress.DetectAlgorithm(name)
	}

	if algo != compress.None {
		c, err := compress.NewReader(finalReader, algo)
		if err != nil {
			return fmt.Errorf("failed to create decompression reader for %s: %w", algo, err)
		}
		defer c.Close()
		finalReader = c
	}

	var runner database.Runner = &database.LocalRunner{}
	if r, ok := m.storage.(database.Runner); ok {
		runner = r
	}

	if err := adapter.RunRestore(ctx, conn, runner, finalReader); err != nil {
		return fmt.Errorf("database restore failed: %w", err)
	}

	if m.Options.Logger != nil {
		m.Options.Logger.Info("Restore completed successfully")
	}

	return nil
}
