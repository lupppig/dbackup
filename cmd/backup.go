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

		var uris []string
		if len(args) > 0 {
			uris = args
		} else if dbURI != "" {
			uris = []string{dbURI}
		}

		if len(uris) == 0 && dbType == "" {
			return fmt.Errorf("at least one database URI or --db flag is required")
		}

		var notifier notify.Notifier
		if SlackWebhook != "" {
			notifier = notify.NewSlackNotifier(SlackWebhook)
		}

		if target == "" {
			target = "."
		}

		if len(uris) == 0 {
			connParams := database.ConnectionParams{
				DBType:   dbType,
				Host:     host,
				Port:     port,
				User:     user,
				Password: password,
				DBName:   dbName,
				TLS: database.TLSConfig{
					Enabled:    tlsEnabled,
					Mode:       tlsMode,
					CACert:     tlsCACert,
					ClientCert: tlsClientCert,
					ClientKey:  tlsClientKey,
				},
			}
			return doBackup(cmd, l, connParams, notifier)
		}

		// Otherwise, loop over URIs concurrently
		var wg sync.WaitGroup
		sem := make(chan struct{}, Concurrency)
		errChan := make(chan string, len(uris))

		for _, uri := range uris {
			wg.Add(1)
			go func(u string) {
				defer wg.Done()
				sem <- struct{}{}
				defer func() { <-sem }()

				// Create a sub-logger for this database to avoid mixed logs
				subL := l.With("uri", storagepkg.Scrub(u))

				connParams := database.ConnectionParams{
					DBUri: u,
					TLS: database.TLSConfig{
						Enabled:    tlsEnabled,
						Mode:       tlsMode,
						CACert:     tlsCACert,
						ClientCert: tlsClientCert,
						ClientKey:  tlsClientKey,
					},
				}
				if err := doBackup(cmd, subL, connParams, notifier); err != nil {
					subL.Error("Backup failed", "error", err)
					errChan <- fmt.Sprintf("%s: %v", u, err)
				}
			}(uri)
		}

		wg.Wait()
		close(errChan)

		errors := []string{}
		for err := range errChan {
			errors = append(errors, err)
		}

		if len(errors) > 0 {
			return fmt.Errorf("some backups failed:\n%s", strings.Join(errors, "\n"))
		}

		return nil
	},
}

func doBackup(cmd *cobra.Command, l *logger.Logger, connParams database.ConnectionParams, notifier notify.Notifier) error {
	if err := connParams.ParseURI(); err != nil {
		return fmt.Errorf("failed to parse URI: %w", err)
	}

	if connParams.DBType == "" {
		return fmt.Errorf("database type could not be determined for %s", connParams.DBUri)
	}

	mgr, err := backup.NewBackupManager(backup.BackupOptions{
		DBType:     connParams.DBType,
		DBName:     connParams.DBName,
		StorageURI: target,
		Compress:   compress,
		Algorithm:  compressionAlgo,
		FileName:   fileName,
		RemoteExec: remoteExec,
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
		mgr.SetStorage(storagepkg.NewDedupeStorage(mgr.GetStorage()))
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

	l.Info("Backup started", "engine", connParams.DBType, "database", connParams.DBName, "target", storagepkg.Scrub(target), "dedupe", dedupe)
	start := time.Now()

	if err := mgr.Run(cmd.Context(), adapter, connParams); err != nil {
		return err
	}

	l.Info("Backup finished",
		"database", connParams.DBName,
		"duration", time.Since(start).String(),
	)

	return nil
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
