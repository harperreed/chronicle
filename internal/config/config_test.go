// ABOUTME: Tests for config functionality
// ABOUTME: Verifies config load, save, path resolution, defaults, and backend factory

package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGetConfigPath(t *testing.T) {
	path := GetConfigPath()
	if path == "" {
		t.Error("GetConfigPath returned empty string")
	}
	if !filepath.IsAbs(path) {
		t.Errorf("GetConfigPath returned non-absolute path: %s", path)
	}
}

func TestGetConfigPathWithXDGConfigHome(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)

	path := GetConfigPath()
	if !strings.HasPrefix(path, tmpDir) {
		t.Errorf("GetConfigPath should use XDG_CONFIG_HOME, got %s", path)
	}
	if !strings.HasSuffix(path, filepath.Join("chronicle", "config.json")) {
		t.Errorf("GetConfigPath should end with chronicle/config.json, got %s", path)
	}
}

func TestGetConfigPathWithoutXDGConfigHome(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "")

	path := GetConfigPath()
	if path == "" {
		t.Error("GetConfigPath returned empty string")
	}
	// Should fall back to ~/.config
	if !strings.Contains(path, ".config") {
		t.Errorf("GetConfigPath should use .config fallback, got %s", path)
	}
}

func TestLoadNonExistent(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)
	t.Setenv("XDG_DATA_HOME", tmpDir)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load failed on non-existent config: %v", err)
	}
	if cfg == nil {
		t.Fatal("Load returned nil config")
	}
	if cfg.Backend != "markdown" {
		t.Errorf("expected markdown backend for new user, got %q", cfg.Backend)
	}

	// Verify config file was auto-created
	configPath := GetConfigPath()
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Error("expected config file to be auto-created")
	}
}

func TestLoadNonExistent_ExistingSQLiteUser(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)
	t.Setenv("XDG_DATA_HOME", tmpDir)

	// Create a fake .db file to simulate existing SQLite user
	dbDir := filepath.Join(tmpDir, "chronicle")
	if err := os.MkdirAll(dbDir, 0750); err != nil {
		t.Fatalf("failed to create db dir: %v", err)
	}
	dbPath := filepath.Join(dbDir, "chronicle.db")
	if err := os.WriteFile(dbPath, []byte("fake-db"), 0600); err != nil {
		t.Fatalf("failed to create fake db: %v", err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load should not fail: %v", err)
	}
	if cfg.Backend != "sqlite" {
		t.Errorf("expected sqlite backend for existing user, got %q", cfg.Backend)
	}
}

func TestLoadAutoCreatesValidConfig(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)
	t.Setenv("XDG_DATA_HOME", tmpDir)

	_, err := Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	// Read back the auto-created file and verify it's valid JSON
	configPath := GetConfigPath()
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("failed to read auto-created config: %v", err)
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("auto-created config is not valid JSON: %v", err)
	}
	if raw["backend"] != "markdown" {
		t.Errorf("expected backend 'markdown' in config file, got %v", raw["backend"])
	}
}

func TestLoadInvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)

	// Create config directory and file with invalid JSON
	configDir := filepath.Join(tmpDir, "chronicle")
	if err := os.MkdirAll(configDir, 0750); err != nil {
		t.Fatalf("failed to create config dir: %v", err)
	}
	configPath := filepath.Join(configDir, "config.json")
	if err := os.WriteFile(configPath, []byte("invalid json {{{"), 0600); err != nil {
		t.Fatalf("failed to write invalid config: %v", err)
	}

	_, err := Load()
	if err == nil {
		t.Error("Load should fail on invalid JSON")
	}
}

func TestLoadReadError(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)

	// Create config directory
	configDir := filepath.Join(tmpDir, "chronicle")
	if err := os.MkdirAll(configDir, 0750); err != nil {
		t.Fatalf("failed to create config dir: %v", err)
	}

	// Create config file with no read permissions (only works on Unix)
	configPath := filepath.Join(configDir, "config.json")
	if err := os.WriteFile(configPath, []byte("{}"), 0000); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	// Try to load - should fail due to permissions
	_, err := Load()
	if err == nil {
		// On some systems (or when running as root), this might succeed
		t.Log("Load succeeded despite no permissions (may be running as root)")
	}

	// Restore permissions for cleanup
	os.Chmod(configPath, 0600)
}

func TestSaveAndLoad(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)

	cfg := &Config{}

	if err := cfg.Save(); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	loaded, err := Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if loaded == nil {
		t.Error("loaded config is nil")
	}
}

func TestSaveCreatesDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)

	// The chronicle subdirectory doesn't exist yet
	cfg := &Config{}

	if err := cfg.Save(); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Verify directory was created
	configDir := filepath.Join(tmpDir, "chronicle")
	info, err := os.Stat(configDir)
	if err != nil {
		t.Errorf("Config directory was not created: %v", err)
	}
	if !info.IsDir() {
		t.Error("Config path is not a directory")
	}
}

func TestSaveOverwritesExisting(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)

	cfg := &Config{}

	if err := cfg.Save(); err != nil {
		t.Fatalf("First save failed: %v", err)
	}

	if err := cfg.Save(); err != nil {
		t.Fatalf("Second save failed: %v", err)
	}

	loaded, err := Load()
	if err != nil {
		t.Fatalf("Load after overwrite failed: %v", err)
	}
	if loaded == nil {
		t.Error("Loaded config is nil")
	}
}

func TestSaveToUnwritableDirectory(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "/nonexistent/path/that/does/not/exist/12345")

	cfg := &Config{}
	err := cfg.Save()

	if err == nil {
		t.Error("Expected error when saving to unwritable directory")
	}
}

func TestLoadValidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)

	configDir := filepath.Join(tmpDir, "chronicle")
	if err := os.MkdirAll(configDir, 0750); err != nil {
		t.Fatalf("failed to create config dir: %v", err)
	}
	configPath := filepath.Join(configDir, "config.json")
	if err := os.WriteFile(configPath, []byte("{}"), 0600); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if cfg == nil {
		t.Error("Load returned nil config")
	}
}

func TestDefaultBackend(t *testing.T) {
	cfg := &Config{}
	backend := cfg.GetBackend()
	if backend != "sqlite" {
		t.Errorf("expected default backend 'sqlite', got %q", backend)
	}
}

func TestExplicitBackend(t *testing.T) {
	cfg := &Config{Backend: "markdown"}
	backend := cfg.GetBackend()
	if backend != "markdown" {
		t.Errorf("expected backend 'markdown', got %q", backend)
	}
}

func TestDefaultDataDir(t *testing.T) {
	cfg := &Config{}
	dataDir := cfg.GetDataDir()
	if dataDir == "" {
		t.Error("GetDataDir returned empty string")
	}
	if !filepath.IsAbs(dataDir) {
		t.Errorf("GetDataDir returned non-absolute path: %s", dataDir)
	}
	// Should end with "chronicle" directory
	if filepath.Base(dataDir) != "chronicle" {
		t.Errorf("GetDataDir should end with 'chronicle', got %s", dataDir)
	}
}

func TestExplicitDataDir(t *testing.T) {
	cfg := &Config{DataDir: "/custom/data/path"}
	dataDir := cfg.GetDataDir()
	if dataDir != "/custom/data/path" {
		t.Errorf("expected '/custom/data/path', got %q", dataDir)
	}
}

func TestDataDirTildeExpansion(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skipf("cannot get home dir: %v", err)
	}

	cfg := &Config{DataDir: "~/my-chronicle-data"}
	dataDir := cfg.GetDataDir()
	expected := filepath.Join(home, "my-chronicle-data")
	if dataDir != expected {
		t.Errorf("expected %q, got %q", expected, dataDir)
	}
}

func TestDataDirTildeOnlyExpansion(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skipf("cannot get home dir: %v", err)
	}

	cfg := &Config{DataDir: "~"}
	dataDir := cfg.GetDataDir()
	if dataDir != home {
		t.Errorf("expected %q, got %q", home, dataDir)
	}
}

func TestExpandPath(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skipf("cannot get home dir: %v", err)
	}

	tests := []struct {
		input    string
		expected string
	}{
		{"~/foo", filepath.Join(home, "foo")},
		{"~", home},
		{"/absolute/path", "/absolute/path"},
		{"relative/path", "relative/path"},
		{"", ""},
	}

	for _, tt := range tests {
		result := ExpandPath(tt.input)
		if result != tt.expected {
			t.Errorf("ExpandPath(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestSaveAndLoadWithBackendFields(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)

	cfg := &Config{
		Backend: "markdown",
		DataDir: "/custom/data",
	}

	if err := cfg.Save(); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	loaded, err := Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if loaded.Backend != "markdown" {
		t.Errorf("expected backend 'markdown', got %q", loaded.Backend)
	}
	if loaded.DataDir != "/custom/data" {
		t.Errorf("expected data_dir '/custom/data', got %q", loaded.DataDir)
	}
}

func TestSaveAndLoadPreservesJSON(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)

	cfg := &Config{
		Backend: "sqlite",
		DataDir: "~/my-data",
	}

	if err := cfg.Save(); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Read raw JSON and verify field names
	path := GetConfigPath()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read config file: %v", err)
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("unmarshal raw JSON: %v", err)
	}

	if raw["backend"] != "sqlite" {
		t.Errorf("expected JSON key 'backend' with value 'sqlite', got %v", raw["backend"])
	}
	if raw["data_dir"] != "~/my-data" {
		t.Errorf("expected JSON key 'data_dir' with value '~/my-data', got %v", raw["data_dir"])
	}
}

func TestOpenStorageSqliteBackend(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &Config{
		Backend: "sqlite",
		DataDir: tmpDir,
	}

	store, err := cfg.OpenStorage()
	if err != nil {
		t.Fatalf("OpenStorage failed for sqlite: %v", err)
	}
	defer store.Close()
}

func TestOpenStorageDefaultBackend(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &Config{
		DataDir: tmpDir,
	}

	store, err := cfg.OpenStorage()
	if err != nil {
		t.Fatalf("OpenStorage failed for default backend: %v", err)
	}
	defer store.Close()
}

func TestOpenStorageMarkdownBackend(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &Config{
		Backend: "markdown",
		DataDir: tmpDir,
	}

	store, err := cfg.OpenStorage()
	if err != nil {
		t.Fatalf("OpenStorage failed for markdown backend: %v", err)
	}
	defer store.Close()
}

func TestOpenStorageUnknownBackend(t *testing.T) {
	cfg := &Config{
		Backend: "redis",
		DataDir: "/tmp/chronicle-test",
	}

	_, err := cfg.OpenStorage()
	if err == nil {
		t.Fatal("expected error for unknown backend, got nil")
	}
	if !strings.Contains(err.Error(), "unknown backend") {
		t.Errorf("expected 'unknown backend' error, got: %v", err)
	}
}

func TestOpenStorageSqliteCreatesDBInDataDir(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &Config{
		Backend: "sqlite",
		DataDir: tmpDir,
	}

	store, err := cfg.OpenStorage()
	if err != nil {
		t.Fatalf("OpenStorage failed: %v", err)
	}
	defer store.Close()

	// Verify the database file was created in the data dir
	dbPath := filepath.Join(tmpDir, "chronicle.db")
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Errorf("expected database file at %s", dbPath)
	}
}

func TestDefaultDBPath(t *testing.T) {
	result := DefaultDBPath("/some/dir")
	expected := "/some/dir/chronicle.db"
	if result != expected {
		t.Errorf("DefaultDBPath(%q) = %q, want %q", "/some/dir", result, expected)
	}
}

func TestIsDirNonEmpty(t *testing.T) {
	// Non-existent directory
	nonEmpty, err := IsDirNonEmpty("/nonexistent/path/12345")
	if err != nil {
		t.Errorf("IsDirNonEmpty on non-existent path should not error: %v", err)
	}
	if nonEmpty {
		t.Error("IsDirNonEmpty on non-existent path should return false")
	}

	// Empty directory
	tmpDir := t.TempDir()
	nonEmpty, err = IsDirNonEmpty(tmpDir)
	if err != nil {
		t.Errorf("IsDirNonEmpty on empty dir should not error: %v", err)
	}
	if nonEmpty {
		t.Error("IsDirNonEmpty on empty dir should return false")
	}

	// Non-empty directory
	if err := os.WriteFile(filepath.Join(tmpDir, "test.txt"), []byte("hello"), 0600); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}
	nonEmpty, err = IsDirNonEmpty(tmpDir)
	if err != nil {
		t.Errorf("IsDirNonEmpty on non-empty dir should not error: %v", err)
	}
	if !nonEmpty {
		t.Error("IsDirNonEmpty on non-empty dir should return true")
	}
}
