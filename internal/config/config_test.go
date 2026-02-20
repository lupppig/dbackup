package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInitialize_DefaultsAndEnv(t *testing.T) {
	// Clear any existing env vars
	os.Clearenv()
	t.Cleanup(os.Clearenv)
	globalConfig = nil

	os.Setenv("DBACKUP_PARALLELISM", "8")
	os.Setenv("DBACKUP_ALLOW_INSECURE", "true")

	err := Initialize("") // empty triggers default paths, but no file should be found if not present
	// We might get an error if it doesn't find the home dir, but we just ignore it if it's missing file
	require.NoError(t, err)

	cfg := GetConfig()
	assert.Equal(t, 8, cfg.Parallelism)
	assert.True(t, cfg.AllowInsecure)
}

func TestInitialize_YamlFile(t *testing.T) {
	globalConfig = nil
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "backup.yaml")

	yamlContent := `
parallelism: 2
allow_insecure: false
log_json: true
backups:
  - id: "test-job"
    engine: "postgres"
    db: "testdb"
    retention: "7d"
`
	err := os.WriteFile(configFile, []byte(yamlContent), 0644)
	require.NoError(t, err)

	err = Initialize(configFile)
	require.NoError(t, err)

	cfg := GetConfig()
	assert.Equal(t, 2, cfg.Parallelism)
	assert.True(t, cfg.LogJSON)
	assert.Len(t, cfg.Backups, 1)
	assert.Equal(t, "test-job", cfg.Backups[0].ID)
	assert.Equal(t, "7d", cfg.Backups[0].Retention)
}

func TestInitialize_HotReload(t *testing.T) {
	globalConfig = nil
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "backup.yaml")

	yamlContent := `parallelism: 4`
	err := os.WriteFile(configFile, []byte(yamlContent), 0644)
	require.NoError(t, err)

	err = Initialize(configFile)
	require.NoError(t, err)

	assert.Equal(t, 4, GetConfig().Parallelism)

	// Update file
	newYaml := `parallelism: 10`
	err = os.WriteFile(configFile, []byte(newYaml), 0644)
	require.NoError(t, err)

	// Wait for fsnotify to pick up change
	time.Sleep(100 * time.Millisecond)

	assert.Equal(t, 10, GetConfig().Parallelism)
}
