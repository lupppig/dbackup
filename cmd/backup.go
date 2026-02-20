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

var mysqlPhysical bool
var keepDaily, keepWeekly, keepMonthly, keepYearly int

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
		l := logger.FromContext(cmd.Context())

		var uris []string
		if len(args) > 0 {
			// Check if first arg is an engine type
			firstArg := strings.ToLower(args[0])
			isEngine := false
			switch firstArg {
			case "postgres", "postgresql", "mysql", "sqlite":
				isEngine = true
			}

			if isEngine {
				dbType = firstArg
				uris = args[1:]
			} else {
				uris = args
			}
		} else if dbURI != "" {
			uris = []string{dbURI}
		}

		if len(uris) == 0 && dbName == "" {
			return fmt.Errorf("database name or URI is required")
		}

		if dbType == "" {
			return fmt.Errorf("database engine is required (e.g. backup sqlite ...)")
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
				IsPhysical: mysqlPhysical,
			}
			return doBackup(cmd, l, connParams, notifier)
		}
		var wg sync.WaitGroup
		sem := make(chan struct{}, Parallelism)
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
					DBType:   dbType,
					Host:     host,
					Port:     port,
					User:     user,
					Password: password,
					DBName:   dbName,
					DBUri:    u,
					TLS: database.TLSConfig{
						Enabled:    tlsEnabled,
						Mode:       tlsMode,
						CACert:     tlsCACert,
						ClientCert: tlsClientCert,
						ClientKey:  tlsClientKey,
					},
					IsPhysical: mysqlPhysical,
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
		DBType:               connParams.DBType,
		DBName:               connParams.DBName,
		StorageURI:           target,
		Compress:             compress,
		Algorithm:            compressionAlgo,
		FileName:             fileName,
		RemoteExec:           remoteExec,
		AllowInsecure:        AllowInsecure,
		Encrypt:              encrypt,
		EncryptionKeyFile:    encryptionKeyFile,
		EncryptionPassphrase: encryptionPassphrase,
		Retention:            parseRetention(retention),
		Keep:                 keep,
		RetentionPolicy: backup.RetentionPolicy{
			KeepDaily:   keepDaily,
			KeepWeekly:  keepWeekly,
			KeepMonthly: keepMonthly,
			KeepYearly:  keepYearly,
		},
		Logger:   l,
		Notifier: notifier,
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

	backupCmd.Flags().BoolVar(&compress, "compress", true, "compress backup output (default true)")
	backupCmd.Flags().StringVar(&compressionAlgo, "compression-algo", "lz4", "compression algorithm (gzip, zstd, lz4, none, defaults to lz4). All are wrapped in a tar archive unless 'none' is specified.")
	backupCmd.Flags().StringVar(&fileName, "name", "", "custom backup file name")
	backupCmd.Flags().StringVar(&retention, "retention", "", "retention period (e.g. 7d, 24h)")
	backupCmd.Flags().IntVar(&keep, "keep", 0, "number of backups to keep")
	backupCmd.Flags().BoolVar(&mysqlPhysical, "mysql-physical", false, "use physical backup mode for MySQL (default false/logical)")
	backupCmd.Flags().IntVar(&keepDaily, "keep-daily", 0, "number of daily backups to keep")
	backupCmd.Flags().IntVar(&keepWeekly, "keep-weekly", 0, "number of weekly backups to keep")
	backupCmd.Flags().IntVar(&keepMonthly, "keep-monthly", 0, "number of monthly backups to keep")
	backupCmd.Flags().IntVar(&keepYearly, "keep-yearly", 0, "number of yearly backups to keep")
}

func parseRetention(s string) time.Duration {
	if s == "" {
		return 0
	}
	dur, _ := time.ParseDuration(s)
	if strings.HasSuffix(s, "d") {
		days := strings.TrimSuffix(s, "d")
		var d int
		fmt.Sscanf(days, "%d", &d)
		dur = time.Duration(d) * 24 * time.Hour
	}
	return dur
}
