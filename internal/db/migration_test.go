//go:build sqlite_fts5

// ABOUTME: Tests for UUID schema migration
// ABOUTME: Verifies entries migrate from INTEGER to TEXT primary keys
package db

import (
	"database/sql"
	"path/filepath"
	"testing"
)

func TestMigrateToUUID(t *testing.T) {
	// Create temp database with old schema
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}

	// Enable foreign keys for test database
	if _, err := db.Exec("PRAGMA foreign_keys = ON"); err != nil {
		t.Fatalf("enable foreign keys: %v", err)
	}

	// Create old schema with INTEGER id
	oldSchema := `
		CREATE TABLE entries (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			timestamp DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			message TEXT NOT NULL,
			hostname TEXT NOT NULL,
			username TEXT NOT NULL,
			working_directory TEXT NOT NULL,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		);
		CREATE TABLE tags (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			entry_id INTEGER NOT NULL,
			tag TEXT NOT NULL,
			FOREIGN KEY (entry_id) REFERENCES entries(id) ON DELETE CASCADE
		);
	`
	if _, err := db.Exec(oldSchema); err != nil {
		t.Fatalf("create old schema: %v", err)
	}

	// Insert test data
	result, err := db.Exec(
		"INSERT INTO entries (message, hostname, username, working_directory) VALUES (?, ?, ?, ?)",
		"test message", "host1", "user1", "/home/user1",
	)
	if err != nil {
		t.Fatalf("insert entry: %v", err)
	}
	entryID, _ := result.LastInsertId()

	_, err = db.Exec("INSERT INTO tags (entry_id, tag) VALUES (?, ?)", entryID, "test-tag")
	if err != nil {
		t.Fatalf("insert tag: %v", err)
	}

	_ = db.Close()

	// Run migration
	db, err = InitDB(dbPath)
	if err != nil {
		t.Fatalf("init db with migration: %v", err)
	}
	defer func() { _ = db.Close() }()

	// Verify entry has UUID
	var id string
	err = db.QueryRow("SELECT id FROM entries LIMIT 1").Scan(&id)
	if err != nil {
		t.Fatalf("query migrated entry: %v", err)
	}

	// UUID should be 36 chars (8-4-4-4-12 format)
	if len(id) != 36 {
		t.Errorf("expected UUID (36 chars), got %q (%d chars)", id, len(id))
	}

	// Verify tag still linked
	var tagCount int
	err = db.QueryRow("SELECT COUNT(*) FROM tags WHERE entry_id = ?", id).Scan(&tagCount)
	if err != nil {
		t.Fatalf("query tags: %v", err)
	}
	if tagCount != 1 {
		t.Errorf("expected 1 tag, got %d", tagCount)
	}
}
