package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/lupppig/dbackup/internal/storage"
	"github.com/spf13/cobra"
)

var verifyCmd = &cobra.Command{
	Use:   "verify",
	Short: "Verify backup integrity by checking if all chunks exist",
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
			fmt.Println("Verify is currently only supported for deduplicated storage targets.")
			return nil
		}

		fmt.Printf("Verifying integrity for %s...\n", target)
		missing, err := ds.Verify(context.Background())
		if err != nil {
			return fmt.Errorf("verify failed: %w", err)
		}

		if len(missing) == 0 {
			fmt.Println("✅ Integrity check passed. All chunks are present.")
		} else {
			fmt.Printf("❌ Integrity check failed! %d missing chunks detected:\n", len(missing))
			for i, c := range missing {
				fmt.Printf("  - %s\n", c)
				if i >= 9 {
					fmt.Printf("  ... and %d more\n", len(missing)-10)
					break
				}
			}
			os.Exit(1)
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(verifyCmd)
	verifyCmd.Flags().String("to", "", "Storage target (e.g. dedupe://local://./backups)")
}
