# Vault Sync Integration Design

E2E encrypted sync for chronicle using the vault library.

## Goals

- Sync activity logs across devices (laptop, desktop, server)
- Single user, same mnemonic everywhere
- Offline-first: local writes always succeed
- Graceful degradation when sync unavailable

## Entity Model

Single entity type: `entry`

```go
type EntryPayload struct {
    ID               string   `json:"id"`
    Timestamp        int64    `json:"timestamp"`
    Message          string   `json:"message"`
    Hostname         string   `json:"hostname"`
    Username         string   `json:"username"`
    WorkingDirectory string   `json:"working_directory"`
    Tags             []string `json:"tags"`
}
```

Tags embed in the payload rather than syncing as separate entities.

## Schema Migration

Migrate from `INTEGER` to `TEXT` (UUID) primary keys.

**Current:**
```sql
entries (id INTEGER PRIMARY KEY, ...)
tags (entry_id INTEGER REFERENCES entries, ...)
```

**New:**
```sql
entries (id TEXT PRIMARY KEY, ...)
tags (entry_id TEXT REFERENCES entries, ...)
```

Migration runs automatically on upgrade:
1. Create new tables with TEXT id columns
2. Copy data, generate UUIDs for existing entries
3. Drop old tables, rename new ones
4. Rebuild FTS5 index and triggers

## File Structure

```
chronicle/
├── internal/
│   ├── sync/
│   │   ├── config.go      # Config struct, Load/Save, paths
│   │   └── sync.go        # Syncer struct, queue/apply methods
│   ├── db/
│   │   ├── db.go          # Migration logic
│   │   └── entries.go     # TEXT id support
│   └── cli/
│       ├── sync.go        # Sync command tree
│       └── add.go         # Wire sync after local write
```

## Configuration

**Location:** `~/.config/chronicle/sync.json`

```go
type Config struct {
    Server       string `json:"server"`
    UserID       string `json:"user_id"`
    Token        string `json:"token"`
    RefreshToken string `json:"refresh_token,omitempty"`
    TokenExpires string `json:"token_expires,omitempty"`
    DerivedKey   string `json:"derived_key"`
    DeviceID     string `json:"device_id"`
    VaultDB      string `json:"vault_db"`
    AutoSync     bool   `json:"auto_sync"`
}
```

**Vault DB:** `~/.config/chronicle/vault.db`

**Server:** `https://api.storeusa.org`

## Commands

```
chronicle sync init      # Generate device ID, create config
chronicle sync login     # Auth + recovery phrase
chronicle sync status    # Show config, pending count
chronicle sync now       # Push/pull changes
chronicle sync pending   # List queued changes
chronicle sync logout    # Clear tokens
chronicle sync wipe      # Nuclear reset
```

## Mutation Integration

Local-first pattern from position codebase:

```go
func addEntry(ctx context.Context, message string, tags []string) error {
    // 1. Create locally (primary)
    entry := models.NewEntry(message, tags)
    if err := db.CreateEntry(dbConn, entry); err != nil {
        return err
    }

    // 2. Queue for sync (secondary, non-blocking)
    if err := queueEntryToSync(ctx, entry, vault.OpUpsert); err != nil {
        log.Printf("warning: sync queue failed: %v", err)
    }

    return nil
}
```

Sync errors warn but don't fail commands.

## Apply Handler

Receiving changes from other devices:

```go
func (s *Syncer) applyEntryChange(ctx context.Context, c vault.Change) error {
    if c.Op == vault.OpDelete || c.Deleted {
        _, err := s.appDB.ExecContext(ctx,
            `DELETE FROM entries WHERE id = ?`, c.EntityID)
        return err
    }

    var payload EntryPayload
    json.Unmarshal(c.Payload, &payload)

    // Upsert entry
    _, err := s.appDB.ExecContext(ctx, `
        INSERT INTO entries (id, timestamp, message, hostname, username, working_directory)
        VALUES (?, ?, ?, ?, ?, ?)
        ON CONFLICT(id) DO UPDATE SET message = excluded.message
    `, payload.ID, time.Unix(payload.Timestamp, 0), ...)

    // Replace tags
    s.appDB.ExecContext(ctx, `DELETE FROM tags WHERE entry_id = ?`, payload.ID)
    for _, tag := range payload.Tags {
        s.appDB.ExecContext(ctx,
            `INSERT INTO tags (entry_id, tag) VALUES (?, ?)`, payload.ID, tag)
    }

    return err
}
```

FTS5 triggers auto-update the search index.

## Error Handling

| Scenario | Behavior |
|----------|----------|
| No sync config | Silent no-op |
| Network failure | Queue locally, sync later |
| Auth expired | Warn, suggest `sync login` |
| Decrypt failure | Log context, suggest `sync wipe` |

## Scope

**In scope:**
- Entry upsert sync
- UUID migration
- All sync commands
- Auto-sync option

**Out of scope:**
- Delete command (add when needed)
- Tag-only sync entity
- Conflict resolution UI
