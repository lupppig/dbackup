package scheduler

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/lupppig/dbackup/internal/backup"
	"github.com/lupppig/dbackup/internal/db"
	"github.com/lupppig/dbackup/internal/logger"
	"github.com/lupppig/dbackup/internal/notify"
	"github.com/robfig/cron/v3"
)

type TaskType string

const (
	BackupTask  TaskType = "backup"
	RestoreTask TaskType = "restore"
)

type TaskStatus string

const (
	StatusPending TaskStatus = "pending"
	StatusRunning TaskStatus = "running"
	StatusSuccess TaskStatus = "success"
	StatusFailed  TaskStatus = "failed"
)

// ScheduledTask represents a recurring job
type ScheduledTask struct {
	ID        string     `json:"id"`
	Type      TaskType   `json:"type"`
	Engine    string     `json:"engine"`
	SourceURI string     `json:"source_uri"`
	TargetURI string     `json:"target_uri"`
	Schedule  string     `json:"schedule"` // Cron or interval (e.g. "@daily" or "24h")
	Status    TaskStatus `json:"status"`
	LastRun   *time.Time `json:"last_run,omitempty"`
	NextRun   *time.Time `json:"next_run,omitempty"`

	// Options required to recreate the managers
	Options TaskOptions `json:"options"`

	cronID cron.EntryID
}

type TaskOptions struct {
	DBType               string `json:"db_type"`
	DBName               string `json:"db_name"`
	Compress             bool   `json:"compress"`
	Algorithm            string `json:"algorithm"`
	FileName             string `json:"file_name"`
	Parallel             int    `json:"parallel"`
	EncryptionKeyFile    string `json:"encryption_key_file,omitempty"`
	EncryptionPassphrase string `json:"-"` // DO NOT STORE PASSPHRASE
	ConfirmRestore       bool   `json:"confirm_restore"`
	Retries              int    `json:"retries"`
	RetryDelay           string `json:"retry_delay"`
	Verify               bool   `json:"verify"`
	Retention            string `json:"retention,omitempty"`
	Keep                 int    `json:"keep,omitempty"`
}

type Scheduler struct {
	cron     *cron.Cron
	tasks    map[string]*ScheduledTask
	mu       sync.RWMutex
	dataDir  string
	maxTasks int
	running  int
}

func NewScheduler() (*Scheduler, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	dir := filepath.Join(home, ".dbackup")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}

	return &Scheduler{
		cron:    cron.New(),
		tasks:   make(map[string]*ScheduledTask),
		dataDir: dir,
	}, nil
}

func (s *Scheduler) Start() {
	s.cron.Start()
}

func (s *Scheduler) Stop() context.Context {
	return s.cron.Stop()
}

func (s *Scheduler) Save() error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	data, err := json.MarshalIndent(s.tasks, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(filepath.Join(s.dataDir, "schedules.json"), data, 0600)
}

func (s *Scheduler) Load() error {
	path := filepath.Join(s.dataDir, "schedules.json")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	return json.Unmarshal(data, &s.tasks)
}

func (s *Scheduler) AddTask(task *ScheduledTask) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Validate schedule - standard cron or @every
	spec := task.Schedule
	if !strings.HasPrefix(spec, "@") && strings.Count(spec, " ") < 4 {
		// Possibly an interval like "24h", convert to @every
		if _, err := time.ParseDuration(spec); err == nil {
			spec = "@every " + spec
		}
	}

	id, err := s.cron.AddFunc(spec, func() {
		s.executeTask(task.ID)
	})
	if err != nil {
		return fmt.Errorf("invalid schedule %q: %w", task.Schedule, err)
	}

	task.cronID = id
	task.Status = StatusPending
	s.tasks[task.ID] = task
	return s.saveLocked()
}

// saveLocked saves tasks without acquiring a lock (caller must hold mu)
func (s *Scheduler) saveLocked() error {
	data, err := json.MarshalIndent(s.tasks, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(s.dataDir, "schedules.json"), data, 0600)
}

func (s *Scheduler) RemoveTask(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	task, ok := s.tasks[id]
	if !ok {
		return fmt.Errorf("task not found: %s", id)
	}

	s.cron.Remove(task.cronID)
	delete(s.tasks, id)
	return s.saveLocked()
}

func (s *Scheduler) ListTasks() []*ScheduledTask {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var list []*ScheduledTask
	for _, t := range s.tasks {
		entry := s.cron.Entry(t.cronID)
		t.NextRun = &entry.Next
		list = append(list, t)
	}
	return list
}

func (s *Scheduler) executeTask(id string) {
	s.mu.RLock()
	task, ok := s.tasks[id]
	running := s.running
	maxTasks := s.maxTasks
	s.mu.RUnlock()
	if !ok {
		return
	}

	l := logger.New(logger.Config{})

	// Constraint: max-tasks
	if maxTasks > 0 && running >= maxTasks {
		l.Warn("Skipping task: max concurrent tasks reached", "id", id, "max", maxTasks, "running", running)
		return
	}

	// Constraint: same task already running
	if task.Status == StatusRunning {
		l.Warn("Skipping task: already running", "id", id)
		return
	}

	s.mu.Lock()
	task.Status = StatusRunning
	now := time.Now()
	task.LastRun = &now
	s.running++
	s.mu.Unlock()
	s.Save()

	var notifier notify.Notifier
	if os.Getenv("SLACK_WEBHOOK") != "" {
		notifier = notify.NewSlackNotifier(os.Getenv("SLACK_WEBHOOK"), "")
	}

	maxRetries := task.Options.Retries
	retryDelay, _ := time.ParseDuration(task.Options.RetryDelay)
	if retryDelay == 0 {
		retryDelay = 5 * time.Minute
	}

	var err error
	for i := 0; i <= maxRetries; i++ {
		if i > 0 {
			l.Info("Retrying task", "id", task.ID, "attempt", i, "delay", retryDelay)
			time.Sleep(retryDelay)
		}
		err = s.runInternal(task, l, notifier)
		if err == nil {
			break
		}
	}

	s.mu.Lock()
	s.running--
	if err != nil {
		task.Status = StatusFailed
		l.Error("Scheduled task failed after retries", "id", task.ID, "error", err)
		if notifier != nil {
			notifier.Notify(context.Background(), notify.Stats{
				Operation: string(task.Type),
				Engine:    task.Engine,
				Database:  task.Options.DBName,
				FileName:  task.Options.FileName,
				Status:    notify.StatusError,
				Error:     err,
			})
		}
	} else {
		task.Status = StatusSuccess
		l.Info("Scheduled task succeeded", "id", task.ID)
		if notifier != nil {
			notifier.Notify(context.Background(), notify.Stats{
				Operation: string(task.Type),
				Engine:    task.Engine,
				Database:  task.Options.DBName,
				FileName:  task.Options.FileName,
				Status:    notify.StatusSuccess,
			})
		}
	}
	s.mu.Unlock()
	s.Save()
}

func (s *Scheduler) runInternal(t *ScheduledTask, l *logger.Logger, n notify.Notifier) error {
	ctx := context.Background()

	conn := db.ConnectionParams{
		DBType: t.Options.DBType,
		DBName: t.Options.DBName,
		DBUri:  t.SourceURI,
	}
	if t.Type == RestoreTask {
		conn.DBUri = t.TargetURI
	}

	if err := conn.ParseURI(); err != nil {
		return err
	}

	opts := backup.BackupOptions{
		DBType:               t.Options.DBType,
		DBName:               t.Options.DBName,
		StorageURI:           t.TargetURI,
		Compress:             t.Options.Compress,
		Algorithm:            t.Options.Algorithm,
		FileName:             t.Options.FileName,
		Dedupe:               true, // Incremental by default for scheduled backups
		Encrypt:              t.Options.EncryptionKeyFile != "" || os.Getenv("DBACKUP_KEY") != "",
		EncryptionKeyFile:    t.Options.EncryptionKeyFile,
		EncryptionPassphrase: os.Getenv("DBACKUP_KEY"),
		ConfirmRestore:       t.Options.ConfirmRestore,
		Logger:               l,
		Notifier:             n,
	}

	if t.Options.Retention != "" {
		dur, _ := time.ParseDuration(t.Options.Retention)
		// Handle daily duration if ends in 'd'
		if strings.HasSuffix(t.Options.Retention, "d") {
			days := strings.TrimSuffix(t.Options.Retention, "d")
			var d int
			fmt.Sscanf(days, "%d", &d)
			dur = time.Duration(d) * 24 * time.Hour
		}
		opts.Retention = dur
	}
	opts.Keep = t.Options.Keep

	if t.Type == RestoreTask {
		opts.StorageURI = t.SourceURI
	}

	var adapter db.DBAdapter
	switch strings.ToLower(conn.DBType) {
	case "postgres", "postgresql":
		adapter = &db.PostgresAdapter{}
	case "mysql":
		adapter = &db.MysqlAdapter{}
	case "sqlite":
		adapter = &db.SqliteAdapter{}
	default:
		return fmt.Errorf("unsupported database: %s", conn.DBType)
	}
	adapter.SetLogger(l)

	if t.Type == BackupTask {
		mgr, err := backup.NewBackupManager(opts)
		if err != nil {
			return err
		}
		return mgr.Run(ctx, adapter, conn)
	} else {
		mgr, err := backup.NewRestoreManager(opts)
		if err != nil {
			return err
		}
		return mgr.Run(ctx, adapter, conn)
	}
}
