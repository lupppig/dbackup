package cmd

import (
	"context"
	"os"

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
		if encryptionPassphrase == "" {
			encryptionPassphrase = os.Getenv("DBACKUP_KEY")
		}
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

	compress        bool
	compressionAlgo string
	fileName        string

	tlsEnabled    bool
	tlsMode       string
	tlsCACert     string
	tlsClientCert string
	tlsClientKey  string

	target     string
	from       string
	remoteExec bool
	dedupe     bool

	SlackWebhook         string
	Parallelism          int
	AllowInsecure        bool
	encrypt              bool
	encryptionKeyFile    string
	encryptionPassphrase string
	confirmRestore       bool

	retention string
	keep      int
)

func init() {
	rootCmd.Version = DBACKUP_VERSION
	rootCmd.SetVersionTemplate("dbackup version {{ .Version }}\n")

	rootCmd.PersistentFlags().BoolVar(&LogJSON, "log-json", false, "output logs in JSON format")
	rootCmd.PersistentFlags().BoolVar(&NoColor, "no-color", false, "disable colored terminal output")
	rootCmd.PersistentFlags().StringVar(&SlackWebhook, "slack-webhook", "", "Slack Incoming Webhook URL for notifications")
	rootCmd.PersistentFlags().IntVar(&Parallelism, "parallelism", 4, "Number of databases to back up/restore simultaneously")
	rootCmd.PersistentFlags().BoolVar(&AllowInsecure, "allow-insecure", false, "Allow insecure protocols (like plain FTP)")
	rootCmd.PersistentFlags().BoolVar(&encrypt, "encrypt", false, "Enable client-side encryption (AES-256-GCM)")
	rootCmd.PersistentFlags().StringVar(&encryptionKeyFile, "encryption-key-file", "", "Path to the encryption key file")
	rootCmd.PersistentFlags().StringVar(&encryptionPassphrase, "encryption-passphrase", "", "Passphrase for encryption key derivation")
	rootCmd.PersistentFlags().BoolVar(&confirmRestore, "confirm-restore", false, "Confirm destructive restore operations")

	// Core database flags
	rootCmd.PersistentFlags().StringVarP(&dbType, "engine", "e", "", "database engine (postgres, mysql, sqlite)")
	rootCmd.PersistentFlags().StringVarP(&dbName, "db", "d", "", "database name or file path")
	rootCmd.PersistentFlags().StringVar(&host, "host", "", "database host")
	rootCmd.PersistentFlags().StringVar(&user, "user", "", "database username")
	rootCmd.PersistentFlags().StringVar(&password, "password", "", "database password")
	rootCmd.PersistentFlags().IntVar(&port, "port", 0, "database port")
	rootCmd.PersistentFlags().StringVar(&dbURI, "db-uri", "", "full database connection URI (overrides individual flags)")
	rootCmd.PersistentFlags().StringVarP(&target, "to", "t", "", "unified targeting URI (e.g. ./local/path, sftp://user@host/path)")
	rootCmd.PersistentFlags().BoolVar(&remoteExec, "remote-exec", false, "execute backup/restore tools on the remote storage host")
	rootCmd.PersistentFlags().BoolVar(&dedupe, "dedupe", true, "Enable storage-level deduplication (CAS, default true)")

	rootCmd.PersistentFlags().BoolVar(&tlsEnabled, "tls", false, "enable TLS/SSL for database connection")
	rootCmd.PersistentFlags().StringVar(&tlsMode, "tls-mode", "disable", "TLS mode (disable, require, verify-ca, verify-full)")
	rootCmd.PersistentFlags().StringVar(&tlsCACert, "tls-ca-cert", "", "path to CA certificate for TLS verification")
	rootCmd.PersistentFlags().StringVar(&tlsClientCert, "tls-client-cert", "", "path to client certificate for mutual TLS (mTLS)")
	rootCmd.PersistentFlags().StringVar(&tlsClientKey, "tls-client-key", "", "path to client private key for mutual TLS (mTLS)")
}

func Execute() error {
	return rootCmd.Execute()
}
