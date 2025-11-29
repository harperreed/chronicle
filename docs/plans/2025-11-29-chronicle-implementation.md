# Chronicle Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Build a Go CLI tool for logging timestamped messages with metadata to SQLite, with optional project-specific log files.

**Architecture:** CLI using cobra for commands, SQLite for global storage in XDG_DATA_HOME, TOML for config, directory walking for project detection, FTS5 for search.

**Tech Stack:** Go 1.21+, cobra (CLI), mattn/go-sqlite3 (database), BurntSushi/toml (config), araddon/dateparse (dates)

---

## Task 1: Project Initialization

**Files:**
- Create: `go.mod`
- Create: `main.go`
- Create: `.gitignore`

**Step 1: Initialize Go module**

Run:
```bash
go mod init github.com/harper/chronicle
```

Expected: Creates `go.mod` with module declaration

**Step 2: Create main.go skeleton**

Create `main.go`:
```go
// ABOUTME: Chronicle CLI - Entry point for timestamped logging tool
// ABOUTME: Initializes CLI and routes commands
package main

import (
	"fmt"
	"os"
)

func main() {
	fmt.Println("chronicle")
	os.Exit(0)
}
```

**Step 3: Create .gitignore**

Create `.gitignore`:
```
# Binaries
chronicle
*.exe
*.dll
*.so
*.dylib

# Test binary
*.test

# Output
*.out

# Go workspace file
go.work

# IDE
.idea/
.vscode/
*.swp
*.swo

# OS
.DS_Store
```

**Step 4: Verify it builds**

Run:
```bash
go build -o chronicle .
./chronicle
```

Expected: Prints "chronicle" and exits

**Step 5: Commit**

Run:
```bash
git add go.mod main.go .gitignore
git commit -m "feat: initialize chronicle Go project

Add basic project structure with main entry point"
```

---

## Task 2: CLI Framework with Cobra

**Files:**
- Modify: `main.go`
- Create: `internal/cli/root.go`
- Create: `internal/cli/add.go`

**Step 1: Install cobra**

Run:
```bash
go get -u github.com/spf13/cobra@latest
```

Expected: Updates go.mod with cobra dependency

**Step 2: Create root command**

Create `internal/cli/root.go`:
```go
// ABOUTME: Root command definition and CLI setup
// ABOUTME: Handles global flags and command initialization
package cli

import (
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "chronicle",
	Short: "Timestamped logging tool",
	Long:  `Chronicle logs timestamped messages with metadata to SQLite and optional project log files.`,
}

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	// Global flags can go here
}
```

**Step 3: Create add command skeleton**

Create `internal/cli/add.go`:
```go
// ABOUTME: Add command for creating new log entries
// ABOUTME: Handles message input and tag flags
package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

var (
	tags []string
)

var addCmd = &cobra.Command{
	Use:   "add [message]",
	Short: "Add a log entry",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		message := args[0]
		fmt.Printf("Adding: %s (tags: %v)\n", message, tags)
	},
}

func init() {
	addCmd.Flags().StringArrayVarP(&tags, "tag", "t", []string{}, "Add tags to entry")
	rootCmd.AddCommand(addCmd)
}
```

**Step 4: Update main.go to use CLI**

Modify `main.go`:
```go
// ABOUTME: Chronicle CLI - Entry point for timestamped logging tool
// ABOUTME: Initializes CLI and routes commands
package main

import (
	"fmt"
	"os"

	"github.com/harper/chronicle/internal/cli"
)

func main() {
	if err := cli.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
```

**Step 5: Test the CLI**

Run:
```bash
go build -o chronicle .
./chronicle add "test message" --tag work --tag golang
./chronicle add "test message" -t work -t golang
```

Expected: Both commands print "Adding: test message (tags: [work golang])"

**Step 6: Commit**

Run:
```bash
git add main.go internal/cli/ go.mod go.sum
git commit -m "feat: add cobra CLI framework with add command

Implement basic add command with tag support"
```

---

## Task 3: XDG Directory Helper

**Files:**
- Create: `internal/config/xdg.go`
- Create: `internal/config/xdg_test.go`

**Step 1: Write test for XDG data home**

Create `internal/config/xdg_test.go`:
```go
// ABOUTME: Tests for XDG directory resolution
// ABOUTME: Validates fallback behavior and path construction
package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGetDataHome(t *testing.T) {
	// Save original env
	original := os.Getenv("XDG_DATA_HOME")
	defer os.Setenv("XDG_DATA_HOME", original)

	t.Run("uses XDG_DATA_HOME when set", func(t *testing.T) {
		os.Setenv("XDG_DATA_HOME", "/custom/data")
		got := GetDataHome()
		if got != "/custom/data" {
			t.Errorf("got %s, want /custom/data", got)
		}
	})

	t.Run("falls back to HOME/.local/share", func(t *testing.T) {
		os.Unsetenv("XDG_DATA_HOME")
		home := os.Getenv("HOME")
		want := filepath.Join(home, ".local", "share")
		got := GetDataHome()
		if got != want {
			t.Errorf("got %s, want %s", got, want)
		}
	})
}

func TestGetConfigHome(t *testing.T) {
	original := os.Getenv("XDG_CONFIG_HOME")
	defer os.Setenv("XDG_CONFIG_HOME", original)

	t.Run("uses XDG_CONFIG_HOME when set", func(t *testing.T) {
		os.Setenv("XDG_CONFIG_HOME", "/custom/config")
		got := GetConfigHome()
		if got != "/custom/config" {
			t.Errorf("got %s, want /custom/config", got)
		}
	})

	t.Run("falls back to HOME/.config", func(t *testing.T) {
		os.Unsetenv("XDG_CONFIG_HOME")
		home := os.Getenv("HOME")
		want := filepath.Join(home, ".config")
		got := GetConfigHome()
		if got != want {
			t.Errorf("got %s, want %s", got, want)
		}
	})
}
```

**Step 2: Run test to verify it fails**

Run:
```bash
go test ./internal/config/... -v
```

Expected: FAIL - GetDataHome and GetConfigHome undefined

**Step 3: Implement XDG helpers**

Create `internal/config/xdg.go`:
```go
// ABOUTME: XDG Base Directory specification helpers
// ABOUTME: Resolves data and config directories with fallbacks
package config

import (
	"os"
	"path/filepath"
)

// GetDataHome returns XDG_DATA_HOME or fallback to ~/.local/share
func GetDataHome() string {
	if xdg := os.Getenv("XDG_DATA_HOME"); xdg != "" {
		return xdg
	}
	home := os.Getenv("HOME")
	return filepath.Join(home, ".local", "share")
}

// GetConfigHome returns XDG_CONFIG_HOME or fallback to ~/.config
func GetConfigHome() string {
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return xdg
	}
	home := os.Getenv("HOME")
	return filepath.Join(home, ".config")
}
```

**Step 4: Run test to verify it passes**

Run:
```bash
go test ./internal/config/... -v
```

Expected: PASS for all tests

**Step 5: Commit**

Run:
```bash
git add internal/config/
git commit -m "feat: add XDG directory helpers

Implement XDG_DATA_HOME and XDG_CONFIG_HOME resolution with fallbacks"
```

---

## Task 4: Database Schema and Initialization

**Files:**
- Create: `internal/db/schema.go`
- Create: `internal/db/db.go`
- Create: `internal/db/db_test.go`

**Step 1: Install SQLite dependency**

Run:
```bash
go get github.com/mattn/go-sqlite3
```

Expected: Updates go.mod with sqlite3 dependency

**Step 2: Write test for database initialization**

Create `internal/db/db_test.go`:
```go
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
```

**Step 3: Run test to verify it fails**

Run:
```bash
go test ./internal/db/... -v
```

Expected: FAIL - InitDB undefined

**Step 4: Create schema SQL**

Create `internal/db/schema.go`:
```go
// ABOUTME: Database schema definitions
// ABOUTME: SQL for tables, indexes, and FTS setup
package db

const schema = `
CREATE TABLE IF NOT EXISTS entries (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    timestamp DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    message TEXT NOT NULL,
    hostname TEXT NOT NULL,
    username TEXT NOT NULL,
    working_directory TEXT NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS tags (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    entry_id INTEGER NOT NULL,
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

**Step 5: Implement database initialization**

Create `internal/db/db.go`:
```go
// ABOUTME: Database connection and initialization
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

	// Execute schema
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, err
	}

	return db, nil
}
```

**Step 6: Run test to verify it passes**

Run:
```bash
go test ./internal/db/... -v
```

Expected: PASS for all tests

**Step 7: Commit**

Run:
```bash
git add internal/db/ go.mod go.sum
git commit -m "feat: add database schema and initialization

Implement SQLite schema with entries, tags, and FTS5 search"
```

---

## Task 5: Entry Creation (Add Command)

**Files:**
- Create: `internal/db/entries.go`
- Create: `internal/db/entries_test.go`
- Modify: `internal/cli/add.go`

**Step 1: Write test for creating entry**

Create `internal/db/entries_test.go`:
```go
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
```

**Step 2: Run test to verify it fails**

Run:
```bash
go test ./internal/db/... -v
```

Expected: FAIL - Entry and CreateEntry undefined

**Step 3: Implement entry creation**

Create `internal/db/entries.go`:
```go
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
```

**Step 4: Run test to verify it passes**

Run:
```bash
go test ./internal/db/... -v
```

Expected: PASS for all tests

**Step 5: Update add command to use database**

Modify `internal/cli/add.go`:
```go
// ABOUTME: Add command for creating new log entries
// ABOUTME: Handles message input and tag flags
package cli

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	"github.com/harper/chronicle/internal/config"
	"github.com/harper/chronicle/internal/db"
	"github.com/spf13/cobra"
)

var (
	tags []string
)

var addCmd = &cobra.Command{
	Use:   "add [message]",
	Short: "Add a log entry",
	Args:  cobra.ExactArgs(1),
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
		defer database.Close()

		// Get metadata
		hostname, _ := os.Hostname()
		username := os.Getenv("USER")
		workingDir, _ := os.Getwd()

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

		fmt.Printf("Entry created (ID: %d)\n", id)
		return nil
	},
}

func init() {
	addCmd.Flags().StringArrayVarP(&tags, "tag", "t", []string{}, "Add tags to entry")
	rootCmd.AddCommand(addCmd)
}
```

**Step 6: Test the add command**

Run:
```bash
go build -o chronicle .
./chronicle add "test entry" --tag work --tag golang
./chronicle add "another entry"
```

Expected: Both commands print "Entry created (ID: ...)"

Verify database:
```bash
sqlite3 ~/.local/share/chronicle/chronicle.db "SELECT * FROM entries"
sqlite3 ~/.local/share/chronicle/chronicle.db "SELECT * FROM tags"
```

Expected: See entries and tags in database

**Step 7: Commit**

Run:
```bash
git add internal/db/entries.go internal/db/entries_test.go internal/cli/add.go
git commit -m "feat: implement entry creation in add command

Connect add command to database with metadata capture"
```

---

## Task 6: List Command

**Files:**
- Create: `internal/cli/list.go`
- Modify: `internal/db/entries.go`
- Modify: `internal/db/entries_test.go`

**Step 1: Write test for listing entries**

Add to `internal/db/entries_test.go`:
```go
func TestListEntries(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := InitDB(dbPath)
	if err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}
	defer db.Close()

	// Create test entries
	for i := 0; i < 5; i++ {
		entry := Entry{
			Message:          fmt.Sprintf("message %d", i),
			Hostname:         "testhost",
			Username:         "testuser",
			WorkingDirectory: "/test/dir",
			Tags:             []string{"test"},
		}
		_, err := CreateEntry(db, entry)
		if err != nil {
			t.Fatalf("CreateEntry failed: %v", err)
		}
	}

	// List with limit
	entries, err := ListEntries(db, 3)
	if err != nil {
		t.Fatalf("ListEntries failed: %v", err)
	}

	if len(entries) != 3 {
		t.Errorf("got %d entries, want 3", len(entries))
	}

	// Verify most recent first
	if entries[0].Message != "message 4" {
		t.Errorf("got first message %s, want message 4", entries[0].Message)
	}
}
```

**Step 2: Run test to verify it fails**

Run:
```bash
go test ./internal/db/... -v -run TestListEntries
```

Expected: FAIL - ListEntries undefined

**Step 3: Implement ListEntries**

Add to `internal/db/entries.go`:
```go
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
```

**Step 4: Run test to verify it passes**

Run:
```bash
go test ./internal/db/... -v -run TestListEntries
```

Expected: PASS

**Step 5: Create list command**

Create `internal/cli/list.go`:
```go
// ABOUTME: List command for displaying recent entries
// ABOUTME: Supports table and JSON output formats
package cli

import (
	"encoding/json"
	"fmt"
	"path/filepath"

	"github.com/harper/chronicle/internal/config"
	"github.com/harper/chronicle/internal/db"
	"github.com/spf13/cobra"
)

var (
	listLimit      int
	listJSONOutput bool
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List recent entries",
	RunE: func(cmd *cobra.Command, args []string) error {
		// Get database path
		dataHome := config.GetDataHome()
		dbPath := filepath.Join(dataHome, "chronicle", "chronicle.db")

		// Open database
		database, err := db.InitDB(dbPath)
		if err != nil {
			return fmt.Errorf("failed to open database: %w", err)
		}
		defer database.Close()

		// List entries
		entries, err := db.ListEntries(database, listLimit)
		if err != nil {
			return fmt.Errorf("failed to list entries: %w", err)
		}

		if listJSONOutput {
			data, err := json.MarshalIndent(entries, "", "  ")
			if err != nil {
				return fmt.Errorf("failed to marshal JSON: %w", err)
			}
			fmt.Println(string(data))
		} else {
			// Print table
			fmt.Println("ID\tTimestamp\t\t\tTags\t\tMessage")
			fmt.Println("--\t---------\t\t\t----\t\t-------")
			for _, entry := range entries {
				tagsStr := ""
				if len(entry.Tags) > 0 {
					tagsStr = fmt.Sprintf("%v", entry.Tags)
				}
				timestamp := entry.Timestamp.Format("2006-01-02 15:04:05")
				fmt.Printf("%d\t%s\t%s\t%s\n", entry.ID, timestamp, tagsStr, entry.Message)
			}
		}

		return nil
	},
}

func init() {
	listCmd.Flags().IntVarP(&listLimit, "limit", "n", 20, "Number of entries to show")
	listCmd.Flags().BoolVar(&listJSONOutput, "json", false, "Output as JSON")
	rootCmd.AddCommand(listCmd)
}
```

**Step 6: Test list command**

Run:
```bash
go build -o chronicle .
./chronicle list
./chronicle list --limit 5
./chronicle list --json
```

Expected: Shows table of entries, limited output, and JSON output respectively

**Step 7: Commit**

Run:
```bash
git add internal/db/entries.go internal/db/entries_test.go internal/cli/list.go
git commit -m "feat: add list command with table and JSON output

Implement entry listing with configurable limit"
```

---

## Task 7: Search Command with Date Parsing

**Files:**
- Create: `internal/cli/search.go`
- Modify: `internal/db/entries.go`
- Modify: `internal/db/entries_test.go`

**Step 1: Install dateparse dependency**

Run:
```bash
go get github.com/araddon/dateparse
```

Expected: Updates go.mod

**Step 2: Write test for search**

Add to `internal/db/entries_test.go`:
```go
func TestSearchEntries(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	database, err := InitDB(dbPath)
	if err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}
	defer database.Close()

	// Create test entries
	entries := []Entry{
		{Message: "deployed app", Hostname: "host1", Username: "user", WorkingDirectory: "/dir", Tags: []string{"work", "deploy"}},
		{Message: "fixed bug", Hostname: "host1", Username: "user", WorkingDirectory: "/dir", Tags: []string{"work", "bug"}},
		{Message: "wrote tests", Hostname: "host1", Username: "user", WorkingDirectory: "/dir", Tags: []string{"test"}},
	}
	for _, e := range entries {
		_, err := CreateEntry(database, e)
		if err != nil {
			t.Fatalf("CreateEntry failed: %v", err)
		}
	}

	t.Run("search by text", func(t *testing.T) {
		results, err := SearchEntries(database, SearchParams{Text: "bug"})
		if err != nil {
			t.Fatalf("SearchEntries failed: %v", err)
		}
		if len(results) != 1 || results[0].Message != "fixed bug" {
			t.Errorf("got %v, want entry with 'fixed bug'", results)
		}
	})

	t.Run("search by tag", func(t *testing.T) {
		results, err := SearchEntries(database, SearchParams{Tags: []string{"work"}})
		if err != nil {
			t.Fatalf("SearchEntries failed: %v", err)
		}
		if len(results) != 2 {
			t.Errorf("got %d results, want 2", len(results))
		}
	})
}
```

**Step 3: Run test to verify it fails**

Run:
```bash
go test ./internal/db/... -v -run TestSearchEntries
```

Expected: FAIL - SearchEntries and SearchParams undefined

**Step 4: Implement search**

Add to `internal/db/entries.go`:
```go
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
```

**Step 5: Run test to verify it passes**

Run:
```bash
go test ./internal/db/... -v -run TestSearchEntries
```

Expected: PASS

**Step 6: Create search command**

Create `internal/cli/search.go`:
```go
// ABOUTME: Search command for querying entries
// ABOUTME: Supports text search, tags, and date ranges
package cli

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"time"

	"github.com/araddon/dateparse"
	"github.com/harper/chronicle/internal/config"
	"github.com/harper/chronicle/internal/db"
	"github.com/spf13/cobra"
)

var (
	searchTags       []string
	searchSince      string
	searchUntil      string
	searchLimit      int
	searchJSONOutput bool
)

var searchCmd = &cobra.Command{
	Use:   "search [text]",
	Short: "Search entries",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		// Get database path
		dataHome := config.GetDataHome()
		dbPath := filepath.Join(dataHome, "chronicle", "chronicle.db")

		// Open database
		database, err := db.InitDB(dbPath)
		if err != nil {
			return fmt.Errorf("failed to open database: %w", err)
		}
		defer database.Close()

		// Build search params
		params := db.SearchParams{
			Tags:  searchTags,
			Limit: searchLimit,
		}

		if len(args) > 0 {
			params.Text = args[0]
		}

		// Parse dates
		if searchSince != "" {
			since, err := dateparse.ParseAny(searchSince)
			if err != nil {
				return fmt.Errorf("invalid --since date: %w", err)
			}
			params.Since = &since
		}

		if searchUntil != "" {
			until, err := dateparse.ParseAny(searchUntil)
			if err != nil {
				return fmt.Errorf("invalid --until date: %w", err)
			}
			params.Until = &until
		}

		// Search
		entries, err := db.SearchEntries(database, params)
		if err != nil {
			return fmt.Errorf("failed to search entries: %w", err)
		}

		// Output
		if searchJSONOutput {
			data, err := json.MarshalIndent(entries, "", "  ")
			if err != nil {
				return fmt.Errorf("failed to marshal JSON: %w", err)
			}
			fmt.Println(string(data))
		} else {
			fmt.Println("ID\tTimestamp\t\t\tTags\t\tMessage")
			fmt.Println("--\t---------\t\t\t----\t\t-------")
			for _, entry := range entries {
				tagsStr := ""
				if len(entry.Tags) > 0 {
					tagsStr = fmt.Sprintf("%v", entry.Tags)
				}
				timestamp := entry.Timestamp.Format("2006-01-02 15:04:05")
				fmt.Printf("%d\t%s\t%s\t%s\n", entry.ID, timestamp, tagsStr, entry.Message)
			}
		}

		return nil
	},
}

func init() {
	searchCmd.Flags().StringArrayVarP(&searchTags, "tag", "t", []string{}, "Filter by tags")
	searchCmd.Flags().StringVar(&searchSince, "since", "", "Start date (natural language or ISO)")
	searchCmd.Flags().StringVar(&searchUntil, "until", "", "End date (natural language or ISO)")
	searchCmd.Flags().IntVarP(&searchLimit, "limit", "n", 100, "Maximum results")
	searchCmd.Flags().BoolVar(&searchJSONOutput, "json", false, "Output as JSON")
	rootCmd.AddCommand(searchCmd)
}
```

**Step 7: Test search command**

Run:
```bash
go build -o chronicle .
./chronicle add "deployed v1.0" --tag deploy
./chronicle add "fixed critical bug" --tag bug --tag critical
./chronicle add "wrote unit tests" --tag test

./chronicle search "bug"
./chronicle search --tag deploy
./chronicle search --since "1 hour ago"
./chronicle search "test" --json
```

Expected: Search results match criteria

**Step 8: Commit**

Run:
```bash
git add internal/cli/search.go internal/db/entries.go internal/db/entries_test.go go.mod go.sum
git commit -m "feat: add search command with text, tags, and date filters

Implement full search with natural language date parsing"
```

---

## Task 8: Project Detection

**Files:**
- Create: `internal/config/project.go`
- Create: `internal/config/project_test.go`

**Step 1: Write test for project detection**

Create `internal/config/project_test.go`:
```go
// ABOUTME: Tests for project .chronicle file detection
// ABOUTME: Validates directory walking and config parsing
package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFindProjectRoot(t *testing.T) {
	// Create temp directory structure
	tmpDir := t.TempDir()

	projectRoot := filepath.Join(tmpDir, "project")
	subDir := filepath.Join(projectRoot, "src", "deep", "nested")
	os.MkdirAll(subDir, 0755)

	// Create .chronicle file
	chronicleFile := filepath.Join(projectRoot, ".chronicle")
	os.WriteFile(chronicleFile, []byte("local_logging = true\n"), 0644)

	t.Run("finds project root from nested directory", func(t *testing.T) {
		root, err := FindProjectRoot(subDir)
		if err != nil {
			t.Fatalf("FindProjectRoot failed: %v", err)
		}
		if root != projectRoot {
			t.Errorf("got %s, want %s", root, projectRoot)
		}
	})

	t.Run("returns empty when no .chronicle found", func(t *testing.T) {
		otherDir := filepath.Join(tmpDir, "other")
		os.MkdirAll(otherDir, 0755)

		root, err := FindProjectRoot(otherDir)
		if err != nil {
			t.Fatalf("FindProjectRoot failed: %v", err)
		}
		if root != "" {
			t.Errorf("got %s, want empty string", root)
		}
	})
}

func TestLoadProjectConfig(t *testing.T) {
	tmpDir := t.TempDir()

	configContent := `
local_logging = true
log_dir = "custom-logs"
log_format = "json"
`
	configPath := filepath.Join(tmpDir, ".chronicle")
	os.WriteFile(configPath, []byte(configContent), 0644)

	cfg, err := LoadProjectConfig(configPath)
	if err != nil {
		t.Fatalf("LoadProjectConfig failed: %v", err)
	}

	if !cfg.LocalLogging {
		t.Error("expected LocalLogging to be true")
	}
	if cfg.LogDir != "custom-logs" {
		t.Errorf("got LogDir %s, want custom-logs", cfg.LogDir)
	}
	if cfg.LogFormat != "json" {
		t.Errorf("got LogFormat %s, want json", cfg.LogFormat)
	}
}
```

**Step 2: Run test to verify it fails**

Run:
```bash
go test ./internal/config/... -v -run TestFindProjectRoot
go test ./internal/config/... -v -run TestLoadProjectConfig
```

Expected: FAIL - FindProjectRoot and LoadProjectConfig undefined

**Step 3: Install TOML dependency**

Run:
```bash
go get github.com/BurntSushi/toml
```

Expected: Updates go.mod

**Step 4: Implement project detection**

Create `internal/config/project.go`:
```go
// ABOUTME: Project .chronicle file detection and config loading
// ABOUTME: Walks directory tree to find project root
package config

import (
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

type ProjectConfig struct {
	LocalLogging bool   `toml:"local_logging"`
	LogDir       string `toml:"log_dir"`
	LogFormat    string `toml:"log_format"`
}

// FindProjectRoot walks up from dir looking for .chronicle file
// Returns empty string if not found
func FindProjectRoot(dir string) (string, error) {
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return "", err
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	current := absDir
	for {
		chroniclePath := filepath.Join(current, ".chronicle")
		if _, err := os.Stat(chroniclePath); err == nil {
			return current, nil
		}

		parent := filepath.Dir(current)

		// Stop at filesystem root or home directory
		if parent == current || current == homeDir {
			return "", nil
		}

		current = parent
	}
}

// LoadProjectConfig loads .chronicle config from path
func LoadProjectConfig(path string) (*ProjectConfig, error) {
	var cfg ProjectConfig

	// Set defaults
	cfg.LogDir = "logs"
	cfg.LogFormat = "markdown"

	if _, err := toml.DecodeFile(path, &cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}
```

**Step 5: Run tests to verify they pass**

Run:
```bash
go test ./internal/config/... -v
```

Expected: PASS for all tests

**Step 6: Commit**

Run:
```bash
git add internal/config/project.go internal/config/project_test.go go.mod go.sum
git commit -m "feat: add project detection and config loading

Implement directory walking to find .chronicle file"
```

---

## Task 9: Project Log File Writing

**Files:**
- Create: `internal/logging/project.go`
- Create: `internal/logging/project_test.go`
- Modify: `internal/cli/add.go`

**Step 1: Write test for project logging**

Create `internal/logging/project_test.go`:
```go
// ABOUTME: Tests for project log file writing
// ABOUTME: Validates log entry formatting and file operations
package logging

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/harper/chronicle/internal/db"
)

func TestWriteProjectLog(t *testing.T) {
	tmpDir := t.TempDir()
	logDir := filepath.Join(tmpDir, "logs")

	entry := db.Entry{
		Timestamp:        time.Date(2025, 11, 29, 14, 30, 0, 0, time.UTC),
		Message:          "test message",
		Hostname:         "testhost",
		Username:         "testuser",
		WorkingDirectory: "/test/dir",
		Tags:             []string{"work", "test"},
	}

	err := WriteProjectLog(logDir, "markdown", entry)
	if err != nil {
		t.Fatalf("WriteProjectLog failed: %v", err)
	}

	// Verify log file was created
	logFile := filepath.Join(logDir, "2025-11-29.log")
	if _, err := os.Stat(logFile); os.IsNotExist(err) {
		t.Fatal("log file was not created")
	}

	// Verify content
	content, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}

	expectedContent := `## 14:30:00 - test message
- **Tags**: work, test
- **User**: testuser@testhost
- **Directory**: /test/dir

`
	if string(content) != expectedContent {
		t.Errorf("got:\n%s\nwant:\n%s", string(content), expectedContent)
	}
}

func TestWriteProjectLogJSON(t *testing.T) {
	tmpDir := t.TempDir()
	logDir := filepath.Join(tmpDir, "logs")

	entry := db.Entry{
		Timestamp:        time.Date(2025, 11, 29, 14, 30, 0, 0, time.UTC),
		Message:          "test message",
		Hostname:         "testhost",
		Username:         "testuser",
		WorkingDirectory: "/test/dir",
		Tags:             []string{"work"},
	}

	err := WriteProjectLog(logDir, "json", entry)
	if err != nil {
		t.Fatalf("WriteProjectLog failed: %v", err)
	}

	// Verify content is valid JSON
	logFile := filepath.Join(logDir, "2025-11-29.log")
	content, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}

	// Should contain JSON fields
	contentStr := string(content)
	if !contains(contentStr, `"message"`) || !contains(contentStr, `"tags"`) {
		t.Errorf("JSON output missing expected fields: %s", contentStr)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && (s[:len(substr)] == substr || contains(s[1:], substr)))
}
```

**Step 2: Run test to verify it fails**

Run:
```bash
go test ./internal/logging/... -v
```

Expected: FAIL - WriteProjectLog undefined

**Step 3: Implement project logging**

Create `internal/logging/project.go`:
```go
// ABOUTME: Project log file writing
// ABOUTME: Formats entries as markdown or JSON and appends to daily logs
package logging

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/harper/chronicle/internal/db"
)

// WriteProjectLog appends entry to project log file
func WriteProjectLog(logDir, format string, entry db.Entry) error {
	// Create log directory if needed
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return err
	}

	// Determine log file name (one per day)
	date := entry.Timestamp.Format("2006-01-02")
	logFile := filepath.Join(logDir, date+".log")

	// Format entry
	var content string
	switch format {
	case "json":
		data, err := json.Marshal(entry)
		if err != nil {
			return err
		}
		content = string(data) + "\n"
	case "markdown":
		fallthrough
	default:
		content = formatMarkdown(entry)
	}

	// Append to file
	f, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = f.WriteString(content)
	return err
}

func formatMarkdown(entry db.Entry) string {
	var sb strings.Builder

	timeStr := entry.Timestamp.Format("15:04:05")
	sb.WriteString(fmt.Sprintf("## %s - %s\n", timeStr, entry.Message))

	if len(entry.Tags) > 0 {
		sb.WriteString(fmt.Sprintf("- **Tags**: %s\n", strings.Join(entry.Tags, ", ")))
	}

	sb.WriteString(fmt.Sprintf("- **User**: %s@%s\n", entry.Username, entry.Hostname))
	sb.WriteString(fmt.Sprintf("- **Directory**: %s\n", entry.WorkingDirectory))
	sb.WriteString("\n")

	return sb.String()
}
```

**Step 4: Run test to verify it passes**

Run:
```bash
go test ./internal/logging/... -v
```

Expected: PASS for all tests

**Step 5: Update add command to use project logging**

Modify `internal/cli/add.go`:
```go
// ABOUTME: Add command for creating new log entries
// ABOUTME: Handles message input and tag flags
package cli

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	"github.com/harper/chronicle/internal/config"
	"github.com/harper/chronicle/internal/db"
	"github.com/harper/chronicle/internal/logging"
	"github.com/spf13/cobra"
)

var (
	tags []string
)

var addCmd = &cobra.Command{
	Use:   "add [message]",
	Short: "Add a log entry",
	Args:  cobra.ExactArgs(1),
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
		defer database.Close()

		// Get metadata
		hostname, _ := os.Hostname()
		username := os.Getenv("USER")
		workingDir, _ := os.Getwd()

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

		// Fetch the created entry to get timestamp
		entries, err := db.SearchEntries(database, db.SearchParams{Limit: 1})
		if err == nil && len(entries) > 0 {
			entry = entries[0]
		}

		fmt.Printf("Entry created (ID: %d)\n", id)

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

func init() {
	addCmd.Flags().StringArrayVarP(&tags, "tag", "t", []string{}, "Add tags to entry")
	rootCmd.AddCommand(addCmd)
}
```

**Step 6: Test project logging end-to-end**

Run:
```bash
# Create test project
mkdir -p /tmp/test-project/src
cd /tmp/test-project

# Create .chronicle config
cat > .chronicle << 'EOF'
local_logging = true
log_dir = "logs"
log_format = "markdown"
EOF

# Build chronicle
cd /path/to/chronicle
go build -o chronicle .

# Add entry from project subdirectory
cd /tmp/test-project/src
/path/to/chronicle/chronicle add "test from project" --tag test

# Verify log file
cat /tmp/test-project/logs/*.log
```

Expected:
- "Entry created" message
- "Project log updated" message
- Log file contains markdown entry

**Step 7: Commit**

Run:
```bash
git add internal/logging/ internal/cli/add.go
git commit -m "feat: add project log file writing

Implement markdown and JSON project logs with .chronicle detection"
```

---

## Task 10: Default Command Behavior

**Files:**
- Modify: `internal/cli/root.go`
- Modify: `internal/cli/add.go`

**Step 1: Update root command to default to add**

Modify `internal/cli/root.go`:
```go
// ABOUTME: Root command definition and CLI setup
// ABOUTME: Handles global flags and command initialization
package cli

import (
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "chronicle",
	Short: "Timestamped logging tool",
	Long:  `Chronicle logs timestamped messages with metadata to SQLite and optional project log files.`,
	// Default to add command when message provided as first arg
	Args: cobra.ArbitraryArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		// If args provided and no subcommand, treat as "add"
		if len(args) > 0 {
			// Delegate to add command
			addCmd.Run(cmd, args)
			return nil
		}
		// Otherwise show help
		return cmd.Help()
	},
}

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	// Global flags can go here
}
```

**Step 2: Update add command to support both forms**

Modify the `addCmd` variable in `internal/cli/add.go` to make the command name optional:

Change:
```go
var addCmd = &cobra.Command{
	Use:   "add [message]",
```

To:
```go
var addCmd = &cobra.Command{
	Use:   "add [message]",
	Aliases: []string{"a"},
```

**Step 3: Test default behavior**

Run:
```bash
go build -o chronicle .
./chronicle "quick message"              # Should work like "add"
./chronicle "message" --tag test         # Should work with flags
./chronicle add "explicit add"           # Should still work
./chronicle                              # Should show help
```

Expected: All forms work correctly

**Step 4: Commit**

Run:
```bash
git add internal/cli/root.go internal/cli/add.go
git commit -m "feat: make add the default command

Allow 'chronicle message' shorthand for quick logging"
```

---

## Task 11: README and Documentation

**Files:**
- Create: `README.md`

**Step 1: Create comprehensive README**

Create `README.md`:
```markdown
# Chronicle

A fast, lightweight CLI tool for logging timestamped messages with metadata.

## Features

- **Global SQLite database** - All entries stored in `~/.local/share/chronicle/chronicle.db`
- **Rich metadata** - Automatic capture of timestamp, hostname, username, working directory
- **Tagging** - Organize entries with multiple tags
- **Full-text search** - Fast FTS5-powered search
- **Project logs** - Optional per-project log files (markdown or JSON)
- **Natural date parsing** - Use "yesterday", "last week", or ISO dates
- **Multiple output formats** - Human-readable tables or JSON

## Installation

```bash
go install github.com/harper/chronicle@latest
```

Or build from source:

```bash
git clone https://github.com/harper/chronicle
cd chronicle
go build -o chronicle .
```

## Quick Start

```bash
# Add an entry (quick form)
chronicle "deployed version 2.1.0"

# Add with tags
chronicle "fixed auth bug" --tag work --tag golang

# List recent entries
chronicle list

# Search
chronicle search "deployment"
chronicle search --tag work --since "last week"
```

## Commands

### Add Entry

```bash
chronicle "message"                      # Quick form
chronicle add "message"                  # Explicit form
chronicle add "message" --tag work -t go # With tags
```

### List Entries

```bash
chronicle list                 # Recent 20 entries
chronicle list --limit 50      # Show more
chronicle list --json          # JSON output
```

### Search

```bash
chronicle search "keyword"                        # Full-text search
chronicle search --tag work                       # By tag
chronicle search --since yesterday --until today  # Date range
chronicle search "bug" --tag golang --json        # Combined with JSON
```

**Date formats:**
- Natural: `yesterday`, `today`, `"3 days ago"`, `"last week"`
- ISO: `2025-11-29`, `2025-11-29T14:30:00`

## Project-Specific Logs

Enable local log files for a project by creating `.chronicle`:

```toml
local_logging = true
log_dir = "logs"
log_format = "markdown"  # or "json"
```

When you run `chronicle add` from anywhere in the project, it will:
1. Store the entry in the global database
2. Append to `logs/YYYY-MM-DD.log` in the project root

Example markdown log entry:
```markdown
## 14:32:15 - deployed v2.1.0
- **Tags**: work, deployment
- **User**: harper@MacBook-Pro
- **Directory**: /Users/harper/mobile-app/src
```

## Configuration

### Global Config

Optional: `~/.config/chronicle/config.toml`

```toml
# Override database location
db_path = "/custom/path/chronicle.db"
```

## Database Schema

- **entries** - Main log entries with timestamp, message, metadata
- **tags** - Many-to-many tag relationships
- **entries_fts** - Full-text search virtual table

Query directly with sqlite3:
```bash
sqlite3 ~/.local/share/chronicle/chronicle.db "SELECT * FROM entries"
```

## Development

```bash
# Run tests
go test ./... -v

# Build
go build -o chronicle .

# Install locally
go install
```

## License

MIT
```

**Step 2: Commit README**

Run:
```bash
git add README.md
git commit -m "docs: add comprehensive README

Document installation, usage, and features"
```

---

## Task 12: Integration Testing and Final Verification

**Files:**
- Create: `test_integration.sh`

**Step 1: Create integration test script**

Create `test_integration.sh`:
```bash
#!/usr/bin/env bash
# ABOUTME: Integration test script for chronicle
# ABOUTME: Validates end-to-end workflows and command interactions

set -euo pipefail

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
NC='\033[0m'

echo "Running chronicle integration tests..."

# Build
echo "Building chronicle..."
go build -o chronicle .

# Setup temp directory
TEST_DIR=$(mktemp -d)
export HOME=$TEST_DIR
export XDG_DATA_HOME="$TEST_DIR/.local/share"
export XDG_CONFIG_HOME="$TEST_DIR/.config"

cleanup() {
  rm -rf "$TEST_DIR"
}
trap cleanup EXIT

# Test 1: Add entry
echo -n "Test 1: Add entry... "
./chronicle add "test entry 1" --tag test
if [ $? -eq 0 ]; then
  echo -e "${GREEN}PASS${NC}"
else
  echo -e "${RED}FAIL${NC}"
  exit 1
fi

# Test 2: Add without explicit command
echo -n "Test 2: Add with shorthand... "
./chronicle "test entry 2" --tag work
if [ $? -eq 0 ]; then
  echo -e "${GREEN}PASS${NC}"
else
  echo -e "${RED}FAIL${NC}"
  exit 1
fi

# Test 3: List entries
echo -n "Test 3: List entries... "
OUTPUT=$(./chronicle list)
if echo "$OUTPUT" | grep -q "test entry 1" && echo "$OUTPUT" | grep -q "test entry 2"; then
  echo -e "${GREEN}PASS${NC}"
else
  echo -e "${RED}FAIL${NC}"
  echo "Output: $OUTPUT"
  exit 1
fi

# Test 4: Search by text
echo -n "Test 4: Search by text... "
OUTPUT=$(./chronicle search "entry 1")
if echo "$OUTPUT" | grep -q "test entry 1"; then
  echo -e "${GREEN}PASS${NC}"
else
  echo -e "${RED}FAIL${NC}"
  exit 1
fi

# Test 5: Search by tag
echo -n "Test 5: Search by tag... "
OUTPUT=$(./chronicle search --tag work)
if echo "$OUTPUT" | grep -q "test entry 2"; then
  echo -e "${GREEN}PASS${NC}"
else
  echo -e "${RED}FAIL${NC}"
  exit 1
fi

# Test 6: JSON output
echo -n "Test 6: JSON output... "
OUTPUT=$(./chronicle list --json)
if echo "$OUTPUT" | grep -q '"message"' && echo "$OUTPUT" | grep -q '"tags"'; then
  echo -e "${GREEN}PASS${NC}"
else
  echo -e "${RED}FAIL${NC}"
  exit 1
fi

# Test 7: Project logging
echo -n "Test 7: Project logging... "
PROJECT_DIR="$TEST_DIR/test-project"
mkdir -p "$PROJECT_DIR/src"
cat > "$PROJECT_DIR/.chronicle" << EOF
local_logging = true
log_dir = "logs"
log_format = "markdown"
EOF

cd "$PROJECT_DIR/src"
$TEST_DIR/chronicle "project entry" --tag project
if [ -f "$PROJECT_DIR/logs/$(date +%Y-%m-%d).log" ]; then
  echo -e "${GREEN}PASS${NC}"
else
  echo -e "${RED}FAIL${NC}"
  exit 1
fi

echo ""
echo -e "${GREEN}All integration tests passed!${NC}"
```

**Step 2: Make script executable**

Run:
```bash
chmod +x test_integration.sh
```

**Step 3: Run integration tests**

Run:
```bash
./test_integration.sh
```

Expected: All tests pass

**Step 4: Commit integration tests**

Run:
```bash
git add test_integration.sh
git commit -m "test: add integration test suite

Validate end-to-end workflows and command interactions"
```

---

## Final Steps

**Step 1: Run all tests**

Run:
```bash
go test ./... -v
./test_integration.sh
```

Expected: All tests pass

**Step 2: Build final binary**

Run:
```bash
go build -o chronicle .
```

**Step 3: Install locally**

Run:
```bash
go install
```

**Step 4: Final commit**

Run:
```bash
git add .
git commit -m "chore: finalize chronicle v1.0.0

Complete implementation of CLI logging tool with all features"
```

---

## Implementation Complete

Chronicle is now ready with:
- ✅ SQLite storage in XDG_DATA_HOME
- ✅ Add command with tags
- ✅ List command with table/JSON output
- ✅ Search with text, tags, and date filters
- ✅ Natural date parsing
- ✅ Project-specific log files
- ✅ Comprehensive tests
- ✅ Documentation

**Next steps for enhancement:**
- Add export command (CSV, JSON export)
- Add stats command (entry counts, tag usage)
- Add delete/edit commands
- Shell completion scripts
- Homebrew formula for distribution
