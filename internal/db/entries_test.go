//go:build sqlite_fts5

// ABOUTME: Tests for entry creation and retrieval
// ABOUTME: Validates insert operations and metadata capture
package db

import (
	"fmt"
	"path/filepath"
	"testing"
	"time"
)

func TestCreateEntry(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := InitDB(dbPath)
	if err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}
	defer func() { _ = db.Close() }()

	entry := Entry{
		Message:          "test message",
		Hostname:         "testhost",
		Username:         "testuser",
		WorkingDirectory: "/test/dir",
		Tags:             []string{"work", "test"},
	}

	id, err := CreateEntry(db, entry)
	if err != nil {
		t.Fatalf("CreateEntry failed: %v", err)
	}

	if id == "" {
		t.Error("expected non-empty ID")
	}
	if len(id) != 36 {
		t.Errorf("expected UUID (36 chars), got %q (%d chars)", id, len(id))
	}

	// Verify entry was created
	var message, hostname, username, workingDir string
	var timestamp time.Time
	err = db.QueryRow("SELECT message, hostname, username, working_directory, timestamp FROM entries WHERE id = ?", id).
		Scan(&message, &hostname, &username, &workingDir, &timestamp)
	if err != nil {
		t.Fatalf("failed to query entry: %v", err)
	}

	if message != entry.Message {
		t.Errorf("got message %s, want %s", message, entry.Message)
	}
	if hostname != entry.Hostname {
		t.Errorf("got hostname %s, want %s", hostname, entry.Hostname)
	}

	// Verify tags were created
	rows, err := db.Query("SELECT tag FROM tags WHERE entry_id = ? ORDER BY tag", id)
	if err != nil {
		t.Fatalf("failed to query tags: %v", err)
	}
	defer func() { _ = rows.Close() }()

	var gotTags []string
	for rows.Next() {
		var tag string
		if err := rows.Scan(&tag); err != nil {
			t.Fatalf("failed to scan tag: %v", err)
		}
		gotTags = append(gotTags, tag)
	}

	if len(gotTags) != 2 || gotTags[0] != "test" || gotTags[1] != "work" {
		t.Errorf("got tags %v, want [test work]", gotTags)
	}
}

func TestListEntries(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := InitDB(dbPath)
	if err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}
	defer func() { _ = db.Close() }()

	// Create test entries
	for i := 0; i < 5; i++ {
		entry := Entry{
			Message:          fmt.Sprintf("message %d", i),
			Hostname:         "testhost",
			Username:         "testuser",
			WorkingDirectory: "/test/dir",
			Tags:             []string{"test"},
		}
		_, err := CreateEntry(db, entry)
		if err != nil {
			t.Fatalf("CreateEntry failed: %v", err)
		}
	}

	// List with limit
	entries, err := ListEntries(db, 3)
	if err != nil {
		t.Fatalf("ListEntries failed: %v", err)
	}

	if len(entries) != 3 {
		t.Errorf("got %d entries, want 3", len(entries))
	}

	// Verify most recent first
	if entries[0].Message != "message 4" {
		t.Errorf("got first message %s, want message 4", entries[0].Message)
	}
}

func TestSearchEntries(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	database, err := InitDB(dbPath)
	if err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}
	defer func() { _ = database.Close() }()

	// Create test entries
	entries := []Entry{
		{Message: "deployed app", Hostname: "host1", Username: "user", WorkingDirectory: "/dir", Tags: []string{"work", "deploy"}},
		{Message: "fixed bug", Hostname: "host1", Username: "user", WorkingDirectory: "/dir", Tags: []string{"work", "bug"}},
		{Message: "wrote tests", Hostname: "host1", Username: "user", WorkingDirectory: "/dir", Tags: []string{"test"}},
	}
	for _, e := range entries {
		_, err := CreateEntry(database, e)
		if err != nil {
			t.Fatalf("CreateEntry failed: %v", err)
		}
	}

	t.Run("search by text", func(t *testing.T) {
		results, err := SearchEntries(database, SearchParams{Text: "bug"})
		if err != nil {
			t.Fatalf("SearchEntries failed: %v", err)
		}
		if len(results) != 1 || results[0].Message != "fixed bug" {
			t.Errorf("got %v, want entry with 'fixed bug'", results)
		}
	})

	t.Run("search by tag", func(t *testing.T) {
		results, err := SearchEntries(database, SearchParams{Tags: []string{"work"}})
		if err != nil {
			t.Fatalf("SearchEntries failed: %v", err)
		}
		if len(results) != 2 {
			t.Errorf("got %d results, want 2", len(results))
		}
	})
}
