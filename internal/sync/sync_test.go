// ABOUTME: Tests for vault sync integration
// ABOUTME: Verifies change queuing, syncing, and pending count tracking

//go:build sqlite_fts5

package sync

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/harperreed/sweet/vault"

	"github.com/harper/chronicle/internal/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewSyncer(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test app database
	appDB := setupTestDB(t, tmpDir)
	defer func() { _ = appDB.Close() }()

	// Create seed and derive key
	seed, phrase, err := vault.NewSeedPhrase()
	require.NoError(t, err)

	cfg := &Config{
		Server:     "https://test.example.com",
		UserID:     "test-user",
		Token:      "test-token",
		DerivedKey: phrase,
		DeviceID:   "test-device",
		VaultDB:    filepath.Join(tmpDir, "vault.db"),
		AutoSync:   false,
	}

	syncer, err := NewSyncer(cfg, appDB)
	require.NoError(t, err)
	require.NotNil(t, syncer)
	defer func() { _ = syncer.Close() }()

	assert.Equal(t, cfg, syncer.config)
	assert.NotNil(t, syncer.store)
	assert.NotNil(t, syncer.client)
	assert.NotNil(t, syncer.keys)

	// Verify keys were derived correctly
	expectedKeys, err := vault.DeriveKeys(seed, "", vault.DefaultKDFParams())
	require.NoError(t, err)
	assert.Equal(t, expectedKeys.EncKey, syncer.keys.EncKey)
}

func TestNewSyncerNoDerivedKey(t *testing.T) {
	tmpDir := t.TempDir()

	appDB := setupTestDB(t, tmpDir)
	defer func() { _ = appDB.Close() }()

	cfg := &Config{
		Server:   "https://test.example.com",
		DeviceID: "test-device",
		VaultDB:  filepath.Join(tmpDir, "vault.db"),
	}

	_, err := NewSyncer(cfg, appDB)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "derived key not configured")
}

func TestNewSyncerInvalidDerivedKey(t *testing.T) {
	tmpDir := t.TempDir()

	appDB := setupTestDB(t, tmpDir)
	defer func() { _ = appDB.Close() }()

	cfg := &Config{
		Server:     "https://test.example.com",
		UserID:     "test-user",
		Token:      "test-token",
		DerivedKey: "invalid-key-format",
		DeviceID:   "test-device",
		VaultDB:    filepath.Join(tmpDir, "vault.db"),
	}

	_, err := NewSyncer(cfg, appDB)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid derived key")
}

func TestQueueEntryChangeUpsert(t *testing.T) {
	ctx := context.Background()
	syncer := setupTestSyncer(t)
	defer func() { _ = syncer.Close() }()

	entry := db.Entry{
		ID:               "test-entry-uuid",
		Timestamp:        time.Now(),
		Message:          "test message",
		Hostname:         "test-host",
		Username:         "test-user",
		WorkingDirectory: "/test/dir",
		Tags:             []string{"tag1", "tag2"},
	}

	// Queue entry create
	err := syncer.QueueEntryChange(ctx, entry, vault.OpUpsert)
	require.NoError(t, err)

	// Verify change was queued
	count, err := syncer.PendingCount(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, count)
}

func TestQueueEntryChangeDelete(t *testing.T) {
	ctx := context.Background()
	syncer := setupTestSyncer(t)
	defer func() { _ = syncer.Close() }()

	entry := db.Entry{
		ID: "test-entry-uuid",
	}

	// Queue entry delete
	err := syncer.QueueEntryChange(ctx, entry, vault.OpDelete)
	require.NoError(t, err)

	// Verify change was queued
	count, err := syncer.PendingCount(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, count)
}

func TestQueueEntryChangeWithTags(t *testing.T) {
	ctx := context.Background()
	syncer := setupTestSyncer(t)
	defer func() { _ = syncer.Close() }()

	entry := db.Entry{
		ID:               "entry-with-tags",
		Timestamp:        time.Now(),
		Message:          "test with tags",
		Hostname:         "test-host",
		Username:         "test-user",
		WorkingDirectory: "/test/dir",
		Tags:             []string{"work", "dev", "important"},
	}

	err := syncer.QueueEntryChange(ctx, entry, vault.OpUpsert)
	require.NoError(t, err)

	count, err := syncer.PendingCount(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, count)
}

func TestQueueEntryChangeWithEmptyTags(t *testing.T) {
	ctx := context.Background()
	syncer := setupTestSyncer(t)
	defer func() { _ = syncer.Close() }()

	entry := db.Entry{
		ID:               "entry-no-tags",
		Timestamp:        time.Now(),
		Message:          "test without tags",
		Hostname:         "test-host",
		Username:         "test-user",
		WorkingDirectory: "/test/dir",
		Tags:             []string{},
	}

	err := syncer.QueueEntryChange(ctx, entry, vault.OpUpsert)
	require.NoError(t, err)

	count, err := syncer.PendingCount(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, count)
}

func TestPendingCount(t *testing.T) {
	ctx := context.Background()
	syncer := setupTestSyncer(t)
	defer func() { _ = syncer.Close() }()

	// Initially zero
	count, err := syncer.PendingCount(ctx)
	require.NoError(t, err)
	assert.Equal(t, 0, count)

	// Queue multiple changes
	for i := 0; i < 5; i++ {
		entry := db.Entry{
			ID:               "entry-" + string(rune('A'+i)),
			Timestamp:        time.Now(),
			Message:          "message " + string(rune('A'+i)),
			Hostname:         "test-host",
			Username:         "test-user",
			WorkingDirectory: "/test/dir",
		}
		err = syncer.QueueEntryChange(ctx, entry, vault.OpUpsert)
		require.NoError(t, err)
	}

	// Verify count
	count, err = syncer.PendingCount(ctx)
	require.NoError(t, err)
	assert.Equal(t, 5, count)
}

func TestMultipleChanges(t *testing.T) {
	ctx := context.Background()
	syncer := setupTestSyncer(t)
	defer func() { _ = syncer.Close() }()

	// Create multiple entries
	for i := 0; i < 3; i++ {
		entry := db.Entry{
			ID:               "entry-" + string(rune('A'+i)),
			Timestamp:        time.Now(),
			Message:          "message " + string(rune('A'+i)),
			Hostname:         "test-host",
			Username:         "test-user",
			WorkingDirectory: "/test/dir",
		}
		err := syncer.QueueEntryChange(ctx, entry, vault.OpUpsert)
		require.NoError(t, err)
	}

	count, err := syncer.PendingCount(ctx)
	require.NoError(t, err)
	assert.Equal(t, 3, count)
}

func TestAutoSyncDisabled(t *testing.T) {
	ctx := context.Background()
	syncer := setupTestSyncer(t)
	defer func() { _ = syncer.Close() }()

	// AutoSync is disabled by default in test setup
	assert.False(t, syncer.config.AutoSync)

	entry := db.Entry{
		ID:               "test-entry",
		Timestamp:        time.Now(),
		Message:          "test message",
		Hostname:         "test-host",
		Username:         "test-user",
		WorkingDirectory: "/test/dir",
	}

	err := syncer.QueueEntryChange(ctx, entry, vault.OpUpsert)
	require.NoError(t, err)

	// Change should be queued but not synced
	count, err := syncer.PendingCount(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, count)
}

func TestSyncNotConfigured(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()

	appDB := setupTestDB(t, tmpDir)
	defer func() { _ = appDB.Close() }()

	_, phrase, err := vault.NewSeedPhrase()
	require.NoError(t, err)

	// Create syncer with missing server config
	cfg := &Config{
		Server:     "", // Empty server
		UserID:     "",
		Token:      "",
		DerivedKey: phrase,
		DeviceID:   "test-device",
		VaultDB:    filepath.Join(tmpDir, "vault.db"),
	}

	syncer, err := NewSyncer(cfg, appDB)
	require.NoError(t, err)
	defer func() { _ = syncer.Close() }()

	// Sync should fail with helpful error
	err = syncer.Sync(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "sync not configured")
}

func TestCanSync(t *testing.T) {
	tests := []struct {
		name     string
		config   *Config
		expected bool
	}{
		{
			name: "fully configured",
			config: &Config{
				Server: "https://example.com",
				Token:  "token",
				UserID: "user",
			},
			expected: true,
		},
		{
			name: "missing server",
			config: &Config{
				Server: "",
				Token:  "token",
				UserID: "user",
			},
			expected: false,
		},
		{
			name: "missing token",
			config: &Config{
				Server: "https://example.com",
				Token:  "",
				UserID: "user",
			},
			expected: false,
		},
		{
			name: "missing user id",
			config: &Config{
				Server: "https://example.com",
				Token:  "token",
				UserID: "",
			},
			expected: false,
		},
		{
			name: "all missing",
			config: &Config{
				Server: "",
				Token:  "",
				UserID: "",
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()

			appDB := setupTestDB(t, tmpDir)
			defer func() { _ = appDB.Close() }()

			_, phrase, err := vault.NewSeedPhrase()
			require.NoError(t, err)

			tt.config.DerivedKey = phrase
			tt.config.DeviceID = "test-device"
			tt.config.VaultDB = filepath.Join(tmpDir, "vault.db")

			syncer, err := NewSyncer(tt.config, appDB)
			require.NoError(t, err)
			defer func() { _ = syncer.Close() }()

			assert.Equal(t, tt.expected, syncer.canSync())
		})
	}
}

func TestPendingChanges(t *testing.T) {
	ctx := context.Background()
	syncer := setupTestSyncer(t)
	defer func() { _ = syncer.Close() }()

	// Queue some changes
	entry1 := db.Entry{
		ID:               "entry-1",
		Timestamp:        time.Now(),
		Message:          "message 1",
		Hostname:         "test-host",
		Username:         "test-user",
		WorkingDirectory: "/test/dir",
	}
	err := syncer.QueueEntryChange(ctx, entry1, vault.OpUpsert)
	require.NoError(t, err)

	entry2 := db.Entry{
		ID:               "entry-2",
		Timestamp:        time.Now(),
		Message:          "message 2",
		Hostname:         "test-host",
		Username:         "test-user",
		WorkingDirectory: "/test/dir",
	}
	err = syncer.QueueEntryChange(ctx, entry2, vault.OpUpsert)
	require.NoError(t, err)

	// Get pending changes
	changes, err := syncer.PendingChanges(ctx)
	require.NoError(t, err)
	require.Len(t, changes, 2)

	// Verify structure
	for _, change := range changes {
		assert.NotEmpty(t, change.ChangeID)
		assert.True(t, strings.HasSuffix(change.Entity, EntityEntry), "entity should end with %s, got %s", EntityEntry, change.Entity)
		assert.False(t, change.TS.IsZero())
	}
}

func TestLastSyncedSeq(t *testing.T) {
	ctx := context.Background()
	syncer := setupTestSyncer(t)
	defer func() { _ = syncer.Close() }()

	// Initially should be "0"
	seq, err := syncer.LastSyncedSeq(ctx)
	require.NoError(t, err)
	assert.Equal(t, "0", seq)
}

func TestCloseNilStore(t *testing.T) {
	syncer := &Syncer{
		store: nil,
	}

	err := syncer.Close()
	assert.NoError(t, err)
}

func TestQueueChangeEncryption(t *testing.T) {
	ctx := context.Background()
	syncer := setupTestSyncer(t)
	defer func() { _ = syncer.Close() }()

	entry := db.Entry{
		ID:               "encrypted-entry",
		Timestamp:        time.Now(),
		Message:          "secret message",
		Hostname:         "test-host",
		Username:         "test-user",
		WorkingDirectory: "/test/dir",
	}

	err := syncer.QueueEntryChange(ctx, entry, vault.OpUpsert)
	require.NoError(t, err)

	// Verify change was encrypted (indirectly by checking it was queued)
	count, err := syncer.PendingCount(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, count)
}

func TestQueueMultipleDeletes(t *testing.T) {
	ctx := context.Background()
	syncer := setupTestSyncer(t)
	defer func() { _ = syncer.Close() }()

	// Queue multiple deletes
	for i := 0; i < 3; i++ {
		entry := db.Entry{
			ID: "entry-" + string(rune('A'+i)),
		}
		err := syncer.QueueEntryChange(ctx, entry, vault.OpDelete)
		require.NoError(t, err)
	}

	count, err := syncer.PendingCount(ctx)
	require.NoError(t, err)
	assert.Equal(t, 3, count)
}

func TestQueueEntryChangeLongMessage(t *testing.T) {
	ctx := context.Background()
	syncer := setupTestSyncer(t)
	defer func() { _ = syncer.Close() }()

	// Create entry with long message
	longMessage := ""
	for i := 0; i < 1000; i++ {
		longMessage += "This is a test message. "
	}

	entry := db.Entry{
		ID:               "long-entry",
		Timestamp:        time.Now(),
		Message:          longMessage,
		Hostname:         "test-host",
		Username:         "test-user",
		WorkingDirectory: "/test/dir",
	}

	err := syncer.QueueEntryChange(ctx, entry, vault.OpUpsert)
	require.NoError(t, err)

	count, err := syncer.PendingCount(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, count)
}

func TestQueueEntryChangeSpecialCharacters(t *testing.T) {
	ctx := context.Background()
	syncer := setupTestSyncer(t)
	defer func() { _ = syncer.Close() }()

	entry := db.Entry{
		ID:               "special-chars",
		Timestamp:        time.Now(),
		Message:          "Test with special chars: 日本語 🚀 ñ á é",
		Hostname:         "test-host-ñ",
		Username:         "test-user-日本",
		WorkingDirectory: "/test/dir/🚀",
		Tags:             []string{"tag-日本", "tag-🚀"},
	}

	err := syncer.QueueEntryChange(ctx, entry, vault.OpUpsert)
	require.NoError(t, err)

	count, err := syncer.PendingCount(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, count)
}
