package cmd

import (
	"fmt"
	"strings"
	"time"

	"github.com/lupppig/dbackup/internal/backup"
	database "github.com/lupppig/dbackup/internal/db"
	"github.com/lupppig/dbackup/internal/logger"
	storagepkg "github.com/lupppig/dbackup/internal/storage"
	"github.com/spf13/cobra"
)

var backupCmd = &cobra.Command{
	Use:   "backup",
	Short: "Create a database backup",
	Long: `Create a backup of the specified database and store it locally or on a remote server.

The backup command supports multiple database engines and allows you to configure
output location, compression, and secure (TLS/SSL) connections. If the backup
process fails, dbackup exits with a non-zero status code.`,
	SilenceUsage:  true,
	SilenceErrors: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		l := logger.New(logger.Config{
			JSON:    LogJSON,
			NoColor: NoColor,
		})

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
			DBType:   dbType,
			Host:     host,
			Port:     port,
			User:     user,
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

		if target == "" {
			if output != "" {
				target = output
			} else {
				target = "."
			}
		}

		mgr, err := backup.NewBackupManager(backup.BackupOptions{
			DBType:     dbType,
			DBName:     dbName,
			Storage:    storageType,
			StorageURI: target,
			Compress:   compress,
			Algorithm:  compressionAlgo,
			FileName:   fileName,
			OutputDir:  output,
			RemoteExec: remoteExec,
			Logger:     l,
		})
		if err != nil {
			return err
		}

		if !cmd.Flags().Changed("dedupe") {
			dedupe = true // Default to true
		}

		if dedupe {
			mgr.SetStorage(storagepkg.NewDedupeStorage(mgr.GetStorage()))
			l.Info("Deduplication (CAS) active")
		}

		var adapter database.DBAdapter
		switch strings.ToLower(dbType) {
		case "postgres":
			adapter = &database.PostgresAdapter{}
		case "mysql":
			adapter = &database.MysqlAdapter{}
		case "sqlite":
			adapter = &database.SqliteAdapter{}
		default:
			return fmt.Errorf("unsupported database type")
		}

		adapter.SetLogger(l)

		var runner database.Runner = &database.LocalRunner{}
		if remoteExec {
			if storageRunner, ok := mgr.GetStorage().(database.Runner); ok {
				runner = storageRunner
			}
		}

		if err := adapter.TestConnection(cmd.Context(), connParams, runner); err != nil {
			return err
		}

		l.Info("Backup started", "engine", dbType, "database", dbName, "target", storagepkg.Scrub(target), "dedupe", dedupe)
		start := time.Now()

		if err := mgr.Run(cmd.Context(), adapter, connParams); err != nil {
			l.Error("Backup failed", "error", err)
			return err
		}

		l.Info("Backup finished",
			"database", dbName,
			"duration", time.Since(start).String(),
		)

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

	backupCmd.Flags().StringVar(&storageType, "storage", "", "remote storage target (if omitted, backup is stored locally)")
	backupCmd.Flags().StringVar(&output, "out", "", "local output path for backup file (defaults to current directory)")
	backupCmd.Flags().BoolVar(&compress, "compress", true, "compress backup output (default true)")
	backupCmd.Flags().StringVar(&compressionAlgo, "compression-algo", "lz4", "compression algorithm (gzip, zstd, lz4, none, defaults to lz4). All are wrapped in a tar archive unless 'none' is specified.")
	backupCmd.Flags().StringVar(&fileName, "name", "", "custom backup file name")

	backupCmd.Flags().BoolVar(&tlsEnabled, "tls", false, "enable TLS/SSL for database connection")
	backupCmd.Flags().StringVar(&tlsMode, "tls-mode", "disable", "TLS mode (disable, require, verify-ca, verify-full)")
	backupCmd.Flags().StringVar(&tlsCACert, "tls-ca-cert", "", "path to CA certificate for TLS verification")
	backupCmd.Flags().StringVar(&tlsClientCert, "tls-client-cert", "", "path to client certificate for mutual TLS (mTLS)")
	backupCmd.Flags().StringVar(&tlsClientKey, "tls-client-key", "", "path to client private key for mutual TLS (mTLS)")

	backupCmd.Flags().StringVarP(&target, "to", "t", "", "unified targeting URI (e.g. sftp://user@host/path, s3://bucket/path, docker://container/path)")
	backupCmd.Flags().BoolVar(&remoteExec, "remote-exec", false, "execute backup tools on the remote storage host (bypasses pg_hba.conf)")
	backupCmd.Flags().BoolVar(&dedupe, "dedupe", true, "Enable storage-level deduplication (CAS, default true)")
}
