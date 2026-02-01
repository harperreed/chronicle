// ABOUTME: Unit tests for the list command
// ABOUTME: Tests list command metadata and flag configuration

package cli

import (
	"testing"
)

func TestListCommand(t *testing.T) {
	t.Run("has correct metadata", func(t *testing.T) {
		if listCmd.Use != "list" {
			t.Errorf("expected Use to be 'list', got: %s", listCmd.Use)
		}

		if listCmd.Short != "List recent entries" {
			t.Errorf("expected Short description, got: %s", listCmd.Short)
		}
	})

	t.Run("has limit flag", func(t *testing.T) {
		flag := listCmd.Flags().Lookup("limit")
		if flag == nil {
			t.Fatal("expected limit flag to exist")
		}
		if flag.Shorthand != "n" {
			t.Errorf("expected limit shorthand to be 'n', got: %s", flag.Shorthand)
		}
		if flag.DefValue != "20" {
			t.Errorf("expected limit default to be '20', got: %s", flag.DefValue)
		}
	})

	t.Run("has json flag", func(t *testing.T) {
		flag := listCmd.Flags().Lookup("json")
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
			if cmd.Name() == "list" {
				found = true
				break
			}
		}
		if !found {
			t.Error("list command not registered as subcommand")
		}
	})
}
