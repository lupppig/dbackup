package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/fsnotify/fsnotify"
	"github.com/spf13/viper"
)

type Config struct {
	Parallelism          int           `mapstructure:"parallelism"`
	AllowInsecure        bool          `mapstructure:"allow_insecure"`
	LogJSON              bool          `mapstructure:"log_json"`
	NoColor              bool          `mapstructure:"no_color"`
	Notifications        Notifications `mapstructure:"notifications"`
	EncryptionPassphrase string        `mapstructure:"encryption_passphrase"`
	EncryptionKeyFile    string        `mapstructure:"encryption_key_file"`
	Backups              []TaskConfig  `mapstructure:"backups"`
	Restores             []TaskConfig  `mapstructure:"restores"`
}

type Notifications struct {
	Slack    SlackConfig     `mapstructure:"slack"`
	Webhooks []WebhookConfig `mapstructure:"webhooks"`
}

type SlackConfig struct {
	WebhookURL string `mapstructure:"webhook_url"`
	Template   string `mapstructure:"template"` // Custom message template
}

type WebhookConfig struct {
	ID       string            `mapstructure:"id"`
	URL      string            `mapstructure:"url"`
	Method   string            `mapstructure:"method"` // Default POST
	Template string            `mapstructure:"template"`
	Headers  map[string]string `mapstructure:"headers"`
}

type TaskConfig struct {
	ID                   string    `mapstructure:"id"`
	Engine               string    `mapstructure:"engine"`
	URI                  string    `mapstructure:"uri"`
	DB                   string    `mapstructure:"db"`
	Host                 string    `mapstructure:"host"`
	User                 string    `mapstructure:"user"`
	Pass                 string    `mapstructure:"pass"`
	Port                 int       `mapstructure:"port"`
	TLS                  TLSConfig `mapstructure:"tls"`
	To                   string    `mapstructure:"to"`
	From                 string    `mapstructure:"from"`
	FileName             string    `mapstructure:"file_name"`
	RemoteExec           bool      `mapstructure:"remote_exec"`
	Dedupe               *bool     `mapstructure:"dedupe"` // Use pointer to distinguish between false and default true
	Compress             bool      `mapstructure:"compress"`
	Algorithm            string    `mapstructure:"algorithm"`
	Encrypt              bool      `mapstructure:"encrypt"`
	EncryptionPassphrase string    `mapstructure:"encryption_passphrase"`
	EncryptionKeyFile    string    `mapstructure:"encryption_key_file"`
	Retention            string    `mapstructure:"retention"`
	Keep                 int       `mapstructure:"keep"`
	Schedule             string    `mapstructure:"schedule"`
	Interval             string    `mapstructure:"interval"`
	DryRun               bool      `mapstructure:"dry_run"`
	ConfirmRestore       bool      `mapstructure:"confirm_restore"`
}

type TLSConfig struct {
	Enabled    bool   `mapstructure:"enabled"`
	Mode       string `mapstructure:"mode"`
	CACert     string `mapstructure:"ca_cert"`
	ClientCert string `mapstructure:"client_cert"`
	ClientKey  string `mapstructure:"client_key"`
}

var (
	globalConfig *Config
	configMutex  sync.RWMutex
)

func Initialize(configPath string) error {
	v := viper.New()

	if configPath != "" {
		v.SetConfigFile(configPath)
	} else {
		v.SetConfigName("backup")
		v.SetConfigType("yaml")
		v.AddConfigPath(".")

		home, err := os.UserHomeDir()
		if err == nil {
			v.AddConfigPath(filepath.Join(home, ".dbackup"))
		}
	}

	v.SetEnvPrefix("DBACKUP")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	// Set defaults
	v.SetDefault("parallelism", 4)
	v.SetDefault("allow_insecure", false)

	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok && configPath != "" {
			return fmt.Errorf("failed to read config file: %w", err)
		}
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return fmt.Errorf("failed to unmarshal config: %w", err)
	}
	configMutex.Lock()
	globalConfig = &cfg
	configMutex.Unlock()

	v.WatchConfig()
	v.OnConfigChange(func(e fsnotify.Event) {
		var newCfg Config
		if err := v.Unmarshal(&newCfg); err == nil {
			configMutex.Lock()
			globalConfig = &newCfg
			configMutex.Unlock()
		}
	})

	return nil
}

func GetConfig() *Config {
	configMutex.RLock()
	defer configMutex.RUnlock()
	if globalConfig == nil {
		return &Config{Parallelism: 4}
	}
	return globalConfig
}
