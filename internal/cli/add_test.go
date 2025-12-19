// ABOUTME: Unit tests for the add command
// ABOUTME: Tests message handling and tag flag validation
package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestAddCommandArgs(t *testing.T) {
	t.Run("rejects empty message", func(t *testing.T) {
		// Reset tags before test
		tags = []string{}

		var stderr bytes.Buffer
		rootCmd.SetOut(&stderr)
		rootCmd.SetErr(&stderr)

		rootCmd.SetArgs([]string{"add", ""})
		err := rootCmd.Execute()

		if err == nil {
			t.Fatal("expected error when empty message provided, got nil")
		}

		if !strings.Contains(err.Error(), "cannot be empty") {
			t.Errorf("expected error about empty message, got: %v", err)
		}
	})

	t.Run("rejects no arguments", func(t *testing.T) {
		// Reset tags before test
		tags = []string{}

		var stderr bytes.Buffer
		rootCmd.SetOut(&stderr)
		rootCmd.SetErr(&stderr)

		rootCmd.SetArgs([]string{"add"})
		err := rootCmd.Execute()

		if err == nil {
			t.Fatal("expected error when no arguments provided, got nil")
		}

		if !strings.Contains(err.Error(), "1 arg(s)") && !strings.Contains(err.Error(), "requires") {
			t.Errorf("expected error message about required args, got: %v", err)
		}
	})

	t.Run("rejects too many arguments", func(t *testing.T) {
		// Reset tags before test
		tags = []string{}

		var stderr bytes.Buffer
		rootCmd.SetOut(&stderr)
		rootCmd.SetErr(&stderr)

		rootCmd.SetArgs([]string{"add", "message one", "message two"})
		err := rootCmd.Execute()

		if err == nil {
			t.Fatal("expected error when too many arguments provided, got nil")
		}

		if !strings.Contains(err.Error(), "1 arg(s)") && !strings.Contains(err.Error(), "accepts") {
			t.Errorf("expected error message about exact args, got: %v", err)
		}
	})

	t.Run("add command has correct metadata", func(t *testing.T) {
		if addCmd.Use != "add [message]" {
			t.Errorf("expected Use to be 'add [message]', got: %s", addCmd.Use)
		}

		if addCmd.Short != "Add a log entry" {
			t.Errorf("expected Short description, got: %s", addCmd.Short)
		}

		// Check aliases
		hasAlias := false
		for _, alias := range addCmd.Aliases {
			if alias == "a" {
				hasAlias = true
				break
			}
		}
		if !hasAlias {
			t.Error("expected 'a' alias for add command")
		}
	})

	t.Run("add command has tag flag", func(t *testing.T) {
		flag := addCmd.Flags().Lookup("tag")
		if flag == nil {
			t.Fatal("expected tag flag to exist")
		}
		if flag.Shorthand != "t" {
			t.Errorf("expected tag shorthand to be 't', got: %s", flag.Shorthand)
		}
	})
}
