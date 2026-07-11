# Chronicle

A fast, lightweight CLI tool for logging timestamped messages with metadata.

## Features

- **Dual storage backends** - SQLite with FTS5 search, or markdown files with YAML frontmatter
- **Rich metadata** - Automatic capture of timestamp, hostname, username, working directory
- **Tagging** - Organize entries with multiple tags
- **Full-text search** - Fast FTS5-powered search (SQLite backend)
- **Project logs** - Optional per-project log files (markdown or JSON)
- **Absolute date filtering** - Use date-only ISO values or timestamps
- **Multiple output formats** - Human-readable tables or JSON
- **Data migration** - Convert between backends with `chronicle migrate`

## Installation

### From Source

```bash
git clone https://github.com/harper/chronicle
cd chronicle
go build -o chronicle .
```

### Install with go install

```bash
go install github.com/harper/chronicle@latest
```

## Quick Start

```bash
# Add an entry (quick form)
chronicle "deployed version 2.1.0"

# Add with tags
chronicle "fixed auth bug" --tag work --tag golang

# List recent entries
chronicle list

# Search
chronicle search "deployment"
chronicle search --tag work --since 2026-01-01
```

## Commands

### Add Entry

```bash
chronicle "message"                      # Quick form
chronicle add "message"                  # Explicit form
chronicle add "message" --tag work -t go # With tags
```

### List Entries

```bash
chronicle list                 # Recent 20 entries
chronicle list --limit 50      # Show more
chronicle list --json          # JSON output
```

### Search

```bash
chronicle search "keyword"                        # Full-text search
chronicle search --tag work                       # By tag
chronicle search --since 2025-11-01 --until 2025-12-01  # Date range
chronicle search "bug" --tag golang --json        # Combined with JSON
```

**Date formats:**
- Date-only ISO: `2025-11-29` (midnight UTC)
- ISO timestamp: `2025-11-29T14:30:00Z`

Relative phrases such as `yesterday` and `last week` are not supported.

### Export

```bash
chronicle export                          # YAML (default)
chronicle export --format markdown        # Markdown
chronicle export --format json            # JSON
```

### Setup & Migration

```bash
chronicle setup                           # Interactive backend configuration
chronicle migrate --to markdown           # Migrate data to markdown backend
chronicle migrate --to sqlite             # Migrate data to SQLite backend
```

### Install Claude Code Skill

```bash
chronicle install-skill                   # Install chronicle skill for Claude Code
```

## MCP Server

Chronicle includes an MCP (Model Context Protocol) server that allows AI assistants to interact with your activity log.

### Running the MCP Server

```bash
# Run the MCP server (stdio transport)
chronicle mcp
```

### Configuring with Claude Desktop

Add to your Claude Desktop MCP settings (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "chronicle": {
      "command": "/path/to/chronicle",
      "args": ["mcp"]
    }
  }
}
```

### Available Tools

**Low-Level Tools:**
- `add_entry` - Log a new entry
- `list_entries` - Retrieve recent entries
- `search_entries` - Search by text or tags

**High-Level Semantic Tools:**
- `remember_this` - Proactively log important information with smart tagging
- `what_was_i_doing` - Recall recent activities and context
- `find_when_i` - Find when you did something specific

`search_entries` accepts `since` and `until`, but currently ignores them. `what_was_i_doing` likewise accepts but ignores `timeframe` and returns up to 20 recent entries. Use the CLI's `search --since/--until` flags when date filtering is required.

### Available Resources

- `chronicle://recent-activity` - Last 10 entries
- `chronicle://tags` - Tag usage statistics
- `chronicle://today-summary` - Today's activity summary
- `chronicle://project-context` - Current project's chronicle config

### Available Prompts

- `chronicle-getting-started` - Introduction to using chronicle with AI

## Project-Specific Logs

Enable local log files for a project by creating `.chronicle`:

```toml
local_logging = true
log_dir = "logs"
log_format = "markdown"  # or "json"
```

When you run `chronicle add` from anywhere in the project, it will:
1. Store the entry in the configured global storage
2. Append to `logs/YYYY-MM-DD.log` in the project root

Example markdown log entry:
```markdown
## 14:32:15 - deployed v2.1.0
- **Tags**: work, deployment
- **User**: harper@MacBook-Pro
- **Directory**: /Users/harper/mobile-app/src
```

## Configuration

### Global Config

Optional: `$XDG_CONFIG_HOME/chronicle/config.json` when `XDG_CONFIG_HOME` is set; otherwise `~/.config/chronicle/config.json`.

```json
{
  "backend": "markdown",
  "data_dir": "~/.local/share/chronicle"
}
```

- `backend` - Storage backend: `"sqlite"` or `"markdown"` (new installs default to markdown)
- `data_dir` - Root directory for data storage (defaults to `$XDG_DATA_HOME/chronicle` when set; otherwise `~/.local/share/chronicle`)

Run `chronicle setup` to configure interactively.

### Storage Backends

**SQLite** stores all entries in `<data_dir>/chronicle.db` with FTS5 full-text search.

**Markdown** stores each entry as a separate markdown file with YAML frontmatter, organized by date: `<data_dir>/YYYY/MM/DD/<slug>-<id-digest>.md`.

## Database Schema (SQLite backend)

- **entries** - Main log entries with timestamp, message, metadata, and tags (JSON array)
- **entries_fts** - Full-text search virtual table (FTS5)

Query directly with sqlite3:
```bash
sqlite3 ~/.local/share/chronicle/chronicle.db "SELECT * FROM entries"
```

## Development

```bash
# Run tests
go test ./... -v

# Build
go build -o chronicle .

# Install locally
go install
```

## License

MIT
