package cmd

import (
	"context"
	"fmt"

	"github.com/lupppig/dbackup/internal/logger"
	"github.com/lupppig/dbackup/internal/storage"
	"github.com/spf13/cobra"
)

var gcCmd = &cobra.Command{
	Use:   "gc",
	Short: "Collect and remove orphaned chunks from deduplicated storage",
	RunE: func(cmd *cobra.Command, args []string) error {
		target, _ := cmd.Flags().GetString("to")
		allowInsecure, _ := cmd.Flags().GetBool("allow-insecure")

		s, err := storage.FromURI(target, storage.StorageOptions{AllowInsecure: allowInsecure})
		if err != nil {
			return err
		}
		defer s.Close()

		ds, ok := s.(*storage.DedupeStorage)
		l := logger.FromContext(cmd.Context())
		if !ok {
			l.Info("GC is currently only supported for deduplicated storage targets.")
			return nil
		}

		l.Info("Running garbage collection...", "target", target)
		count, err := ds.GC(context.Background())
		if err != nil {
			return fmt.Errorf("GC failed: %w", err)
		}

		l.Info("Garbage collection complete", "removed_chunks", count)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(gcCmd)
	gcCmd.Flags().String("to", "", "Storage target (e.g. dedupe://local://./backups)")
}
