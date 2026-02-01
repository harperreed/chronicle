// ABOUTME: Tests for sync command
// ABOUTME: Validates sync command metadata and subcommand registration

package cli

import (
	"testing"
)

func TestSyncCommand(t *testing.T) {
	t.Run("has correct metadata", func(t *testing.T) {
		if syncCmd.Use != "sync" {
			t.Errorf("expected Use to be 'sync', got: %s", syncCmd.Use)
		}

		if syncCmd.Short != "Manage local chronicle database" {
			t.Errorf("expected Short description, got: %s", syncCmd.Short)
		}
	})

	t.Run("has status subcommand", func(t *testing.T) {
		found := false
		for _, cmd := range syncCmd.Commands() {
			if cmd.Name() == "status" {
				found = true
				break
			}
		}
		if !found {
			t.Error("status subcommand not found")
		}
	})

	t.Run("has repair subcommand", func(t *testing.T) {
		found := false
		for _, cmd := range syncCmd.Commands() {
			if cmd.Name() == "repair" {
				found = true
				break
			}
		}
		if !found {
			t.Error("repair subcommand not found")
		}
	})

	t.Run("has reset subcommand", func(t *testing.T) {
		found := false
		for _, cmd := range syncCmd.Commands() {
			if cmd.Name() == "reset" {
				found = true
				break
			}
		}
		if !found {
			t.Error("reset subcommand not found")
		}
	})

	t.Run("has wipe subcommand", func(t *testing.T) {
		found := false
		for _, cmd := range syncCmd.Commands() {
			if cmd.Name() == "wipe" {
				found = true
				break
			}
		}
		if !found {
			t.Error("wipe subcommand not found")
		}
	})

	t.Run("does NOT have link subcommand (removed)", func(t *testing.T) {
		found := false
		for _, cmd := range syncCmd.Commands() {
			if cmd.Name() == "link" {
				found = true
				break
			}
		}
		if found {
			t.Error("link subcommand should have been removed")
		}
	})

	t.Run("does NOT have unlink subcommand (removed)", func(t *testing.T) {
		found := false
		for _, cmd := range syncCmd.Commands() {
			if cmd.Name() == "unlink" {
				found = true
				break
			}
		}
		if found {
			t.Error("unlink subcommand should have been removed")
		}
	})

	t.Run("is registered as subcommand of root", func(t *testing.T) {
		found := false
		for _, cmd := range rootCmd.Commands() {
			if cmd.Name() == "sync" {
				found = true
				break
			}
		}
		if !found {
			t.Error("sync command not registered as subcommand")
		}
	})
}
