//go:build sqlite_fts5

// ABOUTME: Tests for MCP tools
// ABOUTME: Validates tool handlers and input/output types
package mcp

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/harper/chronicle/internal/db"
)

func TestAddEntryTool(t *testing.T) {
	// Create temp DB
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	database, err := db.InitDB(dbPath)
	if err != nil {
		t.Fatalf("failed to init db: %v", err)
	}
	defer func() { _ = database.Close() }()

	// Create server
	server := NewServer(dbPath)

	// Test input
	input := AddEntryInput{
		Message: "test message",
		Tags:    []string{"test", "work"},
	}

	// Call handler directly
	result, output, err := server.handleAddEntry(context.Background(), nil, input)
	if err != nil {
		t.Fatalf("handleAddEntry failed: %v", err)
	}

	if result == nil {
		t.Fatal("expected non-nil result")
	}

	if output.EntryID == 0 {
		t.Error("expected non-zero entry ID")
	}

	if output.Message != "test message" {
		t.Errorf("expected message 'test message', got %s", output.Message)
	}
}
