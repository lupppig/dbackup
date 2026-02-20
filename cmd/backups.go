package cmd

import (
	"fmt"
	"strings"

	"github.com/lupppig/dbackup/internal/logger"
	"github.com/lupppig/dbackup/internal/manifest"
	"github.com/lupppig/dbackup/internal/storage"
	"github.com/spf13/cobra"
)

var backupsCmd = &cobra.Command{
	Use:   "backups",
	Short: "List available backups in a storage location",
	Long: `List all available backups in the specified storage.
You can filter by engine and database name.`,
	RunE: func(cmd *cobra.Command, args []string) error {

		if from != "" {
			target = from
		}

		if target == "" {
			target = "."
		}

		s, err := storage.FromURI(target, storage.StorageOptions{AllowInsecure: AllowInsecure})
		if err != nil {
			return err
		}

		if dedupe {
			s = storage.NewDedupeStorage(s)
		}

		l := logger.FromContext(cmd.Context())
		l.Info("Scanning storage for backups...", "location", target)

		files, err := s.ListMetadata(cmd.Context(), "")
		if err != nil {
			return fmt.Errorf("failed to list manifests: %w", err)
		}

		count := 0
		fmt.Printf("\n%-30s %-10s %-15s %-10s %-10s\n", "CREATED AT", "ENGINE", "DATABASE", "SIZE", "FILE")
		fmt.Println(strings.Repeat("-", 85))

		for _, file := range files {
			if !strings.HasSuffix(file, ".manifest") {
				continue
			}

			data, err := s.GetMetadata(cmd.Context(), file)
			if err != nil {
				continue
			}

			m, err := manifest.Deserialize(data)
			if err != nil {
				continue
			}

			// Filter by engine if provided
			if dbType != "" && !strings.EqualFold(m.Engine, dbType) {
				continue
			}

			// Filter by database name if provided
			if dbName != "" && !strings.EqualFold(m.DBName, dbName) {
				continue
			}

			sizeStr := fmt.Sprintf("%.2f MB", float64(m.Size)/(1024*1024))
			if m.Size < 1024*1024 {
				sizeStr = fmt.Sprintf("%.2f KB", float64(m.Size)/1024)
			}

			fmt.Printf("%-30s %-10s %-15s %-10s %-10s\n",
				m.CreatedAt.Format("2006-01-02 15:04:05"),
				m.Engine,
				m.DBName,
				sizeStr,
				m.FileName,
			)
			count++
		}

		if count == 0 {
			l.Info("No backups found.")
		} else {
			l.Info("Backups listed", "count", count)
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(backupsCmd)
}
