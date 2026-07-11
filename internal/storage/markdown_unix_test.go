// ABOUTME: Unix-specific MarkdownStore filesystem failure tests
// ABOUTME: Covers fail-closed uniqueness scans and move rollback on Darwin and Linux

//go:build darwin || linux

package storage

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestMarkdownCreateEntryFailsClosedForUnreadableCandidate(t *testing.T) {
	store := newTestMarkdownStore(t)
	defer store.Close()

	brokenPath := filepath.Join(store.dataDir, "unreadable.md")
	missingTarget := filepath.Join(store.dataDir, "missing-target")
	if err := os.Symlink(missingTarget, brokenPath); err != nil {
		t.Fatalf("failed to create unreadable markdown symlink: %v", err)
	}
	entry := Entry{
		ID:        "unreadable-candidate-id",
		Timestamp: time.Date(2026, 7, 9, 12, 0, 0, 0, time.UTC),
		Message:   "must not be created",
	}
	targetPath := filepath.Join(store.entryDirPath(entry.Timestamp), entryFileName(entry.Message, entry.ID))

	_, err := store.CreateEntry(entry)
	if err == nil {
		t.Fatal("expected uniqueness scan failure for unreadable candidate")
	}
	if !strings.Contains(err.Error(), "check entry ID uniqueness") {
		t.Errorf("expected uniqueness scan error, got %v", err)
	}
	if errors.Is(err, errMarkdownEntryAlreadyExists) || errors.Is(err, errMarkdownEntryNotFound) {
		t.Errorf("expected inspection failure rather than duplicate/not-found error, got %v", err)
	}
	if _, statErr := os.Stat(targetPath); !os.IsNotExist(statErr) {
		t.Errorf("entry was created despite uniqueness scan failure: stat error %v", statErr)
	}
	if info, statErr := os.Lstat(brokenPath); statErr != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Errorf("unreadable candidate changed after uniqueness scan failure: info=%v error=%v", info, statErr)
	}
}

// Stable destination permissions that allow CreateTemp and Link also allow removing
// the temp name, so source removal is the nearest deterministic rollback boundary.
func TestMarkdownUpdateEntryRollsBackWhenSourceRemovalFails(t *testing.T) {
	store := newTestMarkdownStore(t)
	defer store.Close()

	original := Entry{
		ID:        "source-removal-failure",
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
	originalDir := filepath.Dir(originalPath)
	dirInfo, err := os.Stat(originalDir)
	if err != nil {
		t.Fatalf("failed to inspect original directory: %v", err)
	}
	if err := os.Chmod(originalDir, 0550); err != nil {
		t.Fatalf("failed to make original directory read-only: %v", err)
	}
	t.Cleanup(func() {
		if err := os.Chmod(originalDir, dirInfo.Mode().Perm()); err != nil {
			t.Errorf("failed to restore original directory permissions: %v", err)
		}
	})

	updated := original
	updated.Timestamp = original.Timestamp.Add(24 * time.Hour)
	updated.Message = "moved message"
	destinationPath := filepath.Join(store.entryDirPath(updated.Timestamp), entryFileName(updated.Message, updated.ID))
	beforeModified := store.LastModified()

	err = store.UpdateEntry(updated)
	if err == nil {
		t.Fatal("expected source removal failure")
	}
	if !strings.Contains(err.Error(), "remove previous entry file") {
		t.Errorf("expected source removal error, got %v", err)
	}
	if !store.LastModified().Equal(beforeModified) {
		t.Error("LastModified changed after rolled-back update")
	}
	afterData, err := os.ReadFile(originalPath)
	if err != nil {
		t.Fatalf("original entry path was not retained: %v", err)
	}
	if string(afterData) != string(originalData) {
		t.Error("original entry bytes changed after rollback")
	}
	if _, err := os.Stat(destinationPath); !os.IsNotExist(err) {
		t.Errorf("destination remained after rollback: stat error %v", err)
	}
	paths := markdownEntryFiles(t, store, original.ID)
	if len(paths) != 1 || paths[0] != originalPath {
		t.Errorf("expected one original file after rollback, got %v", paths)
	}
}
