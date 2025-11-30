// ABOUTME: Tests for XDG directory resolution
// ABOUTME: Validates fallback behavior and path construction
package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGetDataHome(t *testing.T) {
	// Save original env
	original := os.Getenv("XDG_DATA_HOME")
	defer func() { _ = os.Setenv("XDG_DATA_HOME", original) }()

	t.Run("uses XDG_DATA_HOME when set", func(t *testing.T) {
		_ = os.Setenv("XDG_DATA_HOME", "/custom/data")
		got := GetDataHome()
		if got != "/custom/data" {
			t.Errorf("got %s, want /custom/data", got)
		}
	})

	t.Run("falls back to HOME/.local/share", func(t *testing.T) {
		_ = os.Unsetenv("XDG_DATA_HOME")
		home := os.Getenv("HOME")
		want := filepath.Join(home, ".local", "share")
		got := GetDataHome()
		if got != want {
			t.Errorf("got %s, want %s", got, want)
		}
	})
}

func TestGetConfigHome(t *testing.T) {
	original := os.Getenv("XDG_CONFIG_HOME")
	defer func() { _ = os.Setenv("XDG_CONFIG_HOME", original) }()

	t.Run("uses XDG_CONFIG_HOME when set", func(t *testing.T) {
		_ = os.Setenv("XDG_CONFIG_HOME", "/custom/config")
		got := GetConfigHome()
		if got != "/custom/config" {
			t.Errorf("got %s, want /custom/config", got)
		}
	})

	t.Run("falls back to HOME/.config", func(t *testing.T) {
		_ = os.Unsetenv("XDG_CONFIG_HOME")
		home := os.Getenv("HOME")
		want := filepath.Join(home, ".config")
		got := GetConfigHome()
		if got != want {
			t.Errorf("got %s, want %s", got, want)
		}
	})
}
