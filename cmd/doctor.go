package cmd

import (
	"fmt"
	"os/exec"
	"runtime"

	"github.com/spf13/cobra"
)

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Check if required native binaries are installed",
	Long:  `Verify that all native tools required for backup and restore operations are present in your system PATH.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("dbackup doctor - System Environment Check\n")
		fmt.Printf("OS: %s, Architecture: %s\n\n", runtime.GOOS, runtime.GOARCH)

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
	},
}

func init() {
	rootCmd.AddCommand(doctorCmd)
}
