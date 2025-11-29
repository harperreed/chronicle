// ABOUTME: Tests for entry creation and retrieval
// ABOUTME: Validates insert operations and metadata capture
package db

import (
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
	defer db.Close()

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

	if id == 0 {
		t.Error("expected non-zero ID")
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
	defer rows.Close()

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
