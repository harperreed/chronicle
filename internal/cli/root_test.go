// ABOUTME: Unit tests for the root command
// ABOUTME: Tests Execute function and help output
package cli

import (
	"bytes"
	"os"
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

	t.Run("executes help command", func(t *testing.T) {
		var stdout bytes.Buffer
		rootCmd.SetOut(&stdout)
		rootCmd.SetErr(&stdout)

		rootCmd.SetArgs([]string{"help"})

		err := Execute()
		if err != nil {
			t.Fatalf("expected Execute() to run without error, got: %v", err)
		}

		output := stdout.String()
		if !strings.Contains(output, "chronicle") {
			t.Error("expected help output to contain 'chronicle'")
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

	t.Run("has list subcommand registered", func(t *testing.T) {
		hasListCmd := false
		for _, cmd := range rootCmd.Commands() {
			if cmd.Name() == "list" {
				hasListCmd = true
				break
			}
		}

		if !hasListCmd {
			t.Error("expected root command to have 'list' subcommand registered")
		}
	})

	t.Run("has search subcommand registered", func(t *testing.T) {
		hasSearchCmd := false
		for _, cmd := range rootCmd.Commands() {
			if cmd.Name() == "search" {
				hasSearchCmd = true
				break
			}
		}

		if !hasSearchCmd {
			t.Error("expected root command to have 'search' subcommand registered")
		}
	})
}

func TestShouldInjectAddCommand(t *testing.T) {
	// Save original args
	originalArgs := os.Args
	defer func() { os.Args = originalArgs }()

	t.Run("returns false with no args", func(t *testing.T) {
		os.Args = []string{"chronicle"}

		result := shouldInjectAddCommand()
		if result {
			t.Error("expected false when no arguments provided")
		}
	})

	t.Run("returns false when arg is a flag", func(t *testing.T) {
		os.Args = []string{"chronicle", "--help"}

		result := shouldInjectAddCommand()
		if result {
			t.Error("expected false when arg is a flag")
		}
	})

	t.Run("returns false when arg is short flag", func(t *testing.T) {
		os.Args = []string{"chronicle", "-h"}

		result := shouldInjectAddCommand()
		if result {
			t.Error("expected false when arg is a short flag")
		}
	})

	t.Run("returns false for known command add", func(t *testing.T) {
		os.Args = []string{"chronicle", "add"}

		result := shouldInjectAddCommand()
		if result {
			t.Error("expected false for known command 'add'")
		}
	})

	t.Run("returns false for known command list", func(t *testing.T) {
		os.Args = []string{"chronicle", "list"}

		result := shouldInjectAddCommand()
		if result {
			t.Error("expected false for known command 'list'")
		}
	})

	t.Run("returns false for known command search", func(t *testing.T) {
		os.Args = []string{"chronicle", "search"}

		result := shouldInjectAddCommand()
		if result {
			t.Error("expected false for known command 'search'")
		}
	})

	t.Run("returns false for known command help", func(t *testing.T) {
		os.Args = []string{"chronicle", "help"}

		result := shouldInjectAddCommand()
		if result {
			t.Error("expected false for known command 'help'")
		}
	})

	t.Run("returns true for unknown message text", func(t *testing.T) {
		os.Args = []string{"chronicle", "deployed"}

		result := shouldInjectAddCommand()
		if !result {
			t.Error("expected true for unknown arg that should be treated as message")
		}
	})

	t.Run("returns true for message starting with letter", func(t *testing.T) {
		os.Args = []string{"chronicle", "fixed", "the", "bug"}

		result := shouldInjectAddCommand()
		if !result {
			t.Error("expected true for message text")
		}
	})

	t.Run("returns false for empty arg", func(t *testing.T) {
		os.Args = []string{"chronicle", ""}

		result := shouldInjectAddCommand()
		if result {
			t.Error("expected false for empty arg")
		}
	})
}

func TestIsKnownCommandCases(t *testing.T) {
	t.Run("recognizes add command", func(t *testing.T) {
		if !isKnownCommand("add") {
			t.Error("expected 'add' to be recognized as known command")
		}
	})

	t.Run("recognizes list command", func(t *testing.T) {
		if !isKnownCommand("list") {
			t.Error("expected 'list' to be recognized as known command")
		}
	})

	t.Run("recognizes search command", func(t *testing.T) {
		if !isKnownCommand("search") {
			t.Error("expected 'search' to be recognized as known command")
		}
	})

	t.Run("recognizes help command", func(t *testing.T) {
		if !isKnownCommand("help") {
			t.Error("expected 'help' to be recognized as known command")
		}
	})

	t.Run("returns false for unknown command", func(t *testing.T) {
		if isKnownCommand("notacommand") {
			t.Error("expected 'notacommand' to not be recognized")
		}
	})

	t.Run("returns false for message text", func(t *testing.T) {
		if isKnownCommand("deployed") {
			t.Error("expected 'deployed' to not be recognized as command")
		}
	})
}

func TestRootCommandLongDescription(t *testing.T) {
	t.Run("contains ASCII art banner", func(t *testing.T) {
		if !strings.Contains(rootCmd.Long, "██") {
			t.Error("expected Long description to contain ASCII art")
		}
	})

	t.Run("contains SQLite mention", func(t *testing.T) {
		if !strings.Contains(rootCmd.Long, "SQLite") {
			t.Error("expected Long description to mention SQLite")
		}
	})
}
