// ABOUTME: Tests for MarkdownStore file-based storage backend
// ABOUTME: Covers CRUD, search, migration, edge cases, and concurrency

package storage

import (
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// newTestMarkdownStore creates a MarkdownStore in a temporary directory for testing.
func newTestMarkdownStore(t *testing.T) *MarkdownStore {
	t.Helper()
	tmpDir := t.TempDir()
	store, err := NewMarkdownStore(tmpDir)
	if err != nil {
		t.Fatalf("failed to create test markdown store: %v", err)
	}
	return store
}

func markdownEntryFiles(t *testing.T, store *MarkdownStore, id string) []string {
	t.Helper()
	var paths []string
	err := filepath.Walk(store.dataDir, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if info.IsDir() || !strings.HasSuffix(path, ".md") {
			return nil
		}
		entry, err := parseEntryFile(path)
		if err != nil {
			return err
		}
		if entry.ID == id {
			paths = append(paths, path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("failed to inspect markdown entry files: %v", err)
	}
	return paths
}

func TestNewMarkdownStore(t *testing.T) {
	tmpDir := t.TempDir()
	dataDir := filepath.Join(tmpDir, "chronicle-data")

	store, err := NewMarkdownStore(dataDir)
	if err != nil {
		t.Fatalf("NewMarkdownStore failed: %v", err)
	}
	defer store.Close()

	if store == nil {
		t.Fatal("NewMarkdownStore returned nil")
	}

	// Verify data directory exists
	if _, err := os.Stat(dataDir); os.IsNotExist(err) {
		t.Fatal("Data directory was not created")
	}
}

func TestMarkdownCreateEntry(t *testing.T) {
	store := newTestMarkdownStore(t)
	defer store.Close()

	entry := Entry{
		Timestamp:        time.Now(),
		Message:          "test message",
		Hostname:         "test-host",
		Username:         "test-user",
		WorkingDirectory: "/tmp",
		Tags:             []string{"test", "unit"},
	}

	id, err := store.CreateEntry(entry)
	if err != nil {
		t.Fatalf("failed to create entry: %v", err)
	}

	if id == "" {
		t.Error("expected non-empty ID")
	}
}

func TestMarkdownGetEntry(t *testing.T) {
	store := newTestMarkdownStore(t)
	defer store.Close()

	now := time.Now().Truncate(time.Second).UTC()
	entry := Entry{
		Timestamp:        now,
		Message:          "test message",
		Hostname:         "test-host",
		Username:         "test-user",
		WorkingDirectory: "/tmp",
		Tags:             []string{"test", "unit"},
	}

	id, err := store.CreateEntry(entry)
	if err != nil {
		t.Fatalf("failed to create entry: %v", err)
	}

	retrieved, err := store.GetEntry(id)
	if err != nil {
		t.Fatalf("failed to get entry: %v", err)
	}

	if retrieved.ID != id {
		t.Errorf("expected ID %s, got %s", id, retrieved.ID)
	}
	if retrieved.Message != entry.Message {
		t.Errorf("expected message %q, got %q", entry.Message, retrieved.Message)
	}
	if retrieved.Hostname != entry.Hostname {
		t.Errorf("expected hostname %q, got %q", entry.Hostname, retrieved.Hostname)
	}
	if retrieved.Username != entry.Username {
		t.Errorf("expected username %q, got %q", entry.Username, retrieved.Username)
	}
	if retrieved.WorkingDirectory != entry.WorkingDirectory {
		t.Errorf("expected working directory %q, got %q", entry.WorkingDirectory, retrieved.WorkingDirectory)
	}
	if len(retrieved.Tags) != len(entry.Tags) {
		t.Errorf("expected %d tags, got %d", len(entry.Tags), len(retrieved.Tags))
	}
}

func TestMarkdownGetEntryNotFound(t *testing.T) {
	store := newTestMarkdownStore(t)
	defer store.Close()

	_, err := store.GetEntry("nonexistent-id")
	if err == nil {
		t.Error("expected error for nonexistent entry")
	}
}

func TestMarkdownUpdateEntry(t *testing.T) {
	store := newTestMarkdownStore(t)
	defer store.Close()

	entry := Entry{
		Timestamp: time.Now(),
		Message:   "original message",
		Hostname:  "test-host",
		Username:  "test-user",
		Tags:      []string{"original"},
	}

	id, err := store.CreateEntry(entry)
	if err != nil {
		t.Fatalf("failed to create entry: %v", err)
	}

	// Update the entry
	entry.ID = id
	entry.Message = "updated message"
	entry.Tags = []string{"updated", "changed"}

	if err := store.UpdateEntry(entry); err != nil {
		t.Fatalf("failed to update entry: %v", err)
	}

	// Verify update
	retrieved, err := store.GetEntry(id)
	if err != nil {
		t.Fatalf("failed to get entry: %v", err)
	}

	if retrieved.Message != "updated message" {
		t.Errorf("expected updated message, got %q", retrieved.Message)
	}
	if len(retrieved.Tags) != 2 {
		t.Errorf("expected 2 tags, got %d", len(retrieved.Tags))
	}
}

func TestMarkdownUpdateEntrySamePathIsAtomic(t *testing.T) {
	store := newTestMarkdownStore(t)
	defer store.Close()

	entry := Entry{
		ID:        "same-path-update",
		Timestamp: time.Date(2026, 7, 9, 12, 0, 0, 0, time.UTC),
		Message:   "stable path",
		Tags:      []string{"before"},
	}
	if _, err := store.CreateEntry(entry); err != nil {
		t.Fatalf("failed to create entry: %v", err)
	}
	path, err := store.findEntryFile(entry.ID)
	if err != nil {
		t.Fatalf("failed to find entry: %v", err)
	}
	beforeData, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read entry before update: %v", err)
	}
	beforeModified := store.LastModified()

	entry.Tags = []string{"after"}
	entry.Hostname = "updated-host"
	expectedContent, err := renderEntry(&entry)
	if err != nil {
		t.Fatalf("failed to render expected entry: %v", err)
	}
	if err := store.UpdateEntry(entry); err != nil {
		t.Fatalf("same-path update failed: %v", err)
	}

	afterPath, err := store.findEntryFile(entry.ID)
	if err != nil {
		t.Fatalf("failed to find updated entry: %v", err)
	}
	if afterPath != path {
		t.Errorf("same-path update moved entry: got %q, want %q", afterPath, path)
	}
	paths := markdownEntryFiles(t, store, entry.ID)
	if len(paths) != 1 {
		t.Fatalf("expected one file for ID, got %d: %v", len(paths), paths)
	}
	afterData, err := os.ReadFile(afterPath)
	if err != nil {
		t.Fatalf("failed to read entry after update: %v", err)
	}
	if string(afterData) == string(beforeData) {
		t.Error("same-path update did not change file bytes")
	}
	if string(afterData) != expectedContent {
		t.Errorf("same-path update bytes differ from rendered entry:\ngot:  %q\nwant: %q", afterData, expectedContent)
	}
	if !store.LastModified().After(beforeModified) {
		t.Error("LastModified did not advance after successful same-path update")
	}
}

func TestMarkdownUpdateEntryNotFound(t *testing.T) {
	store := newTestMarkdownStore(t)
	defer store.Close()
	beforeModified := store.LastModified()

	entry := Entry{
		ID:        "nonexistent-id",
		Timestamp: time.Now(),
		Message:   "test",
	}

	err := store.UpdateEntry(entry)
	if err == nil {
		t.Error("expected error for nonexistent entry")
	}
	if !store.LastModified().Equal(beforeModified) {
		t.Error("LastModified changed after failed update")
	}
}

func TestMarkdownUpdateEntryNoID(t *testing.T) {
	store := newTestMarkdownStore(t)
	defer store.Close()

	entry := Entry{
		Timestamp: time.Now(),
		Message:   "test",
	}

	err := store.UpdateEntry(entry)
	if err == nil {
		t.Error("expected error for entry without ID")
	}
}

func TestMarkdownUpdateEntryRejectsUnexpectedDestination(t *testing.T) {
	store := newTestMarkdownStore(t)
	defer store.Close()

	original := Entry{
		ID:        "entry-being-moved",
		Timestamp: time.Date(2026, 7, 9, 12, 0, 0, 0, time.UTC),
		Message:   "original message",
	}
	if _, err := store.CreateEntry(original); err != nil {
		t.Fatalf("failed to create original entry: %v", err)
	}
	originalPath, err := store.findEntryFile(original.ID)
	if err != nil {
		t.Fatalf("failed to find original entry: %v", err)
	}

	updated := original
	updated.Timestamp = original.Timestamp.Add(24 * time.Hour)
	updated.Message = "moved message"
	destinationPath := filepath.Join(store.entryDirPath(updated.Timestamp), entryFileName(updated.Message, updated.ID))
	if err := os.MkdirAll(filepath.Dir(destinationPath), 0750); err != nil {
		t.Fatalf("failed to create destination directory: %v", err)
	}
	unexpected := Entry{ID: "unexpected-entry", Timestamp: updated.Timestamp, Message: "must be preserved"}
	unexpectedContent, err := renderEntry(&unexpected)
	if err != nil {
		t.Fatalf("failed to render unexpected entry: %v", err)
	}
	if err := os.WriteFile(destinationPath, []byte(unexpectedContent), 0600); err != nil {
		t.Fatalf("failed to create unexpected destination: %v", err)
	}
	before := store.LastModified()

	if err := store.UpdateEntry(updated); err == nil {
		t.Fatal("expected update to reject an unexpected destination")
	}
	if !store.LastModified().Equal(before) {
		t.Error("LastModified changed after rejected update")
	}
	storedOriginal, err := parseEntryFile(originalPath)
	if err != nil {
		t.Fatalf("original entry was not preserved: %v", err)
	}
	if storedOriginal.Message != original.Message {
		t.Errorf("original entry changed: got %q, want %q", storedOriginal.Message, original.Message)
	}
	storedUnexpected, err := parseEntryFile(destinationPath)
	if err != nil {
		t.Fatalf("unexpected destination was not preserved: %v", err)
	}
	if storedUnexpected.ID != unexpected.ID || storedUnexpected.Message != unexpected.Message {
		t.Errorf("unexpected destination changed: got ID %q and message %q", storedUnexpected.ID, storedUnexpected.Message)
	}
}

func TestMarkdownDeleteEntry(t *testing.T) {
	store := newTestMarkdownStore(t)
	defer store.Close()

	entry := Entry{
		Timestamp: time.Now(),
		Message:   "to be deleted",
	}

	id, err := store.CreateEntry(entry)
	if err != nil {
		t.Fatalf("failed to create entry: %v", err)
	}

	if err := store.DeleteEntry(id); err != nil {
		t.Fatalf("failed to delete entry: %v", err)
	}

	// Verify deleted
	_, err = store.GetEntry(id)
	if err == nil {
		t.Error("expected error for deleted entry")
	}
}

func TestMarkdownDeleteEntryNotFound(t *testing.T) {
	store := newTestMarkdownStore(t)
	defer store.Close()
	beforeModified := store.LastModified()

	err := store.DeleteEntry("nonexistent-id")
	if err == nil {
		t.Error("expected error for nonexistent entry")
	}
	if !store.LastModified().Equal(beforeModified) {
		t.Error("LastModified changed after failed delete")
	}
}

func TestMarkdownListEntries(t *testing.T) {
	store := newTestMarkdownStore(t)
	defer store.Close()

	now := time.Now()
	// Create multiple entries with clearly different timestamps
	for i := 0; i < 5; i++ {
		entry := Entry{
			Timestamp: now.Add(time.Duration(i) * time.Hour),
			Message:   fmt.Sprintf("test message %d", i),
		}
		if _, err := store.CreateEntry(entry); err != nil {
			t.Fatalf("failed to create entry: %v", err)
		}
	}

	entries, err := store.ListEntries(3)
	if err != nil {
		t.Fatalf("failed to list entries: %v", err)
	}

	if len(entries) != 3 {
		t.Errorf("expected 3 entries, got %d", len(entries))
	}

	// Verify entries are ordered by timestamp descending (most recent first)
	for i := 1; i < len(entries); i++ {
		if entries[i].Timestamp.After(entries[i-1].Timestamp) {
			t.Errorf("entries not sorted by timestamp descending: entry[%d]=%v > entry[%d]=%v",
				i, entries[i].Timestamp, i-1, entries[i-1].Timestamp)
		}
	}
}

func TestMarkdownListEntriesEmpty(t *testing.T) {
	store := newTestMarkdownStore(t)
	defer store.Close()

	entries, err := store.ListEntries(10)
	if err != nil {
		t.Fatalf("failed to list entries: %v", err)
	}

	if len(entries) != 0 {
		t.Errorf("expected 0 entries, got %d", len(entries))
	}
	if entries != nil {
		t.Errorf("expected nil entries for an empty store, got %#v", entries)
	}
}

func TestMarkdownSearchEntriesByText(t *testing.T) {
	store := newTestMarkdownStore(t)
	defer store.Close()

	entries := []Entry{
		{Timestamp: time.Now(), Message: "deployed app to production"},
		{Timestamp: time.Now(), Message: "fixed bug in login"},
		{Timestamp: time.Now(), Message: "deployed new feature"},
	}

	for _, e := range entries {
		if _, err := store.CreateEntry(e); err != nil {
			t.Fatalf("failed to create entry: %v", err)
		}
	}

	filter := &SearchFilter{Text: "deployed"}
	results, err := store.SearchEntries(filter, 10)
	if err != nil {
		t.Fatalf("failed to search entries: %v", err)
	}

	if len(results) != 2 {
		t.Errorf("expected 2 results, got %d", len(results))
	}
}

func TestMarkdownSearchEntriesNoMatchesReturnsNil(t *testing.T) {
	store := newTestMarkdownStore(t)
	defer store.Close()

	if _, err := store.CreateEntry(Entry{Timestamp: time.Now(), Message: "present text"}); err != nil {
		t.Fatalf("failed to create entry: %v", err)
	}
	results, err := store.SearchEntries(&SearchFilter{Text: "missing text"}, 10)
	if err != nil {
		t.Fatalf("failed to search entries: %v", err)
	}
	if results != nil {
		t.Errorf("expected nil results for a search with no matches, got %#v", results)
	}
}

func TestMarkdownSearchEntriesByTags(t *testing.T) {
	store := newTestMarkdownStore(t)
	defer store.Close()

	entries := []Entry{
		{Timestamp: time.Now(), Message: "deployed app", Tags: []string{"deployment", "production"}},
		{Timestamp: time.Now(), Message: "fixed bug", Tags: []string{"bugfix"}},
		{Timestamp: time.Now(), Message: "new feature", Tags: []string{"feature", "production"}},
	}

	for _, e := range entries {
		if _, err := store.CreateEntry(e); err != nil {
			t.Fatalf("failed to create entry: %v", err)
		}
	}

	filter := &SearchFilter{Tags: []string{"production"}}
	results, err := store.SearchEntries(filter, 10)
	if err != nil {
		t.Fatalf("failed to search entries: %v", err)
	}

	if len(results) != 2 {
		t.Errorf("expected 2 results, got %d", len(results))
	}
}

func TestMarkdownSearchEntriesByDateRange(t *testing.T) {
	store := newTestMarkdownStore(t)
	defer store.Close()

	now := time.Now()
	entries := []Entry{
		{Timestamp: now.Add(-72 * time.Hour), Message: "three days ago"},
		{Timestamp: now.Add(-24 * time.Hour), Message: "yesterday"},
		{Timestamp: now, Message: "today"},
	}

	for _, e := range entries {
		if _, err := store.CreateEntry(e); err != nil {
			t.Fatalf("failed to create entry: %v", err)
		}
	}

	since := now.Add(-48 * time.Hour)
	filter := &SearchFilter{Since: &since}
	results, err := store.SearchEntries(filter, 10)
	if err != nil {
		t.Fatalf("failed to search entries: %v", err)
	}

	if len(results) != 2 {
		t.Errorf("expected 2 results, got %d", len(results))
	}
}

func TestMarkdownSearchEntriesWithUntilFilter(t *testing.T) {
	store := newTestMarkdownStore(t)
	defer store.Close()

	now := time.Now()
	entries := []Entry{
		{Timestamp: now.Add(-72 * time.Hour), Message: "three days ago"},
		{Timestamp: now.Add(-24 * time.Hour), Message: "yesterday"},
		{Timestamp: now, Message: "today"},
	}

	for _, e := range entries {
		if _, err := store.CreateEntry(e); err != nil {
			t.Fatalf("failed to create entry: %v", err)
		}
	}

	until := now.Add(-12 * time.Hour)
	filter := &SearchFilter{Until: &until}
	results, err := store.SearchEntries(filter, 10)
	if err != nil {
		t.Fatalf("failed to search entries: %v", err)
	}

	if len(results) != 2 {
		t.Errorf("expected 2 results (entries before 'until'), got %d", len(results))
	}
}

func TestMarkdownSearchEntriesCombinedFilters(t *testing.T) {
	store := newTestMarkdownStore(t)
	defer store.Close()

	now := time.Now()
	entries := []Entry{
		{Timestamp: now.Add(-48 * time.Hour), Message: "deployed old version", Tags: []string{"deployment"}},
		{Timestamp: now.Add(-24 * time.Hour), Message: "deployed new version", Tags: []string{"deployment"}},
		{Timestamp: now, Message: "fixed bug", Tags: []string{"bugfix"}},
	}

	for _, e := range entries {
		if _, err := store.CreateEntry(e); err != nil {
			t.Fatalf("failed to create entry: %v", err)
		}
	}

	// Search with text and date range
	since := now.Add(-36 * time.Hour)
	filter := &SearchFilter{
		Text:  "deployed",
		Since: &since,
	}
	results, err := store.SearchEntries(filter, 10)
	if err != nil {
		t.Fatalf("failed to search entries: %v", err)
	}

	if len(results) != 1 {
		t.Errorf("expected 1 result (deployed within range), got %d", len(results))
	}
}

func TestMarkdownSearchEntriesTextAndTags(t *testing.T) {
	store := newTestMarkdownStore(t)
	defer store.Close()

	entries := []Entry{
		{Timestamp: time.Now(), Message: "deployed app", Tags: []string{"deployment", "production"}},
		{Timestamp: time.Now(), Message: "deployed app", Tags: []string{"deployment", "staging"}},
		{Timestamp: time.Now(), Message: "fixed bug", Tags: []string{"bugfix"}},
	}

	for _, e := range entries {
		if _, err := store.CreateEntry(e); err != nil {
			t.Fatalf("failed to create entry: %v", err)
		}
	}

	filter := &SearchFilter{
		Text: "deployed",
		Tags: []string{"production"},
	}
	results, err := store.SearchEntries(filter, 10)
	if err != nil {
		t.Fatalf("failed to search entries: %v", err)
	}

	if len(results) != 1 {
		t.Errorf("expected 1 result (deployed + production tag), got %d", len(results))
	}
}

func TestMarkdownSearchEntriesMultipleTags(t *testing.T) {
	store := newTestMarkdownStore(t)
	defer store.Close()

	entries := []Entry{
		{Timestamp: time.Now(), Message: "entry 1", Tags: []string{"work", "golang"}},
		{Timestamp: time.Now(), Message: "entry 2", Tags: []string{"work", "python"}},
		{Timestamp: time.Now(), Message: "entry 3", Tags: []string{"personal"}},
	}

	for _, e := range entries {
		if _, err := store.CreateEntry(e); err != nil {
			t.Fatalf("failed to create entry: %v", err)
		}
	}

	// Search with OR logic for tags
	filter := &SearchFilter{
		Tags: []string{"golang", "python"},
	}
	results, err := store.SearchEntries(filter, 10)
	if err != nil {
		t.Fatalf("failed to search entries: %v", err)
	}

	if len(results) != 2 {
		t.Errorf("expected 2 results (entries with golang OR python), got %d", len(results))
	}
}

func TestMarkdownSearchEntriesNoLimit(t *testing.T) {
	store := newTestMarkdownStore(t)
	defer store.Close()

	// Create many entries
	for i := 0; i < 50; i++ {
		entry := Entry{
			Timestamp: time.Now().Add(time.Duration(i) * time.Second),
			Message:   fmt.Sprintf("entry %d", i),
		}
		if _, err := store.CreateEntry(entry); err != nil {
			t.Fatalf("failed to create entry: %v", err)
		}
	}

	// Search with 0 limit (no limit)
	results, err := store.SearchEntries(nil, 0)
	if err != nil {
		t.Fatalf("failed to search entries: %v", err)
	}

	if len(results) != 50 {
		t.Errorf("expected 50 results with no limit, got %d", len(results))
	}
}

func TestMarkdownSearchEntriesCaseInsensitive(t *testing.T) {
	store := newTestMarkdownStore(t)
	defer store.Close()

	entries := []Entry{
		{Timestamp: time.Now(), Message: "Deployed APP to Production"},
		{Timestamp: time.Now(), Message: "fixed a small bug"},
	}

	for _, e := range entries {
		if _, err := store.CreateEntry(e); err != nil {
			t.Fatalf("failed to create entry: %v", err)
		}
	}

	filter := &SearchFilter{Text: "deployed"}
	results, err := store.SearchEntries(filter, 10)
	if err != nil {
		t.Fatalf("failed to search entries: %v", err)
	}

	if len(results) != 1 {
		t.Errorf("expected 1 result (case-insensitive search), got %d", len(results))
	}
}

func TestMarkdownSearchEntriesEmptyText(t *testing.T) {
	store := newTestMarkdownStore(t)
	defer store.Close()

	entry := Entry{
		Timestamp: time.Now(),
		Message:   "test entry",
	}
	if _, err := store.CreateEntry(entry); err != nil {
		t.Fatalf("failed to create entry: %v", err)
	}

	// Empty text search should return all entries
	filter := &SearchFilter{Text: ""}
	results, err := store.SearchEntries(filter, 10)
	if err != nil {
		t.Fatalf("failed to search entries: %v", err)
	}

	if len(results) != 1 {
		t.Errorf("expected 1 result, got %d", len(results))
	}
}

func TestMarkdownLastModified(t *testing.T) {
	store := newTestMarkdownStore(t)
	defer store.Close()

	before := store.LastModified()

	// Small delay to ensure timestamp difference
	time.Sleep(time.Millisecond)

	// Create an entry
	entry := Entry{
		Timestamp: time.Now(),
		Message:   "test",
	}
	if _, err := store.CreateEntry(entry); err != nil {
		t.Fatalf("failed to create entry: %v", err)
	}

	after := store.LastModified()
	if !after.After(before) && !after.Equal(before) {
		t.Error("LastModified should be updated after creating entry")
	}
}

func TestMarkdownVacuum(t *testing.T) {
	store := newTestMarkdownStore(t)
	defer store.Close()

	// Vacuum is a no-op for markdown store, should not error
	if err := store.Vacuum(); err != nil {
		t.Errorf("vacuum failed: %v", err)
	}
}

func TestMarkdownReset(t *testing.T) {
	store := newTestMarkdownStore(t)
	defer store.Close()

	// Create some entries
	for i := 0; i < 5; i++ {
		entry := Entry{
			Timestamp: time.Now().Add(time.Duration(i) * time.Second),
			Message:   fmt.Sprintf("entry %d", i),
		}
		if _, err := store.CreateEntry(entry); err != nil {
			t.Fatalf("failed to create entry: %v", err)
		}
	}

	// Verify entries exist
	entries, err := store.ListEntries(0)
	if err != nil {
		t.Fatalf("failed to list entries: %v", err)
	}
	if len(entries) != 5 {
		t.Fatalf("expected 5 entries before reset, got %d", len(entries))
	}

	// Reset
	if err := store.Reset(); err != nil {
		t.Fatalf("reset failed: %v", err)
	}

	// Verify entries are gone
	entries, err = store.ListEntries(0)
	if err != nil {
		t.Fatalf("failed to list entries after reset: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("expected 0 entries after reset, got %d", len(entries))
	}
}

func TestMarkdownCreateEntryWithExistingID(t *testing.T) {
	store := newTestMarkdownStore(t)
	defer store.Close()

	entry := Entry{
		ID:        "custom-id-123",
		Timestamp: time.Now(),
		Message:   "test",
	}

	id, err := store.CreateEntry(entry)
	if err != nil {
		t.Fatalf("failed to create entry: %v", err)
	}

	if id != "custom-id-123" {
		t.Errorf("expected ID 'custom-id-123', got '%s'", id)
	}
}

func TestMarkdownCreateEntryRejectsDuplicateIDAcrossPaths(t *testing.T) {
	store := newTestMarkdownStore(t)
	defer store.Close()

	original := Entry{
		ID:        "duplicate-id",
		Timestamp: time.Date(2026, 7, 9, 12, 0, 0, 0, time.UTC),
		Message:   "original message",
	}
	if _, err := store.CreateEntry(original); err != nil {
		t.Fatalf("failed to create original entry: %v", err)
	}
	originalPath, err := store.findEntryFile(original.ID)
	if err != nil {
		t.Fatalf("failed to find original entry: %v", err)
	}
	originalData, err := os.ReadFile(originalPath)
	if err != nil {
		t.Fatalf("failed to read original entry: %v", err)
	}

	duplicate := Entry{
		ID:        original.ID,
		Timestamp: original.Timestamp.Add(24 * time.Hour),
		Message:   "replacement message",
	}
	if _, err = store.CreateEntry(duplicate); !errors.Is(err, errMarkdownEntryAlreadyExists) {
		t.Fatalf("expected duplicate-ID error, got %v", err)
	}
	samePathDuplicate := original
	samePathDuplicate.Tags = []string{"replacement"}
	if _, err = store.CreateEntry(samePathDuplicate); !errors.Is(err, errMarkdownEntryAlreadyExists) {
		t.Fatalf("expected same-path duplicate-ID error, got %v", err)
	}

	entries, err := store.ListEntries(0)
	if err != nil {
		t.Fatalf("failed to list entries after rejection: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected one entry after rejection, got %d", len(entries))
	}
	afterData, err := os.ReadFile(originalPath)
	if err != nil {
		t.Fatalf("failed to read original entry after rejection: %v", err)
	}
	if string(afterData) != string(originalData) {
		t.Error("original entry file changed after duplicate rejection")
	}
	retrieved, err := store.GetEntry(original.ID)
	if err != nil {
		t.Fatalf("failed to get original entry after rejection: %v", err)
	}
	if retrieved.Message != original.Message || !retrieved.Timestamp.Equal(original.Timestamp) {
		t.Errorf("original entry changed after rejection: got message %q at %v", retrieved.Message, retrieved.Timestamp)
	}
}

func TestMarkdownCreateEntryFailsClosedForMalformedCandidate(t *testing.T) {
	store := newTestMarkdownStore(t)
	defer store.Close()

	entry := Entry{
		ID:        "malformed-candidate-id",
		Timestamp: time.Date(2026, 7, 9, 12, 0, 0, 0, time.UTC),
		Message:   "malformed candidate",
	}
	path := filepath.Join(store.entryDirPath(entry.Timestamp), entryFileName(entry.Message, entry.ID))
	if err := os.MkdirAll(filepath.Dir(path), 0750); err != nil {
		t.Fatalf("failed to create candidate directory: %v", err)
	}
	originalData := []byte("not valid chronicle frontmatter\n")
	if err := os.WriteFile(path, originalData, 0600); err != nil {
		t.Fatalf("failed to write malformed candidate: %v", err)
	}

	_, err := store.CreateEntry(entry)
	if err == nil {
		t.Fatal("expected uniqueness scan failure for malformed candidate")
	}
	if !strings.Contains(err.Error(), "check entry ID uniqueness") {
		t.Errorf("expected uniqueness scan error, got %v", err)
	}
	if errors.Is(err, errMarkdownEntryAlreadyExists) || errors.Is(err, errMarkdownEntryNotFound) {
		t.Errorf("expected inspection failure rather than duplicate/not-found error, got %v", err)
	}
	afterData, readErr := os.ReadFile(path)
	if readErr != nil {
		t.Fatalf("failed to read malformed candidate after rejection: %v", readErr)
	}
	if string(afterData) != string(originalData) {
		t.Error("malformed candidate was overwritten after uniqueness scan failure")
	}
}

func TestMarkdownConcurrentCreateEntryRejectsDuplicateID(t *testing.T) {
	store := newTestMarkdownStore(t)
	defer store.Close()

	entries := []Entry{
		{ID: "concurrent-duplicate-id", Timestamp: time.Date(2026, 7, 9, 12, 0, 0, 0, time.UTC), Message: "first message"},
		{ID: "concurrent-duplicate-id", Timestamp: time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC), Message: "second message"},
	}
	start := make(chan struct{})
	type createResult struct {
		entry Entry
		err   error
	}
	results := make(chan createResult, len(entries))
	var wg sync.WaitGroup
	for _, entry := range entries {
		wg.Add(1)
		go func(entry Entry) {
			defer wg.Done()
			<-start
			_, err := store.CreateEntry(entry)
			results <- createResult{entry: entry, err: err}
		}(entry)
	}
	close(start)
	wg.Wait()
	close(results)

	var successes, failures int
	var winner Entry
	for result := range results {
		if result.err == nil {
			successes++
			winner = result.entry
		} else {
			failures++
			if !errors.Is(result.err, errMarkdownEntryAlreadyExists) {
				t.Errorf("expected duplicate-specific rejection, got %v", result.err)
			}
		}
	}
	if successes != 1 || failures != 1 {
		t.Fatalf("expected one successful create and one rejection, got %d successes and %d rejections", successes, failures)
	}
	listed, err := store.ListEntries(0)
	if err != nil {
		t.Fatalf("failed to list entries: %v", err)
	}
	if len(listed) != 1 {
		t.Fatalf("expected one stored entry, got %d", len(listed))
	}
	if listed[0].Message != winner.Message || !listed[0].Timestamp.Equal(winner.Timestamp) {
		t.Errorf("stored entry does not match successful submission: got message %q at %v, want message %q at %v",
			listed[0].Message, listed[0].Timestamp, winner.Message, winner.Timestamp)
	}
}

func TestMarkdownDistinctFullIDsDoNotOverwrite(t *testing.T) {
	store := newTestMarkdownStore(t)
	defer store.Close()

	timestamp := time.Date(2026, 7, 9, 12, 0, 0, 0, time.UTC)
	entries := []Entry{
		{ID: "abcdefgh-first", Timestamp: timestamp, Message: "same message"},
		{ID: "abcdefgh-second", Timestamp: timestamp, Message: "same message"},
	}

	for _, entry := range entries {
		if _, err := store.CreateEntry(entry); err != nil {
			t.Fatalf("failed to create entry %q: %v", entry.ID, err)
		}
	}

	listed, err := store.ListEntries(0)
	if err != nil {
		t.Fatalf("failed to list entries: %v", err)
	}
	if len(listed) != len(entries) {
		t.Errorf("expected %d entries, got %d", len(entries), len(listed))
	}

	for _, entry := range entries {
		retrieved, err := store.GetEntry(entry.ID)
		if err != nil {
			t.Errorf("failed to get entry %q: %v", entry.ID, err)
			continue
		}
		if retrieved.ID != entry.ID {
			t.Errorf("expected ID %q, got %q", entry.ID, retrieved.ID)
		}
	}
}

func TestMarkdownCaseFoldDistinctIDsDoNotOverwrite(t *testing.T) {
	store := newTestMarkdownStore(t)
	defer store.Close()

	timestamp := time.Date(2026, 7, 9, 12, 0, 0, 0, time.UTC)
	entries := []Entry{
		{ID: "aaa", Timestamp: timestamp, Message: "same message"},
		{ID: "aaG", Timestamp: timestamp, Message: "same message"},
	}
	firstEncoding := base64.RawURLEncoding.EncodeToString([]byte(entries[0].ID))
	secondEncoding := base64.RawURLEncoding.EncodeToString([]byte(entries[1].ID))
	if firstEncoding == secondEncoding || !strings.EqualFold(firstEncoding, secondEncoding) {
		t.Fatalf("test IDs must have distinct base64 encodings that differ only by case: %q, %q", firstEncoding, secondEncoding)
	}
	if strings.EqualFold(entryFileName(entries[0].Message, entries[0].ID), entryFileName(entries[1].Message, entries[1].ID)) {
		t.Error("entry filenames collide on a case-insensitive filesystem")
	}

	for _, entry := range entries {
		if _, err := store.CreateEntry(entry); err != nil {
			t.Fatalf("failed to create entry %q: %v", entry.ID, err)
		}
	}
	for _, entry := range entries {
		retrieved, err := store.GetEntry(entry.ID)
		if err != nil {
			t.Errorf("failed to get entry %q: %v", entry.ID, err)
			continue
		}
		if retrieved.ID != entry.ID {
			t.Errorf("expected ID %q, got %q", entry.ID, retrieved.ID)
		}
	}
}

func TestMarkdownLongCustomIDFitsFilenameLimit(t *testing.T) {
	store := newTestMarkdownStore(t)
	defer store.Close()

	entry := Entry{
		ID:        strings.Repeat("very-long-custom-id-", 32),
		Timestamp: time.Date(2026, 7, 9, 12, 0, 0, 0, time.UTC),
		Message:   "long custom ID",
	}
	if _, err := store.CreateEntry(entry); err != nil {
		t.Fatalf("failed to create entry with long custom ID: %v", err)
	}
	retrieved, err := store.GetEntry(entry.ID)
	if err != nil {
		t.Fatalf("failed to get entry with long custom ID: %v", err)
	}
	if retrieved.ID != entry.ID {
		t.Errorf("expected long custom ID to round trip")
	}
}

func TestMarkdownReadsLegacyShortIDFilename(t *testing.T) {
	store := newTestMarkdownStore(t)
	defer store.Close()

	entry := Entry{
		ID:        "abcdefgh-legacy-full-id",
		Timestamp: time.Date(2026, 7, 9, 12, 0, 0, 0, time.UTC),
		Message:   "legacy entry",
	}
	content, err := renderEntry(&entry)
	if err != nil {
		t.Fatalf("failed to render legacy entry: %v", err)
	}
	dir := store.entryDirPath(entry.Timestamp)
	if err := os.MkdirAll(dir, 0750); err != nil {
		t.Fatalf("failed to create legacy directory: %v", err)
	}
	legacyPath := filepath.Join(dir, "legacy-entry-abcdefgh.md")
	if err := os.WriteFile(legacyPath, []byte(content), 0600); err != nil {
		t.Fatalf("failed to write legacy entry: %v", err)
	}

	retrieved, err := store.GetEntry(entry.ID)
	if err != nil {
		t.Fatalf("failed to get legacy entry: %v", err)
	}
	if retrieved.ID != entry.ID {
		t.Errorf("expected ID %q, got %q", entry.ID, retrieved.ID)
	}
}

func TestMarkdownCreateEntryWithoutTimestamp(t *testing.T) {
	store := newTestMarkdownStore(t)
	defer store.Close()

	entry := Entry{
		Message: "test without timestamp",
	}

	id, err := store.CreateEntry(entry)
	if err != nil {
		t.Fatalf("failed to create entry: %v", err)
	}

	// Verify entry was created with a timestamp
	retrieved, err := store.GetEntry(id)
	if err != nil {
		t.Fatalf("failed to get entry: %v", err)
	}

	if retrieved.Timestamp.IsZero() {
		t.Error("expected timestamp to be set")
	}
}

func TestMarkdownCreateEntryWithNullTags(t *testing.T) {
	store := newTestMarkdownStore(t)
	defer store.Close()

	entry := Entry{
		Timestamp: time.Now(),
		Message:   "entry with nil tags",
		Tags:      nil,
	}

	id, err := store.CreateEntry(entry)
	if err != nil {
		t.Fatalf("failed to create entry: %v", err)
	}

	retrieved, err := store.GetEntry(id)
	if err != nil {
		t.Fatalf("failed to get entry: %v", err)
	}

	// Tags should be nil or empty, not cause an error
	if retrieved.Tags != nil && len(retrieved.Tags) != 0 {
		t.Errorf("expected nil/empty tags, got: %v", retrieved.Tags)
	}
}

func TestMarkdownClose(t *testing.T) {
	store := newTestMarkdownStore(t)
	err := store.Close()
	if err != nil {
		t.Fatalf("Close failed: %v", err)
	}
}

func TestMarkdownDirectoryStructure(t *testing.T) {
	store := newTestMarkdownStore(t)
	defer store.Close()

	now := time.Now()
	entry := Entry{
		Timestamp: now,
		Message:   "verify directory structure",
	}

	_, err := store.CreateEntry(entry)
	if err != nil {
		t.Fatalf("failed to create entry: %v", err)
	}

	// Verify the date-based directory exists
	expectedDir := filepath.Join(store.dataDir, now.Format("2006"), now.Format("01"), now.Format("02"))
	if _, err := os.Stat(expectedDir); os.IsNotExist(err) {
		t.Errorf("expected date directory at %s", expectedDir)
	}

	// Verify a .md file exists in the directory
	entries, err := os.ReadDir(expectedDir)
	if err != nil {
		t.Fatalf("failed to read directory: %v", err)
	}

	mdCount := 0
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".md") {
			mdCount++
		}
	}
	if mdCount != 1 {
		t.Errorf("expected 1 .md file, got %d", mdCount)
	}
}

func TestMarkdownMultilineMessage(t *testing.T) {
	store := newTestMarkdownStore(t)
	defer store.Close()

	content := "# Heading\n\nSome paragraph.\n\n```go\nfunc main() {\n\tprintln(\"hello\")\n}\n```\n\n- list item 1\n- list item 2"
	entry := Entry{
		Timestamp: time.Now(),
		Message:   content,
	}

	id, err := store.CreateEntry(entry)
	if err != nil {
		t.Fatalf("failed to create entry: %v", err)
	}

	retrieved, err := store.GetEntry(id)
	if err != nil {
		t.Fatalf("failed to get entry: %v", err)
	}

	if retrieved.Message != content {
		t.Errorf("multiline content mismatch:\nwant: %q\ngot:  %q", content, retrieved.Message)
	}
}

func TestMarkdownEntryWithFrontmatterChars(t *testing.T) {
	store := newTestMarkdownStore(t)
	defer store.Close()

	// Entry with --- in the message (should not corrupt frontmatter parsing)
	content := "Here is some content\n---\nThis looks like frontmatter\n---\nBut it's all one message"
	entry := Entry{
		Timestamp: time.Now(),
		Message:   content,
	}

	id, err := store.CreateEntry(entry)
	if err != nil {
		t.Fatalf("failed to create entry: %v", err)
	}

	retrieved, err := store.GetEntry(id)
	if err != nil {
		t.Fatalf("failed to get entry: %v", err)
	}

	if retrieved.Message != content {
		t.Errorf("content with frontmatter chars was corrupted:\nwant: %q\ngot:  %q", content, retrieved.Message)
	}
}

// --- Concurrency Tests ---

func TestMarkdownConcurrentUpdatesLeaveOneCurrentEntry(t *testing.T) {
	const iterations = 30
	for iteration := 0; iteration < iterations; iteration++ {
		store := newTestMarkdownStore(t)
		entry := Entry{
			ID:        fmt.Sprintf("concurrent-update-%d", iteration),
			Timestamp: time.Date(2026, 7, 8, 12, 0, 0, 0, time.UTC),
			Message:   "original message",
		}
		if _, err := store.CreateEntry(entry); err != nil {
			t.Fatalf("iteration %d: failed to create entry: %v", iteration, err)
		}

		updates := []Entry{entry, entry}
		updates[0].Timestamp = entry.Timestamp.Add(24 * time.Hour)
		updates[0].Message = "first concurrent update"
		updates[1].Timestamp = entry.Timestamp.Add(48 * time.Hour)
		updates[1].Message = "second concurrent update"
		start := make(chan struct{})
		errs := make(chan error, len(updates))
		var wg sync.WaitGroup
		for _, update := range updates {
			wg.Add(1)
			go func(update Entry) {
				defer wg.Done()
				<-start
				errs <- store.UpdateEntry(update)
			}(update)
		}
		close(start)
		wg.Wait()
		close(errs)
		for err := range errs {
			if err != nil {
				t.Fatalf("iteration %d: concurrent update failed: %v", iteration, err)
			}
		}

		paths := markdownEntryFiles(t, store, entry.ID)
		if len(paths) != 1 {
			t.Fatalf("iteration %d: expected one file for ID, got %d: %v", iteration, len(paths), paths)
		}
		stored, err := store.GetEntry(entry.ID)
		if err != nil {
			t.Fatalf("iteration %d: failed to get current entry: %v", iteration, err)
		}
		if stored.Message != updates[0].Message && stored.Message != updates[1].Message {
			t.Errorf("iteration %d: stored message %q does not match either successful update", iteration, stored.Message)
		}
	}
}

func TestMarkdownConcurrentUpdateDeleteHasConsistentOutcome(t *testing.T) {
	const iterations = 30
	for iteration := 0; iteration < iterations; iteration++ {
		store := newTestMarkdownStore(t)
		entry := Entry{
			ID:        fmt.Sprintf("concurrent-update-delete-%d", iteration),
			Timestamp: time.Date(2026, 7, 8, 12, 0, 0, 0, time.UTC),
			Message:   "original message",
		}
		if _, err := store.CreateEntry(entry); err != nil {
			t.Fatalf("iteration %d: failed to create entry: %v", iteration, err)
		}
		updated := entry
		updated.Timestamp = entry.Timestamp.Add(24 * time.Hour)
		updated.Message = "concurrent update"

		start := make(chan struct{})
		updateResult := make(chan error, 1)
		deleteResult := make(chan error, 1)
		go func() {
			<-start
			updateResult <- store.UpdateEntry(updated)
		}()
		go func() {
			<-start
			deleteResult <- store.DeleteEntry(entry.ID)
		}()
		close(start)
		updateErr := <-updateResult
		deleteErr := <-deleteResult
		paths := markdownEntryFiles(t, store, entry.ID)

		if len(paths) > 1 {
			t.Fatalf("iteration %d: expected at most one file for ID, got %d: %v", iteration, len(paths), paths)
		}
		if deleteErr == nil && len(paths) != 0 {
			t.Fatalf("iteration %d: delete reported success but entry remains at %v (update error: %v)", iteration, paths, updateErr)
		}
		if len(paths) == 1 {
			stored, err := parseEntryFile(paths[0])
			if err != nil {
				t.Fatalf("iteration %d: failed to parse surviving entry: %v", iteration, err)
			}
			if updateErr != nil || stored.Message != updated.Message {
				t.Fatalf("iteration %d: survivor is inconsistent with update result: update error=%v, message=%q", iteration, updateErr, stored.Message)
			}
		}
	}
}

func TestMarkdownConcurrentEntryCreation(t *testing.T) {
	store := newTestMarkdownStore(t)
	defer store.Close()

	const numGoroutines = 20
	var wg sync.WaitGroup
	errs := make(chan error, numGoroutines)

	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func(idx int) {
			defer wg.Done()
			entry := Entry{
				Timestamp: time.Now().Add(time.Duration(idx) * time.Millisecond),
				Message:   fmt.Sprintf("concurrent entry %d", idx),
				Tags:      []string{"concurrent"},
			}
			if _, err := store.CreateEntry(entry); err != nil {
				errs <- fmt.Errorf("goroutine %d: %w", idx, err)
			}
		}(i)
	}

	wg.Wait()
	close(errs)

	for err := range errs {
		t.Error(err)
	}

	// Verify all entries are readable
	entries, err := store.ListEntries(0)
	if err != nil {
		t.Fatalf("ListEntries failed: %v", err)
	}
	if len(entries) != numGoroutines {
		t.Errorf("expected %d entries, got %d", numGoroutines, len(entries))
	}
}

func TestMarkdownConcurrentMixedOperations(t *testing.T) {
	store := newTestMarkdownStore(t)
	defer store.Close()

	// Seed some entries
	var ids []string
	for i := 0; i < 10; i++ {
		entry := Entry{
			Timestamp: time.Now().Add(time.Duration(i) * time.Second),
			Message:   fmt.Sprintf("seed entry %d", i),
		}
		id, err := store.CreateEntry(entry)
		if err != nil {
			t.Fatalf("failed to seed entry: %v", err)
		}
		ids = append(ids, id)
	}

	const numGoroutines = 20
	var wg sync.WaitGroup
	errs := make(chan error, numGoroutines*2)

	// Writers: create new entries
	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func(idx int) {
			defer wg.Done()
			entry := Entry{
				Timestamp: time.Now().Add(time.Duration(idx+10) * time.Second),
				Message:   fmt.Sprintf("concurrent write %d", idx),
			}
			if _, err := store.CreateEntry(entry); err != nil {
				errs <- fmt.Errorf("write goroutine %d: %w", idx, err)
			}
		}(i)
	}

	// Readers: list entries concurrently with writes
	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func(idx int) {
			defer wg.Done()
			_, err := store.ListEntries(0)
			if err != nil {
				errs <- fmt.Errorf("read goroutine %d: %w", idx, err)
			}
		}(i)
	}

	wg.Wait()
	close(errs)

	for err := range errs {
		t.Error(err)
	}
}

func TestMarkdownConcurrentCRUDAndLastModified(t *testing.T) {
	store := newTestMarkdownStore(t)
	defer store.Close()

	const iterations = 30
	ready := make(chan struct{})
	start := make(chan struct{})
	done := make(chan struct{})
	errs := make(chan error, 1)

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		close(ready)
		<-start
		for {
			select {
			case <-done:
				return
			default:
				_ = store.LastModified()
			}
		}
	}()

	<-ready
	go func() {
		defer wg.Done()
		defer close(done)
		close(start)
		for i := 0; i < iterations; i++ {
			entry := Entry{
				Timestamp: time.Now(),
				Message:   fmt.Sprintf("entry %d", i),
			}
			id, err := store.CreateEntry(entry)
			if err != nil {
				errs <- fmt.Errorf("create entry %d: %w", i, err)
				return
			}
			entry.ID = id
			entry.Message = fmt.Sprintf("updated entry %d", i)
			if err := store.UpdateEntry(entry); err != nil {
				errs <- fmt.Errorf("update entry %d: %w", i, err)
				return
			}
			if err := store.DeleteEntry(id); err != nil {
				errs <- fmt.Errorf("delete entry %d: %w", i, err)
				return
			}
		}
	}()

	wg.Wait()
	close(errs)
	for err := range errs {
		t.Error(err)
	}
}

// --- Frontmatter Parsing Tests ---

func TestMarkdownEntryFrontmatterRoundTrip(t *testing.T) {
	store := newTestMarkdownStore(t)
	defer store.Close()

	entry := Entry{
		Timestamp:        time.Date(2026, 2, 5, 10, 30, 0, 0, time.UTC),
		Message:          "test frontmatter round trip",
		Hostname:         "my-host",
		Username:         "testuser",
		WorkingDirectory: "/home/testuser/projects",
		Tags:             []string{"test", "frontmatter"},
	}

	id, err := store.CreateEntry(entry)
	if err != nil {
		t.Fatalf("failed to create entry: %v", err)
	}

	retrieved, err := store.GetEntry(id)
	if err != nil {
		t.Fatalf("failed to get entry: %v", err)
	}

	if !retrieved.Timestamp.Equal(entry.Timestamp) {
		t.Errorf("timestamp mismatch: want %v, got %v", entry.Timestamp, retrieved.Timestamp)
	}
	if retrieved.Hostname != entry.Hostname {
		t.Errorf("hostname mismatch: want %q, got %q", entry.Hostname, retrieved.Hostname)
	}
	if retrieved.Username != entry.Username {
		t.Errorf("username mismatch: want %q, got %q", entry.Username, retrieved.Username)
	}
	if retrieved.WorkingDirectory != entry.WorkingDirectory {
		t.Errorf("working directory mismatch: want %q, got %q", entry.WorkingDirectory, retrieved.WorkingDirectory)
	}
}

func TestMarkdownNonUTCTimestampStaysInOriginalDateDirectory(t *testing.T) {
	store := newTestMarkdownStore(t)
	defer store.Close()

	location := time.FixedZone("UTC+14", 14*60*60)
	originalTimestamp := time.Date(2026, 7, 10, 0, 30, 0, 0, location)
	entry := Entry{
		ID:        "offset-entry",
		Timestamp: originalTimestamp,
		Message:   "before update",
	}
	if _, err := store.CreateEntry(entry); err != nil {
		t.Fatalf("failed to create entry: %v", err)
	}
	originalDir := store.entryDirPath(originalTimestamp)

	retrieved, err := store.GetEntry(entry.ID)
	if err != nil {
		t.Fatalf("failed to get entry: %v", err)
	}
	if !retrieved.Timestamp.Equal(originalTimestamp) {
		t.Errorf("timestamp instant changed: want %v, got %v", originalTimestamp, retrieved.Timestamp)
	}
	_, offset := retrieved.Timestamp.Zone()
	if offset != 14*60*60 {
		t.Errorf("timestamp offset changed: want %d, got %d", 14*60*60, offset)
	}

	retrieved.Message = "after update"
	if err := store.UpdateEntry(*retrieved); err != nil {
		t.Fatalf("failed to update entry: %v", err)
	}
	path, err := store.findEntryFile(entry.ID)
	if err != nil {
		t.Fatalf("failed to find updated entry: %v", err)
	}
	if filepath.Dir(path) != originalDir {
		t.Errorf("entry moved from original date directory: want %q, got %q", originalDir, filepath.Dir(path))
	}

	updated, err := store.GetEntry(entry.ID)
	if err != nil {
		t.Fatalf("failed to get updated entry: %v", err)
	}
	if !updated.Timestamp.Equal(originalTimestamp) {
		t.Errorf("updated timestamp instant changed: want %v, got %v", originalTimestamp, updated.Timestamp)
	}
	_, updatedOffset := updated.Timestamp.Zone()
	if updatedOffset != 14*60*60 {
		t.Errorf("updated timestamp offset changed: want %d, got %d", 14*60*60, updatedOffset)
	}
}

func TestMarkdownSlugGeneration(t *testing.T) {
	tests := []struct {
		message  string
		id       string
		expected string
	}{
		{"Hello World", "abc12345-1234", "hello-world-77d0798ff687201ed9679bd3b6ab092d700dba5c6c51d000acdc24b70da28740.md"},
		{"deployed app to production", "def67890-5678", "deployed-app-to-production-ef2214da4bf22306d352379a3fcfd50439fcda0f2370b6f902fdeae93ff3f1ad.md"},
		{"", "ghi11111-9012", "untitled-a58007b1e346b5583aacbea187f5cc3a2c0a5e28b0c51552a16f4e24b03710df.md"},
		{"a very long message that should be truncated for the filename", "jkl22222-3456", "a-very-long-message-that-49947a3361a0f911fc697cd24f0bf97ec48df7808b316fae9778fb9ea340d07f.md"},
		{"path safe", `path/unsafe\id?`, "path-safe-0aab6fca24852573daa381437e01c0fb23315f91e3464247c94a54fb691b9a52.md"},
	}

	for _, tt := range tests {
		got := entryFileName(tt.message, tt.id)
		if got != tt.expected {
			t.Errorf("entryFileName(%q, %q) = %q, want %q", tt.message, tt.id, got, tt.expected)
		}
	}
}

func TestMarkdownDeleteCleansEmptyDirectories(t *testing.T) {
	store := newTestMarkdownStore(t)
	defer store.Close()

	now := time.Now()
	entry := Entry{
		Timestamp: now,
		Message:   "will be deleted",
	}

	id, err := store.CreateEntry(entry)
	if err != nil {
		t.Fatalf("failed to create entry: %v", err)
	}

	// Verify the date directory exists
	dateDir := store.entryDirPath(now)
	if _, err := os.Stat(dateDir); os.IsNotExist(err) {
		t.Fatal("expected date directory to exist")
	}

	// Delete the entry
	if err := store.DeleteEntry(id); err != nil {
		t.Fatalf("failed to delete entry: %v", err)
	}

	// The date directory (and parents) should be cleaned up
	if _, err := os.Stat(dateDir); !os.IsNotExist(err) {
		t.Error("expected date directory to be cleaned up after deleting the only entry")
	}
}
