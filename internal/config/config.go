// ABOUTME: Chronicle configuration management with backend selection
// ABOUTME: Handles settings, preferences, and storage backend factory

package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/harper/chronicle/internal/storage"
)

// Config stores chronicle configuration.
type Config struct {
	// Backend selects the storage backend: "sqlite" (default) or "markdown".
	Backend string `json:"backend,omitempty"`

	// DataDir is the root directory for data storage.
	// SQLite puts chronicle.db here. Markdown puts date-organized entry files here.
	// Supports ~ expansion for home directory. Defaults to ~/.local/share/chronicle.
	DataDir string `json:"data_dir,omitempty"`
}

// GetBackend returns the configured backend, defaulting to "sqlite".
func (c *Config) GetBackend() string {
	if c.Backend == "" {
		return "sqlite"
	}
	return c.Backend
}

// GetDataDir returns the configured data directory with ~ expanded,
// defaulting to the standard XDG data directory.
func (c *Config) GetDataDir() string {
	if c.DataDir == "" {
		return filepath.Join(GetDataHome(), "chronicle")
	}
	return ExpandPath(c.DataDir)
}

// OpenStorage creates a Storage implementation based on the configured backend.
func (c *Config) OpenStorage() (storage.Storage, error) {
	backend := c.GetBackend()
	dataDir := c.GetDataDir()

	switch backend {
	case "sqlite":
		dbPath := DefaultDBPath(dataDir)
		return storage.NewSqliteStore(dbPath)
	case "markdown":
		return storage.NewMarkdownStore(dataDir)
	default:
		return nil, fmt.Errorf("unknown backend: %q", backend)
	}
}

// ExpandPath expands a leading ~ to the user's home directory.
func ExpandPath(path string) string {
	if path == "" {
		return ""
	}
	if path == "~" {
		home, _ := os.UserHomeDir()
		return home
	}
	if strings.HasPrefix(path, "~/") {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, path[2:])
	}
	return path
}

// GetConfigPath returns the config file path.
func GetConfigPath() string {
	configDir := GetConfigHome()
	return filepath.Join(configDir, "chronicle", "config.json")
}

// Load reads config from disk.
func Load() (*Config, error) {
	path := GetConfigPath()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &Config{}, nil
		}
		return nil, err
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// Save writes config to disk.
func (c *Config) Save() error {
	path := GetConfigPath()
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0750); err != nil {
		return err
	}

	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}

// DefaultDBPath returns the default database path within the given data directory.
func DefaultDBPath(dataDir string) string {
	return filepath.Join(dataDir, "chronicle.db")
}

// IsDirNonEmpty checks whether a directory exists and contains any files or subdirectories.
// Returns false if the directory does not exist or is empty.
func IsDirNonEmpty(path string) (bool, error) {
	entries, err := os.ReadDir(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("read directory %q: %w", path, err)
	}
	return len(entries) > 0, nil
}
