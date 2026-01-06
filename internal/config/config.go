package config

import (
	"strings"

	"github.com/spf13/viper"
)

// Config holds all the settings for the worker.
type Config struct {
	OrchestratorURL string `mapstructure:"orchestrator_url"`
	WorkerID        string `mapstructure:"worker_id"`
	HeartbeatSec    int    `mapstructure:"heartbeat_seconds"`
	NASMountPath    string `mapstructure:"nas_mount_path"`
	LogLevel        string `mapstructure:"log_level"`
	EnableHWAccel   bool   `mapstructure:"enable_hw_accel"`
}

// LoadConfig initializes Viper and merges all config sources.
func LoadConfig(path string) (*Config, error) {
	// 1. Set Defaults
	viper.SetDefault("heartbeat_seconds", 15)
	viper.SetDefault("log_level", "info")
	viper.SetDefault("enable_hw_accel", true)

	// 2. Read from File
	viper.SetConfigFile(path)
	viper.SetConfigType("yaml")

	if err := viper.ReadInConfig(); err != nil {
		// It's okay if the config file is missing; we might use Env vars.
	}

	viper.SetEnvPrefix("CINE")
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()

	var cfg Config
	err := viper.Unmarshal(&cfg)
	return &cfg, err
}