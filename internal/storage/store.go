// ABOUTME: SQLite storage layer for chronicle entries
// ABOUTME: Pure Go implementation using modernc.org/sqlite with FTS5 full-text search

package storage

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/harper/chronicle/internal/config"
	_ "modernc.org/sqlite"
)

// Store provides SQLite storage for chronicle entries.
type Store struct {
	db           *sql.DB
	lastModified time.Time
}

// Entry represents a chronicle log entry.
type Entry struct {
	ID               string    `json:"id"`
	Timestamp        time.Time `json:"timestamp"`
	Message          string    `json:"message"`
	Hostname         string    `json:"hostname"`
	Username         string    `json:"username"`
	WorkingDirectory string    `json:"working_directory"`
	Tags             []string  `json:"tags"`
}

// SearchFilter defines search criteria.
type SearchFilter struct {
	Text  string
	Tags  []string
	Since *time.Time
	Until *time.Time
}

// DefaultPath returns the default database path: ~/.local/share/chronicle/chronicle.db.
func DefaultPath() string {
	return filepath.Join(config.GetDataHome(), "chronicle", "chronicle.db")
}

// NewStore creates a new store with the given database path.
// Use ":memory:" for an in-memory database.
func NewStore(dbPath string) (*Store, error) {
	// Create directory if needed (unless in-memory)
	if dbPath != ":memory:" {
		dir := filepath.Dir(dbPath)
		if err := os.MkdirAll(dir, 0750); err != nil {
			return nil, fmt.Errorf("create data directory: %w", err)
		}
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	// Enable WAL mode and foreign keys
	pragmas := []string{
		"PRAGMA journal_mode=WAL",
		"PRAGMA foreign_keys=ON",
		"PRAGMA busy_timeout=5000",
	}
	for _, pragma := range pragmas {
		if _, err := db.Exec(pragma); err != nil {
			_ = db.Close()
			return nil, fmt.Errorf("set pragma: %w", err)
		}
	}

	store := &Store{
		db:           db,
		lastModified: time.Now(),
	}

	if err := store.migrate(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}

	return store, nil
}

// migrate runs database migrations.
func (s *Store) migrate() error {
	schema := `
	CREATE TABLE IF NOT EXISTS entries (
		rowid INTEGER PRIMARY KEY AUTOINCREMENT,
		id TEXT UNIQUE NOT NULL,
		timestamp DATETIME NOT NULL,
		message TEXT NOT NULL,
		hostname TEXT,
		username TEXT,
		working_directory TEXT,
		tags TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE INDEX IF NOT EXISTS idx_entries_id ON entries(id);
	CREATE INDEX IF NOT EXISTS idx_entries_timestamp ON entries(timestamp DESC);

	CREATE VIRTUAL TABLE IF NOT EXISTS entries_fts USING fts5(
		message,
		tags,
		content=entries,
		content_rowid=rowid
	);

	CREATE TRIGGER IF NOT EXISTS entries_ai AFTER INSERT ON entries BEGIN
		INSERT INTO entries_fts(rowid, message, tags) VALUES (new.rowid, new.message, new.tags);
	END;

	CREATE TRIGGER IF NOT EXISTS entries_ad AFTER DELETE ON entries BEGIN
		INSERT INTO entries_fts(entries_fts, rowid, message, tags) VALUES('delete', old.rowid, old.message, old.tags);
	END;

	CREATE TRIGGER IF NOT EXISTS entries_au AFTER UPDATE ON entries BEGIN
		INSERT INTO entries_fts(entries_fts, rowid, message, tags) VALUES('delete', old.rowid, old.message, old.tags);
		INSERT INTO entries_fts(rowid, message, tags) VALUES (new.rowid, new.message, new.tags);
	END;
	`

	_, err := s.db.Exec(schema)
	return err
}

// Close closes the database connection.
func (s *Store) Close() error {
	return s.db.Close()
}

// CreateEntry creates a new entry and returns its ID.
func (s *Store) CreateEntry(entry Entry) (string, error) {
	if entry.ID == "" {
		entry.ID = uuid.New().String()
	}

	if entry.Timestamp.IsZero() {
		entry.Timestamp = time.Now()
	}

	tagsJSON, err := json.Marshal(entry.Tags)
	if err != nil {
		return "", fmt.Errorf("marshal tags: %w", err)
	}

	_, err = s.db.Exec(`
		INSERT INTO entries (id, timestamp, message, hostname, username, working_directory, tags)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, entry.ID, entry.Timestamp, entry.Message, entry.Hostname, entry.Username, entry.WorkingDirectory, string(tagsJSON))

	if err != nil {
		return "", fmt.Errorf("insert entry: %w", err)
	}

	s.lastModified = time.Now()
	return entry.ID, nil
}

// GetEntry retrieves an entry by ID.
func (s *Store) GetEntry(id string) (*Entry, error) {
	var entry Entry
	var tagsJSON string
	var timestamp string

	err := s.db.QueryRow(`
		SELECT id, timestamp, message, hostname, username, working_directory, tags
		FROM entries WHERE id = ?
	`, id).Scan(&entry.ID, &timestamp, &entry.Message, &entry.Hostname, &entry.Username, &entry.WorkingDirectory, &tagsJSON)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("entry not found: %s", id)
	}
	if err != nil {
		return nil, fmt.Errorf("query entry: %w", err)
	}

	// Parse timestamp - try multiple formats that SQLite might return
	entry.Timestamp, err = parseTimestamp(timestamp)
	if err != nil {
		return nil, fmt.Errorf("parse timestamp: %w", err)
	}

	// Parse tags
	if tagsJSON != "" && tagsJSON != "null" {
		if err := json.Unmarshal([]byte(tagsJSON), &entry.Tags); err != nil {
			return nil, fmt.Errorf("unmarshal tags: %w", err)
		}
	}

	return &entry, nil
}

// UpdateEntry updates an existing entry.
func (s *Store) UpdateEntry(entry Entry) error {
	if entry.ID == "" {
		return fmt.Errorf("entry ID required")
	}

	tagsJSON, err := json.Marshal(entry.Tags)
	if err != nil {
		return fmt.Errorf("marshal tags: %w", err)
	}

	result, err := s.db.Exec(`
		UPDATE entries
		SET timestamp = ?, message = ?, hostname = ?, username = ?, working_directory = ?, tags = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, entry.Timestamp, entry.Message, entry.Hostname, entry.Username, entry.WorkingDirectory, string(tagsJSON), entry.ID)

	if err != nil {
		return fmt.Errorf("update entry: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("check rows affected: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("entry not found: %s", entry.ID)
	}

	s.lastModified = time.Now()
	return nil
}

// DeleteEntry removes an entry by ID.
func (s *Store) DeleteEntry(id string) error {
	result, err := s.db.Exec("DELETE FROM entries WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("delete entry: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("check rows affected: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("entry not found: %s", id)
	}

	s.lastModified = time.Now()
	return nil
}

// ListEntries returns entries ordered by timestamp descending.
func (s *Store) ListEntries(limit int) ([]Entry, error) {
	return s.SearchEntries(nil, limit)
}

// SearchEntries returns entries matching the filter.
//
//nolint:gocognit,gocyclo,nestif // Complex but straightforward query building logic
func (s *Store) SearchEntries(filter *SearchFilter, limit int) ([]Entry, error) {
	var entries []Entry
	var args []interface{}

	// Build query
	query := `
		SELECT e.id, e.timestamp, e.message, e.hostname, e.username, e.working_directory, e.tags
		FROM entries e
	`

	var conditions []string

	// FTS text search
	if filter != nil && filter.Text != "" {
		query = `
			SELECT e.id, e.timestamp, e.message, e.hostname, e.username, e.working_directory, e.tags
			FROM entries e
			JOIN entries_fts f ON e.rowid = f.rowid
			WHERE entries_fts MATCH ?
		`
		// Use FTS5 query syntax
		args = append(args, filter.Text)
	}

	// Date range filters
	if filter != nil {
		if filter.Since != nil {
			if filter.Text != "" {
				conditions = append(conditions, "e.timestamp >= ?")
			} else {
				conditions = append(conditions, "timestamp >= ?")
			}
			args = append(args, *filter.Since)
		}
		if filter.Until != nil {
			if filter.Text != "" {
				conditions = append(conditions, "e.timestamp <= ?")
			} else {
				conditions = append(conditions, "timestamp <= ?")
			}
			args = append(args, *filter.Until)
		}
		// Tag filter (search in JSON array)
		if len(filter.Tags) > 0 {
			var tagConditions []string
			for _, tag := range filter.Tags {
				if filter.Text != "" {
					tagConditions = append(tagConditions, "e.tags LIKE ?")
				} else {
					tagConditions = append(tagConditions, "tags LIKE ?")
				}
				args = append(args, "%\""+tag+"\"%")
			}
			conditions = append(conditions, "("+strings.Join(tagConditions, " OR ")+")")
		}
	}

	if len(conditions) > 0 {
		if filter != nil && filter.Text != "" {
			query += " AND " + strings.Join(conditions, " AND ")
		} else {
			query += " WHERE " + strings.Join(conditions, " AND ")
		}
	}

	// Order and limit
	if filter != nil && filter.Text != "" {
		query += " ORDER BY e.timestamp DESC"
	} else {
		query += " ORDER BY timestamp DESC"
	}
	if limit > 0 {
		query += " LIMIT ?"
		args = append(args, limit)
	}

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("query entries: %w", err)
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var entry Entry
		var tagsJSON string
		var timestamp string

		if err := rows.Scan(&entry.ID, &timestamp, &entry.Message, &entry.Hostname, &entry.Username, &entry.WorkingDirectory, &tagsJSON); err != nil {
			return nil, fmt.Errorf("scan entry: %w", err)
		}

		// Parse timestamp
		entry.Timestamp, err = parseTimestamp(timestamp)
		if err != nil {
			// Last resort - just use current time
			entry.Timestamp = time.Now()
		}

		// Parse tags
		if tagsJSON != "" && tagsJSON != "null" {
			if err := json.Unmarshal([]byte(tagsJSON), &entry.Tags); err != nil {
				entry.Tags = nil
			}
		}

		entries = append(entries, entry)
	}

	return entries, rows.Err()
}

// LastModified returns when the database was last modified.
func (s *Store) LastModified() time.Time {
	return s.lastModified
}

// Vacuum runs SQLite VACUUM to optimize the database.
func (s *Store) Vacuum() error {
	_, err := s.db.Exec("VACUUM")
	return err
}

// Reset clears all data from the database.
func (s *Store) Reset() error {
	_, err := s.db.Exec("DELETE FROM entries")
	if err != nil {
		return fmt.Errorf("delete entries: %w", err)
	}
	s.lastModified = time.Now()
	return nil
}

// parseTimestamp tries multiple timestamp formats.
func parseTimestamp(s string) (time.Time, error) {
	formats := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02T15:04:05.999999999-07:00",
		"2006-01-02T15:04:05-07:00",
		"2006-01-02 15:04:05-07:00",
		"2006-01-02T15:04:05Z",
		"2006-01-02 15:04:05",
	}

	for _, format := range formats {
		if t, err := time.Parse(format, s); err == nil {
			return t, nil
		}
	}

	return time.Time{}, fmt.Errorf("cannot parse timestamp: %s", s)
}
