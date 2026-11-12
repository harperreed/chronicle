// ABOUTME: Unix-only test that the markdown reader searches while another process holds the write lock.
// ABOUTME: Holds an exclusive flock on the data directory's .lock file, as a writing process does.

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

// holdMarkdownWriteLock takes an exclusive lock on the data directory's .lock file, as a writing
// process does, and releases it when the test ends. It fails the test unless a second attempt to
// take the lock is refused, because a test that passes with the lock free proves nothing.
func holdMarkdownWriteLock(t *testing.T, dataDir string) {
	t.Helper()
	lockPath := filepath.Join(dataDir, ".lock")
	lockFile, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		t.Fatalf("failed to open lock file: %v", err)
	}
	t.Cleanup(func() { _ = lockFile.Close() }) // Closing the file drops the lock.
	if err := syscall.Flock(int(lockFile.Fd()), syscall.LOCK_EX); err != nil {
		t.Fatalf("failed to lock the data directory: %v", err)
	}

	probe, err := os.OpenFile(lockPath, os.O_RDWR, 0o644)
	if err != nil {
		t.Fatalf("failed to open lock file a second time: %v", err)
	}
	defer probe.Close()
	if err := syscall.Flock(int(probe.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); !errors.Is(err, syscall.EWOULDBLOCK) {
		t.Fatalf("a second lock attempt returned %v, want EWOULDBLOCK: the lock is not held", err)
	}
}

func TestMarkdownReaderSearchesWhileLockHeld(t *testing.T) {
	dataDir := t.TempDir()
	writer, err := NewMarkdownStore(dataDir)
	if err != nil {
		t.Fatalf("failed to create markdown store: %v", err)
	}
	defer writer.Close()
	entryID := createMarkdownEntry(t, writer, Entry{Timestamp: feedTestTime(10, 0), Message: "written before the lock"})

	holdMarkdownWriteLock(t, dataDir)

	type searchResult struct {
		entries []Entry
		err     error
	}
	searched := make(chan searchResult, 1)
	go func() {
		reader, err := OpenMarkdownReader(dataDir)
		if err != nil {
			searched <- searchResult{err: err}
			return
		}
		defer reader.Close()
		entries, err := reader.SearchEntries(nil, 0)
		searched <- searchResult{entries, err}
	}()

	select {
	case result := <-searched:
		if result.err != nil {
			t.Fatalf("the reader failed while a writer held the lock: %v", result.err)
		}
		if got := entryIDs(result.entries); len(got) != 1 || got[0] != entryID {
			t.Fatalf("SearchEntries returned IDs %v, want [%s]", got, entryID)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("the reader was still waiting 2s after a writer took the lock")
	}
}
