// ABOUTME: Configuration for Charm KV backend
// ABOUTME: Handles charm server settings and XDG config paths

package charm

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Config holds charm sync configuration.
type Config struct {
	// CharmHost is the charm server URL (default: charm.2389.dev)
	CharmHost string `json:"charm_host,omitempty"`

	// AutoSync enables automatic sync after writes (default: true)
	AutoSync bool `json:"auto_sync"`
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig() *Config {
	return &Config{
		CharmHost: "charm.2389.dev",
		AutoSync:  true,
	}
}

// ConfigDir returns the configuration directory path.
func ConfigDir() string {
	configHome := os.Getenv("XDG_CONFIG_HOME")
	if configHome == "" {
		home, _ := os.UserHomeDir()
		configHome = filepath.Join(home, ".config")
	}
	return filepath.Join(configHome, "chronicle")
}

// ConfigPath returns the path to the config file.
func ConfigPath() string {
	return filepath.Join(ConfigDir(), "charm.json")
}

// LoadConfig loads configuration from disk, returns defaults if not found.
func LoadConfig() (*Config, error) {
	cfg := DefaultConfig()

	data, err := os.ReadFile(ConfigPath())
	if os.IsNotExist(err) {
		return cfg, nil
	}
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

// SaveConfig writes configuration to disk.
func SaveConfig(cfg *Config) error {
	dir := ConfigDir()
	if err := os.MkdirAll(dir, 0750); err != nil {
		return err
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(ConfigPath(), data, 0600)
}

// ConfigExists returns true if a config file exists.
func ConfigExists() bool {
	_, err := os.Stat(ConfigPath())
	return err == nil
}
