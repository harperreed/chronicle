// ABOUTME: Unit tests for the MCP command
// ABOUTME: Tests MCP command metadata and registration

package cli

import (
	"strings"
	"testing"
)

func TestMCPCommand(t *testing.T) {
	t.Run("has correct metadata", func(t *testing.T) {
		if mcpCmd.Use != "mcp" {
			t.Errorf("expected Use to be 'mcp', got: %s", mcpCmd.Use)
		}

		if mcpCmd.Short != "Run the chronicle MCP server" {
			t.Errorf("expected Short description, got: %s", mcpCmd.Short)
		}

		if !strings.Contains(mcpCmd.Long, "Model Context Protocol") {
			t.Error("expected Long description to mention 'Model Context Protocol'")
		}

		if !strings.Contains(mcpCmd.Long, "stdio") {
			t.Error("expected Long description to mention 'stdio'")
		}
	})

	t.Run("is registered as subcommand", func(t *testing.T) {
		found := false
		for _, cmd := range rootCmd.Commands() {
			if cmd.Name() == "mcp" {
				found = true
				break
			}
		}
		if !found {
			t.Error("mcp command not registered as subcommand")
		}
	})

	t.Run("has RunE function set", func(t *testing.T) {
		if mcpCmd.RunE == nil {
			t.Error("expected RunE to be set")
		}
	})
}
