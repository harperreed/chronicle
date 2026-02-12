# Chronicle - Charm Removal Plan

## Charmbracelet Dependencies

**Direct:**
- `github.com/charmbracelet/charm` (replaced with 2389-research fork)

**Indirect (transitive):**
- bubbles, bubbletea, keygen, lipgloss, log, x/ansi, x/term

## Charm Usage by File

| File | Package | Usage |
|------|---------|-------|
| `internal/charm/entry.go` | `charm/kv` | `kv.DoReadOnly()`, `kv.Do()` |
| `internal/charm/config.go` | `charm/kv` | `kv.DefaultStaleThreshold` |
| `internal/charm/client.go` | `charm/client`, `charm/kv` | Client creation, sync, repair |
| `internal/cli/sync.go` | `charm/client`, `charm/proto` | Link/unlink, user management |

## Removal Strategy

### Phase 1: Create SQLite Storage Layer

**New package:** `internal/storage/`

```go
type Store interface {
    CreateEntry(entry Entry) (string, error)
    GetEntry(id string) (*Entry, error)
    UpdateEntry(entry Entry) error
    DeleteEntry(id string) error
    ListEntries(limit int) ([]Entry, error)
    SearchEntries(filter *SearchFilter, limit int) ([]Entry, error)
    LastModified() time.Time
    Close() error
}
```

**Database:** `~/.local/share/chronicle/chronicle.db`

**Schema:**
```sql
CREATE TABLE entries (
    rowid INTEGER PRIMARY KEY AUTOINCREMENT,  -- Required for FTS5 content_rowid
    id TEXT UNIQUE NOT NULL,
    timestamp DATETIME NOT NULL,
    message TEXT NOT NULL,
    hostname TEXT,
    username TEXT,
    working_directory TEXT,
    tags TEXT,  -- JSON array
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_entries_id ON entries(id);
CREATE INDEX idx_entries_timestamp ON entries(timestamp DESC);

-- FTS5 with external content (uses rowid for content sync)
CREATE VIRTUAL TABLE entries_fts USING fts5(
    message,
    tags,
    content=entries,
    content_rowid=rowid
);

-- Triggers to keep FTS in sync
CREATE TRIGGER entries_ai AFTER INSERT ON entries BEGIN
    INSERT INTO entries_fts(rowid, message, tags) VALUES (new.rowid, new.message, new.tags);
END;
CREATE TRIGGER entries_ad AFTER DELETE ON entries BEGIN
    INSERT INTO entries_fts(entries_fts, rowid, message, tags) VALUES('delete', old.rowid, old.message, old.tags);
END;
CREATE TRIGGER entries_au AFTER UPDATE ON entries BEGIN
    INSERT INTO entries_fts(entries_fts, rowid, message, tags) VALUES('delete', old.rowid, old.message, old.tags);
    INSERT INTO entries_fts(rowid, message, tags) VALUES (new.rowid, new.message, new.tags);
END;
```

### Phase 2: Remove Sync Commands

- `chronicle sync status` - Show local-only status
- `chronicle sync link` - **REMOVE**
- `chronicle sync unlink` - **REMOVE**
- `chronicle sync repair` - Simplify to SQLite VACUUM
- `chronicle sync reset` - Reset local DB only
- `chronicle sync wipe` - Delete local DB only

### Phase 3: Add Export Commands

```bash
chronicle export --format=markdown > backup.md
chronicle export --format=yaml > backup.yaml
chronicle export --format=json > backup.json
chronicle import backup.yaml
```

## Export Formats

### Markdown

```markdown
# Chronicle Export
Generated: 2025-01-31T12:00:00Z

---

## 2025-01-31

### 14:32:15 - deployed v2.1.0
- **ID**: abc12345-...
- **Tags**: deployment, production
- **Host**: MacBook-Pro
- **User**: harper
```

### YAML

```yaml
version: "1.0"
exported_at: "2025-01-31T12:00:00Z"
entries:
  - id: "abc12345-..."
    timestamp: "2025-01-31T14:32:15Z"
    message: "deployed v2.1.0"
    tags: [deployment, production]
    hostname: "MacBook-Pro"
    username: "harper"
```

## Files to Modify

### DELETE:
- `internal/charm/client.go`
- `internal/charm/entry.go`
- `internal/charm/config.go`
- `internal/charm/wal_test.go`

### CREATE:
- `internal/storage/sqlite.go`
- `internal/storage/entry.go`
- `internal/storage/migrations.go`
- `internal/cli/export.go`
- `internal/cli/import.go`
- `internal/export/markdown.go`
- `internal/export/yaml.go`
- `internal/export/json.go`

### MODIFY:
- `go.mod` - Remove charmbracelet/charm, add gopkg.in/yaml.v3
- `internal/cli/sync.go` - Major rewrite for local-only
- `internal/cli/list.go` - Update imports
- `internal/cli/add.go` - Update imports
- `internal/cli/search.go` - Update imports
- `internal/mcp/server.go` - Update imports
- `internal/mcp/tools.go` - Update imports

## Implementation Order

1. Create storage layer (`internal/storage/`)
2. Create export functionality (`internal/export/`)
3. Add CLI commands (export, import)
4. Update existing code to use new storage
5. Simplify sync.go to local-only
6. Delete `internal/charm/`
7. Update go.mod, run `go mod tidy`
8. Optional: Migration tool from Charm KV to SQLite
