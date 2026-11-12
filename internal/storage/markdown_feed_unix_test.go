// ABOUTME: Unix-only test that the markdown feed reads while another process holds the write lock
// ABOUTME: Holds an exclusive flock on the data directory's .lock file, as a writing process does

//go:build darwin || linux

package storage

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

func TestMarkdownFeedDoesNotTakeWriteLock(t *testing.T) {
	store := newTestMarkdownStore(t)
	defer store.Close()

	lockPath := filepath.Join(store.dataDir, ".lock")
	lockFile, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		t.Fatalf("failed to open lock file: %v", err)
	}
	defer lockFile.Close()
	if err := syscall.Flock(int(lockFile.Fd()), syscall.LOCK_EX); err != nil {
		t.Fatalf("failed to lock the data directory: %v", err)
	}
	defer syscall.Flock(int(lockFile.Fd()), syscall.LOCK_UN)

	// A writer that opened the lock file afresh must be shut out, or holding it proves nothing.
	probe, err := os.OpenFile(lockPath, os.O_RDWR, 0o644)
	if err != nil {
		t.Fatalf("failed to open lock file a second time: %v", err)
	}
	defer probe.Close()
	if err := syscall.Flock(int(probe.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); !errors.Is(err, syscall.EWOULDBLOCK) {
		t.Fatalf("a second lock attempt returned %v, want EWOULDBLOCK: the lock is not held", err)
	}

	feed := openMarkdownTestFeed(t, store, nil)
	entry := Entry{ID: "written-while-locked", Timestamp: feedTestTime(10, 0), Message: "written while locked"}
	writeEntryFile(t, store, entry, entryFileName(entry.Message, entry.ID))

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
	case result := <-polled:
		if result.err != nil {
			t.Fatalf("Poll failed: %v", result.err)
		}
		if len(result.entries) != 1 || result.entries[0].ID != entry.ID {
			t.Fatalf("Poll returned %v, want the entry %s", entryIDs(result.entries), entry.ID)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Poll blocked on the write lock")
	}
}
