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

// ListEntries returns the most recent entries, limited by limit
func ListEntries(db *sql.DB, limit int) ([]Entry, error) {
	query := `
		SELECT id, timestamp, message, hostname, username, working_directory
		FROM entries
		ORDER BY timestamp DESC
		LIMIT ?
	`

	rows, err := db.Query(query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []Entry
	for rows.Next() {
		var entry Entry
		err := rows.Scan(&entry.ID, &entry.Timestamp, &entry.Message, &entry.Hostname, &entry.Username, &entry.WorkingDirectory)
		if err != nil {
			return nil, err
		}

		// Load tags for this entry
		tagRows, err := db.Query("SELECT tag FROM tags WHERE entry_id = ? ORDER BY tag", entry.ID)
		if err != nil {
			return nil, err
		}

		var tags []string
		for tagRows.Next() {
			var tag string
			if err := tagRows.Scan(&tag); err != nil {
				tagRows.Close()
				return nil, err
			}
			tags = append(tags, tag)
		}
		tagRows.Close()
		entry.Tags = tags

		entries = append(entries, entry)
	}

	return entries, rows.Err()
}

type SearchParams struct {
	Text  string
	Tags  []string
	Since *time.Time
	Until *time.Time
	Limit int
}

// SearchEntries searches entries based on parameters
func SearchEntries(db *sql.DB, params SearchParams) ([]Entry, error) {
	query := "SELECT DISTINCT e.id, e.timestamp, e.message, e.hostname, e.username, e.working_directory FROM entries e"
	var conditions []string
	var args []interface{}

	// Full-text search
	if params.Text != "" {
		query += " JOIN entries_fts ON entries_fts.rowid = e.id"
		conditions = append(conditions, "entries_fts MATCH ?")
		args = append(args, params.Text)
	}

	// Tag filter
	if len(params.Tags) > 0 {
		query += " JOIN tags t ON t.entry_id = e.id"
		placeholders := ""
		for i, tag := range params.Tags {
			if i > 0 {
				placeholders += " OR "
			}
			placeholders += "t.tag = ?"
			args = append(args, tag)
		}
		conditions = append(conditions, "("+placeholders+")")
	}

	// Date range
	if params.Since != nil {
		conditions = append(conditions, "e.timestamp >= ?")
		args = append(args, params.Since)
	}
	if params.Until != nil {
		conditions = append(conditions, "e.timestamp <= ?")
		args = append(args, params.Until)
	}

	// Build WHERE clause
	if len(conditions) > 0 {
		query += " WHERE " + conditions[0]
		for i := 1; i < len(conditions); i++ {
			query += " AND " + conditions[i]
		}
	}

	query += " ORDER BY e.timestamp DESC"

	if params.Limit > 0 {
		query += " LIMIT ?"
		args = append(args, params.Limit)
	}

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []Entry
	for rows.Next() {
		var entry Entry
		err := rows.Scan(&entry.ID, &entry.Timestamp, &entry.Message, &entry.Hostname, &entry.Username, &entry.WorkingDirectory)
		if err != nil {
			return nil, err
		}

		// Load tags
		tagRows, err := db.Query("SELECT tag FROM tags WHERE entry_id = ? ORDER BY tag", entry.ID)
		if err != nil {
			return nil, err
		}

		var tags []string
		for tagRows.Next() {
			var tag string
			if err := tagRows.Scan(&tag); err != nil {
				tagRows.Close()
				return nil, err
			}
			tags = append(tags, tag)
		}
		tagRows.Close()
		entry.Tags = tags

		entries = append(entries, entry)
	}

	return entries, rows.Err()
}
