// ABOUTME: Tests for vault syncer
// ABOUTME: Verifies change queuing and apply logic
package sync

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/harper/chronicle/internal/db"
)

func TestSyncerQueueEntry(t *testing.T) {
	tmpDir := t.TempDir()
	if err := os.Setenv("HOME", tmpDir); err != nil {
		t.Fatalf("set HOME: %v", err)
	}
	defer func() { _ = os.Unsetenv("HOME") }()

	// Create app database
	appDBPath := filepath.Join(tmpDir, "chronicle.db")
	appDB, err := db.InitDB(appDBPath)
	if err != nil {
		t.Fatalf("init app db: %v", err)
	}
	defer func() { _ = appDB.Close() }()

	// Create config with derived key
	cfg := &Config{
		Server:     "https://api.storeusa.org",
		UserID:     "test-user",
		Token:      "test-token",
		DerivedKey: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		DeviceID:   "test-device",
		VaultDB:    filepath.Join(tmpDir, "vault.db"),
		AutoSync:   false,
	}

	syncer, err := NewSyncer(cfg, appDB)
	if err != nil {
		t.Fatalf("create syncer: %v", err)
	}
	defer func() { _ = syncer.Close() }()

	// Queue an entry change
	entry := db.Entry{
		ID:               "test-entry-uuid",
		Timestamp:        time.Now(),
		Message:          "test message",
		Hostname:         "test-host",
		Username:         "test-user",
		WorkingDirectory: "/test/dir",
		Tags:             []string{"tag1", "tag2"},
	}

	err = syncer.QueueEntryChange(context.Background(), entry, OpUpsert)
	if err != nil {
		t.Fatalf("queue entry: %v", err)
	}

	// Verify pending count
	count, err := syncer.PendingCount(context.Background())
	if err != nil {
		t.Fatalf("pending count: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 pending, got %d", count)
	}
}
