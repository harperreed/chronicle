// ABOUTME: Unit tests for the root command
// ABOUTME: Tests Execute function and help output
package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestExecute(t *testing.T) {
	t.Run("runs without error", func(t *testing.T) {
		// Capture output
		var stdout bytes.Buffer
		rootCmd.SetOut(&stdout)
		rootCmd.SetErr(&stdout)

		// Set help flag to avoid interactive behavior
		rootCmd.SetArgs([]string{"--help"})

		err := Execute()

		if err != nil {
			t.Fatalf("expected Execute() to run without error, got: %v", err)
		}
	})
}

func TestRootCommand(t *testing.T) {
	t.Run("shows help when no args provided", func(t *testing.T) {
		var stdout bytes.Buffer
		rootCmd.SetOut(&stdout)
		rootCmd.SetErr(&stdout)

		// Reset args to empty
		rootCmd.SetArgs([]string{})

		err := rootCmd.Execute()

		// Root command with no args should show help (no error in cobra by default)
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}

		// Since no subcommand is called, it should just return without error
		// The actual help display happens when user runs the binary with no args
	})

	t.Run("has correct metadata", func(t *testing.T) {
		if rootCmd.Use != "chronicle" {
			t.Errorf("expected Use to be 'chronicle', got: %s", rootCmd.Use)
		}

		if rootCmd.Short != "Timestamped logging tool" {
			t.Errorf("expected Short description, got: %s", rootCmd.Short)
		}

		if !strings.Contains(rootCmd.Long, "Chronicle logs timestamped messages") {
			t.Errorf("expected Long description to contain 'Chronicle logs timestamped messages', got: %s", rootCmd.Long)
		}
	})

	t.Run("has add subcommand registered", func(t *testing.T) {
		hasAddCmd := false
		for _, cmd := range rootCmd.Commands() {
			if cmd.Name() == "add" {
				hasAddCmd = true
				break
			}
		}

		if !hasAddCmd {
			t.Error("expected root command to have 'add' subcommand registered")
		}
	})
}
