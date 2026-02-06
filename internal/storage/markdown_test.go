// ABOUTME: Tests for MarkdownStore file-based storage backend
// ABOUTME: Covers CRUD, search, migration, edge cases, and concurrency

package storage

import (
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

func TestMarkdownUpdateEntryNotFound(t *testing.T) {
	store := newTestMarkdownStore(t)
	defer store.Close()

	entry := Entry{
		ID:        "nonexistent-id",
		Timestamp: time.Now(),
		Message:   "test",
	}

	err := store.UpdateEntry(entry)
	if err == nil {
		t.Error("expected error for nonexistent entry")
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

	err := store.DeleteEntry("nonexistent-id")
	if err == nil {
		t.Error("expected error for nonexistent entry")
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

func TestMarkdownSlugGeneration(t *testing.T) {
	tests := []struct {
		message  string
		id       string
		expected string
	}{
		{"Hello World", "abc12345-1234", "hello-world-abc12345.md"},
		{"deployed app to production", "def67890-5678", "deployed-app-to-production-def67890.md"},
		{"", "ghi11111-9012", "untitled-ghi11111.md"},
		{"a very long message that should be truncated for the filename", "jkl22222-3456", "a-very-long-message-that-jkl22222.md"},
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
