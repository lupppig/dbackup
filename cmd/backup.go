package cmd

import (
	"fmt"
	"strings"

	database "github.com/lupppig/dbackup/internal/db"
	"github.com/spf13/cobra"
)

var (
	config   string
	dbType   string
	host     string
	user     string
	password string
	dbName   string
	port     int
	dbURI    string

	storage  string
	output   string
	compress bool

	tlsEnabled    bool
	tlsMode       string
	tlsCACert     string
	tlsClientCert string
	tlsClientKey  string
)

var backupCmd = &cobra.Command{
	Use:   "backup",
	Short: "Create a database backup",
	Long: `Create a backup of the specified database and store it locally or on a remote server.

The backup command supports multiple database engines and allows you to configure
output location, compression, and secure (TLS/SSL) connections. If the backup
process fails, dbackup exits with a non-zero status code.`,
	RunE: func(cmd *cobra.Command, args []string) error {

		if dbURI != "" {
			if host != "" || user != "" || password != "" || dbName != "" {
				return fmt.Errorf(
					"--db-uri cannot be used together with --host, --user, --password, or --dbname",
				)
			}
		} else {
			if dbType == "" {
				return fmt.Errorf("--db is required")
			}
			if dbType != "sqlite" {
				if host == "" || user == "" || password == "" || dbName == "" {
					return fmt.Errorf(
						"--host, --user, --password, and --dbname are required unless --db-uri is provided",
					)
				}
			}
		}

		if tlsEnabled && tlsMode == "disable" {
			return fmt.Errorf("--tls is enabled but --tls-mode is set to disable")
		}

		if tlsClientCert != "" && tlsClientKey == "" {
			return fmt.Errorf("--tls-client-key is required when --tls-client-cert is provided")
		}

		connParams := database.ConnectionParams{
			DBtype:   dbType,
			Host:     host,
			User:     user,
			Port:     port,
			Password: password,
			DBName:   dbName,
			DBUri:    dbURI,
			TLS: database.TLSConfig{
				Enabled:    tlsEnabled,
				Mode:       tlsMode,
				CACert:     tlsCACert,
				ClientCert: tlsClientCert,
				ClientKey:  tlsClientKey,
			},
		}

		// register adapter
		var adapter database.DBAdapter
		switch strings.ToLower(dbType) {
		case "postgres":
			adapter = database.PostgresAdapter{}
		default:
			return fmt.Errorf("unsupported database type")
		}

		if err := adapter.TestConnection(cmd.Context(), connParams); err != nil {
			return err
		}

		outputPath := output
		if outputPath == "" && storage == "" {
			outputPath = "./" // default directory
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(backupCmd)

	backupCmd.Flags().StringVar(&config, "config", "", "path to configuration file")

	backupCmd.Flags().StringVar(&dbType, "db", "", "database engine (postgres, mysql, sqlite, mongodb)")
	backupCmd.Flags().StringVar(&host, "host", "", "database host")
	backupCmd.Flags().StringVar(&user, "user", "", "database username")
	backupCmd.Flags().StringVar(&password, "password", "", "database password")
	backupCmd.Flags().StringVar(&dbName, "dbname", "", "database name")
	backupCmd.Flags().IntVar(&port, "port", 0, "database ports to be provided")

	backupCmd.Flags().StringVar(&dbURI, "db-uri", "", "full database connection URI (overrides individual connection flags)")

	backupCmd.Flags().StringVar(&storage, "storage", "", "remote storage target (if omitted, backup is stored locally)")
	backupCmd.Flags().StringVar(&output, "out", "", "local output path for backup file (defaults to current directory)")
	backupCmd.Flags().BoolVar(&compress, "compress", false, "compress backup output")

	// TLS flags
	backupCmd.Flags().BoolVar(&tlsEnabled, "tls", false, "enable TLS/SSL for database connection")
	backupCmd.Flags().StringVar(&tlsMode, "tls-mode", "disable", "TLS mode (disable, require, verify-ca, verify-full)")
	backupCmd.Flags().StringVar(&tlsCACert, "tls-ca-cert", "", "path to CA certificate for TLS verification")
	backupCmd.Flags().StringVar(&tlsClientCert, "tls-client-cert", "", "path to client certificate for mutual TLS (mTLS)")
	backupCmd.Flags().StringVar(&tlsClientKey, "tls-client-key", "", "path to client private key for mutual TLS (mTLS)")
}
