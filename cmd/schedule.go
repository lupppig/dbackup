package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/google/uuid"
	"github.com/lupppig/dbackup/internal/logger"
	"github.com/lupppig/dbackup/internal/scheduler"
	"github.com/spf13/cobra"
)

var (
	cronSpec   string
	interval   string
	retries    int
	retryDelay string
	daemonMode bool
)

var scheduleCmd = &cobra.Command{
	Use:   "schedule",
	Short: "Manage recurring backup schedules",
}

var scheduleBackupCmd = &cobra.Command{
	Use:   "backup [engine]",
	Short: "Schedule a recurring backup",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		l := logger.New(logger.Config{JSON: LogJSON, NoColor: NoColor})
		engine := args[0]
		s, err := scheduler.NewScheduler()
		if err != nil {
			return err
		}
		if err := s.Load(); err != nil {
			return err
		}

		sched := cronSpec
		if interval != "" {
			sched = interval
		}
		if sched == "" {
			return fmt.Errorf("either --cron or --interval is required")
		}

		task := &scheduler.ScheduledTask{
			ID:        uuid.New().String(),
			Type:      scheduler.BackupTask,
			Engine:    engine,
			SourceURI: dbURI,
			TargetURI: target,
			Schedule:  sched,
			Options: scheduler.TaskOptions{
				DBType:               engine,
				DBName:               dbName,
				Compress:             compress,
				Algorithm:            compressionAlgo,
				FileName:             fileName,
				EncryptionKeyFile:    encryptionKeyFile,
				EncryptionPassphrase: "", // Never store
				Retries:              retries,
				RetryDelay:           retryDelay,
			},
		}

		if err := s.AddTask(task); err != nil {
			return err
		}

		l.Info("Scheduled backup task added", "schedule", sched, "id", task.ID)

		// Spawn background daemon if not already in daemon mode
		if !daemonMode {
			return spawnDaemon(l)
		}
		return nil
	},
}

var scheduleRestoreCmd = &cobra.Command{
	Use:   "restore [engine]",
	Short: "Schedule a recurring restore",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		l := logger.New(logger.Config{JSON: LogJSON, NoColor: NoColor})
		engine := args[0]
		s, err := scheduler.NewScheduler()
		if err != nil {
			return err
		}
		if err := s.Load(); err != nil {
			return err
		}

		sched := cronSpec
		if interval != "" {
			sched = interval
		}
		if sched == "" {
			return fmt.Errorf("either --cron or --interval is required")
		}

		task := &scheduler.ScheduledTask{
			ID:        uuid.New().String(),
			Type:      scheduler.RestoreTask,
			Engine:    engine,
			SourceURI: from,
			TargetURI: target,
			Schedule:  sched,
			Options: scheduler.TaskOptions{
				DBType:               engine,
				DBName:               dbName,
				EncryptionKeyFile:    encryptionKeyFile,
				EncryptionPassphrase: "", // Never store
				ConfirmRestore:       confirmRestore,
				Retries:              retries,
				RetryDelay:           retryDelay,
			},
		}

		if err := s.AddTask(task); err != nil {
			return err
		}

		l.Info("Scheduled restore task added", "schedule", sched, "id", task.ID)

		// Spawn background daemon if not already in daemon mode
		if !daemonMode {
			return spawnDaemon(l)
		}
		return nil
	},
}

var scheduleRemoveCmd = &cobra.Command{
	Use:   "remove [ID]",
	Short: "Remove a scheduled task",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		l := logger.New(logger.Config{JSON: LogJSON, NoColor: NoColor})
		id := args[0]
		s, err := scheduler.NewScheduler()
		if err != nil {
			return err
		}
		if err := s.Load(); err != nil {
			return err
		}

		if err := s.RemoveTask(id); err != nil {
			return err
		}

		l.Info("Task removed successfully", "id", id)
		return nil
	},
}

var scheduleStartCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the scheduler daemon (internal use)",
	RunE: func(cmd *cobra.Command, args []string) error {
		l := logger.New(logger.Config{JSON: LogJSON, NoColor: NoColor})
		s, err := scheduler.NewScheduler()
		if err != nil {
			return err
		}
		if err := s.Load(); err != nil {
			return err
		}

		tasks := s.ListTasks()
		l.Info("Starting scheduler", "task_count", len(tasks))

		for _, t := range tasks {
			if err := s.AddTask(t); err != nil {
				l.Warn("Failed to schedule task", "id", t.ID, "error", err)
			}
		}

		s.Start()
		defer s.Stop()

		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		<-sigChan

		l.Info("Shutting down scheduler")
		return nil
	},
}

var scheduleListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all active schedules",
	RunE: func(cmd *cobra.Command, args []string) error {
		l := logger.New(logger.Config{JSON: LogJSON, NoColor: NoColor})
		s, err := scheduler.NewScheduler()
		if err != nil {
			return err
		}
		if err := s.Load(); err != nil {
			return err
		}

		tasks := s.ListTasks()
		if len(tasks) == 0 {
			l.Info("No active schedules found")
			return nil
		}

		for _, t := range tasks {
			next := "N/A"
			if t.NextRun != nil {
				next = t.NextRun.Format("2006-01-02 15:04:05")
			}
			l.Info("Scheduled Task",
				"id", t.ID,
				"type", t.Type,
				"status", t.Status,
				"schedule", t.Schedule,
				"next_run", next,
			)
		}
		return nil
	},
}

func spawnDaemon(l *logger.Logger) error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}

	// Run `dbackup schedule start` in background
	cmd := exec.Command(exe, "schedule", "start", "--daemon")
	cmd.Dir = filepath.Dir(exe)
	cmd.Stdout = nil
	cmd.Stderr = nil
	cmd.Stdin = nil
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setsid: true, // Create a new session (detach from terminal)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start daemon: %w", err)
	}

	l.Info("Scheduler daemon started", "pid", cmd.Process.Pid)
	return nil
}

func init() {
	rootCmd.AddCommand(scheduleCmd)
	scheduleCmd.AddCommand(scheduleBackupCmd)
	scheduleCmd.AddCommand(scheduleRestoreCmd)
	scheduleCmd.AddCommand(scheduleRemoveCmd)
	scheduleCmd.AddCommand(scheduleStartCmd)
	scheduleCmd.AddCommand(scheduleListCmd)

	// Hidden flag for daemon mode
	scheduleStartCmd.Flags().BoolVar(&daemonMode, "daemon", false, "Run in daemon mode (internal)")
	scheduleStartCmd.Flags().MarkHidden("daemon")

	for _, c := range []*cobra.Command{scheduleBackupCmd, scheduleRestoreCmd} {
		c.Flags().StringVar(&cronSpec, "cron", "", "Cron schedule (e.g. \"0 2 * * *\")")
		c.Flags().StringVar(&interval, "interval", "", "Interval schedule (e.g. \"1h\", \"30m\")")
		c.Flags().IntVar(&retries, "retries", 3, "Number of retries on failure")
		c.Flags().StringVar(&retryDelay, "retry-delay", "5m", "Delay between retries")
	}

	// Schedule Backup specific
	scheduleBackupCmd.Flags().StringVar(&fileName, "name", "", "custom backup file name")

	// Schedule Restore specific
	scheduleRestoreCmd.Flags().StringVar(&fileName, "name", "", "custom backup file name to restore")
}
