package backup

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
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

	// Wrap with dedupe storage if enabled
	if opts.Dedupe {
		s = storage.NewDedupeStorage(s)
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
	if err := conn.ParseURI(); err != nil {
		if m.Options.Logger != nil {
			m.Options.Logger.Warn("Failed to parse DB URI", "error", err)
		}
	}
	name := m.Options.FileName
	if name == "" {
		name = "latest.manifest"
	}

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

	manPath := name
	if !strings.HasSuffix(name, ".manifest") {
		manPath = name + ".manifest"
	}

	// Use a sub-context with a timeout for the metadata check to avoid long hangs
	metaCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	manBytes, err := m.storage.GetMetadata(metaCtx, manPath)
	cancel()

	if err != nil {
		if m.Options.FileName == "" || name == "latest.manifest" {
			return fmt.Errorf("default manifest %s not found and no specific file provided: %w", manPath, err)
		}
		if m.Options.Logger != nil {
			m.Options.Logger.Warn("Manifest not found for backup, skipping integrity check", "file", name)
		}
	}

	var man *manifest.Manifest
	if err == nil {
		man, _ = manifest.Deserialize(manBytes)
		if man != nil {
			if man.Engine != "" && !strings.EqualFold(man.Engine, conn.DBType) {
				return fmt.Errorf("engine mismatch: manifest is for %s but restoring to %s", man.Engine, conn.DBType)
			}
			if man.FileName != "" {
				if m.Options.Logger != nil {
					m.Options.Logger.Info("Manifest resolved backup file", "manifest", name, "backup", man.FileName)
				}
				name = man.FileName
			}
		}
	}

	if m.Options.Logger != nil {
		m.Options.Logger.Debug("Opening storage and downloading...", "uri", m.Options.StorageURI, "file", name)
	}

	// Download to temporary workspace for verification
	tmpDir, err := os.MkdirTemp("", "dbackup-restore-*")
	if err != nil {
		return fmt.Errorf("failed to create temporary workspace: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	tmpFile := filepath.Join(tmpDir, name)
	if err := os.MkdirAll(filepath.Dir(tmpFile), 0755); err != nil {
		return fmt.Errorf("failed to create temp directory: %w", err)
	}
	f, err := os.Create(tmpFile)
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}

	r, err := m.storage.Open(ctx, name)
	if err != nil {
		f.Close()
		return fmt.Errorf("failed to open backup for restore: %w", err)
	}

	var totalSize int64
	if man != nil {
		totalSize = man.Size
	}

	p := m.Options.Progress
	shouldWait := false
	if p == nil {
		p = NewProgressContainer()
		shouldWait = true
	}
	bar := AddRestoreBar(p, "Download", totalSize)

	// Hash while downloading
	hasher := sha256.New()
	pr := NewProgressReader(r, bar)
	tr := io.TeeReader(pr, hasher)

	if m.Options.Logger != nil {
		m.Options.Logger.Info("Downloading backup file...", "name", name, "size", totalSize)
	}
	_, err = io.Copy(f, tr)
	if bar != nil {
		bar.SetTotal(bar.Current(), true)
	}

	if shouldWait && p != nil {
		p.Wait()
	}
	// Do not call p.Wait() here if it's shared, as the caller (dumpCmd) will wait at the end
	// Wait only if created locally.
	// Actually, dumpCmd waits at the end of immediate tasks.
	r.Close()
	f.Close()
	if err != nil {
		msg := "Check storage connectivity and file existence."
		// Check if it's a timeout or connection error
		isTimeout := strings.Contains(err.Error(), "timeout") || strings.Contains(err.Error(), "deadline") || strings.Contains(err.Error(), "refused")
		if ctx.Err() == nil && !isTimeout {
			// Only try to list files if it's likely a 404/Not Found, to avoid doubling the timeout
			if files, listErr := m.storage.ListMetadata(ctx, ""); listErr == nil && len(files) > 0 {
				msg = fmt.Sprintf("Target file not found. Available files: %s", strings.Join(files, ", "))
			}
		}
		return apperrors.Wrap(err, apperrors.TypeResource, "failed to download backup", msg)
	}

	// Verify Integrity
	if man != nil {
		actualChecksum := hex.EncodeToString(hasher.Sum(nil))
		if man.Checksum != "" && man.Checksum != actualChecksum {
			return apperrors.ErrIntegrityMismatch
		}
		if m.Options.Logger != nil {
			m.Options.Logger.Info("Integrity verification passed", "checksum", actualChecksum)
		}
	}

	// Perform Restoration from temp file
	f, err = os.Open(tmpFile)
	if err != nil {
		return fmt.Errorf("failed to open temp file for reading: %w", err)
	}
	defer f.Close()

	var finalReader io.Reader = f

	// Smart Detection
	actualEncrypt := m.Options.Encrypt
	actualAlgo := compress.Algorithm(m.Options.Algorithm)

	if man != nil {
		if man.Encryption != "" && man.Encryption != "none" {
			actualEncrypt = true
		}
		if man.Compression != "" && man.Compression != "none" {
			actualAlgo = compress.Algorithm(man.Compression)
		}
	}

	// Sniff for encryption magic "DBKP"
	header := make([]byte, 4)
	n, _ := io.ReadAtLeast(finalReader, header, 4)
	if n == 4 && string(header) == crypto.MagicBytes {
		actualEncrypt = true
	}
	// Put the header back
	finalReader = io.MultiReader(bytes.NewReader(header[:n]), finalReader)

	if actualEncrypt {
		if m.Options.EncryptionPassphrase == "" && m.Options.EncryptionKeyFile == "" {
			// Try environment variable
			if pass := os.Getenv("DBACKUP_KEY"); pass != "" {
				m.Options.EncryptionPassphrase = pass
			} else {
				return apperrors.New(apperrors.TypeSecurity, "backup is encrypted but no passphrase or key-file was provided", "Set the DBACKUP_KEY environment variable or use --encryption-passphrase.")
			}
		}
		km, err := crypto.NewKeyManager(m.Options.EncryptionPassphrase, m.Options.EncryptionKeyFile)
		if err != nil {
			return err
		}
		dr := crypto.NewDecryptReader(finalReader, km)
		finalReader = dr
	}

	// Handle decompression
	if actualAlgo == "" || actualAlgo == compress.None {
		// Auto-detect from filename if still unknown
		actualAlgo = compress.DetectAlgorithm(name)
	}

	if actualAlgo != compress.None {
		c, err := compress.NewReader(finalReader, actualAlgo)
		if err != nil {
			return fmt.Errorf("failed to create decompression reader for %s: %w", actualAlgo, err)
		}
		defer c.Close()
		finalReader = c
	}

	var runner database.Runner = &database.LocalRunner{}
	if r, ok := m.storage.(database.Runner); ok {
		runner = r
	}

	if m.Options.DryRun {
		runner = database.NewDryRunRunner(m.Options.Logger)
	}

	if err := adapter.RunRestore(ctx, conn, runner, finalReader); err != nil {
		return fmt.Errorf("database restore failed: %w", err)
	}

	if m.Options.Logger != nil {
		m.Options.Logger.Info("Restore completed successfully")
	}

	return nil
}
