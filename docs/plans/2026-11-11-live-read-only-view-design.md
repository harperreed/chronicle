# Live Read-Only View Design

Date: 2026-11-11
Status: draft, waiting for Harper's approval

## Why

Harper's agents log their work to chronicle all day, through `chronicle mcp` and the CLI. The busiest day so far, Nov 7 2026, brought 639 entries. Harper wants to watch that feed as it happens, in a browser tab and in a terminal, without refreshing. The viewer must not be able to change the log, and it must never slow down or break an agent's write.

## Goals

1. `chronicle web --read-only` serves the web page with every way to write removed: no composer, no edit or delete buttons, and no write routes on the server.
2. New entries appear on the page within about a second of being written.
3. `chronicle tail` prints the newest entries, then each new entry as it lands, until Ctrl-C.
4. The viewer never blocks, slows or changes the store. A burst of agent writes never freezes, reloads or scrambles the view.
5. The page drops the Claude look and gets a light and a dark theme that follow the system setting.
6. Both storage backends (markdown and SQLite) work.

## Non-goals

- Showing edits and deletions live. Agents can only add entries (see "What exists today"). An edit made in the full page shows up after a reload.
- Remote access. The server binds to 127.0.0.1, which is Harper's decision. The existing `--allow-remote` flag stays as it is.
- Notifications, sounds, or unread counts in the tab title.
- Any change to how entries get written: the MCP server, `chronicle add`, and the file format stay as they are.

## Anti-goals

Any of these makes the work a failure, even if everything else works:

- The viewer takes the markdown `.lock`, opens a SQLite write transaction, or runs migrations. Each of those can stall an agent.
- The viewer re-reads every entry on every poll, so its cost grows with the size of the log.
- Read-only mode shows write controls, even for a moment while the page loads.
- The page jumps while Harper reads: entries inserted above must not move what is on screen.
- The page still looks like Claude: cream background, serif type, coral accent, the "C" tile.

## What exists today

- `chronicle web` (`internal/cli/web.go`, `internal/web/`) came from kola worker commit 4b851da on 2026-09-03. It sits on local `main` only: `origin/main` is two commits behind, and no release contains it. It offers add, edit and delete, refuses non-loopback Host headers unless `--allow-remote` is set, serves embedded static assets, and listens on 127.0.0.1:8080 by default. Its theme toggle stores `light` or `dark` under the localStorage key `chronicle-theme` and starts in light.
- The HTTP server sets `WriteTimeout: 30s`. That would cut a long-lived stream after 30 seconds.
- The markdown backend (Harper's config) writes each entry to a temp file and renames it into place while holding an exclusive flock on `<data_dir>/.lock`. Readers need no lock. Temp files are named `.tmp-*` and never end in `.md`. Entries live at `<data_dir>/YYYY/MM/DD/<slug>-<id part>.md`. The day folder comes from the entry's timestamp, so a backdated entry lands in an old folder. Editing an entry's message renames its file.
- The SQLite backend runs in WAL mode. `NewSqliteStore` runs its schema migration inside an immediate (write-locking) transaction on every open. Entries have an AUTOINCREMENT `rowid`.
- Agents can only add. The MCP tools are `add_entry`, `list_entries`, `search_entries`, `remember_this`, `what_was_i_doing` and `find_when_i`, and the CLI has no edit or delete command.
- Harper's installed chronicle is Homebrew 1.8.0, which has neither `web` nor `tail`. It writes the older file names, `<slug>-<first 8 characters of the id>.md`, while this repo writes `<slug>-<sha256 of the id>.md`. The viewer must read both.
- If the first word after `chronicle` isn't a subcommand, the CLI logs the words as a quick entry. Once `tail` exists, `chronicle tail ...` runs the command instead of logging "tail ...". `chronicle add "tail ..."` still logs it.

## Decisions

### Harper's decisions

- Both a browser view and a terminal command.
- Bind to 127.0.0.1 only.
- Layout A: today's page structure (top bar, header, filter sidebar, cards on a timeline rail) without the write controls.
- No cream and no Claude look. The light theme is "clean light" (white, cool grays, one blue accent). The dark theme is "dark console" (near-black, cool grays, one green accent). The default follows the system setting.
- The behavior played back in chat and approved ("that sounds good"): read-only, writes can't disturb the view, new entries stream within about a second, filters apply to the stream, and `tail` takes `-n` and `--tag`.

### Claude's calls (Harper can overrule any of them)

1. Read-only is a `--read-only` flag on `chronicle web`, not a second page or command. One set of assets serves both modes, and the server marks the page read-only before it reaches the browser.
2. The stream also runs in the full page. It's the same code, and there's no reason to hold it back.
3. The new look applies to the full page as well, since both modes share one stylesheet.
4. The theme button cycles Auto → Light → Dark. Auto follows the system and is the default. The choice goes under the existing `chronicle-theme` key as `auto`, `light` or `dark`, so a stored `light` or `dark` from today keeps working.
5. A `chronicle_` wordmark in monospace, with the underscore in the accent color, replaces the "C" tile. Nothing uses serif type. Text is sans-serif. Times, counts and paths are monospace. The page uses system fonts only, because its Content-Security-Policy forbids outside fonts.
6. In read-only mode the header reads "LIVE FEED" over "Agent activity", with the total count on the right. The top bar shows a "READ ONLY" badge and a LIVE marker. When the stream is down, the marker says so while the page reconnects.
7. A new entry gets a NEW tag and a tint. It keeps them until the page has been visible for 10 seconds after the entry arrived, so entries that land while the tab is hidden are still marked when Harper comes back.
8. The viewer polls once a second. On markdown it checks the day folders' modified times and reads only new `.md` files in folders that changed. On SQLite it asks for rows whose `rowid` is above the last one it saw.
9. The default port for `chronicle web` moves from 8080 to 1066, the best-known year in the Anglo-Saxon Chronicle. 8080 is the most common dev-server port, and on this Mac moshi-hook probes it whenever a listener opens there. No release contains `chronicle web`, so nothing depends on 8080. Port 1066 was free on 2026-11-11.

## Behavior

### Read-only mode (`chronicle web --read-only`)

- The server registers only reading routes: health, the entry list, a single entry, stats, the stream, and the static assets. POST, PUT and DELETE get 405.
- Storage opens read-only:
  - Markdown: never creates folders or files, and never takes the lock. If the data folder doesn't exist yet, the page shows an empty list and keeps watching.
  - SQLite: opens the database read-only and skips the migration and any pragma that writes. If the database file doesn't exist, the command exits with a clear error.
- On a first run, `config.Load()` writes a default config file, as it does for every command. That file is config, not the log, so leave it alone.
- The page never shows the composer, the Edit and Delete buttons, or the edit dialog, not even while it loads.

### The stream

- New entries reach every open page within about a second.
- Each connection carries the page's active filters: search text, tags and date range. The server sends only matching entries and uses the backend's own search rules for them. Markdown search matches a case-insensitive substring and SQLite uses FTS5. A live entry should appear exactly when it would appear after a reload.
- "New" means an entry ID the viewer hasn't seen before. An edit that renames a markdown file is not new. Deletions are ignored.
- Within one poll, entries arrive oldest first. The page puts each one where its timestamp belongs, which is usually the top.
- A file that fails to read or parse gets skipped, as `loadAllEntries` does today, and a half-read entry never appears. The temp-file-plus-rename writes mean a `.md` file is complete when it appears.
- The stream outlives the server's 30-second write timeout and ends promptly on Ctrl-C.
- If the connection drops, the page shows it and reconnects. After reconnecting it reloads the current list once to fill any gap.
- If Harper has scrolled down, an insert above must not move what's on screen. The script does this itself rather than relying on the browser's scroll anchoring.
- Polling costs the same no matter how many entries exist. Opening a page or reconnecting may read the whole store once.
- No read stays open between polls. On SQLite an open read would hold back WAL checkpoints.

### `chronicle tail`

- Usage: `chronicle tail [-n N] [--tag TAG]...`. It prints the newest N entries (default 10), oldest first, then follows. `--tag` can repeat and matches any of the tags given, like `search`. Ctrl-C or SIGTERM exits with status 0.
- Format, approved in chat:

  ```
  16:04:43  started spaceport kata backlog work  #spaceport #kata
            ~/Public/src/2389/spaceport
  ```

  Times are local. An entry from an earlier day gets the date too, as in `Nov 10 16:04:43`. The second line is the working directory, with the home folder shortened to `~`. Colors come from `fatih/color`, already a dependency, and turn off when output isn't a terminal or `NO_COLOR` is set. A message with more than one line keeps its later lines indented under the first.
- It opens storage read-only and finds new entries the same way the web view does.

### Look

- Layout A, as in the approved mockup, which sits next to this file as `2026-11-11-live-read-only-view-mockup.html`. It was drawn inside the brainstorming tool's frame, so opened on its own it renders only roughly. Treat the palette and structure as the spec and the pixel sizes as a sketch.
- The top bar holds the wordmark, the READ ONLY badge (read-only mode only), the LIVE marker and the theme button. Below it come the header row, the filter sidebar on the left, and the timeline of entry cards on the right. The mobile layout keeps today's breakpoints.
- Palette from the approved mockup:

  | Token | Light ("clean light") | Dark ("dark console") |
  |---|---|---|
  | background | `#f8fafc` | `#0b0f14` |
  | surface | `#ffffff` | `#111821` |
  | surface 2 | `#f1f5f9` | `#17202b` |
  | ink | `#0f172a` | `#e5edf5` |
  | muted | `#64748b` | `#8a99ab` |
  | line | `#e2e8f0` | `#1f2a37` |
  | accent | `#2563eb` | `#4ade80` |
  | live marker | `#16a34a` | `#4ade80` |

- The full page keeps its composer, edit dialog and toasts, restyled to match. Its hero text can stay as it is.

## Tests

Harper's standing rules call for unit, integration and end-to-end tests, with no mocks in end-to-end tests.

- Unit: the feed on each backend (a new file, temp files ignored, a new day, month or year folder, an edit that renames a file, a deleted file, a malformed file, filters); the read-only route table returning 405; the stream's message framing; `tail` formatting.
- Integration, with real stores in temp folders:
  - The markdown feed keeps working while the test holds `.lock` exclusively, which proves the viewer never takes it.
  - The SQLite feed reads while another connection holds a write transaction open.
  - Several concurrent writers add entries while the feed polls, and every entry shows up exactly once.
  - Opening SQLite read-only leaves the database file unchanged.
  - The stream runs over an `httptest` server backed by a real store.
- End-to-end, with the built binary, temp `XDG_DATA_HOME` and `XDG_CONFIG_HOME`, and real processes:
  - `chronicle tail` prints an entry that a separate `chronicle add` wrote, within 3 seconds.
  - `chronicle web --read-only --address 127.0.0.1:0 --open=false` streams an entry that `chronicle add` wrote, and answers POST, PUT and DELETE with 405.
  - A browser check with agent-browser (it drives Chrome for Testing; record the version). Check the page in light and dark with the system setting switched through `set media`. Confirm read-only mode has no write controls, an entry written from the CLI appears with the NEW mark, and the theme button cycles Auto, Light and Dark. Keep the screenshots as evidence.
- Gates: `make ci` (fmt, golangci-lint, tests, the MCP integration script) and `go test -race ./...`, which CI and the pre-commit hook both run.

## Docs

Update the README for `chronicle tail`, `chronicle web --read-only`, the new default port, and the theme behavior.

## Left to the builder

- Go API shapes and names: the feed, the read-only store, where each lives.
- Whether one poller serves every connection or each connection polls on its own.
- The stream's wire format. Server-Sent Events fit, and the CSP's `connect-src 'self'` already allows them. The builder picks event names and keep-alive comments.
- How the server marks the page read-only before it reaches the browser.
- CSS structure. Today's `app.css` is minified into a few long lines, and rewriting it in readable form is fine.
- The exact colors `tail` uses.

## Open after the build

- How Harper runs it: cut a release for Homebrew, or `make install` locally. The installed 1.8.0 has neither command.
- This branch starts from local `main`, which has two commits that `origin/main` lacks: the kola web UI. Push `main` before opening a pull request against GitHub, or merge locally.
