// ABOUTME: Tests for storage migration between chronicle backends
// ABOUTME: Covers sqlite-to-markdown, markdown-to-sqlite, data integrity, and round-trips

package storage

import (
	"path/filepath"
	"testing"
	"time"
)

// seedEntries populates a storage backend with a representative data set.
func seedEntries(t *testing.T, src Storage) []Entry {
	t.Helper()

	entries := []Entry{
		{
			Timestamp:        time.Date(2026, 2, 1, 10, 0, 0, 0, time.UTC),
			Message:          "deployed production release v2.1.0",
			Hostname:         "build-server",
			Username:         "deploy-bot",
			WorkingDirectory: "/app/deploy",
			Tags:             []string{"deployment", "production"},
		},
		{
			Timestamp:        time.Date(2026, 2, 2, 14, 30, 0, 0, time.UTC),
			Message:          "fixed critical bug in authentication",
			Hostname:         "dev-machine",
			Username:         "testuser",
			WorkingDirectory: "/home/testuser/project",
			Tags:             []string{"bugfix", "auth"},
		},
		{
			Timestamp:        time.Date(2026, 2, 3, 9, 0, 0, 0, time.UTC),
			Message:          "updated API documentation",
			Hostname:         "dev-machine",
			Username:         "testuser",
			WorkingDirectory: "/home/testuser/project/docs",
			Tags:             nil,
		},
		{
			Timestamp:        time.Date(2026, 2, 3, 16, 0, 0, 0, time.UTC),
			Message:          "multiline entry\nwith newlines\nand special chars: <>&\"'",
			Hostname:         "dev-machine",
			Username:         "testuser",
			WorkingDirectory: "/tmp",
			Tags:             []string{"test"},
		},
	}

	var created []Entry
	for _, e := range entries {
		id, err := src.CreateEntry(e)
		if err != nil {
			t.Fatalf("failed to create entry: %v", err)
		}
		e.ID = id
		created = append(created, e)
	}

	return created
}

// verifyMigratedEntries checks that all entries exist in the destination with correct data.
func verifyMigratedEntries(t *testing.T, dst Storage, entries []Entry) {
	t.Helper()

	for _, orig := range entries {
		got, err := dst.GetEntry(orig.ID)
		if err != nil {
			t.Errorf("entry %q (%s) not found in destination: %v", orig.Message, orig.ID, err)
			continue
		}
		if got.Message != orig.Message {
			t.Errorf("message mismatch: want %q, got %q", orig.Message, got.Message)
		}
		if !got.Timestamp.Equal(orig.Timestamp) {
			t.Errorf("timestamp mismatch for %q: want %v, got %v", orig.Message, orig.Timestamp, got.Timestamp)
		}
		if got.Hostname != orig.Hostname {
			t.Errorf("hostname mismatch: want %q, got %q", orig.Hostname, got.Hostname)
		}
		if got.Username != orig.Username {
			t.Errorf("username mismatch: want %q, got %q", orig.Username, got.Username)
		}
		if got.WorkingDirectory != orig.WorkingDirectory {
			t.Errorf("working_directory mismatch: want %q, got %q", orig.WorkingDirectory, got.WorkingDirectory)
		}
		if len(orig.Tags) == 0 {
			// nil and empty are both acceptable
			if len(got.Tags) != 0 {
				t.Errorf("expected empty tags, got %v", got.Tags)
			}
		} else {
			if len(got.Tags) != len(orig.Tags) {
				t.Errorf("tags count mismatch: want %d, got %d", len(orig.Tags), len(got.Tags))
			}
		}
	}
}

func TestMigrateData_SqliteToMarkdown(t *testing.T) {
	// Set up source (sqlite)
	srcDir := t.TempDir()
	src, err := NewSqliteStore(filepath.Join(srcDir, "chronicle.db"))
	if err != nil {
		t.Fatalf("create source store: %v", err)
	}
	defer src.Close()

	entries := seedEntries(t, src)

	// Set up destination (markdown)
	dstDir := t.TempDir()
	dst, err := NewMarkdownStore(dstDir)
	if err != nil {
		t.Fatalf("create destination store: %v", err)
	}
	defer dst.Close()

	// Run migration
	summary, err := MigrateData(src, dst)
	if err != nil {
		t.Fatalf("MigrateData failed: %v", err)
	}

	// Verify summary counts
	if summary.Entries != len(entries) {
		t.Errorf("summary entries: want %d, got %d", len(entries), summary.Entries)
	}

	// Verify all data was migrated correctly
	verifyMigratedEntries(t, dst, entries)
}

func TestMigrateData_MarkdownToSqlite(t *testing.T) {
	// Set up source (markdown)
	srcDir := t.TempDir()
	src, err := NewMarkdownStore(srcDir)
	if err != nil {
		t.Fatalf("create source store: %v", err)
	}
	defer src.Close()

	entries := seedEntries(t, src)

	// Set up destination (sqlite)
	dstDir := t.TempDir()
	dst, err := NewSqliteStore(filepath.Join(dstDir, "chronicle.db"))
	if err != nil {
		t.Fatalf("create destination store: %v", err)
	}
	defer dst.Close()

	// Run migration
	summary, err := MigrateData(src, dst)
	if err != nil {
		t.Fatalf("MigrateData failed: %v", err)
	}

	// Verify summary counts
	if summary.Entries != len(entries) {
		t.Errorf("summary entries: want %d, got %d", len(entries), summary.Entries)
	}

	// Verify all data was migrated correctly
	verifyMigratedEntries(t, dst, entries)
}

func TestMigrateData_EmptySource(t *testing.T) {
	// Set up empty source (sqlite)
	srcDir := t.TempDir()
	src, err := NewSqliteStore(filepath.Join(srcDir, "chronicle.db"))
	if err != nil {
		t.Fatalf("create source store: %v", err)
	}
	defer src.Close()

	// Set up destination (markdown)
	dstDir := t.TempDir()
	dst, err := NewMarkdownStore(dstDir)
	if err != nil {
		t.Fatalf("create destination store: %v", err)
	}
	defer dst.Close()

	summary, err := MigrateData(src, dst)
	if err != nil {
		t.Fatalf("MigrateData failed: %v", err)
	}

	if summary.Entries != 0 {
		t.Errorf("expected 0 entries for empty source, got %d", summary.Entries)
	}
}

func TestMigrateRoundTrip_SqliteToMarkdownToSqlite(t *testing.T) {
	// Phase 1: Create data in SQLite
	srcDir := t.TempDir()
	original, err := NewSqliteStore(filepath.Join(srcDir, "original.db"))
	if err != nil {
		t.Fatalf("create original store: %v", err)
	}
	defer original.Close()

	entries := seedEntries(t, original)

	// Phase 2: Migrate SQLite -> Markdown
	mdDir := t.TempDir()
	mdStore, err := NewMarkdownStore(mdDir)
	if err != nil {
		t.Fatalf("create markdown store: %v", err)
	}
	defer mdStore.Close()

	summary1, err := MigrateData(original, mdStore)
	if err != nil {
		t.Fatalf("MigrateData (sqlite->markdown) failed: %v", err)
	}
	if summary1.Entries != len(entries) {
		t.Errorf("phase 1 entries: want %d, got %d", len(entries), summary1.Entries)
	}

	// Phase 3: Migrate Markdown -> new SQLite
	dstDir := t.TempDir()
	final, err := NewSqliteStore(filepath.Join(dstDir, "final.db"))
	if err != nil {
		t.Fatalf("create final store: %v", err)
	}
	defer final.Close()

	summary2, err := MigrateData(mdStore, final)
	if err != nil {
		t.Fatalf("MigrateData (markdown->sqlite) failed: %v", err)
	}
	if summary2.Entries != len(entries) {
		t.Errorf("phase 2 entries: want %d, got %d", len(entries), summary2.Entries)
	}

	// Phase 4: Field-by-field verification against original data
	verifyMigratedEntries(t, final, entries)
}

func TestMigrateRoundTrip_MarkdownToSqliteToMarkdown(t *testing.T) {
	// Phase 1: Create data in Markdown
	srcDir := t.TempDir()
	original, err := NewMarkdownStore(srcDir)
	if err != nil {
		t.Fatalf("create original store: %v", err)
	}
	defer original.Close()

	entries := seedEntries(t, original)

	// Phase 2: Migrate Markdown -> SQLite
	sqlDir := t.TempDir()
	sqlStore, err := NewSqliteStore(filepath.Join(sqlDir, "mid.db"))
	if err != nil {
		t.Fatalf("create sqlite store: %v", err)
	}
	defer sqlStore.Close()

	_, err = MigrateData(original, sqlStore)
	if err != nil {
		t.Fatalf("MigrateData (markdown->sqlite) failed: %v", err)
	}

	// Phase 3: Migrate SQLite -> new Markdown
	dstDir := t.TempDir()
	final, err := NewMarkdownStore(dstDir)
	if err != nil {
		t.Fatalf("create final store: %v", err)
	}
	defer final.Close()

	_, err = MigrateData(sqlStore, final)
	if err != nil {
		t.Fatalf("MigrateData (sqlite->markdown) failed: %v", err)
	}

	// Phase 4: Verify all data
	verifyMigratedEntries(t, final, entries)
}

func TestMigrateMarkdownToSqlitePreservesOffsetAndNanoseconds(t *testing.T) {
	src, err := NewMarkdownStore(t.TempDir())
	if err != nil {
		t.Fatalf("create source store: %v", err)
	}
	defer src.Close()

	location := time.FixedZone("offset", 60*60)
	want := Entry{
		ID:        "offset-fractional",
		Timestamp: time.Date(2026, 7, 10, 12, 0, 0, 123456789, location),
		Message:   "migrated timestamp",
	}
	if _, err := src.CreateEntry(want); err != nil {
		t.Fatalf("create source entry: %v", err)
	}

	dst, err := NewSqliteStore(filepath.Join(t.TempDir(), "chronicle.db"))
	if err != nil {
		t.Fatalf("create destination store: %v", err)
	}
	defer dst.Close()

	summary, err := MigrateData(src, dst)
	if err != nil {
		t.Fatalf("migrate entry: %v", err)
	}
	if summary.Entries != 1 {
		t.Fatalf("migrated entries = %d, want 1", summary.Entries)
	}

	got, err := dst.GetEntry(want.ID)
	if err != nil {
		t.Fatalf("read migrated entry: %v", err)
	}
	if !got.Timestamp.Equal(want.Timestamp) || got.Timestamp.Nanosecond() != want.Timestamp.Nanosecond() {
		t.Fatalf("timestamp = %v, want instant %v", got.Timestamp, want.Timestamp)
	}
	_, gotOffset := got.Timestamp.Zone()
	_, wantOffset := want.Timestamp.Zone()
	if gotOffset != wantOffset {
		t.Fatalf("offset = %d, want %d", gotOffset, wantOffset)
	}
}
