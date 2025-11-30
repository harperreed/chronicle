// ABOUTME: Tests for project .chronicle file detection
// ABOUTME: Validates directory walking and config parsing
package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFindProjectRoot(t *testing.T) {
	// Create temp directory structure
	tmpDir := t.TempDir()

	projectRoot := filepath.Join(tmpDir, "project")
	subDir := filepath.Join(projectRoot, "src", "deep", "nested")
	_ = os.MkdirAll(subDir, 0755) //nolint:gosec // Test directory permissions

	// Create .chronicle file
	chronicleFile := filepath.Join(projectRoot, ".chronicle")
	_ = os.WriteFile(chronicleFile, []byte("local_logging = true\n"), 0644) //nolint:gosec // Test file permissions

	t.Run("finds project root from nested directory", func(t *testing.T) {
		root, err := FindProjectRoot(subDir)
		if err != nil {
			t.Fatalf("FindProjectRoot failed: %v", err)
		}
		if root != projectRoot {
			t.Errorf("got %s, want %s", root, projectRoot)
		}
	})

	t.Run("returns empty when no .chronicle found", func(t *testing.T) {
		otherDir := filepath.Join(tmpDir, "other")
		_ = os.MkdirAll(otherDir, 0755) //nolint:gosec // Test directory permissions

		root, err := FindProjectRoot(otherDir)
		if err != nil {
			t.Fatalf("FindProjectRoot failed: %v", err)
		}
		if root != "" {
			t.Errorf("got %s, want empty string", root)
		}
	})
}

func TestLoadProjectConfig(t *testing.T) {
	tmpDir := t.TempDir()

	configContent := `
local_logging = true
log_dir = "custom-logs"
log_format = "json"
`
	configPath := filepath.Join(tmpDir, ".chronicle")
	_ = os.WriteFile(configPath, []byte(configContent), 0644) //nolint:gosec // Test file permissions

	cfg, err := LoadProjectConfig(configPath)
	if err != nil {
		t.Fatalf("LoadProjectConfig failed: %v", err)
	}

	if !cfg.LocalLogging {
		t.Error("expected LocalLogging to be true")
	}
	if cfg.LogDir != "custom-logs" {
		t.Errorf("got LogDir %s, want custom-logs", cfg.LogDir)
	}
	if cfg.LogFormat != "json" {
		t.Errorf("got LogFormat %s, want json", cfg.LogFormat)
	}
}
