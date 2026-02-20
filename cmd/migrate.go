package cmd

import (
	"fmt"
	"strings"

	"github.com/lupppig/dbackup/internal/logger"
	storagepkg "github.com/lupppig/dbackup/internal/storage"
	"github.com/spf13/cobra"
)

var (
	migrateFrom string
	migrateTo   string
)

var migrateCmd = &cobra.Command{
	Use:   "migrate",
	Short: "Migrate backups between storage backends",
	Long: `Migrate all backup sets and manifests from one storage backend to another.
Example: dbackup migrate --from ./local-backups --to s3://my-bucket/backups`,
	RunE: func(cmd *cobra.Command, args []string) error {
		l := logger.FromContext(cmd.Context())

		if migrateFrom == "" || migrateTo == "" {
			return fmt.Errorf("--from and --to are required")
		}

		src, err := storagepkg.FromURI(migrateFrom, storagepkg.StorageOptions{})
		if err != nil {
			return fmt.Errorf("failed to open source storage: %w", err)
		}
		defer src.Close()

		dst, err := storagepkg.FromURI(migrateTo, storagepkg.StorageOptions{})
		if err != nil {
			return fmt.Errorf("failed to open destination storage: %w", err)
		}
		defer dst.Close()

		// If destination should be deduped, wrap it
		// For now we assume the user might want dedupe if they use it normally.
		// Alternatively, we could add a --dedupe flag to migrate.
		if dedupe {
			dst = storagepkg.NewDedupeStorage(dst)
		}

		l.Info("Starting migration", "from", storagepkg.Scrub(migrateFrom), "to", storagepkg.Scrub(migrateTo))

		files, err := src.ListMetadata(cmd.Context(), "")
		if err != nil {
			return fmt.Errorf("failed to list source manifests: %w", err)
		}

		migratedCount := 0
		for _, file := range files {
			if !strings.HasSuffix(file, ".manifest") {
				continue
			}

			l.Info("Migrating backup", "manifest", file)

			data, err := src.GetMetadata(cmd.Context(), file)
			if err != nil {
				l.Warn("Failed to read manifest", "file", file, "error", err)
				continue
			}

			// Open source backup data
			backupName := strings.TrimSuffix(file, ".manifest")
			r, err := src.Open(cmd.Context(), backupName)
			if err != nil {
				// If it's a dedupe storage, src.Open will reassemble it.
				// If it's a regular storage, it will just open the file.
				l.Warn("Failed to open backup data", "file", backupName, "error", err)
				continue
			}

			// Save to destination
			_, err = dst.Save(cmd.Context(), backupName, r)
			r.Close()
			if err != nil {
				return fmt.Errorf("failed to save backup to destination: %w", err)
			}

			// Save manifest to destination
			if err := dst.PutMetadata(cmd.Context(), file, data); err != nil {
				return fmt.Errorf("failed to save manifest to destination: %w", err)
			}

			migratedCount++
		}

		l.Info("Migration finished", "count", migratedCount)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(migrateCmd)
	migrateCmd.Flags().StringVar(&migrateFrom, "from", "", "Source storage URI")
	migrateCmd.Flags().StringVar(&migrateTo, "to", "", "Destination storage URI")
	migrateCmd.Flags().BoolVar(&dedupe, "dedupe", true, "Enable deduplication at destination")
}
