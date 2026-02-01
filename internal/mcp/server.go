// ABOUTME: MCP server implementation for chronicle
// ABOUTME: Provides tools and resources for AI assistants to interact with chronicle
package mcp

import (
	"context"

	"github.com/harper/chronicle/internal/storage"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Server wraps the MCP server with chronicle-specific functionality.
type Server struct {
	mcpServer *mcp.Server
	store     *storage.Store
}

// NewServer creates a new chronicle MCP server.
func NewServer() (*Server, error) {
	impl := &mcp.Implementation{
		Name:    "chronicle",
		Version: "0.3.0",
	}

	// Open storage
	store, err := storage.NewStore(storage.DefaultPath())
	if err != nil {
		return nil, err
	}

	server := &Server{
		mcpServer: mcp.NewServer(impl, nil),
		store:     store,
	}

	// Register components
	server.registerPrompts()
	server.registerTools()
	server.registerResources()

	return server, nil
}

// Run starts the MCP server with stdio transport.
func (s *Server) Run(ctx context.Context) error {
	defer func() { _ = s.store.Close() }()
	transport := &mcp.StdioTransport{}
	return s.mcpServer.Run(ctx, transport)
}
