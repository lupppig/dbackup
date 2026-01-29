package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/lupppig/dbackup/internal/backup"
	database "github.com/lupppig/dbackup/internal/db"
	"github.com/lupppig/dbackup/internal/logger"
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
			return fmt.Errorf("--f (filename) is required for restore")
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

		if stateDir == "" {
			home, err := os.UserHomeDir()
			if err != nil {
				return fmt.Errorf("failed to get user home directory: %w", err)
			}
			stateDir = filepath.Join(home, ".dbackup")
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
			BackupType: backupType,
			StateDir:   stateDir,
		}

		mgr, err := backup.NewRestoreManager(backup.BackupOptions{
			DBType:     dbType,
			DBName:     dbName,
			Storage:    storage,
			Compress:   compress,
			Algorithm:  compressionAlgo,
			FileName:   fileName,
			BackupType: backupType,
			OutputDir:  output,
			Logger:     l,
		})
		if err != nil {
			return err
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

	restoreCmd.Flags().StringVar(&dbURI, "db-uri", "", "full database connection URI")

	restoreCmd.Flags().StringVar(&storage, "storage", "", "storage target (local, etc.)")
	restoreCmd.Flags().StringVar(&output, "out", "", "local directory for backup files")
	restoreCmd.Flags().BoolVar(&compress, "compress", true, "decompress the backup (default true)")
	restoreCmd.Flags().StringVar(&compressionAlgo, "compression-algo", "lz4", "compression algorithm used for the backup")
	restoreCmd.Flags().StringVar(&fileName, "f", "", "backup file name to restore")
	restoreCmd.Flags().StringVar(&backupType, "backup-type", "full", "type of backup being restored")

	restoreCmd.Flags().BoolVar(&tlsEnabled, "tls", false, "enable TLS/SSL for database connection")
	restoreCmd.Flags().StringVar(&tlsMode, "tls-mode", "disable", "TLS mode")
	restoreCmd.Flags().StringVar(&tlsCACert, "tls-ca-cert", "", "path to CA certificate")
	restoreCmd.Flags().StringVar(&tlsClientCert, "tls-client-cert", "", "path to client certificate")
	restoreCmd.Flags().StringVar(&tlsClientKey, "tls-client-key", "", "path to client private key")
	restoreCmd.Flags().StringVar(&stateDir, "state-dir", "", "directory to store state")
}
