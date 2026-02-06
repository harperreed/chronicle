// ABOUTME: Integration tests for SQLite storage layer
// ABOUTME: Tests concurrent access and WAL mode behavior

package storage

import (
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestWALConcurrentConnections(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "chronicle.db")

	// First, initialize the database
	initStore, err := NewSqliteStore(dbPath)
	if err != nil {
		t.Fatalf("failed to create initial store: %v", err)
	}
	initStore.Close()

	const numConnections = 3
	const writesPerConnection = 5

	var wg sync.WaitGroup
	errors := make(chan error, numConnections*(writesPerConnection+1))

	for i := 0; i < numConnections; i++ {
		wg.Add(1)
		go func(connID int) {
			defer wg.Done()

			// Each goroutine opens its own store (simulates separate processes)
			store, err := NewSqliteStore(dbPath)
			if err != nil {
				errors <- err
				return
			}
			defer store.Close()

			// Perform writes
			for j := 0; j < writesPerConnection; j++ {
				entry := Entry{
					Timestamp: time.Now(),
					Message:   "test message",
					Hostname:  "test-host",
				}
				if _, err := store.CreateEntry(entry); err != nil {
					errors <- err
				}
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	// Collect any errors
	var errs []error
	for err := range errors {
		errs = append(errs, err)
	}

	if len(errs) > 0 {
		t.Errorf("concurrent connections produced %d errors, first: %v", len(errs), errs[0])
	}

	// Verify total entries
	store, err := NewSqliteStore(dbPath)
	if err != nil {
		t.Fatalf("failed to open store for verification: %v", err)
	}
	defer store.Close()

	entries, err := store.ListEntries(0)
	if err != nil {
		t.Fatalf("failed to list entries: %v", err)
	}

	expected := numConnections * writesPerConnection
	if len(entries) != expected {
		t.Errorf("expected %d entries, got %d", expected, len(entries))
	}
}

func TestFTSSearchIntegration(t *testing.T) {
	store, err := NewSqliteStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	// Create entries with various messages
	entries := []Entry{
		{Timestamp: time.Now(), Message: "deployed production release v2.1.0"},
		{Timestamp: time.Now(), Message: "fixed critical bug in authentication"},
		{Timestamp: time.Now(), Message: "updated documentation for API"},
		{Timestamp: time.Now(), Message: "production database migration complete"},
		{Timestamp: time.Now(), Message: "new feature: user authentication flow"},
	}

	for _, e := range entries {
		if _, err := store.CreateEntry(e); err != nil {
			t.Fatalf("failed to create entry: %v", err)
		}
	}

	tests := []struct {
		searchText    string
		expectedCount int
	}{
		{"production", 2},
		{"authentication", 2},
		{"bug", 1},
		{"documentation", 1},
		{"nonexistent", 0},
	}

	for _, tt := range tests {
		t.Run(tt.searchText, func(t *testing.T) {
			filter := &SearchFilter{Text: tt.searchText}
			results, err := store.SearchEntries(filter, 100)
			if err != nil {
				t.Fatalf("search failed: %v", err)
			}
			if len(results) != tt.expectedCount {
				t.Errorf("expected %d results for %q, got %d", tt.expectedCount, tt.searchText, len(results))
			}
		})
	}
}

func TestResetAndVacuum(t *testing.T) {
	store, err := NewSqliteStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	// Create some entries
	for i := 0; i < 10; i++ {
		entry := Entry{
			Timestamp: time.Now(),
			Message:   "test message",
		}
		if _, err := store.CreateEntry(entry); err != nil {
			t.Fatalf("failed to create entry: %v", err)
		}
	}

	// Verify entries exist
	entries, _ := store.ListEntries(0)
	if len(entries) != 10 {
		t.Errorf("expected 10 entries, got %d", len(entries))
	}

	// Reset
	if err := store.Reset(); err != nil {
		t.Fatalf("reset failed: %v", err)
	}

	// Verify entries cleared
	entries, _ = store.ListEntries(0)
	if len(entries) != 0 {
		t.Errorf("expected 0 entries after reset, got %d", len(entries))
	}

	// Vacuum should work after reset
	if err := store.Vacuum(); err != nil {
		t.Errorf("vacuum failed after reset: %v", err)
	}
}
