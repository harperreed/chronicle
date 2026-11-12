// ABOUTME: Tests for the SQLite feed, which reports entries added to a store after it opened.
// ABOUTME: Covers filters that agree with SearchEntries, the row cursor, a writer holding a transaction and concurrent writers.

package storage

import (
	"fmt"
	"path/filepath"
	"slices"
	"sync"
	"testing"
	"time"
)

// newTestSqliteStore creates a SqliteStore on a new database file in a temporary directory.
func newTestSqliteStore(t *testing.T) *SqliteStore {
	t.Helper()
	return openSqliteTestStore(t, filepath.Join(t.TempDir(), "chronicle.db"))
}

// openSqliteTestStore opens a store on the database file at dbPath and closes it when the test
// ends. A second store on the same file is a second connection, as another process would have.
func openSqliteTestStore(t *testing.T, dbPath string) *SqliteStore {
	t.Helper()
	store, err := NewSqliteStore(dbPath)
	if err != nil {
		t.Fatalf("failed to create sqlite store: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store
}

// openSqliteTestFeed opens a feed on the store and fails the test if it can't.
func openSqliteTestFeed(t *testing.T, store *SqliteStore, filter *SearchFilter) Feed {
	t.Helper()
	feed, err := store.OpenFeed(filter)
	if err != nil {
		t.Fatalf("failed to open feed: %v", err)
	}
	return feed
}

// createSqliteEntry creates the entry through the store and returns its ID.
func createSqliteEntry(t *testing.T, store *SqliteStore, entry Entry) string {
	t.Helper()
	id, err := store.CreateEntry(entry)
	if err != nil {
		t.Fatalf("failed to create entry %q: %v", entry.Message, err)
	}
	return id
}

func TestSqliteFeedReportsOnlyEntriesAddedAfterOpen(t *testing.T) {
	store := newTestSqliteStore(t)

	createSqliteEntry(t, store, Entry{Timestamp: feedTestTime(10, 0), Message: "first, before open"})
	createSqliteEntry(t, store, Entry{Timestamp: feedTestTime(10, 1), Message: "second, before open"})
	feed := openSqliteTestFeed(t, store, nil)

	assertPolls(t, feed)

	added := Entry{
		Timestamp:        feedTestTime(10, 2),
		Message:          "added after open",
		Hostname:         "test-host",
		Username:         "test-user",
		WorkingDirectory: "/tmp/work",
		Tags:             []string{"live", "test"},
	}
	added.ID = createSqliteEntry(t, store, added)

	got := assertPolls(t, feed, added.ID)[0]
	if got.Message != added.Message || got.Hostname != added.Hostname || got.Username != added.Username ||
		got.WorkingDirectory != added.WorkingDirectory || !slices.Equal(got.Tags, added.Tags) ||
		!got.Timestamp.Equal(added.Timestamp) {
		t.Errorf("Poll returned %+v, want %+v", got, added)
	}

	assertPolls(t, feed)
}

func TestSqliteFeedOrdersOldestFirst(t *testing.T) {
	store := newTestSqliteStore(t)
	feed := openSqliteTestFeed(t, store, nil)

	// The entries are created newest first, so the order of the rows is the opposite of the
	// order of the timestamps, and only sorting by timestamp gives the right sequence.
	later := createSqliteEntry(t, store, Entry{Timestamp: feedTestTime(10, 2), Message: "alpha"})
	earlier := createSqliteEntry(t, store, Entry{Timestamp: feedTestTime(10, 1), Message: "bravo"})

	assertPolls(t, feed, earlier, later)
}

func TestSqliteFeedMatchesSearchRules(t *testing.T) {
	store := newTestSqliteStore(t)

	// SearchEntries finds this entry under the text, tag and text-and-tag filters below. The
	// feeds open after it, so they must leave it out.
	createSqliteEntry(t, store, Entry{Timestamp: feedTestTime(9, 0), Message: "deploy the app", Tags: []string{"keep"}})

	since := feedTestTime(10, 5)
	until := feedTestTime(10, 25)
	cases := []struct {
		name   string
		filter *SearchFilter
		want   []string // Names of the new entries that must come back, oldest first.
	}{
		// FTS5 matches whole words, so "deploy" doesn't match "deployed".
		{"text", &SearchFilter{Text: "deploy"}, []string{"deploy kept", "deploy dropped"}},
		{"tag", &SearchFilter{Tags: []string{"keep"}}, []string{"deploy kept", "deployed kept"}},
		{"text and tag", &SearchFilter{Text: "deploy", Tags: []string{"keep"}}, []string{"deploy kept"}},
		{"since and until", &SearchFilter{Since: &since, Until: &until}, []string{"deploy dropped", "deployed kept"}},
	}
	feeds := make([]Feed, len(cases))
	for i, tc := range cases {
		feeds[i] = openSqliteTestFeed(t, store, tc.filter)
	}

	entries := []struct {
		name  string
		entry Entry
	}{
		{"deploy kept", Entry{Timestamp: feedTestTime(10, 0), Message: "deploy the app", Tags: []string{"keep"}}},
		{"deploy dropped", Entry{Timestamp: feedTestTime(10, 10), Message: "deploy the app", Tags: []string{"drop"}}},
		{"deployed kept", Entry{Timestamp: feedTestTime(10, 20), Message: "deployed it", Tags: []string{"keep"}}},
		{"other dropped", Entry{Timestamp: feedTestTime(10, 30), Message: "other", Tags: []string{"drop"}}},
	}
	idsByName := make(map[string]string, len(entries))
	isNew := make(map[string]bool, len(entries))
	for _, e := range entries {
		id := createSqliteEntry(t, store, e.entry)
		idsByName[e.name] = id
		isNew[id] = true
	}

	for i, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			searched, err := store.SearchEntries(tc.filter, 0)
			if err != nil {
				t.Fatalf("SearchEntries failed: %v", err)
			}
			var searchedNew []string
			for _, entry := range searched {
				if isNew[entry.ID] {
					searchedNew = append(searchedNew, entry.ID)
				}
			}
			slices.Reverse(searchedNew) // SearchEntries lists newest first, and a feed oldest first.

			polled := entryIDs(pollEntries(t, feeds[i]))
			if !slices.Equal(polled, searchedNew) {
				t.Errorf("Poll returned IDs %v, but SearchEntries finds %v among the new entries", polled, searchedNew)
			}

			// Both agreeing on nothing would pass the check above, so pin what they must find.
			want := make([]string, 0, len(tc.want))
			for _, name := range tc.want {
				want = append(want, idsByName[name])
			}
			if !slices.Equal(polled, want) {
				t.Errorf("Poll returned IDs %v, want %v", polled, want)
			}
		})
	}
}

func TestSqliteFeedCursorPassesNonMatchingRows(t *testing.T) {
	store := newTestSqliteStore(t)
	feed := openSqliteTestFeed(t, store, &SearchFilter{Tags: []string{"keep"}})

	for i := 0; i < 3; i++ {
		createSqliteEntry(t, store, Entry{
			Timestamp: feedTestTime(10, i),
			Message:   fmt.Sprintf("dropped entry %d", i),
			Tags:      []string{"drop"},
		})
	}

	if entries := pollEntries(t, feed); entries != nil {
		t.Fatalf("Poll returned IDs %v for entries that fail the filter, want nil", entryIDs(entries))
	}

	// Rows that fail the filter still move the cursor, so a later poll doesn't scan them again.
	var newestRowID int64
	if err := store.db.QueryRow("SELECT MAX(rowid) FROM entries").Scan(&newestRowID); err != nil {
		t.Fatalf("failed to read the newest row: %v", err)
	}
	sqlFeed, ok := feed.(*sqliteFeed)
	if !ok {
		t.Fatalf("OpenFeed returned a %T, want *sqliteFeed", feed)
	}
	if sqlFeed.lastRowID != newestRowID {
		t.Errorf("cursor is at row %d, want the newest row %d", sqlFeed.lastRowID, newestRowID)
	}

	// A matching entry that comes after the passed rows is still reported, once.
	kept := createSqliteEntry(t, store, Entry{Timestamp: feedTestTime(10, 5), Message: "kept entry", Tags: []string{"keep"}})
	assertPolls(t, feed, kept)
	assertPolls(t, feed)
}

func TestSqliteFeedReadsWhileWriterHoldsTransaction(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "chronicle.db")
	// Both stores open before the writer's transaction starts, because opening a store takes the
	// write lock to run its migration.
	writer := openSqliteTestStore(t, dbPath)
	reader := openSqliteTestStore(t, dbPath)
	feed := openSqliteTestFeed(t, reader, nil)

	// Begin takes the write lock at once, because the store's DSN sets _txlock=immediate.
	tx, err := writer.db.Begin()
	if err != nil {
		t.Fatalf("failed to begin write transaction: %v", err)
	}
	defer func() { _ = tx.Rollback() }()
	timestamp := feedTestTime(10, 0)
	if _, err := tx.Exec(`
		INSERT INTO entries (id, timestamp, timestamp_unix_seconds, timestamp_nanosecond, message, tags)
		VALUES (?, ?, ?, ?, ?, ?)
	`, "uncommitted-entry", formatTimestamp(timestamp), timestamp.Unix(), timestamp.Nanosecond(), "held in a transaction", "[]"); err != nil {
		t.Fatalf("failed to insert entry: %v", err)
	}

	type pollResult struct {
		entries []Entry
		err     error
	}
	polled := make(chan pollResult, 1)
	go func() {
		entries, err := feed.Poll()
		polled <- pollResult{entries, err}
	}()
	select {
	case got := <-polled:
		if got.err != nil {
			t.Fatalf("Poll failed while a writer held a transaction: %v", got.err)
		}
		if len(got.entries) != 0 {
			t.Fatalf("Poll returned IDs %v, want the uncommitted entry left out", entryIDs(got.entries))
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Poll was still waiting 2s after a writer opened a transaction")
	}

	if err := tx.Commit(); err != nil {
		t.Fatalf("failed to commit: %v", err)
	}
	assertPolls(t, feed, "uncommitted-entry")
}

func TestSqliteFeedConcurrentWritersSeenExactlyOnce(t *testing.T) {
	const writers = 4
	const entriesPerWriter = 25

	dbPath := filepath.Join(t.TempDir(), "chronicle.db")
	reader := openSqliteTestStore(t, dbPath)
	feed := openSqliteTestFeed(t, reader, nil)

	var (
		created   = make(map[string]bool)
		createdMu sync.Mutex
		writing   sync.WaitGroup
	)
	for w := 0; w < writers; w++ {
		writing.Add(1)
		go func() {
			defer writing.Done()
			// Each writer is a store of its own on the same file, like separate chronicle processes.
			store, err := NewSqliteStore(dbPath)
			if err != nil {
				t.Errorf("writer %d: failed to create sqlite store: %v", w, err)
				return
			}
			defer store.Close()
			for i := 0; i < entriesPerWriter; i++ {
				id, err := store.CreateEntry(Entry{
					Timestamp: time.Date(2026, 11, 1+w, 10, i, 0, 0, time.UTC),
					Message:   fmt.Sprintf("writer %d entry %d", w, i),
				})
				if err != nil {
					t.Errorf("writer %d: failed to create entry %d: %v", w, i, err)
					return
				}
				createdMu.Lock()
				created[id] = true
				createdMu.Unlock()
			}
		}()
	}

	stopPolling := make(chan struct{})
	polledIDs := make(chan []string)
	go func() {
		var ids []string
		ticker := time.NewTicker(time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				entries, err := feed.Poll()
				if err != nil {
					t.Errorf("Poll failed during writes: %v", err)
					continue
				}
				ids = append(ids, entryIDs(entries)...)
			case <-stopPolling:
				polledIDs <- ids
				return
			}
		}
	}()

	writing.Wait()
	close(stopPolling)
	seen := <-polledIDs
	seen = append(seen, entryIDs(pollEntries(t, feed))...)

	seenCount := make(map[string]int, len(seen))
	for _, id := range seen {
		seenCount[id]++
	}
	if len(created) != writers*entriesPerWriter {
		t.Fatalf("writers created %d entries, want %d", len(created), writers*entriesPerWriter)
	}
	for id := range created {
		if seenCount[id] != 1 {
			t.Errorf("entry %s was reported %d times, want once", id, seenCount[id])
		}
	}
	if len(seen) != len(created) {
		t.Errorf("feed reported %d entries, want %d", len(seen), len(created))
	}
}
