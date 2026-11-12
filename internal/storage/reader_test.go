// ABOUTME: Tests for the read-only openers, which open an existing store without writing, migrating or creating anything.
// ABOUTME: Covers refused writes, an unchanged database file, a missing or old database, awkward paths and a reader beside a live writer.

package storage

import (
	"bytes"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// seedSqliteDatabase creates a database in a temporary directory, adds three entries and closes
// it, so no writer is attached when a test opens a reader. It returns the database path.
func seedSqliteDatabase(t *testing.T) string {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "chronicle.db")
	store, err := NewSqliteStore(dbPath)
	if err != nil {
		t.Fatalf("failed to create sqlite store: %v", err)
	}
	for i := 0; i < 3; i++ {
		createSqliteEntry(t, store, Entry{Timestamp: feedTestTime(10, i), Message: fmt.Sprintf("seed entry %d", i)})
	}
	if err := store.Close(); err != nil {
		t.Fatalf("failed to close sqlite store: %v", err)
	}
	return dbPath
}

// openTestSqliteReader opens a read-only store on the database at dbPath and closes it when the
// test ends.
func openTestSqliteReader(t *testing.T, dbPath string) *SqliteStore {
	t.Helper()
	reader, err := OpenSqliteReader(dbPath)
	if err != nil {
		t.Fatalf("failed to open sqlite reader: %v", err)
	}
	t.Cleanup(func() { _ = reader.Close() })
	return reader
}

func TestOpenMarkdownReaderCreatesNothing(t *testing.T) {
	dataDir := filepath.Join(t.TempDir(), "never-created")

	reader, err := OpenMarkdownReader(dataDir)
	if err != nil {
		t.Fatalf("OpenMarkdownReader failed: %v", err)
	}
	defer reader.Close()

	entries, err := reader.ListEntries(0)
	if err != nil {
		t.Fatalf("ListEntries failed: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("ListEntries returned %d entries, want 0", len(entries))
	}

	// A viewer opens a feed as soon as it starts, so the feed must create nothing either.
	feed, err := reader.OpenFeed(nil)
	if err != nil {
		t.Fatalf("OpenFeed failed: %v", err)
	}
	assertPolls(t, feed)

	if _, err := os.Stat(dataDir); !os.IsNotExist(err) {
		t.Errorf("the reader created %s: stat error %v", dataDir, err)
	}
}

func TestOpenSqliteReaderRefusesWrites(t *testing.T) {
	reader := openTestSqliteReader(t, seedSqliteDatabase(t))

	_, err := reader.db.Exec("INSERT INTO entries(id, timestamp, message) VALUES('x', 'now', 'x')")
	if err == nil {
		t.Fatal("an INSERT through the reader succeeded")
	}
	if !strings.Contains(err.Error(), "readonly") {
		t.Errorf("INSERT failed with %q, want a readonly error", err)
	}
}

func TestOpenSqliteReaderLeavesDatabaseFileUnchanged(t *testing.T) {
	dbPath := seedSqliteDatabase(t)
	before, err := os.ReadFile(dbPath)
	if err != nil {
		t.Fatalf("failed to read the database file: %v", err)
	}

	reader, err := OpenSqliteReader(dbPath)
	if err != nil {
		t.Fatalf("OpenSqliteReader failed: %v", err)
	}
	entries, err := reader.SearchEntries(nil, 0)
	if err != nil {
		t.Fatalf("SearchEntries failed: %v", err)
	}
	// Reading nothing would leave the file unchanged too, so pin that the reader saw the entries.
	if len(entries) != 3 {
		t.Errorf("SearchEntries returned %d entries, want 3", len(entries))
	}
	feed, err := reader.OpenFeed(nil)
	if err != nil {
		t.Fatalf("OpenFeed failed: %v", err)
	}
	assertPolls(t, feed)
	if err := reader.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	// SQLite may add -wal and -shm files beside a WAL database it opens, even read-only. They
	// hold no entries, so only the main file is compared.
	after, err := os.ReadFile(dbPath)
	if err != nil {
		t.Fatalf("failed to read the database file again: %v", err)
	}
	if !bytes.Equal(before, after) {
		t.Error("opening, searching and polling through the reader changed the database file")
	}
}

func TestOpenSqliteReaderMissingFile(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "missing", "chronicle.db")

	reader, err := OpenSqliteReader(dbPath)
	if err == nil {
		_ = reader.Close()
		t.Fatal("OpenSqliteReader succeeded on a database that does not exist")
	}
	want := "no chronicle database at " + dbPath
	if !strings.Contains(err.Error(), want) {
		t.Errorf("error is %q, want it to contain %q", err, want)
	}

	// Writing stores create the folder above the database. A reader must not.
	if _, statErr := os.Stat(filepath.Dir(dbPath)); !os.IsNotExist(statErr) {
		t.Errorf("the reader created %s: stat error %v", filepath.Dir(dbPath), statErr)
	}
}

func TestOpenSqliteReaderOldSchema(t *testing.T) {
	cases := []struct {
		name string
		// extraColumns follows the columns that every schema has.
		extraColumns string
	}{
		{"neither timestamp column", ""},
		{"only timestamp_unix_seconds", ", timestamp_unix_seconds INTEGER"},
		{"only timestamp_nanosecond", ", timestamp_nanosecond INTEGER"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dbPath := filepath.Join(t.TempDir(), "chronicle.db")
			db, err := sql.Open("sqlite", dbPath)
			if err != nil {
				t.Fatalf("failed to open raw database: %v", err)
			}
			_, err = db.Exec(`
				CREATE TABLE entries (
					rowid INTEGER PRIMARY KEY AUTOINCREMENT,
					id TEXT UNIQUE NOT NULL,
					timestamp DATETIME NOT NULL,
					message TEXT NOT NULL` + tc.extraColumns + `
				)
			`)
			if err != nil {
				t.Fatalf("failed to create the old schema: %v", err)
			}
			if err := db.Close(); err != nil {
				t.Fatalf("failed to close raw database: %v", err)
			}

			reader, err := OpenSqliteReader(dbPath)
			if err == nil {
				_ = reader.Close()
				t.Fatal("OpenSqliteReader succeeded on a database that lacks a timestamp column")
			}
			want := fmt.Sprintf(`database at %s predates chronicle 1.8.1; run "chronicle list" once to upgrade it`, dbPath)
			if !strings.Contains(err.Error(), want) {
				t.Errorf("error is %q, want it to contain %q", err, want)
			}
		})
	}
}

// The data directory comes from the user's config, so its path can hold characters that mean
// something in a URI.
func TestOpenSqliteReaderOpensAwkwardPaths(t *testing.T) {
	seed, err := os.ReadFile(seedSqliteDatabase(t))
	if err != nil {
		t.Fatalf("failed to read the seed database: %v", err)
	}

	for _, dirName := range []string{"has space", "odd#?%&=+chars"} {
		t.Run(dirName, func(t *testing.T) {
			dir := filepath.Join(t.TempDir(), dirName)
			if err := os.MkdirAll(dir, 0o750); err != nil {
				t.Fatalf("failed to create %s: %v", dir, err)
			}
			// Copy the closed database in, because the writing store can't open such a path.
			dbPath := filepath.Join(dir, "chronicle.db")
			if err := os.WriteFile(dbPath, seed, 0o600); err != nil {
				t.Fatalf("failed to write %s: %v", dbPath, err)
			}

			reader := openTestSqliteReader(t, dbPath)
			entries, err := reader.SearchEntries(nil, 0)
			if err != nil {
				t.Fatalf("SearchEntries failed: %v", err)
			}
			if len(entries) != 3 {
				t.Errorf("SearchEntries returned %d entries, want 3", len(entries))
			}
		})
	}
}

func TestSqliteReaderFeedSeesNewRows(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "chronicle.db")
	// The writer stays open, as the process that logs entries does while a viewer runs.
	writer := openSqliteTestStore(t, dbPath)
	createSqliteEntry(t, writer, Entry{Timestamp: feedTestTime(10, 0), Message: "written before the reader opened"})

	reader := openTestSqliteReader(t, dbPath)
	feed := openSqliteTestFeed(t, reader, nil)
	assertPolls(t, feed)

	added := Entry{Timestamp: feedTestTime(10, 1), Message: "written after the reader opened", Tags: []string{"live"}}
	added.ID = createSqliteEntry(t, writer, added)

	got := assertPolls(t, feed, added.ID)[0]
	if got.Message != added.Message || len(got.Tags) != 1 || got.Tags[0] != "live" {
		t.Errorf("Poll returned %+v, want %+v", got, added)
	}
	assertPolls(t, feed)
}
