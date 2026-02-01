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

func TestNewStore(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	store, err := NewStore(dbPath)
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
	store, err := NewStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create in-memory store: %v", err)
	}
	defer store.Close()
}

func TestCreateEntry(t *testing.T) {
	store, err := NewStore(":memory:")
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
	store, err := NewStore(":memory:")
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
	store, err := NewStore(":memory:")
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
	store, err := NewStore(":memory:")
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
	store, err := NewStore(":memory:")
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
	store, err := NewStore(":memory:")
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
	store, err := NewStore(":memory:")
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
	store, err := NewStore(":memory:")
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

func TestSearchEntriesByDateRange(t *testing.T) {
	store, err := NewStore(":memory:")
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
	store, err := NewStore(":memory:")
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
	store, err := NewStore(":memory:")
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
