package cmd

import (
	"runtime"

	"github.com/lupppig/dbackup/internal/logger"
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
		l := logger.FromContext(cmd.Context())
		l.Info("dbackup",
			"version", Version,
			"commit", Commit,
			"built_at", BuildDate,
			"go_version", runtime.Version(),
			"os", runtime.GOOS,
			"arch", runtime.GOARCH,
		)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
