// ABOUTME: MCP subcommand for running the chronicle MCP server
// ABOUTME: Handles stdio transport initialization and server lifecycle
package cli

import (
	"context"
	"fmt"

	"github.com/harper/chronicle/internal/mcp"
	"github.com/spf13/cobra"
)

var mcpCmd = &cobra.Command{
	Use:   "mcp",
	Short: "Run the chronicle MCP server",
	Long:  `Start the Model Context Protocol server for AI assistants to interact with chronicle over stdio.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Create and run server
		server, err := mcp.NewServer()
		if err != nil {
			return fmt.Errorf("failed to create MCP server: %w", err)
		}
		return server.Run(context.Background())
	},
}

func init() {
	rootCmd.AddCommand(mcpCmd)
}
