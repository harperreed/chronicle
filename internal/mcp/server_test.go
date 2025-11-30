//go:build sqlite_fts5

// ABOUTME: Tests for MCP server
// ABOUTME: Validates server initialization and configuration
package mcp

import (
	"testing"
)

func TestNewServer(t *testing.T) {
	server := NewServer("/tmp/test.db")

	if server == nil {
		t.Fatal("expected server to be created")
	}

	if server.dbPath != "/tmp/test.db" {
		t.Errorf("expected dbPath to be /tmp/test.db, got %s", server.dbPath)
	}

	if server.mcpServer == nil {
		t.Fatal("expected mcpServer to be initialized")
	}
}
