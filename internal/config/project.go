// ABOUTME: Project .chronicle file detection and config loading
// ABOUTME: Walks directory tree to find project root
package config

import (
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

type ProjectConfig struct {
	LocalLogging bool   `toml:"local_logging"`
	LogDir       string `toml:"log_dir"`
	LogFormat    string `toml:"log_format"`
}

// FindProjectRoot walks up from dir looking for .chronicle file
// Returns empty string if not found
func FindProjectRoot(dir string) (string, error) {
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return "", err
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	current := absDir
	for {
		chroniclePath := filepath.Join(current, ".chronicle")
		if _, err := os.Stat(chroniclePath); err == nil {
			return current, nil
		}

		parent := filepath.Dir(current)

		// Stop at filesystem root or home directory
		if parent == current || current == homeDir {
			return "", nil
		}

		current = parent
	}
}

// LoadProjectConfig loads .chronicle config from path
func LoadProjectConfig(path string) (*ProjectConfig, error) {
	var cfg ProjectConfig

	// Set defaults
	cfg.LogDir = "logs"
	cfg.LogFormat = "markdown"

	if _, err := toml.DecodeFile(path, &cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}
