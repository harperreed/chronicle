// ABOUTME: Database connection and initialization. Requires build tag: sqlite_fts5
// ABOUTME: Handles SQLite setup and schema migration
package db

import (
	"database/sql"
	"os"
	"path/filepath"

	_ "github.com/mattn/go-sqlite3"
)

// InitDB initializes the database at the given path
func InitDB(dbPath string) (*sql.DB, error) {
	// Ensure directory exists
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}

	// Open database
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, err
	}

	// Enable foreign key constraints
	if _, err := db.Exec("PRAGMA foreign_keys = ON"); err != nil {
		db.Close()
		return nil, err
	}

	// Set performance pragmas
	if _, err := db.Exec("PRAGMA journal_mode = WAL"); err != nil {
		db.Close()
		return nil, err
	}
	if _, err := db.Exec("PRAGMA synchronous = NORMAL"); err != nil {
		db.Close()
		return nil, err
	}

	// Execute schema
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, err
	}

	return db, nil
}
