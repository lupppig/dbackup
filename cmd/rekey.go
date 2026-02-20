package cmd

import (
	"fmt"
	"io"
	"strings"

	"github.com/lupppig/dbackup/internal/crypto"
	"github.com/lupppig/dbackup/internal/logger"
	"github.com/lupppig/dbackup/internal/manifest"
	storagepkg "github.com/lupppig/dbackup/internal/storage"
	"github.com/spf13/cobra"
)

var (
	oldPassphrase string
	newPassphrase string
)

var rekeyCmd = &cobra.Command{
	Use:   "rekey",
	Short: "Re-encrypt backups with a new passphrase",
	Long: `Decrypts existing backups using the old passphrase and re-encrypts them with a new one.
This will update both the backup data (chunks if deduped) and the manifests.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		l := logger.FromContext(cmd.Context())

		if oldPassphrase == "" || newPassphrase == "" {
			return fmt.Errorf("both --old-pass and --new-pass are required")
		}

		s, err := storagepkg.FromURI(target, storagepkg.StorageOptions{AllowInsecure: AllowInsecure})
		if err != nil {
			return err
		}
		defer s.Close()

		if dedupe {
			s = storagepkg.NewDedupeStorage(s)
		}

		l.Info("Starting key rotation", "target", storagepkg.Scrub(target))

		files, err := s.ListMetadata(cmd.Context(), "")
		if err != nil {
			return fmt.Errorf("failed to list manifests: %w", err)
		}

		oldKM, _ := crypto.NewKeyManager(oldPassphrase, "")
		newKM, _ := crypto.NewKeyManager(newPassphrase, "")

		rekeyedCount := 0
		for _, file := range files {
			if !strings.HasSuffix(file, ".manifest") || file == "latest.manifest" {
				continue
			}

			l.Info("Rekeying backup", "manifest", file)

			data, err := s.GetMetadata(cmd.Context(), file)
			if err != nil {
				l.Warn("Failed to read manifest", "file", file, "error", err)
				continue
			}

			man, err := manifest.Deserialize(data)
			if err != nil {
				l.Warn("Failed to deserialize manifest", "file", file, "error", err)
				continue
			}

			if man.Encryption == "none" {
				l.Info("Skipping unencrypted backup", "file", file)
				continue
			}

			// 1. Open and decrypt existing data
			backupName := strings.TrimSuffix(file, ".manifest")
			r, err := s.Open(cmd.Context(), backupName)
			if err != nil {
				l.Warn("Failed to open backup data", "file", backupName, "error", err)
				continue
			}

			dr := crypto.NewDecryptReader(r, oldKM)

			// 2. Re-encrypt with new key
			pr, pw := io.Pipe()
			go func() {
				defer pw.Close()
				ew, err := crypto.NewEncryptWriter(pw, newKM)
				if err != nil {
					return
				}
				defer ew.Close()
				_, _ = io.Copy(ew, dr)
			}()

			// 3. Save to storage (this will create new chunks if deduped)
			newLoc, err := s.Save(cmd.Context(), backupName+"_rekeyed", pr)
			r.Close()
			if err != nil {
				return fmt.Errorf("failed to save re-encrypted backup: %w", err)
			}

			// 4. Update manifest and save it
			man.Encryption = "aes-256-gcm"
			man.FileName = backupName + "_rekeyed"
			if cs, ok := s.(storagepkg.ChunkedStorage); ok {
				man.Chunks = cs.LastChunks()
			}

			newManBytes, err := man.Serialize()
			if err != nil {
				return err
			}

			if err := s.PutMetadata(cmd.Context(), file, newManBytes); err != nil {
				return fmt.Errorf("failed to update manifest: %w", err)
			}

			// 5. Cleanup old data (optional, but probably desired for rekey)
			// For safety, we might not delete it immediately, but here we do for simplicity.
			_ = s.Delete(cmd.Context(), backupName)

			rekeyedCount++
			l.Info("Rekeying complete", "manifest", file, "new_location", newLoc)
		}

		l.Info("Key rotation finished", "count", rekeyedCount)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(rekeyCmd)
	rekeyCmd.Flags().StringVar(&oldPassphrase, "old-pass", "", "Current passphrase")
	rekeyCmd.Flags().StringVar(&newPassphrase, "new-pass", "", "New passphrase")
	rekeyCmd.Flags().StringVar(&target, "target", ".", "Storage target URI")
}
