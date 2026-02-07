package cmd

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/lupppig/dbackup/internal/backup"
	"github.com/lupppig/dbackup/internal/config"
	"github.com/lupppig/dbackup/internal/db"
	"github.com/lupppig/dbackup/internal/logger"
	"github.com/lupppig/dbackup/internal/notify"
	"github.com/lupppig/dbackup/internal/scheduler"
	"github.com/spf13/cobra"
	"github.com/vbauerster/mpb/v8"
)

var dumpCmd = &cobra.Command{
	Use:   "dump",
	Short: "Execute all backups and restores defined in the config file",
	Long:  `Reads the configuration file and executes all defined backup and restore tasks. Backups run in parallel, followed by sequential restores.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		conf := config.GetConfig()
		if len(conf.Backups) == 0 && len(conf.Restores) == 0 {
			return fmt.Errorf("no backups or restores defined in config")
		}

		l := logger.New(logger.Config{
			JSON:    conf.LogJSON,
			NoColor: conf.NoColor,
		})
		var notifier notify.Notifier
		if conf.Notifications.Slack.WebhookURL != "" {
			notifier = notify.NewSlackNotifier(conf.Notifications.Slack.WebhookURL)
		}

		ctx := context.Background()

		// Determine if scheduler should start.
		hasSchedule := false
		for _, b := range conf.Backups {
			if b.Schedule != "" || b.Interval != "" {
				hasSchedule = true
				break
			}
		}
		if !hasSchedule {
			for _, r := range conf.Restores {
				if r.Schedule != "" || r.Interval != "" {
					hasSchedule = true
					break
				}
			}
		}

		if hasSchedule {
			l.Info("Scheduling tasks from config")
			s, err := scheduler.NewScheduler()
			if err != nil {
				return fmt.Errorf("failed to initialize scheduler: %w", err)
			}

			// Add backups to scheduler
			for _, b := range conf.Backups {
				if b.Schedule == "" && b.Interval == "" {
					continue
				}
				taskID := b.ID
				if taskID == "" {
					taskID = fmt.Sprintf("backup-%s-%d", b.DB, time.Now().UnixNano())
				}
				sched := b.Schedule
				if sched == "" {
					sched = b.Interval
				}
				st := &scheduler.ScheduledTask{
					ID:        taskID,
					Type:      scheduler.BackupTask,
					Engine:    b.Engine,
					SourceURI: b.URI,
					TargetURI: b.To,
					Schedule:  sched,
					Options: scheduler.TaskOptions{
						DBType:               b.Engine,
						DBName:               b.DB,
						Compress:             b.Compress,
						Algorithm:            b.Algorithm,
						EncryptionKeyFile:    b.EncryptionKeyFile,
						EncryptionPassphrase: b.EncryptionPassphrase,
						Retention:            b.Retention,
						Keep:                 b.Keep,
					},
				}
				if err := s.AddTask(st); err != nil {
					l.Error("Failed to schedule backup task", "id", taskID, "error", err)
				}
			}

			// Add restores to scheduler
			for _, r := range conf.Restores {
				if r.Schedule == "" && r.Interval == "" {
					continue
				}
				taskID := r.ID
				if taskID == "" {
					taskID = fmt.Sprintf("restore-%s-%d", r.From, time.Now().UnixNano())
				}
				sched := r.Schedule
				if sched == "" {
					sched = r.Interval
				}
				st := &scheduler.ScheduledTask{
					ID:        taskID,
					Type:      scheduler.RestoreTask,
					Engine:    r.Engine,
					SourceURI: r.From,
					TargetURI: r.To,
					Schedule:  sched,
					Options: scheduler.TaskOptions{
						DBType:               r.Engine,
						DBName:               r.DB,
						Compress:             r.Compress,
						Algorithm:            r.Algorithm,
						EncryptionKeyFile:    r.EncryptionKeyFile,
						EncryptionPassphrase: r.EncryptionPassphrase,
						ConfirmRestore:       r.ConfirmRestore,
					},
				}
				if err := s.AddTask(st); err != nil {
					l.Error("Failed to schedule restore task", "id", taskID, "error", err)
				}
			}

			l.Info("Scheduler started. Press Ctrl+C to stop.")
			s.Start()
			select {}
		}

		l.Info("Executing immediate tasks", "parallelism", conf.Parallelism)

		var p *mpb.Progress
		if !conf.LogJSON {
			p = backup.NewProgressContainer()
		}

		sem := make(chan struct{}, conf.Parallelism)
		var wg sync.WaitGroup

		// Execute Backups in Parallel
		backupCount := 0
		for _, b := range conf.Backups {
			if b.Schedule != "" || b.Interval != "" {
				continue
			}
			backupCount++
			wg.Add(1)
			go func(b config.TaskConfig) {
				defer wg.Done()
				sem <- struct{}{}
				defer func() { <-sem }()

				l.Info("Starting backup task", "id", b.ID)
				opts := convertToBackupOptions(b, l, notifier, p, *conf)
				adapter, err := db.GetAdapter(opts.DBType)
				if err != nil {
					l.Error("Invalid engine", "id", b.ID, "engine", b.Engine)
					return
				}

				bm, err := backup.NewBackupManager(opts)
				if err != nil {
					l.Error("Failed to initialize backup", "id", b.ID, "error", err)
					return
				}

				conn := db.ConnectionParams{
					DBType:   opts.DBType,
					DBName:   opts.DBName,
					DBUri:    b.URI,
					Host:     b.Host,
					User:     b.User,
					Password: b.Pass,
					Port:     b.Port,
				}

				if err := bm.Run(ctx, adapter, conn); err != nil {
					l.Error("Backup failed", "id", b.ID, "error", err)
				}
			}(b)
		}

		// Wait for all backups to finish
		wg.Wait()
		l.Info("All backups completed. Starting sequential restores if any.")

		// Execute Restores Sequentially
		for _, r := range conf.Restores {
			if r.Schedule != "" || r.Interval != "" {
				continue
			}

			l.Info("Starting sequential restore task", "id", r.ID)
			opts := convertToBackupOptions(r, l, notifier, p, *conf)
			adapter, err := db.GetAdapter(opts.DBType)
			if err != nil {
				l.Error("Invalid engine", "id", r.ID, "engine", r.Engine)
				continue
			}

			rm, err := backup.NewRestoreManager(opts)
			if err != nil {
				l.Error("Failed to initialize restore", "id", r.ID, "error", err)
				continue
			}

			dbUri := r.URI
			if dbUri == "" {
				dbUri = r.To
			}

			conn := db.ConnectionParams{
				DBType:   opts.DBType,
				DBName:   opts.DBName,
				DBUri:    dbUri,
				Host:     r.Host,
				User:     r.User,
				Password: r.Pass,
				Port:     r.Port,
				TLS: db.TLSConfig{
					Enabled:    r.TLS.Enabled,
					Mode:       r.TLS.Mode,
					CACert:     r.TLS.CACert,
					ClientCert: r.TLS.ClientCert,
					ClientKey:  r.TLS.ClientKey,
				},
			}

			if err := rm.Run(ctx, adapter, conn); err != nil {
				l.Error("Restore failed", "id", r.ID, "error", err)
			}
		}

		if p != nil {
			p.Wait()
		}
		l.Info("All immediate tasks completed")
		return nil
	},
}

func convertToBackupOptions(tc config.TaskConfig, l *logger.Logger, n notify.Notifier, p *mpb.Progress, global config.Config) backup.BackupOptions {
	retention, _ := time.ParseDuration(tc.Retention)

	dedupe := true
	if tc.Dedupe != nil {
		dedupe = *tc.Dedupe
	}

	storageURI := tc.To
	fileName := tc.FileName
	if tc.From != "" {
		storageURI = tc.From
		// If From is provided, storageURI IS the source manifest or folder
		if fileName == "" {
			// Extract filename from URI path
			parts := strings.Split(storageURI, "/")
			last := parts[len(parts)-1]
			if strings.Contains(last, ".") {
				fileName = last
			} else {
				// Default to latest.manifest if it's a folder
				fileName = "latest.manifest"
			}
		}
	}

	passphrase := tc.EncryptionPassphrase
	if passphrase == "" {
		passphrase = global.EncryptionPassphrase
	}
	keyFile := tc.EncryptionKeyFile
	if keyFile == "" {
		keyFile = global.EncryptionKeyFile
	}

	return backup.BackupOptions{
		DBType:               tc.Engine,
		DBName:               tc.DB,
		StorageURI:           storageURI,
		Compress:             tc.Compress,
		Algorithm:            tc.Algorithm,
		FileName:             fileName,
		Encrypt:              tc.Encrypt,
		EncryptionPassphrase: passphrase,
		EncryptionKeyFile:    keyFile,
		RemoteExec:           tc.RemoteExec,
		Dedupe:               dedupe,
		Retention:            retention,
		Keep:                 tc.Keep,
		ConfirmRestore:       tc.ConfirmRestore,
		DryRun:               tc.DryRun,
		Logger:               l,
		Notifier:             n,
		Progress:             p,
	}
}

func init() {
	rootCmd.AddCommand(dumpCmd)
}
