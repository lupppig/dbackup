package cmd

import (
	"context"

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
	SilenceUsage:  true,
	SilenceErrors: true,
	Example: `
	dbackup backup mysql --db mydb
	dbackup backup postgres --db app_db --out ./backups
	dbackup restore postgres --file backup.sql
	dbackup version`,
}

func NewContext(ctx context.Context) (context.Context, context.CancelFunc) {
	return nil, nil
}

var (
	LogJSON bool
	NoColor bool

	config   string
	dbType   string
	host     string
	user     string
	password string
	dbName   string
	port     int
	dbURI    string

	storageType     string
	output          string
	compress        bool
	compressionAlgo string
	fileName        string

	tlsEnabled    bool
	tlsMode       string
	tlsCACert     string
	tlsClientCert string
	tlsClientKey  string

	target     string
	remoteExec bool
	dedupe     bool
)

func init() {
	rootCmd.Version = DBACKUP_VERSION
	rootCmd.SetVersionTemplate("dbackup version {{ .Version }}\n")

	rootCmd.PersistentFlags().BoolVar(&LogJSON, "log-json", false, "output logs in JSON format")
	rootCmd.PersistentFlags().BoolVar(&NoColor, "no-color", false, "disable colored terminal output")

	// Register subcommands
}

func Execute() error {
	return rootCmd.Execute()
}
