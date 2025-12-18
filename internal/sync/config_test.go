// ABOUTME: Tests for sync configuration management
// ABOUTME: Verifies config loading, saving, environment overrides, and validation

//go:build sqlite_fts5

package sync

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadConfigNoFile(t *testing.T) {
	tmpDir := t.TempDir()

	// Set XDG_CONFIG_HOME to temp dir so ConfigPath returns a path in temp dir
	t.Setenv("XDG_CONFIG_HOME", tmpDir)

	cfg, err := LoadConfig()
	require.NoError(t, err)
	require.NotNil(t, cfg)

	// Should return defaults
	assert.Equal(t, "", cfg.Server)
	assert.Equal(t, "", cfg.UserID)
	assert.Equal(t, "", cfg.Token)
	assert.Equal(t, "", cfg.DerivedKey)
	assert.Equal(t, "", cfg.DeviceID)
	assert.Contains(t, cfg.VaultDB, "vault.db")
}

func TestLoadConfigValidFile(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)

	// Create config directory structure
	configDir := filepath.Join(tmpDir, "chronicle")
	err := os.MkdirAll(configDir, 0750)
	require.NoError(t, err)

	configPath := filepath.Join(configDir, "sync.json")

	// Create valid config file
	validCfg := &Config{
		Server:       "https://example.com",
		UserID:       "user-123",
		Token:        "token-456",
		RefreshToken: "refresh-789",
		TokenExpires: "2025-12-31T23:59:59Z",
		DerivedKey:   "test-key",
		DeviceID:     "device-abc",
		VaultDB:      "/tmp/vault.db",
	}

	data, err := json.MarshalIndent(validCfg, "", "  ")
	require.NoError(t, err)
	err = os.WriteFile(configPath, data, 0600)
	require.NoError(t, err)

	cfg, err := LoadConfig()
	require.NoError(t, err)

	assert.Equal(t, validCfg.Server, cfg.Server)
	assert.Equal(t, validCfg.UserID, cfg.UserID)
	assert.Equal(t, validCfg.Token, cfg.Token)
	assert.Equal(t, validCfg.RefreshToken, cfg.RefreshToken)
	assert.Equal(t, validCfg.TokenExpires, cfg.TokenExpires)
	assert.Equal(t, validCfg.DerivedKey, cfg.DerivedKey)
	assert.Equal(t, validCfg.DeviceID, cfg.DeviceID)
	assert.Equal(t, validCfg.VaultDB, cfg.VaultDB)
}

func TestLoadConfigCorruptedFile(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)

	// Create config directory structure
	configDir := filepath.Join(tmpDir, "chronicle")
	err := os.MkdirAll(configDir, 0750)
	require.NoError(t, err)

	configPath := filepath.Join(configDir, "sync.json")

	// Create corrupted config file
	err = os.WriteFile(configPath, []byte("{ invalid json }"), 0600)
	require.NoError(t, err)

	_, err = LoadConfig()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "corrupted")

	// Verify backup was created
	files, err := os.ReadDir(configDir)
	require.NoError(t, err)

	foundBackup := false
	for _, f := range files {
		if filepath.Ext(f.Name()) != ".json" && len(f.Name()) > len("sync.json") {
			foundBackup = true
			break
		}
	}
	assert.True(t, foundBackup, "Should have created backup of corrupted config")
}

func TestSaveConfigAndRoundTrip(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)

	cfg := &Config{
		Server:     "https://vault.example.com",
		UserID:     "test-user",
		Token:      "test-token",
		DerivedKey: "derived-key-123",
		DeviceID:   "device-456",
		VaultDB:    "/tmp/vault.db",
	}

	err := SaveConfig(cfg)
	require.NoError(t, err)

	// Verify file exists
	configPath := filepath.Join(tmpDir, "chronicle", "sync.json")
	_, err = os.Stat(configPath)
	require.NoError(t, err)

	// Load it back
	loadedCfg, err := LoadConfig()
	require.NoError(t, err)

	assert.Equal(t, cfg.Server, loadedCfg.Server)
	assert.Equal(t, cfg.UserID, loadedCfg.UserID)
	assert.Equal(t, cfg.Token, loadedCfg.Token)
	assert.Equal(t, cfg.DerivedKey, loadedCfg.DerivedKey)
	assert.Equal(t, cfg.DeviceID, loadedCfg.DeviceID)
	assert.Equal(t, cfg.VaultDB, loadedCfg.VaultDB)
}

func TestEnvOverrides(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)

	// Create config directory structure
	configDir := filepath.Join(tmpDir, "chronicle")
	err := os.MkdirAll(configDir, 0750)
	require.NoError(t, err)

	configPath := filepath.Join(configDir, "sync.json")

	// Create base config
	baseCfg := &Config{
		Server:   "https://original.com",
		UserID:   "original-user",
		Token:    "original-token",
		DeviceID: "original-device",
		VaultDB:  "/original/vault.db",
	}

	data, err := json.MarshalIndent(baseCfg, "", "  ")
	require.NoError(t, err)
	err = os.WriteFile(configPath, data, 0600)
	require.NoError(t, err)

	// Set environment variables
	t.Setenv("CHRONICLE_SERVER", "https://env-override.com")
	t.Setenv("CHRONICLE_TOKEN", "env-token")
	t.Setenv("CHRONICLE_USER_ID", "env-user")
	t.Setenv("CHRONICLE_DEVICE_ID", "env-device")
	t.Setenv("CHRONICLE_VAULT_DB", tmpDir+"/env-vault.db")

	cfg, err := LoadConfig()
	require.NoError(t, err)

	// Verify env overrides took effect
	assert.Equal(t, "https://env-override.com", cfg.Server)
	assert.Equal(t, "env-token", cfg.Token)
	assert.Equal(t, "env-user", cfg.UserID)
	assert.Equal(t, "env-device", cfg.DeviceID)
	assert.Equal(t, tmpDir+"/env-vault.db", cfg.VaultDB)
}

func TestIsConfigured(t *testing.T) {
	tests := []struct {
		name     string
		config   *Config
		expected bool
	}{
		{
			name: "fully configured",
			config: &Config{
				Server:     "https://example.com",
				Token:      "token",
				UserID:     "user",
				DerivedKey: "key",
			},
			expected: true,
		},
		{
			name: "missing server",
			config: &Config{
				Server:     "",
				Token:      "token",
				UserID:     "user",
				DerivedKey: "key",
			},
			expected: false,
		},
		{
			name: "missing token",
			config: &Config{
				Server:     "https://example.com",
				Token:      "",
				UserID:     "user",
				DerivedKey: "key",
			},
			expected: false,
		},
		{
			name: "missing user id",
			config: &Config{
				Server:     "https://example.com",
				Token:      "token",
				UserID:     "",
				DerivedKey: "key",
			},
			expected: false,
		},
		{
			name: "missing derived key",
			config: &Config{
				Server:     "https://example.com",
				Token:      "token",
				UserID:     "user",
				DerivedKey: "",
			},
			expected: false,
		},
		{
			name:     "all empty",
			config:   &Config{},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.config.IsConfigured())
		})
	}
}

func TestInitConfig(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)

	cfg, err := InitConfig()
	require.NoError(t, err)
	require.NotNil(t, cfg)

	// Verify device ID was generated
	assert.NotEmpty(t, cfg.DeviceID)
	assert.Len(t, cfg.DeviceID, 26) // ULID length

	// Verify vault DB path
	configDir := filepath.Join(tmpDir, "chronicle")
	assert.Equal(t, filepath.Join(configDir, "vault.db"), cfg.VaultDB)

	// Verify file was created
	configPath := filepath.Join(configDir, "sync.json")
	_, err = os.Stat(configPath)
	require.NoError(t, err)

	// Verify can load it back
	loadedCfg, err := LoadConfig()
	require.NoError(t, err)
	assert.Equal(t, cfg.DeviceID, loadedCfg.DeviceID)
}

func TestConfigExists(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)

	// Should not exist initially
	assert.False(t, ConfigExists())

	// Create config
	cfg := &Config{DeviceID: "test"}
	err := SaveConfig(cfg)
	require.NoError(t, err)

	// Should exist now
	assert.True(t, ConfigExists())
}

func TestConfigPathIsDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)

	// Create config directory structure
	configDir := filepath.Join(tmpDir, "chronicle")
	err := os.MkdirAll(configDir, 0750)
	require.NoError(t, err)

	// Create a directory where config file should be
	configPath := filepath.Join(configDir, "sync.json")
	err = os.Mkdir(configPath, 0755)
	require.NoError(t, err)

	_, err = LoadConfig()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "is a directory")
}

func TestEnsureConfigDir(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)

	configDir := filepath.Join(tmpDir, "chronicle")

	// Should not exist
	_, err := os.Stat(configDir)
	assert.True(t, os.IsNotExist(err))

	// Ensure it
	err = EnsureConfigDir()
	require.NoError(t, err)

	// Should exist now
	info, err := os.Stat(configDir)
	require.NoError(t, err)
	assert.True(t, info.IsDir())
}

func TestEnsureConfigDirWhenFileExists(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)

	configDir := filepath.Join(tmpDir, "chronicle")

	// Create a file where directory should be
	err := os.WriteFile(configDir, []byte("test"), 0600)
	require.NoError(t, err)

	err = EnsureConfigDir()
	require.NoError(t, err)

	// Should have backed up the file and created dir
	info, err := os.Stat(configDir)
	require.NoError(t, err)
	assert.True(t, info.IsDir())

	// Verify backup exists
	files, err := os.ReadDir(tmpDir)
	require.NoError(t, err)

	foundBackup := false
	for _, f := range files {
		if filepath.Base(f.Name()) != "chronicle" && len(f.Name()) > len("chronicle") {
			foundBackup = true
			break
		}
	}
	assert.True(t, foundBackup, "Should have created backup")
}

func TestExpandPath(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		hasHome bool
	}{
		{
			name:    "tilde path",
			input:   "~/vault.db",
			hasHome: true,
		},
		{
			name:    "absolute path",
			input:   "/tmp/vault.db",
			hasHome: false,
		},
		{
			name:    "relative path",
			input:   "vault.db",
			hasHome: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := expandPath(tt.input)

			if tt.hasHome {
				// Should have expanded to home directory
				assert.NotContains(t, result, "~")
				// Result should be longer than input (expanded)
				assert.Greater(t, len(result), len(tt.input))
			} else {
				// Should be unchanged
				assert.Equal(t, tt.input, result)
			}
		})
	}
}

func TestConfigPath(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)

	path := ConfigPath()
	expected := filepath.Join(tmpDir, "chronicle", "sync.json")
	assert.Equal(t, expected, path)
}

func TestConfigPathXDGTakesPrecedence(t *testing.T) {
	xdgDir := t.TempDir()
	homeDir := t.TempDir()

	t.Setenv("XDG_CONFIG_HOME", xdgDir)
	t.Setenv("HOME", homeDir)

	path := ConfigPath()
	// Should use XDG_CONFIG_HOME, not HOME/.config
	expected := filepath.Join(xdgDir, "chronicle", "sync.json")
	assert.Equal(t, expected, path)
	assert.NotContains(t, path, homeDir)
}
