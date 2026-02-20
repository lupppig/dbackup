package notify

import (
	"context"
	"time"
)

type Status string

const (
	StatusSuccess Status = "success"
	StatusError   Status = "error"
)

type Stats struct {
	Status    Status
	Operation string // "Backup" or "Restore"
	Engine    string
	Database  string
	FileName  string
	Size      int64
	Duration  time.Duration
	Error     error
}

type Notifier interface {
	Notify(ctx context.Context, stats Stats) error
}

type MultiNotifier struct {
	Notifiers []Notifier
}

func (m *MultiNotifier) Notify(ctx context.Context, stats Stats) error {
	for _, n := range m.Notifiers {
		if err := n.Notify(ctx, stats); err != nil {
			// Log error but continue with other notifiers
		}
	}
	return nil
}
