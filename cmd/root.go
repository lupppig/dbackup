package cmd

import "github.com/spf13/cobra"

const DBACKUP_VERSION = "0.1.0"

var rootCmd = &cobra.Command{
	Use:   "dbackup",
	Short: "dbackup is a database backup cli that helps backup data in your database locally or a remote server",
	Long: `dbackup is a command-line tool designed to simplify database backups for developers and teams.
	It supports backing up multiple database types and allows backups to be stored locally or pushed to a remote server. dbackup focuses on reliability, automation, and ease of use, making it suitable for both small projects and production environments.
	With dbackup, you can schedule backups, compress backup files, manage storage locations, and track backup activity through clear logs—all from a single CLI.`,
	Run: func(cmd *cobra.Command, args []string) {
		cmd.Help()
	},
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		return nil
	},
}

func init() {
	rootCmd.Version = DBACKUP_VERSION
	rootCmd.SetVersionTemplate("dbackup version {{ .Version }}\n")
}

func Execute() error {
	return rootCmd.Execute()
}
