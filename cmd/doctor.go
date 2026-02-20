package cmd

import (
	"fmt"
	"os/exec"
	"runtime"

	"time"

	"github.com/lupppig/dbackup/internal/config"
	"github.com/lupppig/dbackup/internal/logger"
	"github.com/lupppig/dbackup/internal/storage"
	"github.com/spf13/cobra"
)

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Check if required native binaries are installed",
	Long:  `Verify that all native tools required for backup and restore operations are present in your system PATH.`,
	Run: func(cmd *cobra.Command, args []string) {
		l := logger.FromContext(cmd.Context())
		l.Info("dbackup doctor - System Environment Check", "os", runtime.GOOS, "arch", runtime.GOARCH)

		groups := []struct {
			name     string
			binaries []string
		}{
			{"Global & Core", []string{"docker", "ssh", "scp", "tar"}},
			{"PostgreSQL", []string{"psql", "pg_dump"}},
			{"MySQL", []string{"mysql", "mysqldump", "xtrabackup"}},
		}

		allOk := true
		for _, group := range groups {
			fmt.Printf("[%s]\n", group.name)
			for _, bin := range group.binaries {
				path, err := exec.LookPath(bin)
				if err != nil {
					fmt.Printf("  [ ] %-12s: NOT FOUND\n", bin)
					allOk = false
				} else {
					fmt.Printf("  [x] %-12s: %s\n", bin, path)
				}
			}
			fmt.Println()
		}

		if allOk {
			fmt.Println("Result: All systems go! Your environment is ready for dbackup.")
		} else {
			fmt.Println("Result: Some dependencies are missing. Please install the required tools for your database engine.")
		}

		// Live Target Checks
		cfg := config.GetConfig()
		targets := make(map[string]bool)
		for _, b := range cfg.Backups {
			if b.To != "" {
				targets[b.To] = true
			}
		}
		for _, r := range cfg.Restores {
			if r.From != "" {
				targets[r.From] = true
			}
		}

		if len(targets) > 0 {
			fmt.Println("\n[Storage Target Checks]")
			for target := range targets {
				scrubbed := storage.Scrub(target)
				fmt.Printf("  Checking %s...\n", scrubbed)

				start := time.Now()
				s, err := storage.FromURI(target, storage.StorageOptions{AllowInsecure: cfg.AllowInsecure})
				if err != nil {
					fmt.Printf("    [ ] Connection: FAILED (%v)\n", err)
					continue
				}

				// Attempt a tiny metadata write/read for permission check
				err = s.PutMetadata(cmd.Context(), ".doctor_check", []byte("ok"))
				latency := time.Since(start)

				if err != nil {
					fmt.Printf("    [ ] Permissions: FAILED (Write failed: %v)\n", err)
				} else {
					fmt.Printf("    [x] Latency: %s\n", latency.Truncate(time.Millisecond))
					fmt.Printf("    [x] Permissions: READ/WRITE OK\n")
					_ = s.Delete(cmd.Context(), ".doctor_check")
				}
				s.Close()
			}
		}
	},
}

func init() {
	rootCmd.AddCommand(doctorCmd)
}
