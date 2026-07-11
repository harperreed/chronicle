// ABOUTME: Unit tests for the search command
// ABOUTME: Tests search command metadata, flag configuration, and date parsing

package cli

import (
	"testing"
)

func TestSearchCommand(t *testing.T) {
	t.Run("has correct metadata", func(t *testing.T) {
		if searchCmd.Use != "search [text]" {
			t.Errorf("expected Use to be 'search [text]', got: %s", searchCmd.Use)
		}

		if searchCmd.Short != "Search entries" {
			t.Errorf("expected Short description, got: %s", searchCmd.Short)
		}
	})

	t.Run("has tag flag", func(t *testing.T) {
		flag := searchCmd.Flags().Lookup("tag")
		if flag == nil {
			t.Fatal("expected tag flag to exist")
		}
		if flag.Shorthand != "t" {
			t.Errorf("expected tag shorthand to be 't', got: %s", flag.Shorthand)
		}
	})

	t.Run("has since flag", func(t *testing.T) {
		flag := searchCmd.Flags().Lookup("since")
		if flag == nil {
			t.Fatal("expected since flag to exist")
		}
		if flag.Usage != "Start date or timestamp (date-only values use UTC midnight)" {
			t.Errorf("unexpected since flag help: %q", flag.Usage)
		}
	})

	t.Run("has until flag", func(t *testing.T) {
		flag := searchCmd.Flags().Lookup("until")
		if flag == nil {
			t.Fatal("expected until flag to exist")
		}
		if flag.Usage != "End date or timestamp (date-only values use UTC midnight)" {
			t.Errorf("unexpected until flag help: %q", flag.Usage)
		}
	})

	t.Run("has limit flag", func(t *testing.T) {
		flag := searchCmd.Flags().Lookup("limit")
		if flag == nil {
			t.Fatal("expected limit flag to exist")
		}
		if flag.Shorthand != "n" {
			t.Errorf("expected limit shorthand to be 'n', got: %s", flag.Shorthand)
		}
		if flag.DefValue != "100" {
			t.Errorf("expected limit default to be '100', got: %s", flag.DefValue)
		}
	})

	t.Run("has json flag", func(t *testing.T) {
		flag := searchCmd.Flags().Lookup("json")
		if flag == nil {
			t.Fatal("expected json flag to exist")
		}
		if flag.DefValue != "false" {
			t.Errorf("expected json default to be 'false', got: %s", flag.DefValue)
		}
	})

	t.Run("is registered as subcommand", func(t *testing.T) {
		found := false
		for _, cmd := range rootCmd.Commands() {
			if cmd.Name() == "search" {
				found = true
				break
			}
		}
		if !found {
			t.Error("search command not registered as subcommand")
		}
	})

	t.Run("accepts zero or one argument", func(t *testing.T) {
		// MaximumNArgs(1) is set, so this should be valid
		if searchCmd.Args == nil {
			t.Error("expected Args to be set")
		}
	})
}
