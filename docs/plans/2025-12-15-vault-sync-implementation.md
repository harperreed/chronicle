# Vault Sync Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add E2E encrypted sync to chronicle using the vault library, enabling multi-device activity log synchronization.

**Architecture:** Local-first pattern where mutations always succeed locally, then queue for sync. Vault library handles encryption (XChaCha20-Poly1305), key derivation (BIP39), and sync protocol. UUID migration for stable cross-device entity IDs.

**Tech Stack:** Go 1.23, SQLite with FTS5, suitesync/vault library, Cobra CLI, PocketBase auth

---

## Task 1: Add suitesync Dependency

**Files:**
- Modify: `go.mod`

**Step 1: Add the vault dependency**

```bash
cd /Users/harper/Public/src/personal/suite/chronicle
go get suitesync@latest
```

**Step 2: Add replace directive for local dev (if needed)**

Edit `go.mod` to add after the require block:

```go
replace suitesync => /Users/harper/workspace/2389/suite-sync
```

**Step 3: Verify dependency resolves**

```bash
go mod tidy
```

Expected: No errors, go.sum updated

**Step 4: Commit**

```bash
git add go.mod go.sum
git commit -m "build: add suitesync vault dependency"
```

---

## Task 2: Create UUID Migration Schema

**Files:**
- Create: `internal/db/migration.go`
- Modify: `internal/db/db.go`

**Step 1: Write failing test for migration**

Create `internal/db/migration_test.go`:

```go
// ABOUTME: Tests for UUID schema migration
// ABOUTME: Verifies entries migrate from INTEGER to TEXT primary keys
package db

import (
	"database/sql"
	"os"
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

	db.Close()

	// Run migration
	db, err = InitDB(dbPath)
	if err != nil {
		t.Fatalf("init db with migration: %v", err)
	}
	defer db.Close()

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
```

**Step 2: Run test to verify it fails**

```bash
go test -tags=sqlite_fts5 -v ./internal/db/... -run TestMigrateToUUID
```

Expected: FAIL (InitDB doesn't handle migration yet)

**Step 3: Create migration.go**

Create `internal/db/migration.go`:

```go
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
			rows.Close()
			return fmt.Errorf("scan entry: %w", err)
		}

		newID := uuid.New().String()
		idMap[oldID] = newID

		_, err = tx.Exec(`
			INSERT INTO entries_new (id, timestamp, message, hostname, username, working_directory, created_at)
			VALUES (?, ?, ?, ?, ?, ?, ?)
		`, newID, timestamp, message, hostname, username, workingDir, createdAt)
		if err != nil {
			rows.Close()
			return fmt.Errorf("insert entry_new: %w", err)
		}
	}
	rows.Close()

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
			tagRows.Close()
			return fmt.Errorf("scan tag: %w", err)
		}

		newEntryID, ok := idMap[oldEntryID]
		if !ok {
			continue // orphan tag, skip
		}

		_, err = tx.Exec(`INSERT INTO tags_new (entry_id, tag) VALUES (?, ?)`, newEntryID, tag)
		if err != nil {
			tagRows.Close()
			return fmt.Errorf("insert tag_new: %w", err)
		}
	}
	tagRows.Close()

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
```

**Step 4: Update db.go to run migration**

Edit `internal/db/db.go`, add migration check after opening database but before executing schema:

```go
// ABOUTME: Database connection and initialization. Requires build tag: sqlite_fts5
// ABOUTME: Handles SQLite setup and schema migration
package db

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "github.com/mattn/go-sqlite3"
)

// InitDB initializes the database at the given path.
func InitDB(dbPath string) (*sql.DB, error) {
	// Ensure directory exists
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil { //nolint:gosec // Standard directory permissions for user data
		return nil, err
	}

	// Open database
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, err
	}

	// Enable foreign key constraints
	if _, err := db.Exec("PRAGMA foreign_keys = ON"); err != nil {
		_ = db.Close()
		return nil, err
	}

	// Set performance pragmas
	if _, err := db.Exec("PRAGMA journal_mode = WAL"); err != nil {
		_ = db.Close()
		return nil, err
	}
	if _, err := db.Exec("PRAGMA synchronous = NORMAL"); err != nil {
		_ = db.Close()
		return nil, err
	}

	// Check for and run UUID migration
	needsMigration, err := needsUUIDMigration(db)
	if err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("check migration: %w", err)
	}
	if needsMigration {
		if err := migrateToUUID(db); err != nil {
			_ = db.Close()
			return nil, fmt.Errorf("run migration: %w", err)
		}
	}

	// Execute schema
	if _, err := db.Exec(schema); err != nil {
		_ = db.Close()
		return nil, err
	}

	return db, nil
}
```

**Step 5: Update schema.go for TEXT primary key**

Edit `internal/db/schema.go`:

```go
// ABOUTME: Database schema definitions
// ABOUTME: SQL for tables, indexes, and FTS setup
package db

const schema = `
CREATE TABLE IF NOT EXISTS entries (
    id TEXT PRIMARY KEY,
    timestamp DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    message TEXT NOT NULL,
    hostname TEXT NOT NULL,
    username TEXT NOT NULL,
    working_directory TEXT NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS tags (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    entry_id TEXT NOT NULL,
    tag TEXT NOT NULL,
    FOREIGN KEY (entry_id) REFERENCES entries(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_timestamp ON entries(timestamp);
CREATE INDEX IF NOT EXISTS idx_tags_entry ON tags(entry_id);
CREATE INDEX IF NOT EXISTS idx_tags_tag ON tags(tag);

CREATE VIRTUAL TABLE IF NOT EXISTS entries_fts USING fts5(message, content=entries, content_rowid=id);

-- Triggers to keep FTS in sync
CREATE TRIGGER IF NOT EXISTS entries_ai AFTER INSERT ON entries BEGIN
  INSERT INTO entries_fts(rowid, message) VALUES (new.id, new.message);
END;

CREATE TRIGGER IF NOT EXISTS entries_ad AFTER DELETE ON entries BEGIN
  INSERT INTO entries_fts(entries_fts, rowid, message) VALUES('delete', old.id, old.message);
END;

CREATE TRIGGER IF NOT EXISTS entries_au AFTER UPDATE ON entries BEGIN
  INSERT INTO entries_fts(entries_fts, rowid, message) VALUES('delete', old.id, old.message);
  INSERT INTO entries_fts(rowid, message) VALUES (new.id, new.message);
END;
`
```

**Step 6: Run test to verify it passes**

```bash
go test -tags=sqlite_fts5 -v ./internal/db/... -run TestMigrateToUUID
```

Expected: PASS

**Step 7: Add google/uuid dependency**

```bash
go get github.com/google/uuid
go mod tidy
```

**Step 8: Commit**

```bash
git add internal/db/migration.go internal/db/migration_test.go internal/db/db.go internal/db/schema.go go.mod go.sum
git commit -m "feat: add UUID migration for sync compatibility"
```

---

## Task 3: Update Entry Struct and CreateEntry

**Files:**
- Modify: `internal/db/entries.go`
- Modify: `internal/db/entries_test.go`

**Step 1: Write failing test for UUID entry creation**

Add to `internal/db/entries_test.go`:

```go
func TestCreateEntryWithUUID(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	entry := Entry{
		Message:          "test message",
		Hostname:         "test-host",
		Username:         "test-user",
		WorkingDirectory: "/test/dir",
		Tags:             []string{"tag1", "tag2"},
	}

	id, err := CreateEntry(db, entry)
	if err != nil {
		t.Fatalf("create entry: %v", err)
	}

	// ID should be a UUID string (36 chars)
	if len(id) != 36 {
		t.Errorf("expected UUID (36 chars), got %q (%d chars)", id, len(id))
	}

	// Verify entry can be retrieved
	var message string
	err = db.QueryRow("SELECT message FROM entries WHERE id = ?", id).Scan(&message)
	if err != nil {
		t.Fatalf("query entry: %v", err)
	}
	if message != "test message" {
		t.Errorf("expected 'test message', got %q", message)
	}
}
```

**Step 2: Run test to verify it fails**

```bash
go test -tags=sqlite_fts5 -v ./internal/db/... -run TestCreateEntryWithUUID
```

Expected: FAIL (CreateEntry returns int64, not string)

**Step 3: Update Entry struct and CreateEntry**

Edit `internal/db/entries.go`:

```go
// ABOUTME: Entry creation and management
// ABOUTME: Handles inserting entries with tags and metadata
package db

import (
	"database/sql"
	"strings"
	"time"

	"github.com/google/uuid"
)

type Entry struct {
	ID               string
	Timestamp        time.Time
	Message          string
	Hostname         string
	Username         string
	WorkingDirectory string
	Tags             []string
}

// CreateEntry inserts a new entry and returns its UUID.
func CreateEntry(db *sql.DB, entry Entry) (string, error) {
	tx, err := db.Begin()
	if err != nil {
		return "", err
	}
	defer func() { _ = tx.Rollback() }()

	// Generate UUID if not provided
	entryID := entry.ID
	if entryID == "" {
		entryID = uuid.New().String()
	}

	// Insert entry
	_, err = tx.Exec(
		"INSERT INTO entries (id, message, hostname, username, working_directory) VALUES (?, ?, ?, ?, ?)",
		entryID, entry.Message, entry.Hostname, entry.Username, entry.WorkingDirectory,
	)
	if err != nil {
		return "", err
	}

	// Insert tags
	for _, tag := range entry.Tags {
		_, err := tx.Exec("INSERT INTO tags (entry_id, tag) VALUES (?, ?)", entryID, tag)
		if err != nil {
			return "", err
		}
	}

	if err := tx.Commit(); err != nil {
		return "", err
	}

	return entryID, nil
}
```

**Step 4: Update ListEntries and SearchEntries for string IDs**

Update the scan calls in `internal/db/entries.go` to use `string` instead of `int64`:

```go
// ListEntries returns the most recent entries, limited by limit.
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
	defer func() { _ = rows.Close() }()

	var entries []Entry
	var entryIDs []string
	for rows.Next() {
		var entry Entry
		err := rows.Scan(&entry.ID, &entry.Timestamp, &entry.Message, &entry.Hostname, &entry.Username, &entry.WorkingDirectory)
		if err != nil {
			return nil, err
		}
		entries = append(entries, entry)
		entryIDs = append(entryIDs, entry.ID)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Load all tags in one query instead of N queries
	if len(entryIDs) > 0 {
		tags, err := loadTagsForEntries(db, entryIDs)
		if err != nil {
			return nil, err
		}

		// Assign tags to entries
		for i := range entries {
			entries[i].Tags = tags[entries[i].ID]
		}
	}

	return entries, nil
}
```

Update `loadTagsForEntries` signature and implementation:

```go
// loadTagsForEntries loads tags for multiple entries in a single query.
func loadTagsForEntries(db *sql.DB, entryIDs []string) (map[string][]string, error) {
	if len(entryIDs) == 0 {
		return make(map[string][]string), nil
	}

	// Build IN clause with placeholders
	placeholders := make([]string, len(entryIDs))
	args := make([]interface{}, len(entryIDs))
	for i, id := range entryIDs {
		placeholders[i] = "?"
		args[i] = id
	}

	var queryBuilder strings.Builder
	queryBuilder.WriteString("SELECT entry_id, tag FROM tags WHERE entry_id IN (")
	queryBuilder.WriteString(strings.Join(placeholders, ","))
	queryBuilder.WriteString(") ORDER BY entry_id, tag")
	query := queryBuilder.String()

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	// Group tags by entry_id
	tagMap := make(map[string][]string)
	for rows.Next() {
		var entryID string
		var tag string
		if err := rows.Scan(&entryID, &tag); err != nil {
			return nil, err
		}
		tagMap[entryID] = append(tagMap[entryID], tag)
	}

	return tagMap, rows.Err()
}
```

Update `executeEntryQuery`:

```go
// executeEntryQuery runs the query and scans results into entries.
func executeEntryQuery(db *sql.DB, query string, args []interface{}) ([]Entry, []string, error) {
	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, nil, err
	}
	defer func() { _ = rows.Close() }()

	var entries []Entry
	var entryIDs []string
	for rows.Next() {
		var entry Entry
		err := rows.Scan(&entry.ID, &entry.Timestamp, &entry.Message, &entry.Hostname, &entry.Username, &entry.WorkingDirectory)
		if err != nil {
			return nil, nil, err
		}
		entries = append(entries, entry)
		entryIDs = append(entryIDs, entry.ID)
	}

	if err := rows.Err(); err != nil {
		return nil, nil, err
	}

	return entries, entryIDs, nil
}

// attachTagsToEntries loads tags and attaches them to entries.
func attachTagsToEntries(db *sql.DB, entries []Entry, entryIDs []string) error {
	if len(entryIDs) == 0 {
		return nil
	}

	tags, err := loadTagsForEntries(db, entryIDs)
	if err != nil {
		return err
	}

	for i := range entries {
		entries[i].Tags = tags[entries[i].ID]
	}

	return nil
}
```

**Step 5: Run tests to verify they pass**

```bash
go test -tags=sqlite_fts5 -v ./internal/db/...
```

Expected: All tests PASS

**Step 6: Commit**

```bash
git add internal/db/entries.go internal/db/entries_test.go
git commit -m "refactor: update Entry to use string UUID"
```

---

## Task 4: Update CLI add.go for String ID

**Files:**
- Modify: `internal/cli/add.go`

**Step 1: Update add.go to use string ID**

Edit `internal/cli/add.go`, update the printf for ID:

```go
id, err := db.CreateEntry(database, entry)
if err != nil {
    return fmt.Errorf("failed to create entry: %w", err)
}

// Fetch the specific entry we just created by ID to get its timestamp
err = database.QueryRow("SELECT timestamp FROM entries WHERE id = ?", id).Scan(&entry.Timestamp)
if err != nil {
    // If we can't get timestamp, use current time as fallback
    entry.Timestamp = time.Now()
}

fmt.Printf("Entry created (ID: %s)\n", id)
```

**Step 2: Build and verify no errors**

```bash
go build -tags=sqlite_fts5 .
```

Expected: Build succeeds

**Step 3: Commit**

```bash
git add internal/cli/add.go
git commit -m "refactor: update add command for UUID entries"
```

---

## Task 5: Create Sync Config Package

**Files:**
- Create: `internal/sync/config.go`
- Create: `internal/sync/config_test.go`

**Step 1: Write failing test for config**

Create `internal/sync/config_test.go`:

```go
// ABOUTME: Tests for sync configuration
// ABOUTME: Verifies config load, save, and validation
package sync

import (
	"os"
	"path/filepath"
	"testing"
)

func TestConfigLoadSave(t *testing.T) {
	// Use temp directory
	tmpDir := t.TempDir()
	os.Setenv("HOME", tmpDir)
	defer os.Unsetenv("HOME")

	// Initial load should return empty config
	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("initial load: %v", err)
	}
	if cfg.IsConfigured() {
		t.Error("empty config should not be configured")
	}

	// Save config
	cfg.Server = "https://api.storeusa.org"
	cfg.UserID = "user123"
	cfg.Token = "token123"
	cfg.DerivedKey = "deadbeef"
	cfg.DeviceID = "device123"

	if err := SaveConfig(cfg); err != nil {
		t.Fatalf("save config: %v", err)
	}

	// Reload and verify
	cfg2, err := LoadConfig()
	if err != nil {
		t.Fatalf("reload config: %v", err)
	}

	if cfg2.Server != "https://api.storeusa.org" {
		t.Errorf("server mismatch: %s", cfg2.Server)
	}
	if !cfg2.IsConfigured() {
		t.Error("saved config should be configured")
	}
}

func TestConfigPath(t *testing.T) {
	tmpDir := t.TempDir()
	os.Setenv("HOME", tmpDir)
	defer os.Unsetenv("HOME")

	path := ConfigPath()
	expected := filepath.Join(tmpDir, ".config", "chronicle", "sync.json")
	if path != expected {
		t.Errorf("expected %s, got %s", expected, path)
	}
}
```

**Step 2: Run test to verify it fails**

```bash
go test -tags=sqlite_fts5 -v ./internal/sync/... -run TestConfig
```

Expected: FAIL (package doesn't exist)

**Step 3: Create config.go**

Create `internal/sync/config.go`:

```go
// ABOUTME: Sync configuration management for vault integration
// ABOUTME: Handles loading, saving, and environment overrides for sync settings
package sync

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/oklog/ulid/v2"
)

// Config represents the sync configuration.
type Config struct {
	Server       string `json:"server"`
	UserID       string `json:"user_id"`
	Token        string `json:"token"`
	RefreshToken string `json:"refresh_token,omitempty"`
	TokenExpires string `json:"token_expires,omitempty"`
	DerivedKey   string `json:"derived_key"`
	DeviceID     string `json:"device_id"`
	VaultDB      string `json:"vault_db"`
	AutoSync     bool   `json:"auto_sync"`
}

// ConfigPath returns the path to the sync config file.
func ConfigPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(os.TempDir(), ".chronicle", "sync.json")
	}
	return filepath.Join(home, ".config", "chronicle", "sync.json")
}

// ConfigDir returns the directory containing the config file.
func ConfigDir() string {
	return filepath.Dir(ConfigPath())
}

// EnsureConfigDir creates the config directory if it doesn't exist.
func EnsureConfigDir() error {
	dir := ConfigDir()
	info, err := os.Stat(dir)
	if err == nil {
		if !info.IsDir() {
			backup := dir + ".backup." + time.Now().Format("20060102-150405")
			if err := os.Rename(dir, backup); err != nil {
				return fmt.Errorf("config path %s is a file, failed to backup: %w", dir, err)
			}
		} else {
			return nil
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("check config dir: %w", err)
	}
	return os.MkdirAll(dir, 0o750)
}

// LoadConfig loads config from file and applies environment variable overrides.
func LoadConfig() (*Config, error) {
	cfg := defaultConfig()

	configPath := ConfigPath()

	info, statErr := os.Stat(configPath)
	if statErr == nil && info.IsDir() {
		return nil, fmt.Errorf("config path %s is a directory, not a file", configPath)
	}

	data, err := os.ReadFile(configPath)
	if err == nil {
		if jsonErr := json.Unmarshal(data, cfg); jsonErr != nil {
			backup := configPath + ".corrupt." + time.Now().Format("20060102-150405")
			if renameErr := os.Rename(configPath, backup); renameErr == nil {
				fmt.Fprintf(os.Stderr, "Warning: corrupted config backed up to %s\n", backup)
			}
			return nil, fmt.Errorf("config file corrupted: %w", jsonErr)
		}
	} else if !os.IsNotExist(err) {
		return nil, fmt.Errorf("read config: %w", err)
	}

	applyEnvOverrides(cfg)

	if cfg.VaultDB == "" {
		cfg.VaultDB = filepath.Join(ConfigDir(), "vault.db")
	}

	return cfg, nil
}

func defaultConfig() *Config {
	return &Config{
		VaultDB: filepath.Join(ConfigDir(), "vault.db"),
	}
}

func applyEnvOverrides(cfg *Config) {
	if server := os.Getenv("CHRONICLE_SERVER"); server != "" {
		cfg.Server = server
	}
	if token := os.Getenv("CHRONICLE_TOKEN"); token != "" {
		cfg.Token = token
	}
	if userID := os.Getenv("CHRONICLE_USER_ID"); userID != "" {
		cfg.UserID = userID
	}
	if vaultDB := os.Getenv("CHRONICLE_VAULT_DB"); vaultDB != "" {
		cfg.VaultDB = expandPath(vaultDB)
	}
	if deviceID := os.Getenv("CHRONICLE_DEVICE_ID"); deviceID != "" {
		cfg.DeviceID = deviceID
	}
	if autoSync := os.Getenv("CHRONICLE_AUTO_SYNC"); autoSync == "1" || autoSync == "true" {
		cfg.AutoSync = true
	}
}

// SaveConfig writes config to file.
func SaveConfig(cfg *Config) error {
	if err := EnsureConfigDir(); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	if err := os.WriteFile(ConfigPath(), data, 0o600); err != nil {
		return fmt.Errorf("write config: %w", err)
	}

	return nil
}

// InitConfig creates a new config with device ID.
func InitConfig() (*Config, error) {
	deviceID := ulid.Make().String()

	cfg := &Config{
		DeviceID: deviceID,
		VaultDB:  filepath.Join(ConfigDir(), "vault.db"),
	}

	if err := SaveConfig(cfg); err != nil {
		return nil, err
	}

	fmt.Fprintf(os.Stderr, "Config created at %s\n", ConfigPath())
	fmt.Fprintf(os.Stderr, "Device ID: %s\n", deviceID)

	return cfg, nil
}

// ConfigExists returns true if config file exists.
func ConfigExists() bool {
	_, err := os.Stat(ConfigPath())
	return err == nil
}

// IsConfigured returns true if sync is fully configured.
func (c *Config) IsConfigured() bool {
	return c.Server != "" && c.Token != "" && c.UserID != "" && c.DerivedKey != ""
}

func expandPath(path string) string {
	if !strings.HasPrefix(path, "~/") {
		return path
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}
	return filepath.Join(home, path[2:])
}
```

**Step 4: Add ulid dependency**

```bash
go get github.com/oklog/ulid/v2
go mod tidy
```

**Step 5: Run tests to verify they pass**

```bash
go test -tags=sqlite_fts5 -v ./internal/sync/...
```

Expected: PASS

**Step 6: Commit**

```bash
git add internal/sync/config.go internal/sync/config_test.go go.mod go.sum
git commit -m "feat: add sync config management"
```

---

## Task 6: Create Syncer Package

**Files:**
- Create: `internal/sync/sync.go`
- Create: `internal/sync/sync_test.go`

**Step 1: Write failing test for syncer**

Create `internal/sync/sync_test.go`:

```go
// ABOUTME: Tests for vault syncer
// ABOUTME: Verifies change queuing and apply logic
package sync

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/harper/chronicle/internal/db"
)

func TestSyncerQueueEntry(t *testing.T) {
	tmpDir := t.TempDir()
	os.Setenv("HOME", tmpDir)
	defer os.Unsetenv("HOME")

	// Create app database
	appDBPath := filepath.Join(tmpDir, "chronicle.db")
	appDB, err := db.InitDB(appDBPath)
	if err != nil {
		t.Fatalf("init app db: %v", err)
	}
	defer appDB.Close()

	// Create config with derived key
	cfg := &Config{
		Server:     "https://api.storeusa.org",
		UserID:     "test-user",
		Token:      "test-token",
		DerivedKey: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		DeviceID:   "test-device",
		VaultDB:    filepath.Join(tmpDir, "vault.db"),
		AutoSync:   false,
	}

	syncer, err := NewSyncer(cfg, appDB)
	if err != nil {
		t.Fatalf("create syncer: %v", err)
	}
	defer syncer.Close()

	// Queue an entry change
	entry := db.Entry{
		ID:               "test-entry-uuid",
		Timestamp:        time.Now(),
		Message:          "test message",
		Hostname:         "test-host",
		Username:         "test-user",
		WorkingDirectory: "/test/dir",
		Tags:             []string{"tag1", "tag2"},
	}

	err = syncer.QueueEntryChange(context.Background(), entry, OpUpsert)
	if err != nil {
		t.Fatalf("queue entry: %v", err)
	}

	// Verify pending count
	count, err := syncer.PendingCount(context.Background())
	if err != nil {
		t.Fatalf("pending count: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 pending, got %d", count)
	}
}
```

**Step 2: Run test to verify it fails**

```bash
go test -tags=sqlite_fts5 -v ./internal/sync/... -run TestSyncerQueueEntry
```

Expected: FAIL (NewSyncer not defined)

**Step 3: Create sync.go**

Create `internal/sync/sync.go`:

```go
// ABOUTME: Vault sync integration for chronicle
// ABOUTME: Handles change queuing, syncing, and applying remote changes
package sync

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"suitesync/vault"

	"github.com/harper/chronicle/internal/db"
)

// Op represents a sync operation type.
type Op = vault.Op

// Operation constants.
const (
	OpUpsert = vault.OpUpsert
	OpDelete = vault.OpDelete
)

const (
	EntityEntry = "entry"
)

// Syncer manages vault sync for chronicle data.
type Syncer struct {
	config *Config
	store  *vault.Store
	keys   vault.Keys
	client *vault.Client
	appDB  *sql.DB
}

// NewSyncer creates a new syncer from config.
func NewSyncer(cfg *Config, appDB *sql.DB) (*Syncer, error) {
	if cfg.DerivedKey == "" {
		return nil, errors.New("derived key not configured - run 'chronicle sync login' first")
	}

	// DerivedKey is stored as hex-encoded seed
	seed, err := vault.ParseSeedPhrase(cfg.DerivedKey)
	if err != nil {
		return nil, fmt.Errorf("invalid derived key: %w", err)
	}

	keys, err := vault.DeriveKeys(seed, "", vault.DefaultKDFParams())
	if err != nil {
		return nil, fmt.Errorf("derive keys: %w", err)
	}

	store, err := vault.OpenStore(cfg.VaultDB)
	if err != nil {
		return nil, fmt.Errorf("open vault store: %w", err)
	}

	client := vault.NewClient(vault.SyncConfig{
		BaseURL:   cfg.Server,
		DeviceID:  cfg.DeviceID,
		AuthToken: cfg.Token,
	})

	return &Syncer{
		config: cfg,
		store:  store,
		keys:   keys,
		client: client,
		appDB:  appDB,
	}, nil
}

// Close releases syncer resources.
func (s *Syncer) Close() error {
	if s.store != nil {
		return s.store.Close()
	}
	return nil
}

// EntryPayload is the sync payload for an entry.
type EntryPayload struct {
	ID               string   `json:"id"`
	Timestamp        int64    `json:"timestamp"`
	Message          string   `json:"message"`
	Hostname         string   `json:"hostname"`
	Username         string   `json:"username"`
	WorkingDirectory string   `json:"working_directory"`
	Tags             []string `json:"tags"`
}

// QueueEntryChange queues a change for an entry.
func (s *Syncer) QueueEntryChange(ctx context.Context, entry db.Entry, op Op) error {
	var payload map[string]any
	if op != OpDelete {
		payload = map[string]any{
			"id":                entry.ID,
			"timestamp":         entry.Timestamp.Unix(),
			"message":           entry.Message,
			"hostname":          entry.Hostname,
			"username":          entry.Username,
			"working_directory": entry.WorkingDirectory,
			"tags":              entry.Tags,
		}
	}

	return s.queueChange(ctx, EntityEntry, entry.ID, op, payload)
}

func (s *Syncer) queueChange(ctx context.Context, entity, entityID string, op Op, payload map[string]any) error {
	change, err := vault.NewChange(entity, entityID, op, payload)
	if err != nil {
		return fmt.Errorf("create change: %w", err)
	}
	if op == OpDelete {
		change.Deleted = true
	}

	plain, err := json.Marshal(change)
	if err != nil {
		return fmt.Errorf("marshal change: %w", err)
	}

	aad := change.AAD(s.config.UserID, s.config.DeviceID)
	env, err := vault.Encrypt(s.keys.EncKey, plain, aad)
	if err != nil {
		return fmt.Errorf("encrypt change: %w", err)
	}

	if err := s.store.EnqueueEncryptedChange(ctx, change, s.config.UserID, s.config.DeviceID, env); err != nil {
		return fmt.Errorf("enqueue change: %w", err)
	}

	// Auto-sync if enabled
	if s.config.AutoSync && s.canSync() {
		return s.Sync(ctx)
	}

	return nil
}

func (s *Syncer) canSync() bool {
	return s.config.Server != "" && s.config.Token != "" && s.config.UserID != ""
}

// Sync pushes local changes and pulls remote changes.
func (s *Syncer) Sync(ctx context.Context) error {
	return s.SyncWithEvents(ctx, nil)
}

// SyncWithEvents pushes local changes and pulls remote changes with progress callbacks.
func (s *Syncer) SyncWithEvents(ctx context.Context, events *vault.SyncEvents) error {
	if !s.canSync() {
		return errors.New("sync not configured - run 'chronicle sync login' first")
	}

	return vault.Sync(ctx, s.store, s.client, s.keys, s.config.UserID, s.applyChange, events)
}

// applyChange applies a remote change to the local database.
func (s *Syncer) applyChange(ctx context.Context, c vault.Change) error {
	switch c.Entity {
	case EntityEntry:
		return s.applyEntryChange(ctx, c)
	default:
		// Ignore unknown entities for forward compatibility
		return nil
	}
}

func (s *Syncer) applyEntryChange(ctx context.Context, c vault.Change) error {
	if c.Op == OpDelete || c.Deleted {
		// Delete entry (tags cascade)
		_, err := s.appDB.ExecContext(ctx, `DELETE FROM entries WHERE id = ?`, c.EntityID)
		return err
	}

	var payload EntryPayload
	if err := json.Unmarshal(c.Payload, &payload); err != nil {
		return fmt.Errorf("unmarshal entry payload: %w", err)
	}

	timestamp := time.Unix(payload.Timestamp, 0)

	// Upsert entry
	_, err := s.appDB.ExecContext(ctx, `
		INSERT INTO entries (id, timestamp, message, hostname, username, working_directory)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			message = excluded.message,
			hostname = excluded.hostname,
			username = excluded.username,
			working_directory = excluded.working_directory
	`, payload.ID, timestamp, payload.Message, payload.Hostname, payload.Username, payload.WorkingDirectory)
	if err != nil {
		return fmt.Errorf("upsert entry: %w", err)
	}

	// Replace tags: delete existing, insert new
	_, err = s.appDB.ExecContext(ctx, `DELETE FROM tags WHERE entry_id = ?`, payload.ID)
	if err != nil {
		return fmt.Errorf("delete old tags: %w", err)
	}

	for _, tag := range payload.Tags {
		_, err = s.appDB.ExecContext(ctx, `INSERT INTO tags (entry_id, tag) VALUES (?, ?)`, payload.ID, tag)
		if err != nil {
			return fmt.Errorf("insert tag: %w", err)
		}
	}

	return nil
}

// PendingCount returns the number of changes waiting to be synced.
func (s *Syncer) PendingCount(ctx context.Context) (int, error) {
	batch, err := s.store.DequeueBatch(ctx, 1000)
	if err != nil {
		return 0, err
	}
	return len(batch), nil
}

// PendingItem represents a change waiting to be synced.
type PendingItem struct {
	ChangeID string
	Entity   string
	TS       time.Time
}

// PendingChanges returns details of changes waiting to be synced.
func (s *Syncer) PendingChanges(ctx context.Context) ([]PendingItem, error) {
	batch, err := s.store.DequeueBatch(ctx, 100)
	if err != nil {
		return nil, err
	}

	items := make([]PendingItem, len(batch))
	for i, b := range batch {
		items[i] = PendingItem{
			ChangeID: b.ChangeID,
			Entity:   b.Entity,
			TS:       time.Unix(b.TS, 0),
		}
	}
	return items, nil
}

// LastSyncedSeq returns the last pulled sequence number.
func (s *Syncer) LastSyncedSeq(ctx context.Context) (string, error) {
	return s.store.GetState(ctx, "last_pulled_seq", "0")
}
```

**Step 4: Run tests to verify they pass**

```bash
go test -tags=sqlite_fts5 -v ./internal/sync/...
```

Expected: PASS (or may need vault mock - adjust as needed)

**Step 5: Commit**

```bash
git add internal/sync/sync.go internal/sync/sync_test.go
git commit -m "feat: add vault syncer for entries"
```

---

## Task 7: Add Sync CLI Commands

**Files:**
- Create: `internal/cli/sync.go`

**Step 1: Create sync.go with all subcommands**

Create `internal/cli/sync.go`:

```go
// ABOUTME: Sync subcommand for vault integration
// ABOUTME: Provides init, login, status, now, pending, logout, and wipe commands
package cli

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"suitesync/vault"

	"github.com/harper/chronicle/internal/config"
	"github.com/harper/chronicle/internal/db"
	"github.com/harper/chronicle/internal/sync"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Manage cloud sync for chronicle data",
	Long: `Sync your chronicle data securely to the cloud using E2E encryption.

Commands:
  init    - Initialize sync configuration
  login   - Login to sync server
  status  - Show sync status
  now     - Manually trigger sync
  pending - Show changes waiting to sync
  logout  - Clear authentication
  wipe    - Clear all sync data

Examples:
  chronicle sync init
  chronicle sync login --server https://api.storeusa.org
  chronicle sync status
  chronicle sync now`,
}

var syncInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize sync configuration",
	RunE: func(cmd *cobra.Command, args []string) error {
		if sync.ConfigExists() {
			return fmt.Errorf("config already exists at %s\nUse 'chronicle sync status' to view or delete the file to reinitialize", sync.ConfigPath())
		}

		cfg, err := sync.InitConfig()
		if err != nil {
			return fmt.Errorf("failed to initialize config: %w", err)
		}

		fmt.Println("Sync initialized")
		fmt.Printf("  Config: %s\n", sync.ConfigPath())
		fmt.Printf("  Device: %s\n", cfg.DeviceID)
		fmt.Println("\nNext: Run 'chronicle sync login' to authenticate")

		return nil
	},
}

var syncLoginCmd = &cobra.Command{
	Use:   "login",
	Short: "Login to sync server",
	RunE: func(cmd *cobra.Command, args []string) error {
		server, _ := cmd.Flags().GetString("server")

		cfg, _ := sync.LoadConfig()
		if cfg == nil {
			cfg = &sync.Config{}
		}

		serverURL := server
		if serverURL == "" {
			serverURL = cfg.Server
		}
		if serverURL == "" {
			serverURL = "https://api.storeusa.org"
		}

		reader := bufio.NewReader(os.Stdin)

		fmt.Print("Email: ")
		email, _ := reader.ReadString('\n')
		email = strings.TrimSpace(email)
		if email == "" {
			return fmt.Errorf("email required")
		}

		fmt.Print("Password: ")
		passwordBytes, err := term.ReadPassword(syscall.Stdin)
		fmt.Println()
		if err != nil {
			return fmt.Errorf("read password: %w", err)
		}
		password := string(passwordBytes)

		fmt.Print("\nEnter your recovery phrase:\n> ")
		mnemonic, _ := reader.ReadString('\n')
		mnemonic = strings.TrimSpace(mnemonic)

		if _, err := vault.ParseMnemonic(mnemonic); err != nil {
			return fmt.Errorf("invalid recovery phrase: %w", err)
		}

		fmt.Printf("\nLogging in to %s...\n", serverURL)
		client := vault.NewPBAuthClient(serverURL)
		result, err := client.Login(context.Background(), email, password)
		if err != nil {
			return fmt.Errorf("login failed: %w", err)
		}

		seed, err := vault.ParseSeedPhrase(mnemonic)
		if err != nil {
			return fmt.Errorf("parse mnemonic: %w", err)
		}
		derivedKeyHex := hex.EncodeToString(seed.Raw)

		cfg.Server = serverURL
		cfg.UserID = result.UserID
		cfg.Token = result.Token.Token
		cfg.RefreshToken = result.RefreshToken
		cfg.TokenExpires = result.Token.Expires.Format(time.RFC3339)
		cfg.DerivedKey = derivedKeyHex
		if cfg.DeviceID == "" {
			cfg.DeviceID = randHex(16)
		}
		if cfg.VaultDB == "" {
			cfg.VaultDB = filepath.Join(sync.ConfigDir(), "vault.db")
		}

		if err := sync.SaveConfig(cfg); err != nil {
			return fmt.Errorf("save config: %w", err)
		}

		fmt.Println("\nLogged in to chronicle sync")
		fmt.Printf("Token expires: %s\n", result.Token.Expires.Format(time.RFC3339))

		return nil
	},
}

var syncStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show sync status",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := sync.LoadConfig()
		if err != nil {
			return fmt.Errorf("load config: %w", err)
		}

		fmt.Printf("Config:    %s\n", sync.ConfigPath())
		fmt.Printf("Server:    %s\n", valueOrNone(cfg.Server))
		fmt.Printf("User ID:   %s\n", valueOrNone(cfg.UserID))
		fmt.Printf("Device ID: %s\n", valueOrNone(cfg.DeviceID))
		fmt.Printf("Vault DB:  %s\n", valueOrNone(cfg.VaultDB))
		fmt.Printf("Auto-sync: %v\n", cfg.AutoSync)

		if cfg.DerivedKey != "" {
			fmt.Println("Keys:      configured")
		} else {
			fmt.Println("Keys:      (not set)")
		}

		printTokenStatus(cfg)

		if cfg.IsConfigured() {
			dataHome := config.GetDataHome()
			dbPath := filepath.Join(dataHome, "chronicle", "chronicle.db")
			appDB, err := db.InitDB(dbPath)
			if err == nil {
				defer appDB.Close()

				syncer, err := sync.NewSyncer(cfg, appDB)
				if err == nil {
					defer syncer.Close()
					ctx := context.Background()

					pending, err := syncer.PendingCount(ctx)
					if err == nil {
						fmt.Printf("\nPending:   %d changes\n", pending)
					}

					lastSeq, err := syncer.LastSyncedSeq(ctx)
					if err == nil && lastSeq != "0" {
						fmt.Printf("Last sync: seq %s\n", lastSeq)
					}
				}
			}
		}

		return nil
	},
}

var syncNowCmd = &cobra.Command{
	Use:   "now",
	Short: "Manually trigger sync",
	RunE: func(cmd *cobra.Command, args []string) error {
		verbose, _ := cmd.Flags().GetBool("verbose")

		cfg, err := sync.LoadConfig()
		if err != nil {
			return fmt.Errorf("load config: %w", err)
		}

		if !cfg.IsConfigured() {
			return fmt.Errorf("sync not configured - run 'chronicle sync login' first")
		}

		dataHome := config.GetDataHome()
		dbPath := filepath.Join(dataHome, "chronicle", "chronicle.db")
		appDB, err := db.InitDB(dbPath)
		if err != nil {
			return fmt.Errorf("open database: %w", err)
		}
		defer appDB.Close()

		syncer, err := sync.NewSyncer(cfg, appDB)
		if err != nil {
			return fmt.Errorf("create syncer: %w", err)
		}
		defer syncer.Close()

		ctx := context.Background()

		var events *vault.SyncEvents
		if verbose {
			events = &vault.SyncEvents{
				OnStart: func() {
					fmt.Println("Syncing...")
				},
				OnPush: func(pushed, remaining int) {
					fmt.Printf("  Pushed %d changes (%d remaining)\n", pushed, remaining)
				},
				OnPull: func(pulled int) {
					if pulled > 0 {
						fmt.Printf("  Pulled %d changes\n", pulled)
					}
				},
				OnComplete: func(pushed, pulled int) {
					fmt.Printf("  Total: %d pushed, %d pulled\n", pushed, pulled)
				},
			}
		} else {
			fmt.Println("Syncing...")
		}

		if err := syncer.SyncWithEvents(ctx, events); err != nil {
			return fmt.Errorf("sync failed: %w", err)
		}

		fmt.Println("Sync complete")
		return nil
	},
}

var syncPendingCmd = &cobra.Command{
	Use:   "pending",
	Short: "Show changes waiting to sync",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := sync.LoadConfig()
		if err != nil {
			return fmt.Errorf("load config: %w", err)
		}

		if !cfg.IsConfigured() {
			fmt.Println("Sync not configured. Run 'chronicle sync login' first.")
			return nil
		}

		dataHome := config.GetDataHome()
		dbPath := filepath.Join(dataHome, "chronicle", "chronicle.db")
		appDB, err := db.InitDB(dbPath)
		if err != nil {
			return fmt.Errorf("open database: %w", err)
		}
		defer appDB.Close()

		syncer, err := sync.NewSyncer(cfg, appDB)
		if err != nil {
			return fmt.Errorf("create syncer: %w", err)
		}
		defer syncer.Close()

		items, err := syncer.PendingChanges(context.Background())
		if err != nil {
			return fmt.Errorf("get pending: %w", err)
		}

		if len(items) == 0 {
			fmt.Println("No pending changes - everything is synced!")
			return nil
		}

		fmt.Printf("Pending changes (%d):\n\n", len(items))
		for _, item := range items {
			fmt.Printf("  %s  %-10s  %s\n",
				item.ChangeID[:8],
				item.Entity,
				item.TS.Format("2006-01-02 15:04:05"))
		}
		fmt.Printf("\nRun 'chronicle sync now' to push these changes.\n")

		return nil
	},
}

var syncLogoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "Clear authentication",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := sync.LoadConfig()
		if err != nil {
			return fmt.Errorf("load config: %w", err)
		}

		if cfg.Token == "" {
			fmt.Println("Not logged in")
			return nil
		}

		cfg.Token = ""
		cfg.RefreshToken = ""
		cfg.TokenExpires = ""

		if err := sync.SaveConfig(cfg); err != nil {
			return fmt.Errorf("save config: %w", err)
		}

		fmt.Println("Logged out successfully")
		return nil
	},
}

var syncWipeCmd = &cobra.Command{
	Use:   "wipe",
	Short: "Wipe all sync data and start fresh",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := sync.LoadConfig()
		if err != nil {
			return fmt.Errorf("load config: %w", err)
		}

		if !cfg.IsConfigured() {
			return fmt.Errorf("sync not configured - run 'chronicle sync login' first")
		}

		fmt.Println("This will DELETE all sync data on the server and locally.")
		fmt.Println("Your local chronicle data will NOT be affected.")
		fmt.Print("\nType 'wipe' to confirm: ")

		reader := bufio.NewReader(os.Stdin)
		confirmation, _ := reader.ReadString('\n')
		confirmation = strings.TrimSpace(confirmation)

		if confirmation != "wipe" {
			fmt.Println("Aborted.")
			return nil
		}

		fmt.Println("\nWiping server data...")
		client := vault.NewClient(vault.SyncConfig{
			BaseURL:   cfg.Server,
			DeviceID:  cfg.DeviceID,
			AuthToken: cfg.Token,
		})

		ctx := context.Background()
		deleted, err := client.Wipe(ctx)
		if err != nil {
			return fmt.Errorf("wipe server data: %w", err)
		}
		fmt.Printf("Server data wiped (%d records deleted)\n", deleted)

		fmt.Println("Removing local vault database...")
		if err := os.Remove(cfg.VaultDB); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove vault.db: %w", err)
		}
		fmt.Println("Local vault.db removed")

		fmt.Println("\nSync data cleared. Run 'chronicle sync now' to re-push local data.")
		return nil
	},
}

func init() {
	syncLoginCmd.Flags().String("server", "", "sync server URL (default: https://api.storeusa.org)")
	syncNowCmd.Flags().BoolP("verbose", "v", false, "show detailed sync information")

	syncCmd.AddCommand(syncInitCmd)
	syncCmd.AddCommand(syncLoginCmd)
	syncCmd.AddCommand(syncStatusCmd)
	syncCmd.AddCommand(syncNowCmd)
	syncCmd.AddCommand(syncPendingCmd)
	syncCmd.AddCommand(syncLogoutCmd)
	syncCmd.AddCommand(syncWipeCmd)

	rootCmd.AddCommand(syncCmd)
}

func valueOrNone(s string) string {
	if s == "" {
		return "(not set)"
	}
	return s
}

func printTokenStatus(cfg *sync.Config) {
	if cfg.Token == "" {
		fmt.Println("\nStatus:    Not logged in")
		return
	}

	fmt.Println()
	if cfg.TokenExpires == "" {
		fmt.Println("Token:     valid (no expiry info)")
		return
	}

	expires, err := time.Parse(time.RFC3339, cfg.TokenExpires)
	if err != nil {
		fmt.Printf("Token:     valid (invalid expiry: %v)\n", err)
		return
	}

	now := time.Now()
	if expires.Before(now) {
		fmt.Printf("Token:     EXPIRED (%s ago)\n", now.Sub(expires).Round(time.Second))
		if cfg.RefreshToken != "" {
			fmt.Println("           (has refresh token - run 'chronicle sync now' to refresh)")
		}
	} else {
		fmt.Printf("Token:     valid (expires in %s)\n", formatDuration(expires.Sub(now)))
	}
}

func formatDuration(d time.Duration) string {
	d = d.Round(time.Second)
	h := d / time.Hour
	d -= h * time.Hour
	m := d / time.Minute
	d -= m * time.Minute
	s := d / time.Second

	if h > 0 {
		return fmt.Sprintf("%dh %dm", h, m)
	}
	if m > 0 {
		return fmt.Sprintf("%dm %ds", m, s)
	}
	return fmt.Sprintf("%ds", s)
}

func randHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
```

**Step 2: Add golang.org/x/term dependency**

```bash
go get golang.org/x/term
go mod tidy
```

**Step 3: Build and verify**

```bash
go build -tags=sqlite_fts5 .
./chronicle sync --help
```

Expected: Shows sync subcommands

**Step 4: Commit**

```bash
git add internal/cli/sync.go go.mod go.sum
git commit -m "feat: add sync CLI commands"
```

---

## Task 8: Wire Sync into Add Command

**Files:**
- Modify: `internal/cli/add.go`

**Step 1: Update add.go to queue sync after local write**

Edit `internal/cli/add.go` to add sync integration:

```go
// ABOUTME: Add command for creating new log entries
// ABOUTME: Handles message input and tag flags with optional sync
package cli

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/harper/chronicle/internal/config"
	"github.com/harper/chronicle/internal/db"
	"github.com/harper/chronicle/internal/logging"
	"github.com/harper/chronicle/internal/sync"
	"github.com/spf13/cobra"
)

const (
	unknownValue = "unknown"
)

var (
	tags []string
)

var addCmd = &cobra.Command{
	Use:     "add [message]",
	Aliases: []string{"a"},
	Short:   "Add a log entry",
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		message := args[0]

		// Get database path
		dataHome := config.GetDataHome()
		dbPath := filepath.Join(dataHome, "chronicle", "chronicle.db")

		// Open database
		database, err := db.InitDB(dbPath)
		if err != nil {
			return fmt.Errorf("failed to open database: %w", err)
		}
		defer func() {
			if closeErr := database.Close(); closeErr != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to close database: %v\n", closeErr)
			}
		}()

		// Get metadata
		hostname, err := os.Hostname()
		if err != nil {
			hostname = unknownValue
		}
		username := os.Getenv("USER")
		if username == "" {
			username = unknownValue
		}
		workingDir, err := os.Getwd()
		if err != nil {
			workingDir = unknownValue
		}

		// Create entry
		entry := db.Entry{
			Message:          message,
			Hostname:         hostname,
			Username:         username,
			WorkingDirectory: workingDir,
			Tags:             tags,
		}

		id, err := db.CreateEntry(database, entry)
		if err != nil {
			return fmt.Errorf("failed to create entry: %w", err)
		}

		// Update entry with the returned ID
		entry.ID = id

		// Fetch the specific entry we just created by ID to get its timestamp
		err = database.QueryRow("SELECT timestamp FROM entries WHERE id = ?", id).Scan(&entry.Timestamp)
		if err != nil {
			// If we can't get timestamp, use current time as fallback
			entry.Timestamp = time.Now()
		}

		fmt.Printf("Entry created (ID: %s)\n", id)

		// Queue for sync (secondary, non-blocking)
		if err := queueEntryToSync(cmd.Context(), database, entry); err != nil {
			log.Printf("warning: sync queue failed: %v", err)
		}

		// Check for project logging
		projectRoot, err := config.FindProjectRoot(workingDir)
		if err == nil && projectRoot != "" {
			chroniclePath := filepath.Join(projectRoot, ".chronicle")
			projectCfg, err := config.LoadProjectConfig(chroniclePath)
			if err == nil && projectCfg.LocalLogging {
				logDir := filepath.Join(projectRoot, projectCfg.LogDir)
				if err := logging.WriteProjectLog(logDir, projectCfg.LogFormat, entry); err != nil {
					fmt.Fprintf(os.Stderr, "Warning: failed to write project log: %v\n", err)
				} else {
					fmt.Printf("Project log updated: %s\n", logDir)
				}
			}
		}

		return nil
	},
}

func queueEntryToSync(ctx context.Context, appDB *db.DB, entry db.Entry) error {
	cfg, err := sync.LoadConfig()
	if err != nil {
		return nil // No config, skip silently
	}

	if !cfg.IsConfigured() {
		return nil // Not configured, skip silently
	}

	syncer, err := sync.NewSyncer(cfg, appDB)
	if err != nil {
		return fmt.Errorf("create syncer: %w", err)
	}
	defer syncer.Close()

	if err := syncer.QueueEntryChange(ctx, entry, sync.OpUpsert); err != nil {
		return fmt.Errorf("queue entry: %w", err)
	}

	return nil
}

func init() {
	addCmd.Flags().StringArrayVarP(&tags, "tag", "t", []string{}, "Add tags to entry")
	rootCmd.AddCommand(addCmd)
}
```

Note: The `queueEntryToSync` function takes `*sql.DB` not `*db.DB`. Fix the type:

```go
func queueEntryToSync(ctx context.Context, appDB *sql.DB, entry db.Entry) error {
```

And add the import:

```go
import (
	"context"
	"database/sql"
	// ... rest of imports
)
```

**Step 2: Build and test**

```bash
go build -tags=sqlite_fts5 .
./chronicle add "test sync integration" --tag test
```

Expected: Entry created, sync queues silently (or warns if not configured)

**Step 3: Commit**

```bash
git add internal/cli/add.go
git commit -m "feat: wire sync into add command"
```

---

## Task 9: Run Full Test Suite

**Step 1: Run all tests**

```bash
go test -tags=sqlite_fts5 -v ./...
```

Expected: All tests pass

**Step 2: Build final binary**

```bash
go build -tags=sqlite_fts5 -o chronicle .
```

**Step 3: Manual smoke test**

```bash
./chronicle add "first synced entry" --tag test
./chronicle sync init
./chronicle sync status
./chronicle list
```

**Step 4: Final commit**

```bash
git add -A
git commit -m "feat: complete vault sync integration"
```

---

## Summary

| Task | Description | Files |
|------|-------------|-------|
| 1 | Add suitesync dependency | go.mod |
| 2 | UUID migration schema | migration.go, db.go, schema.go |
| 3 | Update Entry struct | entries.go |
| 4 | Update CLI for string ID | add.go |
| 5 | Create sync config | sync/config.go |
| 6 | Create syncer | sync/sync.go |
| 7 | Add sync commands | cli/sync.go |
| 8 | Wire sync into add | cli/add.go |
| 9 | Full test suite | all |
