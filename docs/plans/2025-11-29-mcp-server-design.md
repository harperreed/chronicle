# Chronicle MCP Server Design

**Date:** 2025-11-29
**Status:** Approved
**Author:** AI + Harper

## Overview

Add an MCP (Model Context Protocol) server to chronicle, enabling AI assistants to log and retrieve user activities through a standardized interface. The server will be embedded as a subcommand in the chronicle CLI.

## Architecture

### Component Structure

```
internal/
├── mcp/
│   ├── server.go       # Main MCP server implementation
│   ├── tools.go        # Tool definitions and handlers
│   ├── resources.go    # Resource definitions and handlers
│   └── prompts.go      # Static prompt definitions
└── cli/
    └── mcp.go          # CLI command for 'chronicle mcp'
```

### Server Lifecycle

1. User runs `chronicle mcp` (or `chronicle mcp serve`)
2. Server initializes with stdio transport (MCP standard)
3. Registers tools (both low-level and high-level)
4. Registers resources (dynamic queries)
5. Registers prompts (static context)
6. Runs event loop listening on stdin/stdout

The server will reuse all existing `internal/db`, `internal/config`, and `internal/logging` packages.

### Dependencies

```bash
go get github.com/modelcontextprotocol/go-sdk
```

## Tool Definitions

### Low-Level Tools (CLI Mapping)

#### 1. `add_entry`
- **Description:** "Log a timestamped entry to chronicle. Use this when the user explicitly asks to log, track, or record something they did or are doing."
- **Input:**
  - `message` (string, required)
  - `tags` ([]string, optional)
- **Returns:** Entry ID, timestamp, confirmation
- **Maps to:** `chronicle add`

#### 2. `list_entries`
- **Description:** "Retrieve recent chronicle entries. Use this to answer questions like 'what did I do today/recently' or 'show my recent work'."
- **Input:**
  - `limit` (int, default 10)
- **Returns:** Array of entries with timestamps, messages, tags
- **Maps to:** `chronicle list`

#### 3. `search_entries`
- **Description:** "Search chronicle history by text, tags, or date range. Use this when the user wants to find specific past activities or recall when something happened."
- **Input:**
  - `text` (string, optional)
  - `tags` ([]string, optional)
  - `since` (string, optional)
  - `until` (string, optional)
  - `limit` (int, default 20)
- **Returns:** Matching entries sorted by relevance/recency
- **Maps to:** `chronicle search`

### High-Level Semantic Tools

#### 4. `remember_this`
- **Description:** "Proactively log important information the user shares about their work, decisions, or progress. Use this when you notice the user accomplished something worth tracking, even if they don't explicitly ask to log it. Automatically suggests relevant tags based on context."
- **Input:**
  - `activity` (string, required)
  - `context` (string, optional - why this matters)
- **Behavior:** Calls `add_entry` with smart tag suggestions (work, decision, bug-fix, deployment, etc.)

#### 5. `what_was_i_doing`
- **Description:** "Recall the user's recent activities and context. Use this at the start of conversations to understand what they've been working on, or when they ask 'what was I doing' or 'where did I leave off'."
- **Input:**
  - `timeframe` (string, default "today" - options: today, yesterday, this week, last N hours)
- **Behavior:** Calls `search_entries` with date filtering + formats as narrative summary

#### 6. `find_when_i`
- **Description:** "Find when the user did something specific. Use this to answer questions like 'when did I deploy X' or 'when did I fix that bug'."
- **Input:**
  - `what` (string - description of the activity)
- **Behavior:** Calls `search_entries` with smart query parsing

## Resources (Dynamic Query-able Context)

Resources are fetched on-demand when AI needs current information:

### 1. `chronicle://recent-activity`
- **Returns:** Last 10 entries with full metadata
- **Use case:** "What have I been working on?"
- **Format:** Structured JSON with timestamps, messages, tags

### 2. `chronicle://tags`
- **Returns:** All tags sorted by frequency with usage counts
- **Use case:** Understanding user's taxonomy, suggesting tags
- **Format:** `{"work": 45, "golang": 23, "debugging": 12, ...}`

### 3. `chronicle://today-summary`
- **Returns:** All entries from today grouped by tags
- **Use case:** Daily standup, end-of-day review
- **Format:** Narrative summary with time ranges

### 4. `chronicle://project-context`
- **Returns:** Current directory's .chronicle config if exists, plus recent project-specific logs
- **Use case:** Understanding project-specific logging setup
- **Format:** Config + recent project entries

## Prompts (Static Context)

### `chronicle-getting-started`

```
Chronicle is a personal activity logging system that helps you track what you're working on.

When to use chronicle:
- User accomplishes something worth remembering (deployed, fixed, decided, learned)
- User asks about past activities ("what did I do yesterday?")
- User wants to recall when something happened
- At start of work sessions to load context

Best practices:
- Log activities as they happen, not just when asked
- Use specific tags (work, personal, golang, debugging, deployment, etc.)
- Include enough detail to jog memory later
- Think of it as a work journal that can be searched

The user has configured chronicle to track their development work and important decisions.
```

## Error Handling

All MCP tools will follow this pattern:

- **Database errors:** Return clear error messages like "Failed to access chronicle database: {error}"
- **Invalid input:** Validate before calling DB, return "Invalid {field}: {reason}"
- **Empty results:** Not an error - return empty array with message "No entries found matching criteria"
- **Permission issues:** If DB file not accessible, guide user to check paths

Every tool uses the existing transaction-safe database code, inheriting all the N+1 query optimizations and proper error handling.

## Testing Strategy

### Unit Tests
- Mock database responses
- Verify input validation
- Check error cases

### Integration Tests
- Run actual MCP server over stdio
- Send tool requests via MCP client
- Verify responses match expected format

### Manual Testing
- Test with Claude Desktop MCP inspector
- Verify tool descriptions are compelling
- Test resource fetching

## Configuration

The MCP server will:
- Use same database path as CLI (`~/.local/share/chronicle/chronicle.db`)
- Respect `.chronicle` project configs
- Support optional `CHRONICLE_DB_PATH` env var for custom locations
- Run on stdio transport (standard for MCP)

## Implementation Files

New files to create:
- `internal/mcp/server.go` - Main server initialization
- `internal/mcp/tools.go` - Tool handlers (all 6 tools)
- `internal/mcp/resources.go` - Resource handlers (all 4 resources)
- `internal/mcp/prompts.go` - Static prompt definitions
- `internal/cli/mcp.go` - CLI command `chronicle mcp`
- `internal/mcp/server_test.go` - Tests

## Success Criteria

- AI assistants can successfully log user activities
- Tool descriptions are compelling enough that AI uses them proactively
- Resources provide accurate, up-to-date context
- Server runs stably over stdio transport
- All tests pass
- Works with Claude Desktop and other MCP clients
