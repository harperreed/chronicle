//go:build sqlite_fts5

// ABOUTME: MCP subcommand for running the chronicle MCP server
// ABOUTME: Handles stdio transport initialization and server lifecycle
package cli

import (
	"context"
	"os"
	"path/filepath"

	"github.com/harper/chronicle/internal/config"
	"github.com/harper/chronicle/internal/mcp"
	"github.com/spf13/cobra"
)

var mcpCmd = &cobra.Command{
	Use:   "mcp",
	Short: "Run the chronicle MCP server",
	Long:  `Start the Model Context Protocol server for AI assistants to interact with chronicle over stdio.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Get database path
		dbPath := os.Getenv("CHRONICLE_DB_PATH")
		if dbPath == "" {
			dataHome := config.GetDataHome()
			dbPath = filepath.Join(dataHome, "chronicle", "chronicle.db")
		}

		// Create and run server
		server := mcp.NewServer(dbPath)
		return server.Run(context.Background())
	},
}

func init() {
	rootCmd.AddCommand(mcpCmd)
}
