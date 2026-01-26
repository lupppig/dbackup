package cmd

import (
	"github.com/spf13/cobra"
)

const DBACKUP_VERSION = "0.1.0"

var rootCmd = &cobra.Command{
	Use:   "dbackup [OPTIONS] COMMAND",
	Short: "Database backup CLI for local and remote storage",
	Long: `dbackup is a command-line tool designed to simplify database backups for developers and teams.
	It supports backing up multiple database types and allows backups to be stored locally or pushed to a remote server.
	dbackup focuses on reliability, automation, and ease of use, making it suitable for both small projects and production environments.
	With dbackup, you can schedule backups, compress backup files, manage storage locations, and track backup activity through clear logs—all from a single CLI.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return cmd.Help()
	},
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		return nil
	},
	Example: `
	dbackup backup mysql --db mydb
	dbackup backup postgres --db app_db --out ./backups
	dbackup restore postgres --file backup.sql
	dbackup version`,
}

func init() {
	rootCmd.Version = DBACKUP_VERSION
	rootCmd.SetVersionTemplate("dbackup version {{ .Version }}\n")
}

func Execute() error {
	return rootCmd.Execute()
}
