// ABOUTME: Tests for export command
// ABOUTME: Validates export command metadata and flag configuration

package cli

import (
	"testing"
)

func TestExportCommand(t *testing.T) {
	t.Run("has correct metadata", func(t *testing.T) {
		if exportCmd.Use != "export" {
			t.Errorf("expected Use to be 'export', got: %s", exportCmd.Use)
		}

		if exportCmd.Short != "Export entries to file" {
			t.Errorf("expected Short description, got: %s", exportCmd.Short)
		}
	})

	t.Run("has format flag", func(t *testing.T) {
		flag := exportCmd.Flags().Lookup("format")
		if flag == nil {
			t.Fatal("expected format flag to exist")
		}
		if flag.Shorthand != "f" {
			t.Errorf("expected format shorthand to be 'f', got: %s", flag.Shorthand)
		}
		if flag.DefValue != "yaml" {
			t.Errorf("expected format default to be 'yaml', got: %s", flag.DefValue)
		}
	})

	t.Run("has output flag", func(t *testing.T) {
		flag := exportCmd.Flags().Lookup("output")
		if flag == nil {
			t.Fatal("expected output flag to exist")
		}
		if flag.Shorthand != "o" {
			t.Errorf("expected output shorthand to be 'o', got: %s", flag.Shorthand)
		}
	})

	t.Run("is registered as subcommand", func(t *testing.T) {
		found := false
		for _, cmd := range rootCmd.Commands() {
			if cmd.Name() == "export" {
				found = true
				break
			}
		}
		if !found {
			t.Error("export command not registered as subcommand")
		}
	})
}
