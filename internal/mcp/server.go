// ABOUTME: MCP server implementation for chronicle
// ABOUTME: Provides tools and resources for AI assistants to interact with chronicle
package mcp

import (
	"context"

	"github.com/harper/chronicle/internal/charm"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Server wraps the MCP server with chronicle-specific functionality.
type Server struct {
	mcpServer *mcp.Server
	client    *charm.Client
}

// NewServer creates a new chronicle MCP server.
func NewServer() (*Server, error) {
	impl := &mcp.Implementation{
		Name:    "chronicle",
		Version: "0.2.0",
	}

	// Get Charm client
	client, err := charm.GetClient()
	if err != nil {
		return nil, err
	}

	server := &Server{
		mcpServer: mcp.NewServer(impl, nil),
		client:    client,
	}

	// Register components
	server.registerPrompts()
	server.registerTools()
	server.registerResources()

	return server, nil
}

// Run starts the MCP server with stdio transport.
func (s *Server) Run(ctx context.Context) error {
	transport := &mcp.StdioTransport{}
	return s.mcpServer.Run(ctx, transport)
}
