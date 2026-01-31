package scheduler

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestScheduler_Core(t *testing.T) {
	s, err := NewScheduler()
	require.NoError(t, err)
	defer s.Stop() // Stop cron to prevent hanging

	testFile := filepath.Join(s.dataDir, "schedules.json")
	os.Remove(testFile)
	defer os.Remove(testFile) // Clean up at end

	task := &ScheduledTask{
		ID:       "test-task",
		Type:     BackupTask,
		Schedule: "@daily",
		Options: TaskOptions{
			DBType: "sqlite",
		},
	}

	err = s.AddTask(task)
	assert.NoError(t, err)

	tasks := s.ListTasks()
	assert.Len(t, tasks, 1)
	assert.Equal(t, "test-task", tasks[0].ID)

	// Test persistence
	s2, _ := NewScheduler()
	defer s2.Stop()
	err = s2.Load()
	require.NoError(t, err)
	assert.Len(t, s2.ListTasks(), 1)
}
