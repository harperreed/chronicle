// ABOUTME: Database tests for schema initialization
// ABOUTME: Validates table creation and connection handling
package db

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInitDB(t *testing.T) {
	// Create temp directory
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := InitDB(dbPath)
	if err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}
	defer db.Close()

	// Verify database file was created
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Error("database file was not created")
	}

	// Verify tables exist
	tables := []string{"entries", "tags"}
	for _, table := range tables {
		var name string
		err := db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name=?", table).Scan(&name)
		if err != nil {
			t.Errorf("table %s does not exist: %v", table, err)
		}
	}

	// Verify FTS table exists
	var ftsName string
	err = db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name='entries_fts'").Scan(&ftsName)
	if err != nil {
		t.Errorf("FTS table does not exist: %v", err)
	}
}
