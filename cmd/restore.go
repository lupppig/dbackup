package cmd

import (
	"fmt"
	"strings"
	"time"

	"github.com/lupppig/dbackup/internal/backup"
	database "github.com/lupppig/dbackup/internal/db"
	"github.com/lupppig/dbackup/internal/logger"
	"github.com/lupppig/dbackup/internal/storage"
	"github.com/spf13/cobra"
)

var restoreCmd = &cobra.Command{
	Use:   "restore",
	Short: "Restore a database backup",
	Long: `Restore a previously created backup to the specified database.
	
This command retrieves the backup from the specified storage, decompresses it if necessary,
and streams it directly into the database engine.`,
	SilenceUsage:  true,
	SilenceErrors: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		l := logger.New(logger.Config{
			JSON:    LogJSON,
			NoColor: NoColor,
		})

		if fileName == "" {
			return fmt.Errorf("--name is required for restore")
		}

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

		connParams := database.ConnectionParams{
			DBType:   dbType,
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

		if target == "" {
			if output != "" {
				target = output
			} else {
				target = "."
			}
		}

		mgr, err := backup.NewRestoreManager(backup.BackupOptions{
			DBType:     dbType,
			DBName:     dbName,
			StorageURI: target,
			Compress:   compress,
			Algorithm:  compressionAlgo,
			FileName:   fileName,
			Logger:     l,
		})
		if err != nil {
			return err
		}

		if !cmd.Flags().Changed("dedupe") {
			dedupe = true // Default to true
		}

		if dedupe {
			mgr.SetStorage(storage.NewDedupeStorage(mgr.GetStorage()))
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

		l.Info("Restore started", "engine", dbType, "database", dbName, "file", fileName)
		start := time.Now()

		if err := mgr.Run(cmd.Context(), adapter, connParams); err != nil {
			l.Error("Restore failed", "error", err)
			return err
		}

		l.Info("Restore finished",
			"database", dbName,
			"duration", time.Since(start).String(),
		)

		return nil
	},
}

func init() {
	rootCmd.AddCommand(restoreCmd)

	restoreCmd.Flags().StringVar(&dbType, "db", "", "database engine (postgres, mysql, sqlite)")
	restoreCmd.Flags().StringVar(&host, "host", "", "database host")
	restoreCmd.Flags().StringVar(&user, "user", "", "database username")
	restoreCmd.Flags().StringVar(&password, "password", "", "database password")
	restoreCmd.Flags().StringVar(&dbName, "dbname", "", "database name")
	restoreCmd.Flags().IntVar(&port, "port", 0, "database port")

	restoreCmd.Flags().StringVar(&dbURI, "db-uri", "", "Full database connection URI (overrides individual flags)")
	restoreCmd.Flags().BoolVar(&dedupe, "dedupe", true, "Enable storage-level deduplication (CAS, default true)")

	restoreCmd.Flags().StringVar(&storageType, "storage", "", "storage target (local, etc.)")
	restoreCmd.Flags().StringVar(&output, "out", "", "local directory for backup files")
	restoreCmd.Flags().BoolVar(&compress, "compress", true, "decompress the backup (default true)")
	restoreCmd.Flags().StringVar(&compressionAlgo, "compression-algo", "lz4", "compression algorithm used for the backup")
	restoreCmd.Flags().StringVar(&fileName, "name", "", "backup file name to restore")

	restoreCmd.Flags().BoolVar(&tlsEnabled, "tls", false, "enable TLS/SSL for database connection")
	restoreCmd.Flags().StringVar(&tlsMode, "tls-mode", "disable", "TLS mode")
	restoreCmd.Flags().StringVar(&tlsCACert, "tls-ca-cert", "", "path to CA certificate")
	restoreCmd.Flags().StringVar(&tlsClientCert, "tls-client-cert", "", "path to client certificate")
	restoreCmd.Flags().StringVar(&tlsClientKey, "tls-client-key", "", "path to client private key")
	restoreCmd.Flags().BoolVar(&remoteExec, "remote-exec", false, "execute restore tools on the remote storage host (bypasses pg_hba.conf)")
}
