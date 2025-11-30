//go:build sqlite_fts5

// ABOUTME: Tests for MCP tools
// ABOUTME: Validates tool handlers and input/output types
package mcp

import (
	"context"
	"fmt"
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

func TestListEntriesTool(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	database, err := db.InitDB(dbPath)
	if err != nil {
		t.Fatalf("failed to init db: %v", err)
	}
	defer func() { _ = database.Close() }()

	// Add test entries
	for i := 0; i < 5; i++ {
		entry := db.Entry{
			Message:          fmt.Sprintf("message %d", i),
			Hostname:         "testhost",
			Username:         "testuser",
			WorkingDirectory: "/test",
			Tags:             []string{"test"},
		}
		_, err := db.CreateEntry(database, entry)
		if err != nil {
			t.Fatalf("failed to create entry: %v", err)
		}
	}

	server := NewServer(dbPath)

	input := ListEntriesInput{Limit: 3}
	result, output, err := server.handleListEntries(context.Background(), nil, input)

	if err != nil {
		t.Fatalf("handleListEntries failed: %v", err)
	}

	if result == nil {
		t.Fatal("expected non-nil result")
	}

	if len(output.Entries) != 3 {
		t.Errorf("expected 3 entries, got %d", len(output.Entries))
	}
}

func TestSearchEntriesTool(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	database, err := db.InitDB(dbPath)
	if err != nil {
		t.Fatalf("failed to init db: %v", err)
	}
	defer func() { _ = database.Close() }()

	// Add test entries
	entries := []db.Entry{
		{Message: "deployed app", Hostname: "h", Username: "u", WorkingDirectory: "/", Tags: []string{"work", "deploy"}},
		{Message: "fixed bug", Hostname: "h", Username: "u", WorkingDirectory: "/", Tags: []string{"work", "bug"}},
		{Message: "wrote tests", Hostname: "h", Username: "u", WorkingDirectory: "/", Tags: []string{"test"}},
	}
	for _, entry := range entries {
		_, err := db.CreateEntry(database, entry)
		if err != nil {
			t.Fatalf("failed to create entry: %v", err)
		}
	}

	server := NewServer(dbPath)

	// Search by text
	input := SearchEntriesInput{Text: "bug"}
	result, output, err := server.handleSearchEntries(context.Background(), nil, input)

	if err != nil {
		t.Fatalf("handleSearchEntries failed: %v", err)
	}

	if result == nil {
		t.Fatal("expected non-nil result")
	}

	if len(output.Entries) != 1 {
		t.Errorf("expected 1 entry, got %d", len(output.Entries))
	}

	if output.Entries[0].Message != "fixed bug" {
		t.Errorf("expected 'fixed bug', got %s", output.Entries[0].Message)
	}
}
