# Chronicle

A fast, lightweight CLI tool for logging timestamped messages with metadata.

## Features

- **Global SQLite database** - All entries stored in `~/.local/share/chronicle/chronicle.db`
- **Rich metadata** - Automatic capture of timestamp, hostname, username, working directory
- **Tagging** - Organize entries with multiple tags
- **Full-text search** - Fast FTS5-powered search
- **Project logs** - Optional per-project log files (markdown or JSON)
- **Natural date parsing** - Use "yesterday", "last week", or ISO dates
- **Multiple output formats** - Human-readable tables or JSON

## Installation

### From Source

**Important:** Chronicle requires the `sqlite_fts5` build tag for full-text search support.

```bash
git clone https://github.com/harper/chronicle
cd chronicle
go build -tags=sqlite_fts5 -o chronicle .
```

### Install with go install

```bash
go install -tags=sqlite_fts5 github.com/harper/chronicle@latest
```

> **Note:** The `-tags=sqlite_fts5` flag is required to enable SQLite FTS5 (Full-Text Search) support. Without this flag, the application will not compile or run correctly.

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
chronicle search --tag work --since "last week"
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
chronicle search --since yesterday --until today  # Date range
chronicle search "bug" --tag golang --json        # Combined with JSON
```

**Date formats:**
- Natural: `yesterday`, `today`, `"3 days ago"`, `"last week"`
- ISO: `2025-11-29`, `2025-11-29T14:30:00`

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
- `search_entries` - Search by text, tags, or dates

**High-Level Semantic Tools:**
- `remember_this` - Proactively log important information with smart tagging
- `what_was_i_doing` - Recall recent activities and context
- `find_when_i` - Find when you did something specific

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
1. Store the entry in the global database
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

Optional: `~/.config/chronicle/config.toml`

```toml
# Override database location
db_path = "/custom/path/chronicle.db"
```

## Database Schema

- **entries** - Main log entries with timestamp, message, metadata
- **tags** - Many-to-many tag relationships
- **entries_fts** - Full-text search virtual table (FTS5)

Query directly with sqlite3:
```bash
sqlite3 ~/.local/share/chronicle/chronicle.db "SELECT * FROM entries"
```

## Development

```bash
# Run tests
go test ./... -v

# Build (remember the build tag!)
go build -tags=sqlite_fts5 -o chronicle .

# Install locally
go install -tags=sqlite_fts5
```

## License

MIT
