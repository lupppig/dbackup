package cmd

import (
	"fmt"
	"strings"
	"time"

	"sync"

	"github.com/lupppig/dbackup/internal/backup"
	database "github.com/lupppig/dbackup/internal/db"
	"github.com/lupppig/dbackup/internal/logger"
	"github.com/lupppig/dbackup/internal/manifest"
	"github.com/lupppig/dbackup/internal/notify"
	"github.com/lupppig/dbackup/internal/storage"
	"github.com/spf13/cobra"
)

var (
	restoreAuto   bool
	restoreDryRun bool
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
		l := logger.FromContext(cmd.Context())

		if from != "" {
			target = from
		}

		if target == "" {
			target = "."
		}

		var notifier notify.Notifier
		if SlackWebhook != "" {
			notifier = notify.NewSlackNotifier(SlackWebhook)
		}

		// Handle positional engine for restore
		if len(args) > 0 {
			firstArg := strings.ToLower(args[0])
			switch firstArg {
			case "postgres", "postgresql", "mysql", "sqlite":
				dbType = firstArg
				args = args[1:]
			}
		}

		if restoreAuto || (len(args) == 0 && fileName == "") {
			if len(args) > 0 {
				return fmt.Errorf("extra arguments provided with auto-restore: %v", args)
			}
			msg := "Scanning for latest backups to restore..."
			if dbType != "" {
				msg = fmt.Sprintf("Scanning for latest %s backups to restore...", dbType)
			}
			l.Info(msg, "target", target)

			s, err := storage.FromURI(target, storage.StorageOptions{AllowInsecure: AllowInsecure})
			if err != nil {
				return err
			}

			if dedupe {
				s = storage.NewDedupeStorage(s)
			}

			files, err := s.ListMetadata(cmd.Context(), "")
			if err != nil {
				return fmt.Errorf("failed to list manifests: %w", err)
			}

			latestBackups := make(map[string]*struct {
				Manifest *manifest.Manifest
				Path     string
			})

			for _, file := range files {
				if !strings.HasSuffix(file, ".manifest") {
					continue
				}

				data, err := s.GetMetadata(cmd.Context(), file)
				if err != nil {
					l.Warn("Failed to read manifest", "file", file, "error", err)
					continue
				}

				m, err := manifest.Deserialize(data)
				if err != nil {
					l.Warn("Failed to parse manifest", "file", file, "error", err)
					continue
				}

				// Engine Filter
				if dbType != "" && !strings.EqualFold(m.Engine, dbType) {
					continue
				}

				key := fmt.Sprintf("%s:%s", m.Engine, m.DBName)
				if current, ok := latestBackups[key]; !ok || m.CreatedAt.After(current.Manifest.CreatedAt) {
					latestBackups[key] = &struct {
						Manifest *manifest.Manifest
						Path     string
					}{m, file}
				}
			}

			if len(latestBackups) == 0 {
				l.Info("No applicable manifests found in storage")
				return nil
			}

			l.Info(fmt.Sprintf("Found %d unique database(s) to restore", len(latestBackups)))

			var wg sync.WaitGroup
			sem := make(chan struct{}, Parallelism)
			errChan := make(chan string, len(latestBackups))

			for key, lb := range latestBackups {
				l.Info("Queueing restore", "db", key, "manifest", lb.Path)
				wg.Add(1)
				go func(mName string, m *manifest.Manifest) {
					defer wg.Done()
					sem <- struct{}{}
					defer func() { <-sem }()

					subL := l.With("db", m.DBName, "engine", m.Engine)
					connParams := database.ConnectionParams{
						DBType:   m.Engine,
						DBName:   m.DBName,
						Host:     host,
						Port:     port,
						User:     user,
						Password: password,
						TLS: database.TLSConfig{
							Enabled:    tlsEnabled,
							Mode:       tlsMode,
							CACert:     tlsCACert,
							ClientCert: tlsClientCert,
							ClientKey:  tlsClientKey,
						},
						IsPhysical: mysqlPhysical,
					}

					if err := doRestore(cmd, subL, connParams, mName, notifier); err != nil {
						subL.Error("Auto restore failed", "error", err)
						errChan <- fmt.Sprintf("%s (%s): %v", m.DBName, m.Engine, err)
					}
				}(lb.Path, lb.Manifest)
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
				IsPhysical: mysqlPhysical,
			}
			return doRestore(cmd, l, connParams, fileName, notifier)
		}

		// Otherwise loop over args: manifest[:db-uri] concurrently
		var wg sync.WaitGroup
		sem := make(chan struct{}, Parallelism)
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
					DBType:   dbType,
					Host:     host,
					Port:     port,
					User:     user,
					Password: password,
					DBName:   dbName,
					DBUri:    mURI,
					TLS: database.TLSConfig{
						Enabled:    tlsEnabled,
						Mode:       tlsMode,
						CACert:     tlsCACert,
						ClientCert: tlsClientCert,
						ClientKey:  tlsClientKey,
					},
					IsPhysical: mysqlPhysical,
				}

				if mURI == "" && dbURI != "" {
					connParams.DBUri = dbURI
				}

				if connParams.DBType == "" && dbType != "" {
					connParams.DBType = dbType
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
		DBType:               connParams.DBType,
		DBName:               connParams.DBName,
		StorageURI:           target,
		Compress:             true,  // Default to true during restore
		Algorithm:            "lz4", // Default to lz4
		FileName:             mName,
		AllowInsecure:        AllowInsecure,
		Encrypt:              encrypt,
		EncryptionKeyFile:    encryptionKeyFile,
		EncryptionPassphrase: encryptionPassphrase,
		ConfirmRestore:       confirmRestore,
		DryRun:               restoreDryRun,
		Logger:               l,
		Notifier:             notifier,
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

	if restoreDryRun {
		runner = database.NewDryRunRunner(l)
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

	restoreCmd.Flags().StringVar(&fileName, "name", "", "backup file name to restore")
	restoreCmd.Flags().StringVarP(&from, "from", "f", "", "unified source URI for restore (alias for --to)")
	restoreCmd.Flags().BoolVarP(&restoreAuto, "auto", "a", false, "automatically restore latest backups (default if no manifest is specified)")
	restoreCmd.Flags().BoolVar(&restoreDryRun, "dry-run", false, "simulation mode (don't actually run restore)")
	restoreCmd.Flags().BoolVar(&mysqlPhysical, "mysql-physical", false, "use physical backup mode for MySQL restores")
}
