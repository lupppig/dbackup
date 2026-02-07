package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/fsnotify/fsnotify"
	"github.com/spf13/viper"
)

type Config struct {
	Parallelism   int           `mapstructure:"parallelism"`
	AllowInsecure bool          `mapstructure:"allow_insecure"`
	LogJSON       bool          `mapstructure:"log_json"`
	NoColor       bool          `mapstructure:"no_color"`
	Notifications Notifications `mapstructure:"notifications"`
	Backups       []TaskConfig  `mapstructure:"backups"`
	Restores      []TaskConfig  `mapstructure:"restores"`
}

type Notifications struct {
	Slack SlackConfig `mapstructure:"slack"`
}

type SlackConfig struct {
	WebhookURL string `mapstructure:"webhook_url"`
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
	Enabled bool   `mapstructure:"enabled"`
	Mode    string `mapstructure:"mode"`
}

var globalConfig *Config

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

	if err := v.Unmarshal(&globalConfig); err != nil {
		return fmt.Errorf("failed to unmarshal config: %w", err)
	}

	v.WatchConfig()
	v.OnConfigChange(func(e fsnotify.Event) {
		_ = v.Unmarshal(&globalConfig)
	})

	return nil
}

func GetConfig() *Config {
	if globalConfig == nil {
		return &Config{Parallelism: 4}
	}
	return globalConfig
}
