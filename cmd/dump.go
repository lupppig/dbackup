package cmd

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/lupppig/dbackup/internal/backup"
	"github.com/lupppig/dbackup/internal/config"
	"github.com/lupppig/dbackup/internal/db"
	"github.com/lupppig/dbackup/internal/logger"
	"github.com/lupppig/dbackup/internal/notify"
	"github.com/lupppig/dbackup/internal/scheduler"
	"github.com/spf13/cobra"
)

var dumpCmd = &cobra.Command{
	Use:   "dump",
	Short: "Execute all backups and restores defined in the config file",
	Long:  `Reads the configuration file and executes all defined backup and restore tasks in parallel.`,
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

		// Handle Scheduling if any task has a schedule
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
					l.Error("Failed to schedule task", "id", taskID, "error", err)
				}
			}

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
					l.Error("Failed to schedule task", "id", taskID, "error", err)
				}
			}

			l.Info("Scheduler started. Press Ctrl+C to stop.")
			s.Start()
			// Block indefinitely to keep the scheduler running
			select {}
		}

		// Run immediate tasks in parallel
		l.Info("Executing immediate tasks", "parallelism", conf.Parallelism)

		sem := make(chan struct{}, conf.Parallelism)
		var wg sync.WaitGroup

		for _, b := range conf.Backups {
			if b.Schedule != "" || b.Interval != "" {
				continue
			}

			wg.Add(1)
			go func(b config.TaskConfig) {
				defer wg.Done()
				sem <- struct{}{}
				defer func() { <-sem }()

				l.Info("Starting backup task", "id", b.ID)
				opts := convertToBackupOptions(b, l, notifier)
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

		for _, r := range conf.Restores {
			if r.Schedule != "" || r.Interval != "" {
				continue
			}

			wg.Add(1)
			go func(r config.TaskConfig) {
				defer wg.Done()
				sem <- struct{}{}
				defer func() { <-sem }()

				l.Info("Starting restore task", "id", r.ID)
				opts := convertToBackupOptions(r, l, notifier)
				adapter, err := db.GetAdapter(opts.DBType)
				if err != nil {
					l.Error("Invalid engine", "id", r.ID, "engine", r.Engine)
					return
				}

				rm, err := backup.NewRestoreManager(opts)
				if err != nil {
					l.Error("Failed to initialize restore", "id", r.ID, "error", err)
					return
				}

				conn := db.ConnectionParams{
					DBType:   opts.DBType,
					DBName:   opts.DBName,
					DBUri:    r.URI,
					Host:     r.Host,
					User:     r.User,
					Password: r.Pass,
					Port:     r.Port,
				}

				if err := rm.Run(ctx, adapter, conn); err != nil {
					l.Error("Restore failed", "id", r.ID, "error", err)
				}
			}(r)
		}

		wg.Wait()
		l.Info("All immediate tasks completed")
		return nil
	},
}

func convertToBackupOptions(tc config.TaskConfig, l *logger.Logger, n notify.Notifier) backup.BackupOptions {
	retention, _ := time.ParseDuration(tc.Retention)

	dedupe := true
	if tc.Dedupe != nil {
		dedupe = *tc.Dedupe
	}

	return backup.BackupOptions{
		DBType:               tc.Engine,
		DBName:               tc.DB,
		StorageURI:           tc.To,
		Compress:             tc.Compress,
		Algorithm:            tc.Algorithm,
		Encrypt:              tc.Encrypt,
		EncryptionPassphrase: tc.EncryptionPassphrase,
		EncryptionKeyFile:    tc.EncryptionKeyFile,
		RemoteExec:           tc.RemoteExec,
		Dedupe:               dedupe,
		Retention:            retention,
		Keep:                 tc.Keep,
		ConfirmRestore:       tc.ConfirmRestore,
		DryRun:               tc.DryRun,
		Logger:               l,
		Notifier:             n,
	}
}

func init() {
	rootCmd.AddCommand(dumpCmd)
}
