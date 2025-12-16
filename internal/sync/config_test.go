// ABOUTME: Tests for sync configuration
// ABOUTME: Verifies config load, save, and validation
package sync

import (
	"os"
	"path/filepath"
	"testing"
)

func TestConfigLoadSave(t *testing.T) {
	// Use temp directory
	tmpDir := t.TempDir()
	if err := os.Setenv("HOME", tmpDir); err != nil {
		t.Fatalf("set HOME: %v", err)
	}
	defer func() {
		_ = os.Unsetenv("HOME")
	}()

	// Initial load should return empty config
	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("initial load: %v", err)
	}
	if cfg.IsConfigured() {
		t.Error("empty config should not be configured")
	}

	// Save config
	cfg.Server = "https://api.storeusa.org"
	cfg.UserID = "user123"
	cfg.Token = "token123"
	cfg.DerivedKey = "deadbeef"
	cfg.DeviceID = "device123"

	if err := SaveConfig(cfg); err != nil {
		t.Fatalf("save config: %v", err)
	}

	// Reload and verify
	cfg2, err := LoadConfig()
	if err != nil {
		t.Fatalf("reload config: %v", err)
	}

	if cfg2.Server != "https://api.storeusa.org" {
		t.Errorf("server mismatch: %s", cfg2.Server)
	}
	if !cfg2.IsConfigured() {
		t.Error("saved config should be configured")
	}
}

func TestConfigPath(t *testing.T) {
	tmpDir := t.TempDir()
	if err := os.Setenv("HOME", tmpDir); err != nil {
		t.Fatalf("set HOME: %v", err)
	}
	defer func() {
		_ = os.Unsetenv("HOME")
	}()

	path := ConfigPath()
	expected := filepath.Join(tmpDir, ".config", "chronicle", "sync.json")
	if path != expected {
		t.Errorf("expected %s, got %s", expected, path)
	}
}
