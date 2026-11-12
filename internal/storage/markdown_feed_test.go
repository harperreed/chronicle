// ABOUTME: Tests for the markdown feed, which reports entries added to a store after it opened
// ABOUTME: Covers new, edited, deleted and malformed files, folder layout, filters and concurrent writers

package storage

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"testing"
	"time"
)

// feedTestTime returns a UTC time on 2026-11-11, so entries land in the 2026/11/11 day folder.
func feedTestTime(hour, minute int) time.Time {
	return time.Date(2026, 11, 11, hour, minute, 0, 0, time.UTC)
}

// openMarkdownTestFeed opens a feed on the store and fails the test if it can't.
func openMarkdownTestFeed(t *testing.T, store *MarkdownStore, filter *SearchFilter) Feed {
	t.Helper()
	feed, err := store.OpenFeed(filter)
	if err != nil {
		t.Fatalf("failed to open feed: %v", err)
	}
	return feed
}

// createMarkdownEntry creates the entry through the store and returns its ID.
func createMarkdownEntry(t *testing.T, store *MarkdownStore, entry Entry) string {
	t.Helper()
	id, err := store.CreateEntry(entry)
	if err != nil {
		t.Fatalf("failed to create entry %q: %v", entry.Message, err)
	}
	return id
}

// writeTestFile writes content at path, creating the folders above it.
func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("failed to create %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write %s: %v", path, err)
	}
}

// writeEntryFile renders the entry into its day folder under fileName without going through
// the store, the way another tool would. It returns the file's path.
func writeEntryFile(t *testing.T, store *MarkdownStore, entry Entry, fileName string) string {
	t.Helper()
	content, err := renderEntry(&entry)
	if err != nil {
		t.Fatalf("failed to render entry %q: %v", entry.Message, err)
	}
	path := filepath.Join(store.entryDirPath(entry.Timestamp), fileName)
	writeTestFile(t, path, content)
	return path
}

// entryIDs returns the IDs of the entries, in order.
func entryIDs(entries []Entry) []string {
	ids := make([]string, 0, len(entries))
	for _, entry := range entries {
		ids = append(ids, entry.ID)
	}
	return ids
}

// pollEntries polls the feed and fails the test if Poll returns an error.
func pollEntries(t *testing.T, feed Feed) []Entry {
	t.Helper()
	entries, err := feed.Poll()
	if err != nil {
		t.Fatalf("Poll failed: %v", err)
	}
	return entries
}

// assertPolls polls the feed and fails the test unless it returns exactly the wanted IDs, in
// that order. It returns the entries so a test can look at them further.
func assertPolls(t *testing.T, feed Feed, want ...string) []Entry {
	t.Helper()
	entries := pollEntries(t, feed)
	if got := entryIDs(entries); !slices.Equal(got, want) {
		t.Fatalf("Poll returned IDs %v, want %v", got, want)
	}
	return entries
}

func TestMarkdownFeedReportsOnlyEntriesAddedAfterOpen(t *testing.T) {
	store := newTestMarkdownStore(t)
	defer store.Close()

	createMarkdownEntry(t, store, Entry{Timestamp: feedTestTime(10, 0), Message: "first, before open"})
	createMarkdownEntry(t, store, Entry{Timestamp: feedTestTime(10, 1), Message: "second, before open"})
	feed := openMarkdownTestFeed(t, store, nil)

	assertPolls(t, feed)

	added := Entry{
		Timestamp:        feedTestTime(10, 2),
		Message:          "added after open",
		Hostname:         "test-host",
		Username:         "test-user",
		WorkingDirectory: "/tmp/work",
		Tags:             []string{"live", "test"},
	}
	added.ID = createMarkdownEntry(t, store, added)

	got := assertPolls(t, feed, added.ID)[0]
	if got.Message != added.Message || got.Hostname != added.Hostname || got.Username != added.Username ||
		got.WorkingDirectory != added.WorkingDirectory || !slices.Equal(got.Tags, added.Tags) ||
		!got.Timestamp.Equal(added.Timestamp) {
		t.Errorf("Poll returned %+v, want %+v", got, added)
	}

	assertPolls(t, feed)
}

func TestMarkdownFeedOrdersOldestFirst(t *testing.T) {
	store := newTestMarkdownStore(t)
	defer store.Close()
	feed := openMarkdownTestFeed(t, store, nil)

	// File names sort in the opposite order to the timestamps, and the entries are written
	// newest first, so only sorting by timestamp gives the right sequence.
	later := createMarkdownEntry(t, store, Entry{Timestamp: feedTestTime(10, 2), Message: "alpha"})
	earlier := createMarkdownEntry(t, store, Entry{Timestamp: feedTestTime(10, 1), Message: "bravo"})

	assertPolls(t, feed, earlier, later)
}

func TestMarkdownFeedIgnoresTempFiles(t *testing.T) {
	store := newTestMarkdownStore(t)
	defer store.Close()
	feed := openMarkdownTestFeed(t, store, nil)

	// A writer that is mid-publish leaves its content in a .tmp-* file until the rename.
	writeEntryFile(t, store,
		Entry{ID: "temp-file-entry", Timestamp: feedTestTime(10, 0), Message: "still being written"},
		".tmp-12345")

	assertPolls(t, feed)
}

func TestMarkdownFeedSeesNewDayMonthYearFolders(t *testing.T) {
	store := newTestMarkdownStore(t)
	defer store.Close()
	feed := openMarkdownTestFeed(t, store, nil)

	first := createMarkdownEntry(t, store, Entry{
		Timestamp: time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC),
		Message:   "start of 2030",
	})
	second := createMarkdownEntry(t, store, Entry{
		Timestamp: time.Date(2031, 2, 3, 12, 0, 0, 0, time.UTC),
		Message:   "mid 2031",
	})

	assertPolls(t, feed, first, second)
}

func TestMarkdownFeedSkipsEditedEntries(t *testing.T) {
	store := newTestMarkdownStore(t)
	defer store.Close()

	timestamp := feedTestTime(10, 0)
	id := createMarkdownEntry(t, store, Entry{Timestamp: timestamp, Message: "before the edit"})
	feed := openMarkdownTestFeed(t, store, nil)
	pathBefore := markdownEntryFiles(t, store, id)

	if err := store.UpdateEntry(Entry{ID: id, Timestamp: timestamp, Message: "after the edit"}); err != nil {
		t.Fatalf("failed to update entry: %v", err)
	}

	// A new message gives the entry a new file name, so the edit shows up as a new file.
	pathAfter := markdownEntryFiles(t, store, id)
	if len(pathBefore) != 1 || len(pathAfter) != 1 || pathBefore[0] == pathAfter[0] {
		t.Fatalf("update did not rename the file: before %v, after %v", pathBefore, pathAfter)
	}

	assertPolls(t, feed)
}

func TestMarkdownFeedIgnoresDeletions(t *testing.T) {
	store := newTestMarkdownStore(t)
	defer store.Close()

	id := createMarkdownEntry(t, store, Entry{Timestamp: feedTestTime(10, 0), Message: "soon deleted"})
	feed := openMarkdownTestFeed(t, store, nil)

	if err := store.DeleteEntry(id); err != nil {
		t.Fatalf("failed to delete entry: %v", err)
	}

	assertPolls(t, feed)
}

func TestMarkdownFeedSkipsMalformedFiles(t *testing.T) {
	store := newTestMarkdownStore(t)
	defer store.Close()
	feed := openMarkdownTestFeed(t, store, nil)

	timestamp := feedTestTime(10, 0)
	writeTestFile(t, filepath.Join(store.entryDirPath(timestamp), "bad.md"), "no frontmatter in this file\n")
	good := createMarkdownEntry(t, store, Entry{Timestamp: timestamp, Message: "a good entry"})

	assertPolls(t, feed, good)
}

func TestMarkdownFeedRetriesFilesThatFailedToParse(t *testing.T) {
	store := newTestMarkdownStore(t)
	defer store.Close()
	feed := openMarkdownTestFeed(t, store, nil)

	entry := Entry{ID: "half-written-id", Timestamp: feedTestTime(10, 0), Message: "finished later"}
	path := filepath.Join(store.entryDirPath(entry.Timestamp), "half-written.md")
	writeTestFile(t, path, "---\nid: half-written-id\n")
	assertPolls(t, feed)

	// The writer finishes the file in place, which doesn't change the folder's modified time. The
	// feed lists the folder again only because the folder changed less than 2 seconds ago
	// (markdownRecentFolderWindow), so this test relies on reaching the second poll inside that window.
	writeEntryFile(t, store, entry, "half-written.md")

	assertPolls(t, feed, entry.ID)
}

func TestMarkdownFeedIgnoresFilesThatWereMalformedAtOpen(t *testing.T) {
	store := newTestMarkdownStore(t)
	defer store.Close()

	entry := Entry{ID: "old-broken-id", Timestamp: feedTestTime(10, 0), Message: "broken before open"}
	writeTestFile(t, filepath.Join(store.entryDirPath(entry.Timestamp), "old-broken.md"), "not an entry\n")
	feed := openMarkdownTestFeed(t, store, nil)

	// The file existed when the feed opened, so repairing it later doesn't make it new.
	writeEntryFile(t, store, entry, "old-broken.md")

	assertPolls(t, feed)
}

func TestMarkdownFeedReadsLegacyFileNames(t *testing.T) {
	store := newTestMarkdownStore(t)
	defer store.Close()
	feed := openMarkdownTestFeed(t, store, nil)

	// Homebrew 1.8.0 still names files <slug>-<first 8 characters of the id>.md.
	entry := Entry{ID: "0a1b2c3d-1111-4222-8333-444455556666", Timestamp: feedTestTime(10, 0), Message: "legacy name"}
	writeEntryFile(t, store, entry, "legacy-name-"+entry.ID[:8]+".md")

	assertPolls(t, feed, entry.ID)
}

func TestMarkdownFeedReadsOnlyDayFolders(t *testing.T) {
	store := newTestMarkdownStore(t)
	defer store.Close()
	feed := openMarkdownTestFeed(t, store, nil)

	// Day folders are <data dir>/YYYY/MM/DD. An entry file anywhere else doesn't count.
	misplaced := []string{
		"stray.md",
		"2026/stray.md",
		"2026/11/stray.md",
		"2026/11/11/deeper/nested.md",
		"notes/11/11/not-a-year.md",
		"26/11/11/short-year.md",
		"20266/11/11/long-year.md",
		"2026/1/11/short-month.md",
		"2026/11/1/short-day.md",
		"2026/11/ab/not-a-day.md",
	}
	for i, relPath := range misplaced {
		entry := Entry{ID: fmt.Sprintf("misplaced-%d", i), Timestamp: feedTestTime(10, i), Message: "misplaced"}
		content, err := renderEntry(&entry)
		if err != nil {
			t.Fatalf("failed to render entry: %v", err)
		}
		writeTestFile(t, filepath.Join(store.dataDir, filepath.FromSlash(relPath)), content)
	}
	inPlace := Entry{ID: "in-place", Timestamp: feedTestTime(11, 0), Message: "in a day folder"}
	writeEntryFile(t, store, inPlace, "in-place.md")

	assertPolls(t, feed, inPlace.ID)
}

func TestMarkdownFeedAppliesFilter(t *testing.T) {
	store := newTestMarkdownStore(t)
	defer store.Close()

	since := feedTestTime(10, 15)
	until := feedTestTime(11, 30)
	cases := []struct {
		name   string
		filter *SearchFilter
		want   []string // Names of the entries that must come back, oldest first.
	}{
		{"no filter", nil, []string{"ship", "lunch", "retro", "reading"}},
		{"tag", &SearchFilter{Tags: []string{"keep"}}, []string{"ship", "lunch"}},
		{"text ignores case", &SearchFilter{Text: "DEPLOY"}, []string{"ship", "retro"}},
		{"since and until", &SearchFilter{Since: &since, Until: &until}, []string{"lunch", "retro"}},
		{"tag and since", &SearchFilter{Tags: []string{"keep"}, Since: &since}, []string{"lunch"}},
	}
	feeds := make([]Feed, len(cases))
	for i, tc := range cases {
		feeds[i] = openMarkdownTestFeed(t, store, tc.filter)
	}

	entries := []struct {
		name  string
		entry Entry
	}{
		{"ship", Entry{Timestamp: feedTestTime(10, 0), Message: "deploy finished", Tags: []string{"keep"}}},
		{"lunch", Entry{Timestamp: feedTestTime(10, 30), Message: "lunch break", Tags: []string{"keep"}}},
		{"retro", Entry{Timestamp: feedTestTime(11, 0), Message: "Deploy retro", Tags: []string{"team"}}},
		{"reading", Entry{Timestamp: feedTestTime(12, 0), Message: "reading papers"}},
	}
	idsByName := make(map[string]string, len(entries))
	for _, e := range entries {
		idsByName[e.name] = createMarkdownEntry(t, store, e.entry)
	}

	for i, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			want := make([]string, 0, len(tc.want))
			for _, name := range tc.want {
				want = append(want, idsByName[name])
			}
			assertPolls(t, feeds[i], want...)
		})
	}
}

func TestMarkdownFeedMissingDataDir(t *testing.T) {
	dataDir := filepath.Join(t.TempDir(), "absent")

	feed, err := newMarkdownFeed(dataDir, nil)
	if err != nil {
		t.Fatalf("failed to open feed on a missing data directory: %v", err)
	}
	entries, err := feed.Poll()
	if entries != nil || err != nil {
		t.Fatalf("Poll returned %v, %v; want nil, nil", entries, err)
	}
	if _, err := os.Stat(dataDir); !os.IsNotExist(err) {
		t.Fatalf("feed created the data directory: stat error %v", err)
	}

	// The data directory appears once a writer starts using it.
	store, err := NewMarkdownStore(dataDir)
	if err != nil {
		t.Fatalf("failed to create markdown store: %v", err)
	}
	defer store.Close()
	id := createMarkdownEntry(t, store, Entry{Timestamp: feedTestTime(10, 0), Message: "first entry ever"})

	assertPolls(t, feed, id)
}

func TestMarkdownFeedFailsWhenDataDirIsAFile(t *testing.T) {
	notAFolder := filepath.Join(t.TempDir(), "chronicle-data")
	writeTestFile(t, notAFolder, "a file where the data directory should be\n")

	// A missing data directory is an empty store, but one that can't be read is an error.
	if feed, err := newMarkdownFeed(notAFolder, nil); err == nil {
		t.Fatalf("expected an error opening a feed on a file, got feed %v", feed)
	}
}

func TestMarkdownFeedRelistsRecentFolderWithUnchangedModTime(t *testing.T) {
	store := newTestMarkdownStore(t)
	defer store.Close()
	feed := openMarkdownTestFeed(t, store, nil)

	timestamp := feedTestTime(10, 0)
	a := createMarkdownEntry(t, store, Entry{Timestamp: timestamp, Message: "entry a"})
	assertPolls(t, feed, a)

	folder := store.entryDirPath(timestamp)
	info, err := os.Stat(folder)
	if err != nil {
		t.Fatalf("failed to stat day folder: %v", err)
	}
	recorded := info.ModTime()

	// A file system with a coarse clock gives two changes made close together the same
	// modified time. Put the time back to what the feed recorded to stand in for that.
	b := Entry{ID: "entry-b", Timestamp: timestamp.Add(time.Minute), Message: "entry b"}
	writeEntryFile(t, store, b, entryFileName(b.Message, b.ID))
	if err := os.Chtimes(folder, recorded, recorded); err != nil {
		t.Fatalf("failed to reset day folder time: %v", err)
	}
	if info, err = os.Stat(folder); err != nil || !info.ModTime().Equal(recorded) {
		t.Fatalf("day folder time was not reset: got %v (error %v), want %v", info.ModTime(), err, recorded)
	}

	assertPolls(t, feed, b.ID)
}

func TestMarkdownFeedSkipsUnchangedOldFolders(t *testing.T) {
	store := newTestMarkdownStore(t)
	defer store.Close()

	timestamp := feedTestTime(10, 0)
	createMarkdownEntry(t, store, Entry{Timestamp: timestamp, Message: "an old entry"})
	folder := store.entryDirPath(timestamp)
	old := time.Now().Add(-time.Hour)
	if err := os.Chtimes(folder, old, old); err != nil {
		t.Fatalf("failed to age day folder: %v", err)
	}
	feed := openMarkdownTestFeed(t, store, nil)

	// Polling costs the same however many entries exist because a folder whose modified time
	// is old and unchanged isn't listed again. Hide a new file from the feed that way.
	hidden := Entry{ID: "hidden-entry", Timestamp: timestamp.Add(time.Minute), Message: "hidden entry"}
	writeEntryFile(t, store, hidden, entryFileName(hidden.Message, hidden.ID))
	if err := os.Chtimes(folder, old, old); err != nil {
		t.Fatalf("failed to age day folder again: %v", err)
	}
	assertPolls(t, feed)

	// Once the folder's modified time changes, the file shows up.
	changed := old.Add(time.Second)
	if err := os.Chtimes(folder, changed, changed); err != nil {
		t.Fatalf("failed to change day folder time: %v", err)
	}
	assertPolls(t, feed, hidden.ID)
}

func TestMarkdownFeedConcurrentWritersSeenExactlyOnce(t *testing.T) {
	const writers = 4
	const entriesPerWriter = 25

	dataDir := t.TempDir()
	reader, err := NewMarkdownStore(dataDir)
	if err != nil {
		t.Fatalf("failed to create markdown store: %v", err)
	}
	defer reader.Close()
	feed := openMarkdownTestFeed(t, reader, nil)

	var (
		created   = make(map[string]bool)
		createdMu sync.Mutex
		writing   sync.WaitGroup
	)
	for w := 0; w < writers; w++ {
		writing.Add(1)
		go func() {
			defer writing.Done()
			// Each writer is a store of its own on the same folder, like separate chronicle processes.
			store, err := NewMarkdownStore(dataDir)
			if err != nil {
				t.Errorf("writer %d: failed to create markdown store: %v", w, err)
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
		ticker := time.NewTicker(5 * time.Millisecond)
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
