// ABOUTME: XDG Base Directory specification helpers
// ABOUTME: Resolves data and config directories with fallbacks
package config

import (
	"os"
	"path/filepath"
)

// GetDataHome returns XDG_DATA_HOME or fallback to ~/.local/share
func GetDataHome() string {
	if xdg := os.Getenv("XDG_DATA_HOME"); xdg != "" {
		return xdg
	}
	home := os.Getenv("HOME")
	return filepath.Join(home, ".local", "share")
}

// GetConfigHome returns XDG_CONFIG_HOME or fallback to ~/.config
func GetConfigHome() string {
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return xdg
	}
	home := os.Getenv("HOME")
	return filepath.Join(home, ".config")
}
