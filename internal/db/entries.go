// ABOUTME: Entry creation and management
// ABOUTME: Handles inserting entries with tags and metadata
package db

import (
	"database/sql"
	"time"
)

type Entry struct {
	ID               int64
	Timestamp        time.Time
	Message          string
	Hostname         string
	Username         string
	WorkingDirectory string
	Tags             []string
}

// CreateEntry inserts a new entry and returns its ID
func CreateEntry(db *sql.DB, entry Entry) (int64, error) {
	tx, err := db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	// Insert entry
	result, err := tx.Exec(
		"INSERT INTO entries (message, hostname, username, working_directory) VALUES (?, ?, ?, ?)",
		entry.Message, entry.Hostname, entry.Username, entry.WorkingDirectory,
	)
	if err != nil {
		return 0, err
	}

	entryID, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}

	// Insert tags
	for _, tag := range entry.Tags {
		_, err := tx.Exec("INSERT INTO tags (entry_id, tag) VALUES (?, ?)", entryID, tag)
		if err != nil {
			return 0, err
		}
	}

	if err := tx.Commit(); err != nil {
		return 0, err
	}

	return entryID, nil
}
