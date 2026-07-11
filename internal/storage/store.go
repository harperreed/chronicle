// ABOUTME: SQLite storage backend for chronicle entries
// ABOUTME: Implements Storage interface using modernc.org/sqlite with FTS5 full-text search

package storage

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	_ "modernc.org/sqlite"
)

// SqliteStore provides SQLite storage for chronicle entries.
type SqliteStore struct {
	db             *sql.DB
	lastModifiedMu sync.RWMutex
	lastModified   time.Time
}

// Compile-time check that SqliteStore implements Storage.
var _ Storage = (*SqliteStore)(nil)

var sqliteMigrationMu sync.Mutex

const (
	sqliteBusyTimeout     = 15 * time.Second
	sqliteTimestampFormat = "2006-01-02 15:04:05.999999999 -0700 MST"
)

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
	dataHome := os.Getenv("XDG_DATA_HOME")
	if dataHome == "" {
		home := os.Getenv("HOME")
		dataHome = filepath.Join(home, ".local", "share")
	}
	return filepath.Join(dataHome, "chronicle", "chronicle.db")
}

// NewSqliteStore creates a new SQLite store with the given database path.
// Use ":memory:" for an in-memory database.
func NewSqliteStore(dbPath string) (*SqliteStore, error) {
	// Create directory if needed (unless in-memory)
	if dbPath != ":memory:" {
		dir := filepath.Dir(dbPath)
		// #nosec G703 -- dbPath is the explicit caller-selected database destination, not a child path beneath a restricted root.
		if err := os.MkdirAll(dir, 0750); err != nil {
			return nil, fmt.Errorf("create data directory: %w", err)
		}
	}

	db, err := sql.Open("sqlite", sqliteDataSourceName(dbPath))
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	sqliteMigrationMu.Lock()
	defer sqliteMigrationMu.Unlock()

	// Enable WAL mode and foreign keys
	pragmas := []string{
		fmt.Sprintf("PRAGMA busy_timeout=%d", sqliteBusyTimeout.Milliseconds()),
		"PRAGMA journal_mode=WAL",
		"PRAGMA foreign_keys=ON",
	}
	for _, pragma := range pragmas {
		if _, err := db.Exec(pragma); err != nil {
			_ = db.Close()
			return nil, fmt.Errorf("set %q: %w", pragma, err)
		}
	}

	store := &SqliteStore{
		db:           db,
		lastModified: time.Now(),
	}

	if err := store.migrate(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}

	return store, nil
}

func sqliteDataSourceName(dbPath string) string {
	separator := "?"
	if strings.Contains(dbPath, "?") {
		separator = "&"
	}
	return fmt.Sprintf(
		"%s%s_txlock=immediate&_pragma=busy_timeout(%d)",
		dbPath,
		separator,
		sqliteBusyTimeout.Milliseconds(),
	)
}

// migrate runs database migrations.
func (s *SqliteStore) migrate() error {
	schema := `
	CREATE TABLE IF NOT EXISTS entries (
		rowid INTEGER PRIMARY KEY AUTOINCREMENT,
		id TEXT UNIQUE NOT NULL,
		timestamp DATETIME NOT NULL,
		timestamp_unix_seconds INTEGER,
		timestamp_nanosecond INTEGER,
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

	CREATE TRIGGER IF NOT EXISTS entries_au AFTER UPDATE OF message, tags ON entries BEGIN
		INSERT INTO entries_fts(entries_fts, rowid, message, tags) VALUES('delete', old.rowid, old.message, old.tags);
		INSERT INTO entries_fts(rowid, message, tags) VALUES (new.rowid, new.message, new.tags);
	END;
	`

	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin schema migration: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.Exec(schema); err != nil {
		return fmt.Errorf("apply schema: %w", err)
	}

	triggerNeedsScope, err := sqliteTriggerNeedsTimestampScope(tx)
	if err != nil {
		return fmt.Errorf("inspect entries update trigger: %w", err)
	}
	if triggerNeedsScope {
		if _, err := tx.Exec(`
			DROP TRIGGER entries_au;
			CREATE TRIGGER entries_au AFTER UPDATE OF message, tags ON entries BEGIN
				INSERT INTO entries_fts(entries_fts, rowid, message, tags) VALUES('delete', old.rowid, old.message, old.tags);
				INSERT INTO entries_fts(rowid, message, tags) VALUES (new.rowid, new.message, new.tags);
			END;
		`); err != nil {
			return fmt.Errorf("scope entries update trigger: %w", err)
		}
	}

	for _, column := range []string{"timestamp_unix_seconds", "timestamp_nanosecond"} {
		hasColumn, err := sqliteColumnExists(tx, "entries", column)
		if err != nil {
			return fmt.Errorf("inspect entries schema for %s: %w", column, err)
		}
		if !hasColumn {
			if _, err := tx.Exec("ALTER TABLE entries ADD COLUMN " + column + " INTEGER"); err != nil {
				return fmt.Errorf("add timestamp instant key %s: %w", column, err)
			}
		}
	}

	if err := backfillTimestampInstantKeys(tx); err != nil {
		return err
	}
	hasInstantIndex, err := sqliteSchemaObjectExists(tx, "index", "idx_entries_timestamp_instant")
	if err != nil {
		return fmt.Errorf("inspect timestamp instant index: %w", err)
	}
	if !hasInstantIndex {
		if _, err := tx.Exec(`
			CREATE INDEX idx_entries_timestamp_instant
			ON entries(timestamp_unix_seconds DESC, timestamp_nanosecond DESC)
		`); err != nil {
			return fmt.Errorf("index timestamp instant key: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit schema migration: %w", err)
	}
	return nil
}

func sqliteTriggerNeedsTimestampScope(tx *sql.Tx) (bool, error) {
	var definition string
	err := tx.QueryRow(`
		SELECT sql
		FROM sqlite_master
		WHERE type = 'trigger' AND name = 'entries_au'
	`).Scan(&definition)
	if err != nil {
		return false, err
	}
	return !strings.Contains(definition, "AFTER UPDATE OF message, tags"), nil
}

func sqliteSchemaObjectExists(tx *sql.Tx, objectType, name string) (bool, error) {
	var exists int
	err := tx.QueryRow(`
		SELECT EXISTS(
			SELECT 1 FROM sqlite_master WHERE type = ? AND name = ?
		)
	`, objectType, name).Scan(&exists)
	return exists != 0, err
}

func sqliteColumnExists(tx *sql.Tx, table, column string) (bool, error) {
	rows, err := tx.Query("PRAGMA table_info(" + table + ")")
	if err != nil {
		return false, err
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var cid int
		var name, columnType string
		var notNull, primaryKey int
		var defaultValue any
		if err := rows.Scan(&cid, &name, &columnType, &notNull, &defaultValue, &primaryKey); err != nil {
			return false, err
		}
		if name == column {
			return true, nil
		}
	}
	return false, rows.Err()
}

func backfillTimestampInstantKeys(tx *sql.Tx) error {
	rows, err := tx.Query(`
		SELECT rowid, timestamp
		FROM entries
		WHERE timestamp_unix_seconds IS NULL OR timestamp_nanosecond IS NULL
	`)
	if err != nil {
		return fmt.Errorf("query timestamp instant key backfill: %w", err)
	}

	type timestampBackfill struct {
		rowID       int64
		unixSeconds int64
		nanosecond  int
	}
	var backfills []timestampBackfill
	for rows.Next() {
		var rowID int64
		var rawTimestamp string
		if err := rows.Scan(&rowID, &rawTimestamp); err != nil {
			_ = rows.Close()
			return fmt.Errorf("scan timestamp instant key backfill: %w", err)
		}
		timestamp, err := parseTimestamp(rawTimestamp)
		if err == nil {
			backfills = append(backfills, timestampBackfill{
				rowID:       rowID,
				unixSeconds: timestamp.Unix(),
				nanosecond:  timestamp.Nanosecond(),
			})
		}
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return fmt.Errorf("iterate timestamp instant key backfill: %w", err)
	}
	if err := rows.Close(); err != nil {
		return fmt.Errorf("close timestamp instant key backfill: %w", err)
	}

	for _, backfill := range backfills {
		if _, err := tx.Exec(
			"UPDATE entries SET timestamp_unix_seconds = ?, timestamp_nanosecond = ? WHERE rowid = ?",
			backfill.unixSeconds,
			backfill.nanosecond,
			backfill.rowID,
		); err != nil {
			return fmt.Errorf("backfill timestamp instant key for row %d: %w", backfill.rowID, err)
		}
	}
	return nil
}

// Close closes the database connection.
func (s *SqliteStore) Close() error {
	return s.db.Close()
}

// CreateEntry creates a new entry and returns its ID.
func (s *SqliteStore) CreateEntry(entry Entry) (string, error) {
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
		INSERT INTO entries (id, timestamp, timestamp_unix_seconds, timestamp_nanosecond, message, hostname, username, working_directory, tags)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, entry.ID, formatTimestamp(entry.Timestamp), entry.Timestamp.Unix(), entry.Timestamp.Nanosecond(), entry.Message, entry.Hostname, entry.Username, entry.WorkingDirectory, string(tagsJSON))

	if err != nil {
		return "", fmt.Errorf("insert entry: %w", err)
	}

	s.markModified()
	return entry.ID, nil
}

// GetEntry retrieves an entry by ID.
func (s *SqliteStore) GetEntry(id string) (*Entry, error) {
	entry, timestamp, tagsJSON, err := scanEntry(s.db.QueryRow(`
		SELECT id, timestamp, message, hostname, username, working_directory, tags
		FROM entries WHERE id = ?
	`, id))

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
func (s *SqliteStore) UpdateEntry(entry Entry) error {
	if entry.ID == "" {
		return fmt.Errorf("entry ID required")
	}

	tagsJSON, err := json.Marshal(entry.Tags)
	if err != nil {
		return fmt.Errorf("marshal tags: %w", err)
	}

	result, err := s.db.Exec(`
		UPDATE entries
		SET timestamp = ?, timestamp_unix_seconds = ?, timestamp_nanosecond = ?, message = ?, hostname = ?, username = ?, working_directory = ?, tags = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, formatTimestamp(entry.Timestamp), entry.Timestamp.Unix(), entry.Timestamp.Nanosecond(), entry.Message, entry.Hostname, entry.Username, entry.WorkingDirectory, string(tagsJSON), entry.ID)

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

	s.markModified()
	return nil
}

// DeleteEntry removes an entry by ID.
func (s *SqliteStore) DeleteEntry(id string) error {
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

	s.markModified()
	return nil
}

// ListEntries returns entries ordered by timestamp descending.
func (s *SqliteStore) ListEntries(limit int) ([]Entry, error) {
	return s.SearchEntries(nil, limit)
}

// SearchEntries returns entries matching the filter.
//
//nolint:gocognit,gocyclo,nestif // Complex but straightforward query building logic
func (s *SqliteStore) SearchEntries(filter *SearchFilter, limit int) ([]Entry, error) {
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
			conditions = append(conditions, "(e.timestamp_unix_seconds IS NOT NULL AND e.timestamp_nanosecond IS NOT NULL AND (e.timestamp_unix_seconds > ? OR (e.timestamp_unix_seconds = ? AND e.timestamp_nanosecond >= ?)))")
			args = append(args, filter.Since.Unix(), filter.Since.Unix(), filter.Since.Nanosecond())
		}
		if filter.Until != nil {
			conditions = append(conditions, "(e.timestamp_unix_seconds IS NOT NULL AND e.timestamp_nanosecond IS NOT NULL AND (e.timestamp_unix_seconds < ? OR (e.timestamp_unix_seconds = ? AND e.timestamp_nanosecond <= ?)))")
			args = append(args, filter.Until.Unix(), filter.Until.Unix(), filter.Until.Nanosecond())
		}
		// Tag filter (exact membership in the JSON array)
		if len(filter.Tags) > 0 {
			tagConditions := make([]string, 0, len(filter.Tags))
			for _, tag := range filter.Tags {
				tagConditions = append(tagConditions, "EXISTS (SELECT 1 FROM json_each(CASE WHEN json_valid(e.tags) THEN CASE WHEN json_type(e.tags) = 'array' THEN e.tags ELSE '[]' END ELSE '[]' END) AS tag_value WHERE tag_value.value = ?)")
				args = append(args, tag)
			}
			conditions = append(conditions, "("+strings.Join(tagConditions, " OR ")+")")
		}
	}

	if len(conditions) > 0 {
		conditionPrefix := " WHERE "
		if filter != nil && filter.Text != "" {
			conditionPrefix = " AND "
		}
		query = strings.Join([]string{query, conditionPrefix, strings.Join(conditions, " AND ")}, "")
	}

	// Order and limit
	query += " ORDER BY (e.timestamp_unix_seconds IS NULL OR e.timestamp_nanosecond IS NULL), e.timestamp_unix_seconds DESC, e.timestamp_nanosecond DESC"
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
		entry, timestamp, tagsJSON, scanErr := scanEntry(rows)
		if scanErr != nil {
			return nil, fmt.Errorf("scan entry: %w", scanErr)
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

type entryScanner interface {
	Scan(dest ...any) error
}

// scanEntry maps nullable database metadata to the Entry zero values used by callers.
func scanEntry(scanner entryScanner) (Entry, string, string, error) {
	var entry Entry
	var timestamp string
	var hostname sql.NullString
	var username sql.NullString
	var workingDirectory sql.NullString
	var tagsJSON sql.NullString

	err := scanner.Scan(
		&entry.ID,
		&timestamp,
		&entry.Message,
		&hostname,
		&username,
		&workingDirectory,
		&tagsJSON,
	)
	if err != nil {
		return Entry{}, "", "", err
	}

	entry.Hostname = hostname.String
	entry.Username = username.String
	entry.WorkingDirectory = workingDirectory.String
	return entry, timestamp, tagsJSON.String, nil
}

// LastModified returns when the database was last modified.
func (s *SqliteStore) LastModified() time.Time {
	s.lastModifiedMu.RLock()
	defer s.lastModifiedMu.RUnlock()
	return s.lastModified
}

// markModified records a successful mutation.
func (s *SqliteStore) markModified() {
	s.lastModifiedMu.Lock()
	s.lastModified = time.Now()
	s.lastModifiedMu.Unlock()
}

// Vacuum runs SQLite VACUUM to optimize the database.
func (s *SqliteStore) Vacuum() error {
	_, err := s.db.Exec("VACUUM")
	return err
}

// Reset clears all data from the database.
func (s *SqliteStore) Reset() error {
	_, err := s.db.Exec("DELETE FROM entries")
	if err != nil {
		return fmt.Errorf("delete entries: %w", err)
	}
	s.markModified()
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
		"2006-01-02 15:04:05.999999999 -0700",
		"2006-01-02T15:04:05Z",
		"2006-01-02 15:04:05",
	}

	for _, format := range formats {
		if t, err := time.Parse(format, s); err == nil {
			return t, nil
		}
	}

	// Older SQLite driver versions appended a location name after a numeric offset.
	legacyParts := strings.Fields(s)
	if len(legacyParts) == 4 {
		legacyTimestamp := strings.Join(legacyParts[:3], " ")
		if t, err := time.Parse("2006-01-02 15:04:05.999999999 -0700", legacyTimestamp); err == nil {
			return t, nil
		}
	}

	return time.Time{}, fmt.Errorf("cannot parse timestamp: %s", s)
}

func formatTimestamp(timestamp time.Time) string {
	return timestamp.Format(sqliteTimestampFormat)
}
