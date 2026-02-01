---
name: chronicle
description: Activity and accomplishment logging - track what was done and when. Use when the user completes something significant, wants to log an activity, or asks what they did recently.
---

# chronicle - Activity Log

Timestamped activity logging with tags and full-text search. Track accomplishments, decisions, and work history.

## When to use chronicle

- User just completed something significant
- User asks "what did I do today/yesterday/this week?"
- User wants to log an accomplishment or decision
- User needs to recall when something happened

## Available MCP tools

| Tool | Purpose |
|------|---------|
| `mcp__chronicle__add_entry` | Log a new entry |
| `mcp__chronicle__list_entries` | Get recent entries |
| `mcp__chronicle__search_entries` | Search by text/tags/date |
| `mcp__chronicle__find_when_i` | Find when something happened |
| `mcp__chronicle__what_was_i_doing` | Recall recent context |
| `mcp__chronicle__remember_this` | Quick log with auto-tags |

## Common patterns

### Log an accomplishment
```
mcp__chronicle__add_entry(message="Deployed v2.0 to production", tags=["deployment", "milestone"])
```

### What did I do today?
```
mcp__chronicle__what_was_i_doing(timeframe="today")
```

### Find when something happened
```
mcp__chronicle__find_when_i(what="deploy the authentication service")
```

### Search with filters
```
mcp__chronicle__search_entries(text="refactor", tags=["backend"], since="2026-01-01")
```

### Quick remember
```
mcp__chronicle__remember_this(activity="Fixed the memory leak in the worker pool", context="Part of performance optimization sprint")
```

## Proactive usage

**Log automatically when the user:**
- Deploys code
- Fixes a bug
- Makes a decision
- Completes a significant task
- Solves a problem

## CLI commands (if MCP unavailable)

```bash
chronicle add "Did the thing"           # Log entry
chronicle add "Entry" --tag deploy      # With tags
chronicle list                          # Recent entries
chronicle search "query"                # Full-text search
chronicle export --format markdown      # Export
```

## Data location

`~/.local/share/chronicle/chronicle.db` (SQLite with FTS5, respects XDG_DATA_HOME)
