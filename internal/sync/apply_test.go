// ABOUTME: Tests for applying remote changes to local database
// ABOUTME: Verifies entry change application, including edge cases

//go:build sqlite_fts5

package sync

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/harperreed/sweet/vault"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestApplyEntryChangeUpsert(t *testing.T) {
	ctx := context.Background()
	syncer, appDB, cleanup := setupTestSyncerWithDB(t)
	defer cleanup()

	entryID := "test-entry-uuid"
	timestamp := time.Now().UTC().Unix()

	payload := map[string]any{
		"id":                entryID,
		"timestamp":         timestamp,
		"message":           "test message",
		"hostname":          "test-host",
		"username":          "test-user",
		"working_directory": "/test/dir",
		"tags":              []any{"tag1", "tag2"},
	}

	payloadBytes, err := json.Marshal(payload)
	require.NoError(t, err)

	change := vault.Change{
		Entity:   EntityEntry,
		EntityID: entryID,
		Op:       vault.OpUpsert,
		Payload:  payloadBytes,
	}

	err = syncer.applyEntryChange(ctx, change)
	require.NoError(t, err)

	// Verify entry was created
	var message, hostname, username, workingDir string
	var dbTimestamp time.Time
	err = appDB.QueryRowContext(ctx,
		`SELECT message, hostname, username, working_directory, timestamp FROM entries WHERE id = ?`,
		entryID).Scan(&message, &hostname, &username, &workingDir, &dbTimestamp)
	require.NoError(t, err)
	assert.Equal(t, "test message", message)
	assert.Equal(t, "test-host", hostname)
	assert.Equal(t, "test-user", username)
	assert.Equal(t, "/test/dir", workingDir)
	assert.Equal(t, time.Unix(timestamp, 0).Unix(), dbTimestamp.Unix())

	// Verify tags were created
	var tagCount int
	err = appDB.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM tags WHERE entry_id = ?`, entryID).Scan(&tagCount)
	require.NoError(t, err)
	assert.Equal(t, 2, tagCount)
}

func TestApplyEntryChangeUpdate(t *testing.T) {
	ctx := context.Background()
	syncer, appDB, cleanup := setupTestSyncerWithDB(t)
	defer cleanup()

	entryID := "test-entry-uuid"

	// Insert initial entry
	_, err := appDB.ExecContext(ctx,
		`INSERT INTO entries (id, timestamp, message, hostname, username, working_directory) VALUES (?, ?, ?, ?, ?, ?)`,
		entryID, time.Now(), "original message", "original-host", "original-user", "/original/dir")
	require.NoError(t, err)

	// Apply update
	timestamp := time.Now().UTC().Unix()
	payload := map[string]any{
		"id":                entryID,
		"timestamp":         timestamp,
		"message":           "updated message",
		"hostname":          "updated-host",
		"username":          "updated-user",
		"working_directory": "/updated/dir",
		"tags":              []any{},
	}

	payloadBytes, err := json.Marshal(payload)
	require.NoError(t, err)

	change := vault.Change{
		Entity:   EntityEntry,
		EntityID: entryID,
		Op:       vault.OpUpsert,
		Payload:  payloadBytes,
	}

	err = syncer.applyEntryChange(ctx, change)
	require.NoError(t, err)

	// Verify entry was updated
	var message string
	err = appDB.QueryRowContext(ctx,
		`SELECT message FROM entries WHERE id = ?`,
		entryID).Scan(&message)
	require.NoError(t, err)
	assert.Equal(t, "updated message", message)
}

func TestApplyEntryChangeDelete(t *testing.T) {
	ctx := context.Background()
	syncer, appDB, cleanup := setupTestSyncerWithDB(t)
	defer cleanup()

	entryID := "test-entry-uuid"

	// Insert entry
	_, err := appDB.ExecContext(ctx,
		`INSERT INTO entries (id, timestamp, message, hostname, username, working_directory) VALUES (?, ?, ?, ?, ?, ?)`,
		entryID, time.Now(), "test message", "test-host", "test-user", "/test/dir")
	require.NoError(t, err)

	// Apply delete
	change := vault.Change{
		Entity:   EntityEntry,
		EntityID: entryID,
		Op:       vault.OpDelete,
		Deleted:  true,
	}

	err = syncer.applyEntryChange(ctx, change)
	require.NoError(t, err)

	// Verify entry was deleted
	var count int
	err = appDB.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM entries WHERE id = ?`,
		entryID).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 0, count)
}

func TestApplyEntryChangeDeleteWithDeletedFlag(t *testing.T) {
	ctx := context.Background()
	syncer, appDB, cleanup := setupTestSyncerWithDB(t)
	defer cleanup()

	entryID := "test-entry-uuid"

	// Insert entry
	_, err := appDB.ExecContext(ctx,
		`INSERT INTO entries (id, timestamp, message, hostname, username, working_directory) VALUES (?, ?, ?, ?, ?, ?)`,
		entryID, time.Now(), "test message", "test-host", "test-user", "/test/dir")
	require.NoError(t, err)

	// Apply delete with Deleted flag
	change := vault.Change{
		Entity:   EntityEntry,
		EntityID: entryID,
		Op:       vault.OpUpsert,
		Deleted:  true,
	}

	err = syncer.applyEntryChange(ctx, change)
	require.NoError(t, err)

	// Verify entry was deleted
	var count int
	err = appDB.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM entries WHERE id = ?`,
		entryID).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 0, count)
}

func TestApplyEntryChangeWithTags(t *testing.T) {
	ctx := context.Background()
	syncer, appDB, cleanup := setupTestSyncerWithDB(t)
	defer cleanup()

	entryID := "entry-with-tags"
	timestamp := time.Now().UTC().Unix()

	payload := map[string]any{
		"id":                entryID,
		"timestamp":         timestamp,
		"message":           "test message",
		"hostname":          "test-host",
		"username":          "test-user",
		"working_directory": "/test/dir",
		"tags":              []any{"work", "dev", "important"},
	}

	payloadBytes, err := json.Marshal(payload)
	require.NoError(t, err)

	change := vault.Change{
		Entity:   EntityEntry,
		EntityID: entryID,
		Op:       vault.OpUpsert,
		Payload:  payloadBytes,
	}

	err = syncer.applyEntryChange(ctx, change)
	require.NoError(t, err)

	// Verify tags were created
	rows, err := appDB.QueryContext(ctx,
		`SELECT tag FROM tags WHERE entry_id = ? ORDER BY tag`, entryID)
	require.NoError(t, err)
	defer rows.Close()

	var tags []string
	for rows.Next() {
		var tag string
		err = rows.Scan(&tag)
		require.NoError(t, err)
		tags = append(tags, tag)
	}

	assert.Equal(t, []string{"dev", "important", "work"}, tags)
}

func TestApplyEntryChangeReplaceTags(t *testing.T) {
	ctx := context.Background()
	syncer, appDB, cleanup := setupTestSyncerWithDB(t)
	defer cleanup()

	entryID := "entry-replace-tags"

	// Create entry with initial tags
	timestamp := time.Now()
	_, err := appDB.ExecContext(ctx,
		`INSERT INTO entries (id, timestamp, message, hostname, username, working_directory) VALUES (?, ?, ?, ?, ?, ?)`,
		entryID, timestamp, "test message", "test-host", "test-user", "/test/dir")
	require.NoError(t, err)

	_, err = appDB.ExecContext(ctx, `INSERT INTO tags (entry_id, tag) VALUES (?, ?)`, entryID, "old-tag1")
	require.NoError(t, err)
	_, err = appDB.ExecContext(ctx, `INSERT INTO tags (entry_id, tag) VALUES (?, ?)`, entryID, "old-tag2")
	require.NoError(t, err)

	// Apply update with new tags
	payload := map[string]any{
		"id":                entryID,
		"timestamp":         timestamp.Unix(),
		"message":           "test message",
		"hostname":          "test-host",
		"username":          "test-user",
		"working_directory": "/test/dir",
		"tags":              []any{"new-tag1", "new-tag2", "new-tag3"},
	}

	payloadBytes, err := json.Marshal(payload)
	require.NoError(t, err)

	change := vault.Change{
		Entity:   EntityEntry,
		EntityID: entryID,
		Op:       vault.OpUpsert,
		Payload:  payloadBytes,
	}

	err = syncer.applyEntryChange(ctx, change)
	require.NoError(t, err)

	// Verify tags were replaced
	rows, err := appDB.QueryContext(ctx,
		`SELECT tag FROM tags WHERE entry_id = ? ORDER BY tag`, entryID)
	require.NoError(t, err)
	defer rows.Close()

	var tags []string
	for rows.Next() {
		var tag string
		err = rows.Scan(&tag)
		require.NoError(t, err)
		tags = append(tags, tag)
	}

	assert.Equal(t, []string{"new-tag1", "new-tag2", "new-tag3"}, tags)
}

func TestApplyEntryChangeDeleteCascadesTags(t *testing.T) {
	ctx := context.Background()
	syncer, appDB, cleanup := setupTestSyncerWithDB(t)
	defer cleanup()

	entryID := "entry-cascade-tags"

	// Create entry with tags
	timestamp := time.Now()
	_, err := appDB.ExecContext(ctx,
		`INSERT INTO entries (id, timestamp, message, hostname, username, working_directory) VALUES (?, ?, ?, ?, ?, ?)`,
		entryID, timestamp, "test message", "test-host", "test-user", "/test/dir")
	require.NoError(t, err)

	_, err = appDB.ExecContext(ctx, `INSERT INTO tags (entry_id, tag) VALUES (?, ?)`, entryID, "tag1")
	require.NoError(t, err)
	_, err = appDB.ExecContext(ctx, `INSERT INTO tags (entry_id, tag) VALUES (?, ?)`, entryID, "tag2")
	require.NoError(t, err)

	// Apply delete
	change := vault.Change{
		Entity:   EntityEntry,
		EntityID: entryID,
		Op:       vault.OpDelete,
		Deleted:  true,
	}

	err = syncer.applyEntryChange(ctx, change)
	require.NoError(t, err)

	// Verify entry was deleted
	var entryCount int
	err = appDB.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM entries WHERE id = ?`, entryID).Scan(&entryCount)
	require.NoError(t, err)
	assert.Equal(t, 0, entryCount)

	// Verify tags were cascaded
	var tagCount int
	err = appDB.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM tags WHERE entry_id = ?`, entryID).Scan(&tagCount)
	require.NoError(t, err)
	assert.Equal(t, 0, tagCount)
}

func TestApplyChangeUnknownEntity(t *testing.T) {
	ctx := context.Background()
	syncer, _, cleanup := setupTestSyncerWithDB(t)
	defer cleanup()

	change := vault.Change{
		Entity:   "unknown-entity",
		EntityID: "some-id",
		Op:       vault.OpUpsert,
		Payload:  []byte("{}"),
	}

	// Should not error, just ignore
	err := syncer.applyChange(ctx, change)
	assert.NoError(t, err)
}

func TestApplyChangeEntry(t *testing.T) {
	ctx := context.Background()
	syncer, appDB, cleanup := setupTestSyncerWithDB(t)
	defer cleanup()

	entryID := "routed-entry"
	timestamp := time.Now().UTC().Unix()

	payload := map[string]any{
		"id":                entryID,
		"timestamp":         timestamp,
		"message":           "routed message",
		"hostname":          "test-host",
		"username":          "test-user",
		"working_directory": "/test/dir",
		"tags":              []any{},
	}

	payloadBytes, err := json.Marshal(payload)
	require.NoError(t, err)

	change := vault.Change{
		Entity:   EntityEntry,
		EntityID: entryID,
		Op:       vault.OpUpsert,
		Payload:  payloadBytes,
	}

	err = syncer.applyChange(ctx, change)
	require.NoError(t, err)

	// Verify entry was created
	var message string
	err = appDB.QueryRowContext(ctx,
		`SELECT message FROM entries WHERE id = ?`,
		entryID).Scan(&message)
	require.NoError(t, err)
	assert.Equal(t, "routed message", message)
}

func TestApplyEntryChangeInvalidPayload(t *testing.T) {
	ctx := context.Background()
	syncer, _, cleanup := setupTestSyncerWithDB(t)
	defer cleanup()

	change := vault.Change{
		Entity:   EntityEntry,
		EntityID: "invalid-entry",
		Op:       vault.OpUpsert,
		Payload:  []byte("invalid json"),
	}

	err := syncer.applyEntryChange(ctx, change)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unmarshal")
}

func TestApplyEntryChangeEmptyTags(t *testing.T) {
	ctx := context.Background()
	syncer, appDB, cleanup := setupTestSyncerWithDB(t)
	defer cleanup()

	entryID := "entry-empty-tags"
	timestamp := time.Now().UTC().Unix()

	payload := map[string]any{
		"id":                entryID,
		"timestamp":         timestamp,
		"message":           "test message",
		"hostname":          "test-host",
		"username":          "test-user",
		"working_directory": "/test/dir",
		"tags":              []any{},
	}

	payloadBytes, err := json.Marshal(payload)
	require.NoError(t, err)

	change := vault.Change{
		Entity:   EntityEntry,
		EntityID: entryID,
		Op:       vault.OpUpsert,
		Payload:  payloadBytes,
	}

	err = syncer.applyEntryChange(ctx, change)
	require.NoError(t, err)

	// Verify no tags were created
	var tagCount int
	err = appDB.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM tags WHERE entry_id = ?`, entryID).Scan(&tagCount)
	require.NoError(t, err)
	assert.Equal(t, 0, tagCount)
}

func TestApplyEntryChangeLongMessage(t *testing.T) {
	ctx := context.Background()
	syncer, appDB, cleanup := setupTestSyncerWithDB(t)
	defer cleanup()

	entryID := "long-message-entry"
	timestamp := time.Now().UTC().Unix()

	// Create long message
	longMessage := ""
	for i := 0; i < 1000; i++ {
		longMessage += "This is a test message. "
	}

	payload := map[string]any{
		"id":                entryID,
		"timestamp":         timestamp,
		"message":           longMessage,
		"hostname":          "test-host",
		"username":          "test-user",
		"working_directory": "/test/dir",
		"tags":              []any{},
	}

	payloadBytes, err := json.Marshal(payload)
	require.NoError(t, err)

	change := vault.Change{
		Entity:   EntityEntry,
		EntityID: entryID,
		Op:       vault.OpUpsert,
		Payload:  payloadBytes,
	}

	err = syncer.applyEntryChange(ctx, change)
	require.NoError(t, err)

	// Verify entry was created with long message
	var message string
	err = appDB.QueryRowContext(ctx,
		`SELECT message FROM entries WHERE id = ?`,
		entryID).Scan(&message)
	require.NoError(t, err)
	assert.Equal(t, longMessage, message)
}

func TestApplyEntryChangeSpecialCharacters(t *testing.T) {
	ctx := context.Background()
	syncer, appDB, cleanup := setupTestSyncerWithDB(t)
	defer cleanup()

	entryID := "special-chars-entry"
	timestamp := time.Now().UTC().Unix()

	payload := map[string]any{
		"id":                entryID,
		"timestamp":         timestamp,
		"message":           "Test with special chars: 日本語 🚀 ñ á é",
		"hostname":          "test-host-ñ",
		"username":          "test-user-日本",
		"working_directory": "/test/dir/🚀",
		"tags":              []any{"tag-日本", "tag-🚀"},
	}

	payloadBytes, err := json.Marshal(payload)
	require.NoError(t, err)

	change := vault.Change{
		Entity:   EntityEntry,
		EntityID: entryID,
		Op:       vault.OpUpsert,
		Payload:  payloadBytes,
	}

	err = syncer.applyEntryChange(ctx, change)
	require.NoError(t, err)

	// Verify entry was created with special characters
	var message, hostname, username, workingDir string
	err = appDB.QueryRowContext(ctx,
		`SELECT message, hostname, username, working_directory FROM entries WHERE id = ?`,
		entryID).Scan(&message, &hostname, &username, &workingDir)
	require.NoError(t, err)
	assert.Equal(t, "Test with special chars: 日本語 🚀 ñ á é", message)
	assert.Equal(t, "test-host-ñ", hostname)
	assert.Equal(t, "test-user-日本", username)
	assert.Equal(t, "/test/dir/🚀", workingDir)

	// Verify tags with special characters
	rows, err := appDB.QueryContext(ctx,
		`SELECT tag FROM tags WHERE entry_id = ? ORDER BY tag`, entryID)
	require.NoError(t, err)
	defer rows.Close()

	var tags []string
	for rows.Next() {
		var tag string
		err = rows.Scan(&tag)
		require.NoError(t, err)
		tags = append(tags, tag)
	}

	assert.Equal(t, []string{"tag-日本", "tag-🚀"}, tags)
}
