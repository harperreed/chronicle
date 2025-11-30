//go:build sqlite_fts5

// ABOUTME: MCP server implementation for chronicle
// ABOUTME: Provides tools and resources for AI assistants to interact with chronicle
package mcp

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Server wraps the MCP server with chronicle-specific functionality.
type Server struct {
	mcpServer *mcp.Server
	dbPath    string
}

// NewServer creates a new chronicle MCP server.
func NewServer(dbPath string) *Server {
	impl := &mcp.Implementation{
		Name:    "chronicle",
		Version: "0.1.1",
	}

	server := &Server{
		mcpServer: mcp.NewServer(impl, nil),
		dbPath:    dbPath,
	}

	// Register components
	server.registerPrompts()
	server.registerTools()

	return server
}

// Run starts the MCP server with stdio transport.
func (s *Server) Run(ctx context.Context) error {
	transport := &mcp.StdioTransport{}
	return s.mcpServer.Run(ctx, transport)
}
