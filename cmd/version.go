package cmd

import (
	"fmt"
	"runtime"

	"github.com/spf13/cobra"
)

var (
	Version   = "dev"
	Commit    = "none"
	BuildDate = "unknown"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the dbackup version",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("dbackup version %s\n", Version)
		fmt.Printf("commit: %s\n", Commit)
		fmt.Printf("built at: %s\n", BuildDate)
		fmt.Printf("go version: %s %s/%s\n", runtime.Version(), runtime.GOOS, runtime.GOARCH)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
