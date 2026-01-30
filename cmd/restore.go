package cmd

import (
	"fmt"
	"strings"
	"time"

	"sync"

	"github.com/lupppig/dbackup/internal/backup"
	database "github.com/lupppig/dbackup/internal/db"
	"github.com/lupppig/dbackup/internal/logger"
	"github.com/lupppig/dbackup/internal/notify"
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

		if target == "" {
			if output != "" {
				target = output
			} else {
				target = "."
			}
		}

		var notifier notify.Notifier
		if SlackWebhook != "" {
			notifier = notify.NewSlackNotifier(SlackWebhook)
		}

		// If no args, use flags
		if len(args) == 0 {
			if fileName == "" {
				return fmt.Errorf("--name is required for restore")
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
			return doRestore(cmd, l, connParams, fileName, notifier)
		}

		// Otherwise loop over args: manifest[:db-uri] concurrently
		var wg sync.WaitGroup
		sem := make(chan struct{}, Concurrency)
		errChan := make(chan string, len(args))

		for _, arg := range args {
			wg.Add(1)
			go func(a string) {
				defer wg.Done()
				sem <- struct{}{}
				defer func() { <-sem }()

				var mName, mURI string
				if strings.Contains(a, ":") && !strings.HasPrefix(a, "/") && !strings.HasPrefix(a, "./") {
					parts := strings.SplitN(a, ":", 2)
					if strings.Contains(parts[1], "://") {
						mName = parts[0]
						mURI = parts[1]
					} else {
						mName = a
					}
				} else {
					mName = a
				}

				// Create sub-logger
				subL := l.With("manifest", mName)
				if mURI != "" {
					subL = subL.With("target", mURI)
				}

				connParams := database.ConnectionParams{
					DBType: dbType,
					DBUri:  mURI,
					TLS: database.TLSConfig{
						Enabled:    tlsEnabled,
						Mode:       tlsMode,
						CACert:     tlsCACert,
						ClientCert: tlsClientCert,
						ClientKey:  tlsClientKey,
					},
				}

				if mURI == "" && dbURI != "" {
					connParams.DBUri = dbURI
				}

				if err := doRestore(cmd, subL, connParams, mName, notifier); err != nil {
					subL.Error("Restore failed", "error", err)
					errChan <- fmt.Sprintf("%s: %v", mName, err)
				}
			}(arg)
		}

		wg.Wait()
		close(errChan)

		errors := []string{}
		for err := range errChan {
			errors = append(errors, err)
		}

		if len(errors) > 0 {
			return fmt.Errorf("some restores failed:\n%s", strings.Join(errors, "\n"))
		}

		return nil
	},
}

func doRestore(cmd *cobra.Command, l *logger.Logger, connParams database.ConnectionParams, mName string, notifier notify.Notifier) error {
	if err := connParams.ParseURI(); err != nil {
		return fmt.Errorf("failed to parse URI: %w", err)
	}

	if connParams.DBType == "" {
		// Try to infer from manifest name? risky. let's require it via flags or URI.
		if dbType != "" {
			connParams.DBType = dbType
		} else {
			return fmt.Errorf("database type could not be determined for manifest %s", mName)
		}
	}

	mgr, err := backup.NewRestoreManager(backup.BackupOptions{
		DBType:     connParams.DBType,
		DBName:     connParams.DBName,
		StorageURI: target,
		Compress:   compress,
		Algorithm:  compressionAlgo,
		FileName:   mName,
		Logger:     l,
		Notifier:   notifier,
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
	switch strings.ToLower(connParams.DBType) {
	case "postgres", "postgresql":
		adapter = &database.PostgresAdapter{}
	case "mysql":
		adapter = &database.MysqlAdapter{}
	case "sqlite":
		adapter = &database.SqliteAdapter{}
	default:
		return fmt.Errorf("unsupported database type: %s", connParams.DBType)
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

	l.Info("Restore started", "engine", connParams.DBType, "database", connParams.DBName, "file", mName)
	start := time.Now()

	if err := mgr.Run(cmd.Context(), adapter, connParams); err != nil {
		return err
	}

	l.Info("Restore finished",
		"database", connParams.DBName,
		"duration", time.Since(start).String(),
	)

	return nil
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
	restoreCmd.Flags().StringVarP(&target, "to", "t", "", "unified targeting URI (e.g. sftp://user@host/path, s3://bucket/path, docker://container/path)")
	restoreCmd.Flags().BoolVar(&remoteExec, "remote-exec", false, "execute restore tools on the remote storage host (bypasses pg_hba.conf)")
}
