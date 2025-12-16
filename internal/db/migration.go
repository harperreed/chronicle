// ABOUTME: Schema migration from INTEGER to UUID primary keys
// ABOUTME: Handles one-time migration for sync compatibility
package db

import (
	"database/sql"
	"fmt"

	"github.com/google/uuid"
)

// needsUUIDMigration checks if the entries table uses INTEGER primary key.
func needsUUIDMigration(db *sql.DB) (bool, error) {
	var colType string
	err := db.QueryRow(`
		SELECT type FROM pragma_table_info('entries') WHERE name = 'id'
	`).Scan(&colType)
	if err == sql.ErrNoRows {
		// Table doesn't exist yet, no migration needed
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("check column type: %w", err)
	}
	return colType == "INTEGER", nil
}

// migrateToUUID migrates entries and tags tables to use TEXT (UUID) primary keys.
func migrateToUUID(db *sql.DB) error {
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// Create new entries table with TEXT id
	_, err = tx.Exec(`
		CREATE TABLE entries_new (
			id TEXT PRIMARY KEY,
			timestamp DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			message TEXT NOT NULL,
			hostname TEXT NOT NULL,
			username TEXT NOT NULL,
			working_directory TEXT NOT NULL,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		return fmt.Errorf("create entries_new: %w", err)
	}

	// Migrate entries with generated UUIDs
	rows, err := tx.Query(`SELECT id, timestamp, message, hostname, username, working_directory, created_at FROM entries`)
	if err != nil {
		return fmt.Errorf("select entries: %w", err)
	}

	// Build id mapping for tags migration
	idMap := make(map[int64]string)

	for rows.Next() {
		var oldID int64
		var timestamp, message, hostname, username, workingDir, createdAt string
		if err := rows.Scan(&oldID, &timestamp, &message, &hostname, &username, &workingDir, &createdAt); err != nil {
			_ = rows.Close()
			return fmt.Errorf("scan entry: %w", err)
		}

		newID := uuid.New().String()
		idMap[oldID] = newID

		_, err = tx.Exec(`
			INSERT INTO entries_new (id, timestamp, message, hostname, username, working_directory, created_at)
			VALUES (?, ?, ?, ?, ?, ?, ?)
		`, newID, timestamp, message, hostname, username, workingDir, createdAt)
		if err != nil {
			_ = rows.Close()
			return fmt.Errorf("insert entry_new: %w", err)
		}
	}
	_ = rows.Close()

	// Create new tags table with TEXT entry_id
	_, err = tx.Exec(`
		CREATE TABLE tags_new (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			entry_id TEXT NOT NULL,
			tag TEXT NOT NULL,
			FOREIGN KEY (entry_id) REFERENCES entries_new(id) ON DELETE CASCADE
		)
	`)
	if err != nil {
		return fmt.Errorf("create tags_new: %w", err)
	}

	// Migrate tags with updated entry_id references
	tagRows, err := tx.Query(`SELECT entry_id, tag FROM tags`)
	if err != nil {
		return fmt.Errorf("select tags: %w", err)
	}

	for tagRows.Next() {
		var oldEntryID int64
		var tag string
		if err := tagRows.Scan(&oldEntryID, &tag); err != nil {
			_ = tagRows.Close()
			return fmt.Errorf("scan tag: %w", err)
		}

		newEntryID, ok := idMap[oldEntryID]
		if !ok {
			continue // orphan tag, skip
		}

		_, err = tx.Exec(`INSERT INTO tags_new (entry_id, tag) VALUES (?, ?)`, newEntryID, tag)
		if err != nil {
			_ = tagRows.Close()
			return fmt.Errorf("insert tag_new: %w", err)
		}
	}
	_ = tagRows.Close()

	// Drop FTS table and triggers (will be recreated by schema)
	_, _ = tx.Exec(`DROP TABLE IF EXISTS entries_fts`)
	_, _ = tx.Exec(`DROP TRIGGER IF EXISTS entries_ai`)
	_, _ = tx.Exec(`DROP TRIGGER IF EXISTS entries_ad`)
	_, _ = tx.Exec(`DROP TRIGGER IF EXISTS entries_au`)

	// Drop old tables
	_, err = tx.Exec(`DROP TABLE tags`)
	if err != nil {
		return fmt.Errorf("drop tags: %w", err)
	}
	_, err = tx.Exec(`DROP TABLE entries`)
	if err != nil {
		return fmt.Errorf("drop entries: %w", err)
	}

	// Rename new tables
	_, err = tx.Exec(`ALTER TABLE entries_new RENAME TO entries`)
	if err != nil {
		return fmt.Errorf("rename entries_new: %w", err)
	}
	_, err = tx.Exec(`ALTER TABLE tags_new RENAME TO tags`)
	if err != nil {
		return fmt.Errorf("rename tags_new: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit migration: %w", err)
	}

	return nil
}
