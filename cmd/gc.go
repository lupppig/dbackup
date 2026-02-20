package cmd

import (
	"context"
	"fmt"

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
		if !ok {
			fmt.Println("GC is currently only supported for deduplicated storage targets.")
			return nil
		}

		fmt.Printf("Running garbage collection for %s...\n", target)
		count, err := ds.GC(context.Background())
		if err != nil {
			return fmt.Errorf("GC failed: %w", err)
		}

		fmt.Printf("✅ Garbage collection complete. Removed %d orphaned chunks.\n", count)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(gcCmd)
	gcCmd.Flags().String("to", "", "Storage target (e.g. dedupe://local://./backups)")
}
