// ABOUTME: Vault sync integration for chronicle
// ABOUTME: Handles change queuing, syncing, and applying remote changes
package sync

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"suitesync/vault"

	"github.com/harper/chronicle/internal/db"
)

// Op represents a sync operation type.
type Op = vault.Op

// Operation constants.
const (
	OpUpsert = vault.OpUpsert
	OpDelete = vault.OpDelete
)

const (
	EntityEntry = "entry"
	// AppID is the stable UUID for chronicle app namespace isolation
	AppID = "8ef3529f-0978-4a10-ab4a-b9a960d6ffff"
)

// Syncer manages vault sync for chronicle data.
type Syncer struct {
	config *Config
	store  *vault.Store
	keys   vault.Keys
	client *vault.Client
	appDB  *sql.DB
}

// NewSyncer creates a new syncer from config.
func NewSyncer(cfg *Config, appDB *sql.DB) (*Syncer, error) {
	if cfg.DerivedKey == "" {
		return nil, errors.New("derived key not configured - run 'chronicle sync login' first")
	}

	// DerivedKey is stored as hex-encoded seed
	seed, err := vault.ParseSeedPhrase(cfg.DerivedKey)
	if err != nil {
		return nil, fmt.Errorf("invalid derived key: %w", err)
	}

	keys, err := vault.DeriveKeys(seed, "", vault.DefaultKDFParams())
	if err != nil {
		return nil, fmt.Errorf("derive keys: %w", err)
	}

	store, err := vault.OpenStore(cfg.VaultDB)
	if err != nil {
		return nil, fmt.Errorf("open vault store: %w", err)
	}

	client := vault.NewClient(vault.SyncConfig{
		AppID:     AppID,
		BaseURL:   cfg.Server,
		DeviceID:  cfg.DeviceID,
		AuthToken: cfg.Token,
	})

	return &Syncer{
		config: cfg,
		store:  store,
		keys:   keys,
		client: client,
		appDB:  appDB,
	}, nil
}

// Close releases syncer resources.
func (s *Syncer) Close() error {
	if s.store != nil {
		return s.store.Close()
	}
	return nil
}

// EntryPayload is the sync payload for an entry.
type EntryPayload struct {
	ID               string   `json:"id"`
	Timestamp        int64    `json:"timestamp"`
	Message          string   `json:"message"`
	Hostname         string   `json:"hostname"`
	Username         string   `json:"username"`
	WorkingDirectory string   `json:"working_directory"`
	Tags             []string `json:"tags"`
}

// QueueEntryChange queues a change for an entry.
func (s *Syncer) QueueEntryChange(ctx context.Context, entry db.Entry, op Op) error {
	var payload map[string]any
	if op != OpDelete {
		payload = map[string]any{
			"id":                entry.ID,
			"timestamp":         entry.Timestamp.Unix(),
			"message":           entry.Message,
			"hostname":          entry.Hostname,
			"username":          entry.Username,
			"working_directory": entry.WorkingDirectory,
			"tags":              entry.Tags,
		}
	}

	return s.queueChange(ctx, EntityEntry, entry.ID, op, payload)
}

func (s *Syncer) queueChange(ctx context.Context, entity, entityID string, op Op, payload map[string]any) error {
	change, err := vault.NewChange(entity, entityID, op, payload)
	if err != nil {
		return fmt.Errorf("create change: %w", err)
	}
	if op == OpDelete {
		change.Deleted = true
	}

	plain, err := json.Marshal(change)
	if err != nil {
		return fmt.Errorf("marshal change: %w", err)
	}

	aad := change.AAD(s.config.UserID, s.config.DeviceID)
	env, err := vault.Encrypt(s.keys.EncKey, plain, aad)
	if err != nil {
		return fmt.Errorf("encrypt change: %w", err)
	}

	if err := s.store.EnqueueEncryptedChange(ctx, change, s.config.UserID, s.config.DeviceID, env); err != nil {
		return fmt.Errorf("enqueue change: %w", err)
	}

	// Auto-sync if enabled
	if s.config.AutoSync && s.canSync() {
		return s.Sync(ctx)
	}

	return nil
}

func (s *Syncer) canSync() bool {
	return s.config.Server != "" && s.config.Token != "" && s.config.UserID != ""
}

// Sync pushes local changes and pulls remote changes.
func (s *Syncer) Sync(ctx context.Context) error {
	return s.SyncWithEvents(ctx, nil)
}

// SyncWithEvents pushes local changes and pulls remote changes with progress callbacks.
func (s *Syncer) SyncWithEvents(ctx context.Context, events *vault.SyncEvents) error {
	if !s.canSync() {
		return errors.New("sync not configured - run 'chronicle sync login' first")
	}

	return vault.Sync(ctx, s.store, s.client, s.keys, s.config.UserID, s.applyChange, events)
}

// applyChange applies a remote change to the local database.
func (s *Syncer) applyChange(ctx context.Context, c vault.Change) error {
	switch c.Entity {
	case EntityEntry:
		return s.applyEntryChange(ctx, c)
	default:
		// Ignore unknown entities for forward compatibility
		return nil
	}
}

func (s *Syncer) applyEntryChange(ctx context.Context, c vault.Change) error {
	if c.Op == OpDelete || c.Deleted {
		// Delete entry (tags cascade)
		_, err := s.appDB.ExecContext(ctx, `DELETE FROM entries WHERE id = ?`, c.EntityID)
		return err
	}

	var payload EntryPayload
	if err := json.Unmarshal(c.Payload, &payload); err != nil {
		return fmt.Errorf("unmarshal entry payload: %w", err)
	}

	timestamp := time.Unix(payload.Timestamp, 0)

	// Upsert entry
	_, err := s.appDB.ExecContext(ctx, `
		INSERT INTO entries (id, timestamp, message, hostname, username, working_directory)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			message = excluded.message,
			hostname = excluded.hostname,
			username = excluded.username,
			working_directory = excluded.working_directory
	`, payload.ID, timestamp, payload.Message, payload.Hostname, payload.Username, payload.WorkingDirectory)
	if err != nil {
		return fmt.Errorf("upsert entry: %w", err)
	}

	// Replace tags: delete existing, insert new
	_, err = s.appDB.ExecContext(ctx, `DELETE FROM tags WHERE entry_id = ?`, payload.ID)
	if err != nil {
		return fmt.Errorf("delete old tags: %w", err)
	}

	for _, tag := range payload.Tags {
		_, err = s.appDB.ExecContext(ctx, `INSERT INTO tags (entry_id, tag) VALUES (?, ?)`, payload.ID, tag)
		if err != nil {
			return fmt.Errorf("insert tag: %w", err)
		}
	}

	return nil
}

// PendingCount returns the number of changes waiting to be synced.
func (s *Syncer) PendingCount(ctx context.Context) (int, error) {
	batch, err := s.store.DequeueBatch(ctx, 1000)
	if err != nil {
		return 0, err
	}
	return len(batch), nil
}

// PendingItem represents a change waiting to be synced.
type PendingItem struct {
	ChangeID string
	Entity   string
	TS       time.Time
}

// PendingChanges returns details of changes waiting to be synced.
func (s *Syncer) PendingChanges(ctx context.Context) ([]PendingItem, error) {
	batch, err := s.store.DequeueBatch(ctx, 100)
	if err != nil {
		return nil, err
	}

	items := make([]PendingItem, len(batch))
	for i, b := range batch {
		items[i] = PendingItem{
			ChangeID: b.ChangeID,
			Entity:   b.Entity,
			TS:       time.Unix(b.TS, 0),
		}
	}
	return items, nil
}

// LastSyncedSeq returns the last pulled sequence number.
func (s *Syncer) LastSyncedSeq(ctx context.Context) (string, error) {
	return s.store.GetState(ctx, "last_pulled_seq", "0")
}
