package tests

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/lupppig/dbackup/internal/scheduler"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestScheduler_AddAndList(t *testing.T) {
	s, err := scheduler.NewScheduler()
	require.NoError(t, err)
	defer s.Stop() // Stop cron to prevent hanging

	// Ensure clean state for test
	home, _ := os.UserHomeDir()
	testFile := filepath.Join(home, ".dbackup", "schedules.json")
	os.Remove(testFile)
	defer os.Remove(testFile) // Clean up at end

	task := &scheduler.ScheduledTask{
		ID:       "test-task",
		Type:     scheduler.BackupTask,
		Schedule: "@daily",
		Options: scheduler.TaskOptions{
			DBType: "sqlite",
		},
	}

	err = s.AddTask(task)
	assert.NoError(t, err)

	tasks := s.ListTasks()
	assert.Len(t, tasks, 1)
	assert.Equal(t, "test-task", tasks[0].ID)

	// Verify persistence
	s2, err := scheduler.NewScheduler()
	require.NoError(t, err)
	defer s2.Stop()
	err = s2.Load()
	require.NoError(t, err)
	assert.Len(t, s2.ListTasks(), 1)
}
