# Documentation Audit Report

Generated: 2026-02-11 | Commit: 2dd9644

## Executive Summary

| Metric | Count |
|--------|-------|
| Documents scanned | 4 |
| Claims verified | ~55 |
| Verified TRUE | ~40 (73%) |
| **Verified FALSE** | **15 (27%)** |

### Documents Audited

| Document | Status |
|----------|--------|
| `README.md` | Multiple inaccuracies |
| `CHARM_REMOVAL_PLAN.md` | Stale/completed - should be archived |
| `.claude/skills/chronicle/SKILL.md` | Minor issue |
| `internal/cli/skill/SKILL.md` | Minor issue |
| `Makefile` | One dead reference |

---

## False Claims Requiring Fixes

### README.md

| Line | Claim | Reality | Severity | Fix |
|------|-------|---------|----------|-----|
| 7 | "Global SQLite database" only | Project now supports **both** SQLite and markdown backends (config-selected) | Medium | Document the markdown backend and config-based backend selection |
| 19-33 | `-tags=sqlite_fts5` is **required** ("will not compile or run correctly") | Using `modernc.org/sqlite` (pure Go) which includes FTS5 by default. Tests pass without the tag. | High | Remove or soften the requirement. The tag is harmless but not required with this driver. |
| 22 | `git clone https://github.com/harper/chronicle` | Unverified URL. May not be publicly accessible at this path. | Low | Verify repo is public or update URL |
| 30 | `go install -tags=sqlite_fts5 github.com/harper/chronicle@latest` | Same URL concern as above | Low | Verify |
| 158-163 | Config file: `~/.config/chronicle/config.toml` with TOML format and `db_path` key | Actual config is `~/.config/chronicle/config.json` (JSON format) with `data_dir` and `backend` keys | **High** | Update to show JSON format with correct keys |
| 167 | Schema includes "tags" table | No separate `tags` table exists. Tags are stored as JSON array in `entries.tags` column. | Medium | Change "tags - Many-to-many tag relationships" to "tags stored as JSON array in entries table" |
| 173 | `sqlite3 ~/.local/share/chronicle/chronicle.db "SELECT * FROM entries"` | Only works if backend is sqlite (not markdown). Path is correct for sqlite backend. | Low | Add note about backend-dependent behavior |

### README.md - Missing Documentation

| Topic | Reality | Fix |
|-------|---------|-----|
| Markdown storage backend | Full implementation exists at `internal/storage/markdown.go`. Entries stored as markdown files with YAML frontmatter in `<dataDir>/YYYY/MM/DD/` | Add section documenting the markdown backend |
| `chronicle setup` command | TUI setup wizard exists (`internal/tui/setup.go`) | Document the setup command |
| `chronicle migrate` command | Migration command exists (`internal/cli/migrate.go`) | Document the migrate command |
| `chronicle install-skill` command | Skill installation exists (`internal/cli/skill.go`) | Document the install-skill command |
| `config.json` keys | Backend selection (`"backend": "sqlite"` or `"backend": "markdown"`) and `data_dir` | Document actual config structure |

### CHARM_REMOVAL_PLAN.md

| Line | Claim | Reality | Severity | Fix |
|------|-------|---------|----------|-----|
| 13-18 | Files to DELETE listed (internal/charm/*) | All files **already deleted** - plan is complete | Info | Archive or delete this document |
| 82-87 | `chronicle sync` commands exist | Sync command **fully removed**, not simplified | Info | Plan is stale |
| 91-96 | `chronicle export/import` commands planned | Export exists but `chronicle import` was **never implemented** | Medium | Either implement import or remove from plan |
| 139-147 | Files to CREATE listed | Created with different names: `sqlite.go` → `store.go`, `migrations.go` → `migrate.go`, separate format files → single `export.go` | Info | Plan is stale |

### SKILL.md (both copies)

| Line | Claim | Reality | Severity | Fix |
|------|-------|---------|----------|-----|
| 67 | `chronicle export --format markdown` | Command works correctly | OK | - |
| 76 | Data at `~/.local/share/chronicle/chronicle.db` | Only true for sqlite backend. Markdown backend uses `~/.local/share/chronicle/YYYY/` directory structure | Low | Add note about backend-dependent paths |

### Makefile

| Line | Claim | Reality | Severity | Fix |
|------|-------|---------|----------|-----|
| 72 | `CHRONICLE_DB_PATH=/path/to/db` env var override | **Not implemented in code**. Config uses `data_dir` in `config.json` instead. | Medium | Remove dead env var reference or implement it |

---

## Pattern Summary

| Pattern | Count | Root Cause |
|---------|-------|------------|
| Config format mismatch (TOML vs JSON) | 1 | Config migrated from TOML to JSON; README not updated |
| Stale charm removal docs | 5 | CHARM_REMOVAL_PLAN completed but document not archived |
| Missing new feature docs | 4 | New features (markdown backend, setup, migrate, install-skill) added without README updates |
| Build tag over-claim | 2 | SQLite driver changed from CGo to pure-Go; build tag no longer required |
| Dead env var reference | 1 | Config approach changed; Makefile not updated |
| Planned-but-unimplemented features | 1 | `chronicle import` in plan but never built |

---

## Human Review Queue

- [ ] Line 22, 30 README: Verify `github.com/harper/chronicle` is a valid public GitHub URL
- [ ] Decide whether `CHARM_REMOVAL_PLAN.md` should be archived (moved to `docs/plans/`) or deleted
- [ ] Decide whether to implement `chronicle import` or remove it from plans
- [ ] Verify whether `-tags=sqlite_fts5` should be kept for safety or removed (works without it using `modernc.org/sqlite`)

---

## Verified TRUE Claims (for reference)

### README.md
- CLI commands: `chronicle add`, `chronicle list`, `chronicle search`, `chronicle mcp`, `chronicle export` all exist and work as documented
- Quick form `chronicle "message"` works (injects `add` command)
- All flags documented (`--tag/-t`, `--limit/-n`, `--json`, `--since`, `--until`, `--format/-f`) exist
- MCP server: All 6 tools, 4 resources, 1 prompt exist exactly as documented
- MCP uses stdio transport
- Natural date parsing works via `araddon/dateparse`
- `.chronicle` project config with TOML format works correctly
- Project local logging (markdown/JSON) works
- Rich metadata capture (hostname, username, working_directory) works
- Default data path `~/.local/share/chronicle/` with XDG_DATA_HOME support works
- FTS5 full-text search works

### SKILL.md
- All MCP tool names and signatures are accurate
- CLI commands listed are accurate
