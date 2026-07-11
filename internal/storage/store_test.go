// ABOUTME: Tests for SQLite storage layer
// ABOUTME: Uses in-memory database for fast unit tests

package storage

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewSqliteStore(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	store, err := NewSqliteStore(dbPath)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	// Verify database file was created
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Error("database file was not created")
	}
}

func TestNewStoreInMemory(t *testing.T) {
	store, err := NewSqliteStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create in-memory store: %v", err)
	}
	defer store.Close()
}

func TestCreateEntry(t *testing.T) {
	store, err := NewSqliteStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
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

func TestGetEntry(t *testing.T) {
	store, err := NewSqliteStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	now := time.Now().Truncate(time.Second)
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

func TestGetEntryNotFound(t *testing.T) {
	store, err := NewSqliteStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	_, err = store.GetEntry("nonexistent-id")
	if err == nil {
		t.Error("expected error for nonexistent entry")
	}
}

func TestUpdateEntry(t *testing.T) {
	store, err := NewSqliteStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
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

func TestDeleteEntry(t *testing.T) {
	store, err := NewSqliteStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
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

func TestListEntries(t *testing.T) {
	store, err := NewSqliteStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
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
	// Entry 4 (now+4h) should be first, then 3 (now+3h), then 2 (now+2h)
	for i := 1; i < len(entries); i++ {
		if entries[i].Timestamp.After(entries[i-1].Timestamp) {
			t.Errorf("entries not sorted by timestamp descending: entry[%d]=%v > entry[%d]=%v",
				i, entries[i].Timestamp, i-1, entries[i-1].Timestamp)
		}
	}
}

func TestSearchEntriesByText(t *testing.T) {
	store, err := NewSqliteStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
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

func TestSearchEntriesByTags(t *testing.T) {
	store, err := NewSqliteStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
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

func TestSearchEntriesBySpecialCharacterTags(t *testing.T) {
	store, err := NewSqliteStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	entries := []Entry{
		{ID: "percent", Timestamp: time.Now(), Message: "percent", Tags: []string{"%"}},
		{ID: "underscore", Timestamp: time.Now(), Message: "underscore", Tags: []string{"_"}},
		{ID: "quote", Timestamp: time.Now(), Message: "quote", Tags: []string{`quote"tag`}},
		{ID: "backslash", Timestamp: time.Now(), Message: "backslash", Tags: []string{`back\slash`}},
		{ID: "alpha", Timestamp: time.Now(), Message: "alpha", Tags: []string{"alpha"}},
		{ID: "beta", Timestamp: time.Now(), Message: "beta", Tags: []string{"beta"}},
	}

	for _, entry := range entries {
		if _, err := store.CreateEntry(entry); err != nil {
			t.Fatalf("failed to create entry %q: %v", entry.ID, err)
		}
	}

	tests := []struct {
		name string
		tag  string
		id   string
	}{
		{name: "percent", tag: "%", id: "percent"},
		{name: "underscore", tag: "_", id: "underscore"},
		{name: "quote", tag: `quote"tag`, id: "quote"},
		{name: "backslash", tag: `back\slash`, id: "backslash"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results, err := store.SearchEntries(&SearchFilter{Tags: []string{tt.tag}}, 10)
			if err != nil {
				t.Fatalf("failed to search entries: %v", err)
			}
			if len(results) != 1 {
				t.Fatalf("expected one exact match for tag %q, got %d: %+v", tt.tag, len(results), results)
			}
			if results[0].ID != tt.id {
				t.Errorf("expected entry %q for tag %q, got %q", tt.id, tt.tag, results[0].ID)
			}
		})
	}
}

func TestSearchEntriesUsesExactJSONTagMembership(t *testing.T) {
	store, err := NewSqliteStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	entries := []Entry{
		{ID: "exact-alpha", Timestamp: time.Now(), Message: "needle exact alpha", Tags: []string{"alpha"}},
		{ID: "escaped-quote-container", Timestamp: time.Now(), Message: "needle escaped quote", Tags: []string{`x"alpha`}},
		{ID: "exact-beta", Timestamp: time.Now(), Message: "needle exact beta", Tags: []string{"beta"}},
		{ID: "unrelated", Timestamp: time.Now(), Message: "needle unrelated", Tags: []string{"gamma"}},
	}
	for _, entry := range entries {
		if _, err := store.CreateEntry(entry); err != nil {
			t.Fatalf("failed to create entry %q: %v", entry.ID, err)
		}
	}

	tests := []struct {
		name     string
		filter   *SearchFilter
		expected map[string]bool
	}{
		{
			name:     "exact tag",
			filter:   &SearchFilter{Tags: []string{"alpha"}},
			expected: map[string]bool{"exact-alpha": true},
		},
		{
			name:     "exact tag with text search",
			filter:   &SearchFilter{Text: "needle", Tags: []string{"alpha"}},
			expected: map[string]bool{"exact-alpha": true},
		},
		{
			name:     "OR tags",
			filter:   &SearchFilter{Tags: []string{"alpha", "beta"}},
			expected: map[string]bool{"exact-alpha": true, "exact-beta": true},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results, err := store.SearchEntries(tt.filter, 10)
			if err != nil {
				t.Fatalf("failed to search entries: %v", err)
			}
			if len(results) != len(tt.expected) {
				t.Errorf("expected %d results, got %d: %+v", len(tt.expected), len(results), results)
			}
			for _, result := range results {
				if !tt.expected[result.ID] {
					t.Errorf("unexpected entry %q for filter %+v", result.ID, tt.filter)
				}
			}
		})
	}
}

func TestSearchEntriesSkipsMalformedJSONForExactTagMatching(t *testing.T) {
	store, err := NewSqliteStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	wantTime := time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC)
	if _, err := store.CreateEntry(Entry{
		ID:        "valid-tag",
		Timestamp: wantTime,
		Message:   "needle valid",
		Tags:      []string{"alpha"},
	}); err != nil {
		t.Fatalf("failed to create valid entry: %v", err)
	}
	for i, row := range []struct {
		id   string
		tags string
	}{
		{id: "malformed-tag", tags: "not-json"},
		{id: "scalar-tag", tags: `"alpha"`},
		{id: "object-tag", tags: `{"x":"alpha"}`},
	} {
		timestamp := wantTime.Add(-time.Duration(i+1) * time.Hour)
		if _, err := store.db.Exec(
			"INSERT INTO entries(id, timestamp, timestamp_unix_seconds, timestamp_nanosecond, message, tags) VALUES(?, ?, ?, ?, ?, ?)",
			row.id, formatTimestamp(timestamp), timestamp.Unix(), timestamp.Nanosecond(), "needle "+row.id, row.tags,
		); err != nil {
			t.Fatalf("failed to insert %s row: %v", row.id, err)
		}
	}

	listed, err := store.ListEntries(0)
	if err != nil {
		t.Fatalf("list entries with malformed tags: %v", err)
	}
	if len(listed) != 4 {
		t.Fatalf("listed entries = %d, want 4", len(listed))
	}
	listedByID := make(map[string]Entry, len(listed))
	for _, entry := range listed {
		listedByID[entry.ID] = entry
	}
	for _, id := range []string{"malformed-tag", "scalar-tag", "object-tag"} {
		if listedByID[id].Tags != nil {
			t.Fatalf("%s listing tags = %#v, want nil fallback", id, listedByID[id].Tags)
		}
	}

	for _, filter := range []*SearchFilter{
		{Tags: []string{"alpha"}},
		{Text: "needle", Tags: []string{"alpha"}},
	} {
		results, err := store.SearchEntries(filter, 0)
		if err != nil {
			t.Fatalf("search entries with malformed tags using %+v: %v", filter, err)
		}
		if len(results) != 1 || results[0].ID != "valid-tag" {
			t.Fatalf("search results using %+v = %+v, want only valid-tag", filter, results)
		}
	}
}

func TestSearchEntriesByDateRange(t *testing.T) {
	store, err := NewSqliteStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
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

func TestLastModified(t *testing.T) {
	store, err := NewSqliteStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	before := store.LastModified()

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

func TestVacuum(t *testing.T) {
	store, err := NewSqliteStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	// Just verify it doesn't error
	if err := store.Vacuum(); err != nil {
		t.Errorf("vacuum failed: %v", err)
	}
}

func TestDefaultPath(t *testing.T) {
	path := DefaultPath()
	if path == "" {
		t.Error("expected non-empty default path")
	}
	if !filepath.IsAbs(path) {
		t.Error("expected absolute path")
	}
}

func TestReset(t *testing.T) {
	store, err := NewSqliteStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	// Create some entries
	for i := 0; i < 5; i++ {
		entry := Entry{
			Timestamp: time.Now(),
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

func TestUpdateEntryNotFound(t *testing.T) {
	store, err := NewSqliteStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	entry := Entry{
		ID:        "nonexistent-id",
		Timestamp: time.Now(),
		Message:   "test",
	}

	err = store.UpdateEntry(entry)
	if err == nil {
		t.Error("expected error for nonexistent entry")
	}
}

func TestUpdateEntryNoID(t *testing.T) {
	store, err := NewSqliteStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	entry := Entry{
		Timestamp: time.Now(),
		Message:   "test",
	}

	err = store.UpdateEntry(entry)
	if err == nil {
		t.Error("expected error for entry without ID")
	}
}

func TestDeleteEntryNotFound(t *testing.T) {
	store, err := NewSqliteStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	err = store.DeleteEntry("nonexistent-id")
	if err == nil {
		t.Error("expected error for nonexistent entry")
	}
}

func TestCreateEntryWithExistingID(t *testing.T) {
	store, err := NewSqliteStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
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

func TestCreateEntryWithoutTimestamp(t *testing.T) {
	store, err := NewSqliteStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
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

func TestSearchEntriesWithUntilFilter(t *testing.T) {
	store, err := NewSqliteStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
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

func TestSearchEntriesCombinedFilters(t *testing.T) {
	store, err := NewSqliteStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
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

func TestSearchEntriesTextAndTags(t *testing.T) {
	store, err := NewSqliteStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
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

func TestSearchEntriesMultipleTags(t *testing.T) {
	store, err := NewSqliteStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
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

func TestSearchEntriesNoLimit(t *testing.T) {
	store, err := NewSqliteStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	// Create many entries
	for i := 0; i < 50; i++ {
		entry := Entry{
			Timestamp: time.Now(),
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

func TestParseTimestamp(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"RFC3339", "2025-01-15T10:30:00Z", false},
		{"RFC3339Nano", "2025-01-15T10:30:00.123456789Z", false},
		{"with timezone", "2025-01-15T10:30:00-07:00", false},
		{"simple datetime", "2025-01-15 10:30:00", false},
		{"invalid format", "not a date", true},
		{"empty string", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parseTimestamp(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseTimestamp(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
		})
	}
}

func TestCreateEntryWithNullTags(t *testing.T) {
	store, err := NewSqliteStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
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

func TestListEntriesEmpty(t *testing.T) {
	store, err := NewSqliteStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	entries, err := store.ListEntries(10)
	if err != nil {
		t.Fatalf("failed to list entries: %v", err)
	}

	// entries can be nil for empty results, which is valid
	if len(entries) != 0 {
		t.Errorf("expected 0 entries, got %d", len(entries))
	}
}

func TestSearchEntriesEmptyText(t *testing.T) {
	store, err := NewSqliteStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
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

func TestSqliteTimestampRoundTripUsesStableTextAndInstantKey(t *testing.T) {
	store, err := NewSqliteStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	location := time.FixedZone("offset", 60*60)
	entry := Entry{
		ID:        "named-zone",
		Timestamp: time.Date(2026, 7, 10, 12, 0, 0, 123456789, location),
		Message:   "created",
	}
	if _, err := store.CreateEntry(entry); err != nil {
		t.Fatalf("failed to create entry: %v", err)
	}

	assertRawTimestamp := func(want time.Time, expectedText string) {
		t.Helper()
		var raw string
		var unixSeconds, nanosecond int64
		if err := store.db.QueryRow(
			"SELECT timestamp, timestamp_unix_seconds, timestamp_nanosecond FROM entries WHERE id = ?",
			entry.ID,
		).Scan(&raw, &unixSeconds, &nanosecond); err != nil {
			t.Fatalf("failed to read raw timestamp: %v", err)
		}
		if raw != expectedText {
			t.Fatalf("raw timestamp = %q, want %q", raw, expectedText)
		}
		if unixSeconds != want.Unix() || nanosecond != int64(want.Nanosecond()) {
			t.Fatalf(
				"timestamp instant key = (%d, %d), want (%d, %d)",
				unixSeconds,
				nanosecond,
				want.Unix(),
				want.Nanosecond(),
			)
		}
	}
	assertRoundTrip := func(want time.Time) {
		t.Helper()
		got, err := store.GetEntry(entry.ID)
		if err != nil {
			t.Fatalf("failed to get entry: %v", err)
		}
		if !got.Timestamp.Equal(want) || got.Timestamp.Nanosecond() != want.Nanosecond() {
			t.Fatalf("timestamp = %v, want instant %v", got.Timestamp, want)
		}
		_, gotOffset := got.Timestamp.Zone()
		_, wantOffset := want.Zone()
		if gotOffset != wantOffset {
			t.Fatalf("timestamp offset = %d, want %d", gotOffset, wantOffset)
		}
	}

	assertRawTimestamp(entry.Timestamp, "2026-07-10 12:00:00.123456789 +0100 offset")
	assertRoundTrip(entry.Timestamp)

	entry.Timestamp = time.Date(2026, 7, 11, 13, 14, 15, 987654321, location)
	entry.Message = "updated"
	if err := store.UpdateEntry(entry); err != nil {
		t.Fatalf("failed to update entry: %v", err)
	}
	assertRawTimestamp(entry.Timestamp, "2026-07-11 13:14:15.987654321 +0100 offset")
	assertRoundTrip(entry.Timestamp)
}

func TestSqliteOrdersAndFiltersTimestampsBeyondUnixNanoRange(t *testing.T) {
	store, err := NewSqliteStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	past := time.Date(1600, 1, 2, 3, 4, 5, 6, time.UTC)
	futureLow := time.Date(2500, 1, 2, 3, 4, 5, 123456789, time.UTC)
	futureHigh := time.Date(2500, 1, 2, 3, 4, 5, 123456790, time.UTC)
	updatedFuture := time.Date(3000, 1, 2, 3, 4, 5, 7, time.UTC)
	for _, entry := range []Entry{
		{ID: "past", Timestamp: past, Message: "past"},
		{ID: "future-low", Timestamp: futureLow, Message: "future low"},
		{ID: "future-high", Timestamp: futureHigh, Message: "future high"},
		{ID: "updated-future", Timestamp: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), Message: "updated future"},
	} {
		if _, err := store.CreateEntry(entry); err != nil {
			t.Fatalf("create entry %q: %v", entry.ID, err)
		}
	}
	if err := store.UpdateEntry(Entry{ID: "updated-future", Timestamp: updatedFuture, Message: "updated future"}); err != nil {
		t.Fatalf("update entry beyond UnixNano range: %v", err)
	}

	listed, err := store.ListEntries(0)
	if err != nil {
		t.Fatalf("list entries beyond UnixNano range: %v", err)
	}
	wantOrder := []string{"updated-future", "future-high", "future-low", "past"}
	if len(listed) != len(wantOrder) {
		t.Fatalf("listed entries = %d, want %d", len(listed), len(wantOrder))
	}
	for i, wantID := range wantOrder {
		if listed[i].ID != wantID {
			t.Fatalf("listed entry %d = %q, want %q; full order: %+v", i, listed[i].ID, wantID, listed)
		}
	}

	sinceResults, err := store.SearchEntries(&SearchFilter{Since: &futureLow}, 0)
	if err != nil {
		t.Fatalf("search since beyond UnixNano range: %v", err)
	}
	if len(sinceResults) != 3 || sinceResults[0].ID != "updated-future" || sinceResults[1].ID != "future-high" || sinceResults[2].ID != "future-low" {
		t.Fatalf("since results = %+v, want updated-future, future-high, future-low", sinceResults)
	}

	untilResults, err := store.SearchEntries(&SearchFilter{Until: &futureLow}, 0)
	if err != nil {
		t.Fatalf("search until beyond UnixNano range: %v", err)
	}
	if len(untilResults) != 2 || untilResults[0].ID != "future-low" || untilResults[1].ID != "past" {
		t.Fatalf("until results = %+v, want future-low, past", untilResults)
	}
}

func TestSqliteReadsNullableMetadataAndRetainsCorruptFallbacks(t *testing.T) {
	store, err := NewSqliteStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	validTime := time.Date(2026, 7, 10, 12, 0, 0, 123456789, time.UTC)
	validTimestamp := validTime.Format(time.RFC3339Nano)
	for _, row := range []struct {
		id          string
		timestamp   string
		unixSeconds any
		nanosecond  any
		message     string
		tags        any
	}{
		{id: "nullable", timestamp: validTimestamp, unixSeconds: validTime.Unix(), nanosecond: validTime.Nanosecond(), message: "nullable metadata", tags: nil},
		{id: "bad-time", timestamp: "not-a-time", unixSeconds: nil, nanosecond: nil, message: "bad time", tags: "[]"},
		{id: "bad-tags", timestamp: validTimestamp, unixSeconds: validTime.Unix(), nanosecond: validTime.Nanosecond(), message: "bad tags", tags: "not-json"},
	} {
		if _, err := store.db.Exec(
			"INSERT INTO entries(id, timestamp, timestamp_unix_seconds, timestamp_nanosecond, message, tags) VALUES(?, ?, ?, ?, ?, ?)",
			row.id, row.timestamp, row.unixSeconds, row.nanosecond, row.message, row.tags,
		); err != nil {
			t.Fatalf("failed to insert %s: %v", row.id, err)
		}
	}
	canonicalTime := time.Date(2026, 7, 11, 12, 0, 0, 0, time.UTC)
	if _, err := store.CreateEntry(Entry{ID: "canonical", Timestamp: canonicalTime, Message: "canonical"}); err != nil {
		t.Fatalf("failed to insert canonical entry: %v", err)
	}

	nullable, err := store.GetEntry("nullable")
	if err != nil {
		t.Fatalf("failed to get nullable metadata entry: %v", err)
	}
	if nullable.Hostname != "" || nullable.Username != "" || nullable.WorkingDirectory != "" {
		t.Fatalf("nullable metadata was not normalized to empty strings: %#v", nullable)
	}
	if nullable.Tags != nil {
		t.Fatalf("nullable tags = %#v, want nil", nullable.Tags)
	}
	searched, err := store.SearchEntries(&SearchFilter{Text: "nullable"}, 10)
	if err != nil || len(searched) != 1 {
		t.Fatalf("search nullable metadata entry: len=%d err=%v", len(searched), err)
	}
	if searched[0].Hostname != "" || searched[0].Username != "" || searched[0].WorkingDirectory != "" {
		t.Fatalf("search did not normalize nullable metadata: %#v", searched[0])
	}
	if searched[0].Tags != nil {
		t.Fatalf("search nullable tags = %#v, want nil", searched[0].Tags)
	}

	if _, err := store.GetEntry("bad-time"); err == nil {
		t.Fatal("GetEntry accepted invalid timestamp")
	}
	if _, err := store.GetEntry("bad-tags"); err == nil {
		t.Fatal("GetEntry accepted invalid tags")
	}

	listed, err := store.ListEntries(0)
	if err != nil {
		t.Fatalf("failed to list corrupt rows: %v", err)
	}
	seen := make(map[string]Entry, len(listed))
	for _, entry := range listed {
		seen[entry.ID] = entry
	}
	if seen["nullable"].Tags != nil {
		t.Fatalf("ListEntries nullable tags = %#v, want nil", seen["nullable"].Tags)
	}
	if seen["bad-time"].Timestamp.IsZero() {
		t.Fatal("ListEntries did not substitute a timestamp for invalid data")
	}
	if seen["bad-tags"].Tags != nil {
		t.Fatalf("ListEntries tags = %#v, want nil fallback", seen["bad-tags"].Tags)
	}
	if listed[len(listed)-1].ID != "bad-time" {
		t.Fatalf("last listed entry = %q, want corrupt timestamp row last", listed[len(listed)-1].ID)
	}
	var corruptUnixSeconds, corruptNanosecond any
	if err := store.db.QueryRow(
		"SELECT timestamp_unix_seconds, timestamp_nanosecond FROM entries WHERE id = ?",
		"bad-time",
	).Scan(&corruptUnixSeconds, &corruptNanosecond); err != nil {
		t.Fatalf("read corrupt timestamp instant key: %v", err)
	}
	if corruptUnixSeconds != nil || corruptNanosecond != nil {
		t.Fatalf("corrupt timestamp instant keys = (%v, %v), want (NULL, NULL)", corruptUnixSeconds, corruptNanosecond)
	}
	since := canonicalTime.Add(-time.Hour)
	filtered, err := store.SearchEntries(&SearchFilter{Since: &since}, 0)
	if err != nil {
		t.Fatalf("filter entries with corrupt timestamp: %v", err)
	}
	if len(filtered) != 1 || filtered[0].ID != "canonical" {
		t.Fatalf("filtered entries = %+v, want only canonical entry", filtered)
	}
}

func TestSqliteDateFiltersExcludePartialTimestampKeys(t *testing.T) {
	store, err := NewSqliteStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	boundary := time.Date(2026, 7, 10, 12, 0, 0, 123, time.UTC)
	if _, err := store.CreateEntry(Entry{ID: "complete", Timestamp: boundary, Message: "complete"}); err != nil {
		t.Fatalf("create complete entry: %v", err)
	}
	for _, row := range []struct {
		id          string
		timestamp   string
		unixSeconds any
		nanosecond  any
	}{
		{
			id:          "partial-future",
			timestamp:   formatTimestamp(boundary.Add(time.Hour)),
			unixSeconds: boundary.Add(time.Hour).Unix(),
			nanosecond:  nil,
		},
		{
			id:          "partial-past",
			timestamp:   formatTimestamp(boundary.Add(-time.Hour)),
			unixSeconds: boundary.Add(-time.Hour).Unix(),
			nanosecond:  nil,
		},
		{
			id:          "partial-corrupt",
			timestamp:   "not-a-time",
			unixSeconds: nil,
			nanosecond:  123,
		},
	} {
		if _, err := store.db.Exec(
			"INSERT INTO entries(id, timestamp, timestamp_unix_seconds, timestamp_nanosecond, message, tags) VALUES(?, ?, ?, ?, ?, ?)",
			row.id, row.timestamp, row.unixSeconds, row.nanosecond, row.id, "[]",
		); err != nil {
			t.Fatalf("insert %s: %v", row.id, err)
		}
	}

	listed, err := store.ListEntries(0)
	if err != nil {
		t.Fatalf("list entries with partial timestamp keys: %v", err)
	}
	if len(listed) != 4 || listed[0].ID != "complete" {
		t.Fatalf("listed entries = %+v, want complete first and all partial keys last", listed)
	}
	partialIDs := map[string]bool{
		"partial-future":  true,
		"partial-past":    true,
		"partial-corrupt": true,
	}
	for _, entry := range listed[1:] {
		if !partialIDs[entry.ID] {
			t.Fatalf("entry after complete = %q, want a partial-key row", entry.ID)
		}
		delete(partialIDs, entry.ID)
	}
	if len(partialIDs) != 0 {
		t.Fatalf("partial-key rows missing from listing: %v", partialIDs)
	}
	listedByID := make(map[string]Entry, len(listed))
	for _, entry := range listed {
		listedByID[entry.ID] = entry
	}
	if listedByID["partial-corrupt"].Timestamp.IsZero() {
		t.Fatalf("corrupt partial entry = %+v, want nonzero fallback", listedByID["partial-corrupt"])
	}

	for name, filter := range map[string]*SearchFilter{
		"since": {Since: &boundary},
		"until": {Until: &boundary},
	} {
		results, err := store.SearchEntries(filter, 0)
		if err != nil {
			t.Fatalf("search %s with partial timestamp keys: %v", name, err)
		}
		if len(results) != 1 || results[0].ID != "complete" {
			t.Fatalf("%s results = %+v, want only complete inclusive match", name, results)
		}
	}
}
