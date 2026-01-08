package config

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// Config holds all static configuration required by the worker.
type Config struct {
	OrchestratorURL string        `mapstructure:"orchestrator_url"`
	WorkerID        string        `mapstructure:"worker_id"`
	NasMountPath    string        `mapstructure:"nas_mount_path"`
	TempDir         string        `mapstructure:"temp_dir"`
	SyncInterval    time.Duration `mapstructure:"sync_interval"`
	LogLevel        string        `mapstructure:"log_level"`
}

// Load reads configuration from config.yml and environment variables.
// Priority: Env Vars > Config File > Defaults.
func Load(path string) (*Config, error) {
	v := viper.New()

	// 1. Set Defaults
	v.SetDefault("temp_dir", "/tmp/transcode")
	v.SetDefault("sync_interval", "10s")
	v.SetDefault("log_level", "info")

	// 2. Load from File
	v.SetConfigName("config") // name of config file (without extension)
	v.SetConfigType("yaml")   // REQUIRED if the config file does not have the extension in the name
	
	// Look for config in these paths
	v.AddConfigPath(path)      // Custom path provided by caller
	v.AddConfigPath(".")       // Current directory
	v.AddConfigPath("./config") // Config directory

	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("error reading config file: %w", err)
		}
		// It's okay if config file is missing, provided Env Vars are set.
	}

	// 3. Load from Environment Variables
	// Env vars will be uppercase and match the struct fields.
	// Example: orchestrator_url becomes WORKER_ORCHESTRATOR_URL
	v.SetEnvPrefix("WORKER")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	// 4. Unmarshal into Struct
	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("unable to decode config into struct: %w", err)
	}

	// 5. Validation & Post-Processing
	if err := validate(&cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

func validate(cfg *Config) error {
	if cfg.OrchestratorURL == "" {
		return errors.New("configuration 'orchestrator_url' is required")
	}
	if cfg.NasMountPath == "" {
		return errors.New("configuration 'nas_mount_path' is required")
	}

	// If WorkerID is not set in config, try to get the OS hostname
	if cfg.WorkerID == "" {
		hostname, err := os.Hostname()
		if err != nil {
			return fmt.Errorf("worker_id not set and unable to retrieve hostname: %w", err)
		}
		cfg.WorkerID = hostname
	}

	// Ensure temp dir exists or can be created
	if err := os.MkdirAll(cfg.TempDir, 0755); err != nil {
		return fmt.Errorf("unable to create temp_dir at %s: %w", cfg.TempDir, err)
	}

	return nil
}