# Chronicle - CLI Logging Tool Design

## Overview

Chronicle is a Go CLI tool for logging timestamped messages with metadata. It stores entries in a global SQLite database and optionally writes project-specific log files.

## Architecture

### Database Layer

SQLite database stored in `XDG_DATA_HOME/chronicle/chronicle.db` (defaults to `~/.local/share/chronicle/chronicle.db`).

**Schema:**

```sql
CREATE TABLE entries (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    timestamp DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    message TEXT NOT NULL,
    hostname TEXT NOT NULL,
    username TEXT NOT NULL,
    working_directory TEXT NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE tags (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    entry_id INTEGER NOT NULL,
    tag TEXT NOT NULL,
    FOREIGN KEY (entry_id) REFERENCES entries(id) ON DELETE CASCADE
);

CREATE INDEX idx_timestamp ON entries(timestamp);
CREATE INDEX idx_tags_entry ON tags(entry_id);
CREATE INDEX idx_tags_tag ON tags(tag);
CREATE VIRTUAL TABLE entries_fts USING fts5(message, content=entries);
```

**Key Design Choices:**
- Separate tags table enables fast tag filtering and multiple tags per entry
- FTS5 virtual table enables fast full-text search
- Indexed timestamp makes date range queries fast
- Automatic metadata capture: hostname, username, working_directory

### Configuration System

Two config levels:

1. **Global config**: `XDG_CONFIG_HOME/chronicle/config.toml`
   - Override database path
   - Global preferences

2. **Project config**: `.chronicle` file in project root
   ```toml
   local_logging = true
   log_dir = "logs"
   log_format = "markdown"  # or "json", "text"
   ```

### CLI Interface

Uses `cobra` or `urfave/cli` for subcommand structure.

**Commands:**

1. **Add** (default):
   ```bash
   chronicle "deployed v2.1.0"
   chronicle add "fixed auth bug"
   chronicle add "merged PR" --tag work --tag go
   ```

2. **List**:
   ```bash
   chronicle list
   chronicle list --limit 50
   chronicle list --json
   ```

3. **Search**:
   ```bash
   chronicle search "deployment"
   chronicle search --tag work
   chronicle search --since yesterday --until today
   chronicle search "bug" --tag golang --since "last week"
   chronicle search --json
   ```

**Date Parsing:**
- Natural language: `yesterday`, `today`, `"3 days ago"`, `"last week"`
- ISO formats: `2025-11-29`, `2025-11-29T14:30:00`
- Uses `github.com/tj/go-naturaldate` or `github.com/araddon/dateparse`

### Project-Specific Logging

**Detection**: Walks up directory tree looking for `.chronicle` file (stops at filesystem root or home directory).

**Behavior:**
1. Always writes to global SQLite database
2. If `.chronicle` exists with `local_logging = true`, also appends to project log file
3. Creates `log_dir` if needed
4. Uses atomic file operations for safe concurrent writes

**Log File Format** (markdown):
```markdown
## 14:32:15 - deployed v2.1.0
- **Tags**: work, deployment
- **User**: harper@MacBook-Pro
- **Directory**: /Users/harper/mobile-app/src
```

**File Naming**: `logs/2025-11-29.log` (one file per day)

## Error Handling

1. **Database errors**: Fail with clear message about XDG_DATA_HOME location
2. **Config errors**: Fail with parse error details if malformed
3. **Project logging errors**: Warn but don't fail (still write to global DB)
4. **Search errors**: Show helpful examples for invalid date formats
5. **Graceful degradation**: Fall back to LIKE queries if FTS5 unavailable

## Testing Strategy

**Unit Tests:**
- Config parsing (global and project)
- Date parsing (natural language + ISO)
- Database operations (CRUD)
- Project detection (directory walking)

**Integration Tests:**
- End-to-end command execution in temp directories
- SQLite database creation and queries
- Project log file writing

**Table-Driven Tests:**
- Date parsing variations
- Search query building

## Project Structure

```
chronicle/
├── cmd/chronicle/        # main entry point
├── internal/
│   ├── config/          # config parsing
│   ├── db/              # SQLite operations
│   ├── logging/         # project log file handling
│   └── cli/             # command handlers
├── go.mod
└── README.md
```

## Implementation Notes

- Database created automatically on first run
- Migrations handled via embedded SQL or `golang-migrate`
- All metadata captured automatically from system
- Project log files are optional enhancement; global DB is primary storage
