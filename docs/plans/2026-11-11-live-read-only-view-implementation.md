# Live Read-Only View Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Let Harper watch agents write to chronicle live, through `chronicle web --read-only` and `chronicle tail`, without the viewer ever slowing or changing the log.

**Architecture:** Each storage backend gains a `Feed` that reports entries added after it opens: markdown checks day-folder modified times and parses only new files, and SQLite reads rows past a rowid cursor. A read-only `Reader` interface and read-only openers keep the viewer away from locks, migrations and folder creation. The web server streams feed results over Server-Sent Events at `GET /api/stream`. A `--read-only` flag serves only reading routes and marks the page, and `chronicle tail` prints feed results in the terminal. The page gets a new light/dark theme with system default.

**Tech Stack:** Go (stdlib `net/http`, `database/sql`, modernc.org/sqlite, cobra, fatih/color), plain browser JavaScript with no build step, `node --test` for the JavaScript helpers.

**Spec:** `docs/plans/2026-11-11-live-read-only-view-design.md` (approved 2026-11-11). Mockup: `docs/plans/2026-11-11-live-read-only-view-mockup.html`.

## Global Constraints

- Go code must build with the language and stdlib of go.mod's `go 1.24.2`. Don't use `sync.WaitGroup.Go` or `testing/synctest`, which are Go 1.25 additions. The local toolchain is Go 1.26.6, so the compiler will not catch this.
- No new Go module dependencies, and no npm dependencies. JavaScript tests use Node's built-in `node:test` and `node:assert`.
- Every new hand-written file starts with two `ABOUTME:` comment lines (`//` in Go and JS, `/* */` in CSS, `<!-- -->` in HTML). Comments end with a period, because golangci-lint's `godot` checks that.
- The viewer never takes `<data_dir>/.lock`, never calls `db.Begin()` on a read path (the read-write DSN sets `_txlock=immediate`, so `Begin` takes the write lock), never runs migrations, and never creates folders or files in the data directory.
- Poll interval: 1 second, for the web stream and for `tail`.
- `chronicle web` default address: `127.0.0.1:1066`.
- Stream route: `GET /api/stream`. The first line is `retry: 3000`, then `event: ready` once the feed is open, then one `event: entry` per new entry, whose `data:` is the entry's JSON on one line, in the same shape as `/api/entries`.
- The theme is stored in localStorage under `chronicle-theme` as `auto`, `light` or `dark`. A missing or unknown value means `auto`.
- Palette, copied from the spec:

  | Token | Light | Dark |
  |---|---|---|
  | `--bg` | `#f8fafc` | `#0b0f14` |
  | `--surface` | `#ffffff` | `#111821` |
  | `--surface2` | `#f1f5f9` | `#17202b` |
  | `--ink` | `#0f172a` | `#e5edf5` |
  | `--muted` | `#64748b` | `#8a99ab` |
  | `--line` | `#e2e8f0` | `#1f2a37` |
  | `--accent` | `#2563eb` | `#4ade80` |
  | `--live` | `#16a34a` | `#4ade80` |

- System fonts only: sans-serif for text, `ui-monospace, "SF Mono", Menlo, monospace` for times, counts, paths and the wordmark. No serif and no `Georgia` anywhere.
- A NEW mark clears once the page has been visible for 10 seconds after the entry arrived.
- Tests never touch the real `~/.local/share/chronicle` or `~/.config/chronicle`. Every test uses temp folders, and every subprocess gets temp `HOME`, `XDG_DATA_HOME` and `XDG_CONFIG_HOME`.
- **Executor notes (this machine):** the session runs inside the worktree `/Users/harper/Public/src/personal/suite/chronicle/.claude/worktrees/live-view`, under a guard that refuses complex shell lines that mention git. Run one plain command per Bash call, with literal paths, from the worktree root. Don't `cd` elsewhere. The guard also refuses `agent-browser eval`; use `snapshot`, `get`, `click`, `scroll` and `screenshot <absolute path>` instead. Commit after each task with plain `git add <paths>` and `git commit -m ...` calls. Use conventional commit messages, and never pass `--no-verify`.

## Review Focus

These failure modes follow from the spec, but no task's main tests would catch them. Each line names the test that now covers it.

1. **Coarse folder timestamps.** On a filesystem with 1-second modified times, a second write in the same second leaves the folder's time unchanged, and the feed would miss the entry. Rule: re-list any day folder whose modified time is within 2 seconds of now. Test: `TestMarkdownFeedRelistsRecentFolderWithUnchangedModTime` (Task 1).
2. **Two open tabs.** Two stream connections at once must both get every entry. Test: `TestStreamServesTwoClients` (Task 4).
3. **Server restart while the page is open.** The page shows it's reconnecting, and an entry written during the outage shows up afterward. Test: the restart step of the browser check (Task 9). The server side is pinned by `TestStreamSendsReadyBeforeEntries` (Task 4).
4. **Messages containing HTML.** A live-inserted entry must render as text and never as markup. Test: the HTML-message step of the browser check (Task 9), plus Task 7's rule that cards are built only with `textContent`.
5. **Timestamps in another zone.** Entries written with a UTC or other non-local timestamp must still show up in the feed and print in local time in `tail`. Tests: `TestMarkdownFeedSeesNewDayMonthYearFolders` uses UTC timestamps (Task 1), and `TestFormatTailEntryConvertsToLocalTime` (Task 8).

---

### Task 1: Markdown feed

**Files:**
- Modify: `internal/storage/interface.go` (add `Feed`)
- Create: `internal/storage/markdown_feed.go`
- Test: `internal/storage/markdown_feed_test.go`, `internal/storage/markdown_feed_unix_test.go` (`//go:build darwin || linux`)

**Interfaces:**
- Produces:
  - `type Feed interface { Poll() ([]Entry, error) }`. `Poll` returns entries that appeared since the previous call, or since the feed opened, that match the feed's filter, oldest first by `Timestamp`. It returns `nil` when there's nothing new.
  - `func (s *MarkdownStore) OpenFeed(filter *SearchFilter) (Feed, error)`. A nil filter matches everything.
  - Unexported: `newMarkdownFeed(dataDir string, filter *SearchFilter) (*markdownFeed, error)` and `const markdownRecentFolderWindow = 2 * time.Second`.

**The algorithm.** The test names don't determine it, so here it is:

- Day folders are exactly `<dataDir>/YYYY/MM/DD`: three levels of all-digit names, 4, 2 and 2 characters long. Anything else gets skipped.
- State: `folderModTimes map[string]time.Time`, `seenPaths map[string]bool` and `seenIDs map[string]bool`.
- Open: walk every day folder. Record its modified time before listing it, then parse every `*.md` file and record its path and ID. Record the paths of files that fail to parse as well, so they never count as new. A missing `dataDir` gives an empty state and no error.
- Poll: list the day folders again, using `os.ReadDir` per level and `Stat` per day folder. Skip a folder if its modified time equals the recorded one and is older than `markdownRecentFolderWindow`. Otherwise:
  1. Record the new modified time.
  2. `ReadDir` the folder.
  3. For each `*.md` name not in `seenPaths`, parse it with `parseEntryFile`.
  4. On a parse or read error, skip the file without recording it, so it's retried the next time the folder changes.
  5. If the ID is already in `seenIDs`, the file is an edit that renamed it: record the path and skip it.
  6. Otherwise record the path and ID, and keep the entry if `matchesFilter(entry, filter)`.

  Sort the kept entries by `Timestamp` ascending. A missing `dataDir` returns `nil, nil`.
- Never take the lock, and never create anything.

- [ ] **Step 1: Write the failing tests** in `markdown_feed_test.go`, using `newTestMarkdownStore(t)` for the writer:
  - `TestMarkdownFeedReportsOnlyEntriesAddedAfterOpen`: create 2 entries, open the feed. The first `Poll` returns 0 entries. Create one more, and `Poll` returns exactly that ID. The next `Poll` returns 0.
  - `TestMarkdownFeedOrdersOldestFirst`: after open, create the entries at 10:02 and then 10:01. `Poll` returns `[10:01, 10:02]`.
  - `TestMarkdownFeedIgnoresTempFiles`: after open, write a valid rendered entry as `.tmp-12345` in a day folder. `Poll` returns 0.
  - `TestMarkdownFeedSeesNewDayMonthYearFolders`: on an empty store, open the feed, then create entries at `2030-01-01T00:00:00Z` and `2031-02-03T12:00:00Z` (UTC). `Poll` returns both, oldest first.
  - `TestMarkdownFeedSkipsEditedEntries`: create an entry and open the feed. `UpdateEntry` with a new message, which renames the file. `Poll` returns 0.
  - `TestMarkdownFeedIgnoresDeletions`: create an entry, open the feed, `DeleteEntry`. `Poll` returns 0 and no error.
  - `TestMarkdownFeedSkipsMalformedFiles`: after open, write `bad.md` with no frontmatter into a day folder and create one good entry. `Poll` returns only the good one, with no error.
  - `TestMarkdownFeedReadsLegacyFileNames`: after open, write a rendered entry under the older name `<slug>-<first 8 characters of the id>.md`, which Homebrew 1.8.0 still writes. `Poll` returns it.
  - `TestMarkdownFeedAppliesFilter`: run a table over `Tags: ["keep"]`, `Text: "DEPLOY"` (matching is case-insensitive) and a `Since`/`Until` window. Each returns exactly the matching IDs.
  - `TestMarkdownFeedMissingDataDir`: open a feed on `filepath.Join(t.TempDir(), "absent")`. `Poll` returns `nil, nil` and the folder still doesn't exist. Then `NewMarkdownStore` on that path plus `CreateEntry`, and `Poll` returns the entry.
  - `TestMarkdownFeedRelistsRecentFolderWithUnchangedModTime`: open the feed and create entry A, then `Poll`. Record the folder's modified time, write entry B's file directly with `renderEntry` and `os.WriteFile`, and `os.Chtimes` the folder back to the recorded time. `Poll` returns B.
  - `TestMarkdownFeedConcurrentWritersSeenExactlyOnce`: open the feed. Start 4 goroutines, each with its own `NewMarkdownStore` on the same dir, each creating 25 entries, while one goroutine polls every 5 ms. After they finish, poll one last time. All 100 IDs appear exactly once.
- [ ] **Step 2: Write the failing lock test** in `markdown_feed_unix_test.go`. `TestMarkdownFeedDoesNotTakeWriteLock`: open `<dir>/.lock` and `syscall.Flock(fd, LOCK_EX)`. While holding the lock, open the feed, write an entry file directly, and run `Poll` in a goroutine. It must return the entry within 2 seconds, otherwise `t.Fatal("Poll blocked on the write lock")`.
- [ ] **Step 3: Run the tests to confirm they fail.** Run `go test ./internal/storage/ -run 'TestMarkdownFeed' -count=1`. Expected: build failure, `OpenFeed` undefined.
- [ ] **Step 4: Implement** `Feed` in `interface.go`, and `markdownFeed` and `MarkdownStore.OpenFeed` in `markdown_feed.go`, following the algorithm above.
- [ ] **Step 5: Run the tests.** Run `go test ./internal/storage/ -run 'TestMarkdownFeed' -count=1 -race`. Expected: PASS.
- [ ] **Step 6: Commit.** Run `git add internal/storage/interface.go internal/storage/markdown_feed.go internal/storage/markdown_feed_test.go internal/storage/markdown_feed_unix_test.go`, then `git commit -m "feat(storage): add markdown feed for new entries"`.

### Task 2: SQLite feed

**Files:**
- Modify: `internal/storage/store.go` (pull the filter SQL out of `SearchEntries` into a shared builder)
- Create: `internal/storage/sqlite_feed.go`
- Test: `internal/storage/sqlite_feed_test.go`

**Interfaces:**
- Consumes: `Feed` (Task 1).
- Produces: `func (s *SqliteStore) OpenFeed(filter *SearchFilter) (Feed, error)`, and unexported `type sqliteFeed struct { db *sql.DB; filter *SearchFilter; lastRowID int64 }`.

**Rules:**
- `SearchEntries` and the feed must build their filter SQL (the FTS5 join, the date and tag conditions) from one shared helper, so the feed matches exactly what a reload shows. Don't copy the SQL.
- Open: `lastRowID = SELECT COALESCE(MAX(rowid), 0) FROM entries`.
- Poll: run two plain statements and no transaction. First, `max := SELECT COALESCE(MAX(rowid), 0) FROM entries`. If `max <= lastRowID`, return nil. Otherwise select the filtered rows with `e.rowid > lastRowID AND e.rowid <= max ORDER BY e.rowid`, then set `lastRowID = max`. This is exact because SQLite has one writer at a time, so AUTOINCREMENT rowids are assigned in commit order. It also means rows that fail the filter never get scanned twice.

- [ ] **Step 1: Write the failing tests.** Each uses a file database in `t.TempDir()` through `NewSqliteStore`:
  - `TestSqliteFeedReportsOnlyEntriesAddedAfterOpen`: same shape as the markdown test.
  - `TestSqliteFeedMatchesSearchRules`: after open, create entries with the messages "deploy the app", "deployed it" and "other", with the tags `keep` and `drop`. For each filter in a table (`Text: "deploy"`, `Tags: ["keep"]`, and the two combined), the IDs from `Poll` equal the IDs from `SearchEntries(filter, 0)` limited to those new entries. FTS5 doesn't match "deploy" to "deployed", and the test pins that the feed agrees.
  - `TestSqliteFeedCursorPassesNonMatchingRows`: with filter `Tags: ["keep"]`, create 3 `drop` entries. `Poll` returns 0, and `feed.(*sqliteFeed).lastRowID` equals the table's max rowid.
  - `TestSqliteFeedReadsWhileWriterHoldsTransaction`: writer store A begins a transaction (`A.db.Begin()`, which is immediate) and inserts a row without committing. Run feed B (opened from a second `NewSqliteStore` on the same file) with `Poll` in a goroutine. It must return within 2 seconds, without the uncommitted row. After `Commit`, `Poll` returns it.
  - `TestSqliteFeedConcurrentWritersSeenExactlyOnce`: 4 writers, each with its own store, create 25 entries each while a poller runs. All 100 are seen exactly once.
- [ ] **Step 2: Run the tests to confirm they fail.** Run `go test ./internal/storage/ -run 'TestSqliteFeed' -count=1`. Expected: build failure.
- [ ] **Step 3: Implement** the shared filter builder, `sqliteFeed`, and `SqliteStore.OpenFeed`.
- [ ] **Step 4: Run the storage suite**, since `SearchEntries` changed. Run `go test ./internal/storage/ -count=1 -race`. Expected: PASS, including every existing `TestFTS*` and `TestSqlite*` test.
- [ ] **Step 5: Commit.** Run `git add internal/storage/store.go internal/storage/sqlite_feed.go internal/storage/sqlite_feed_test.go`, then `git commit -m "feat(storage): add sqlite feed sharing search filter sql"`.

### Task 3: Read-only stores

**Files:**
- Modify: `internal/storage/interface.go`, `internal/storage/markdown.go`, `internal/storage/store.go`, `internal/config/config.go`
- Test: `internal/storage/reader_test.go`, `internal/storage/reader_unix_test.go` (`//go:build darwin || linux`), `internal/config/config_test.go`

**Interfaces:**
- Consumes: `OpenFeed` on both stores (Tasks 1 and 2).
- Produces:
  - `type Reader interface { GetEntry(id string) (*Entry, error); ListEntries(limit int) ([]Entry, error); SearchEntries(filter *SearchFilter, limit int) ([]Entry, error); OpenFeed(filter *SearchFilter) (Feed, error); Close() error }`.
  - `Storage` becomes `Reader` plus the write methods: `CreateEntry`, `UpdateEntry`, `DeleteEntry`, `LastModified`, `Vacuum`, `Reset`. Every existing caller keeps compiling.
  - `func OpenMarkdownReader(dataDir string) (*MarkdownStore, error)`. It does what `NewMarkdownStore` does, minus `EnsureDir`.
  - `func OpenSqliteReader(dbPath string) (*SqliteStore, error)`:
    - If the file is missing, it errors with `no chronicle database at <path>`.
    - It opens DSN `"file:" + (&url.URL{Path: dbPath}).EscapedPath() + "?mode=ro&_pragma=busy_timeout(15000)"`. modernc passes a `file:` DSN to SQLite whole as a URI, and SQLite honors `mode=ro`.
    - It skips `migrate()`, `journal_mode` and `foreign_keys`.
    - If `PRAGMA table_info(entries)` lacks `timestamp_unix_seconds` or `timestamp_nanosecond`, it errors with `database at <path> predates chronicle 1.8.1; run "chronicle list" once to upgrade it`.
  - `func (c *Config) OpenReader() (storage.Reader, error)`: markdown uses `OpenMarkdownReader(c.GetDataDir())`, and sqlite uses `OpenSqliteReader(DefaultDBPath(c.GetDataDir()))`.

- [ ] **Step 1: Write the failing tests:**
  - `TestOpenMarkdownReaderCreatesNothing`: point it at a path that doesn't exist. `ListEntries(0)` returns 0 entries and no error, and the path still doesn't exist.
  - `TestOpenSqliteReaderRefusesWrites`: create a database with `NewSqliteStore` and close it. Open the reader. `reader.db.Exec("INSERT INTO entries(id, timestamp, message) VALUES('x', 'now', 'x')")` errors, with "readonly" in the message.
  - `TestOpenSqliteReaderLeavesDatabaseFileUnchanged`: create a database holding 3 entries and close it, then read the main file's bytes. Open the reader, run `SearchEntries(nil, 0)`, `OpenFeed(nil)` and `Poll`, then `Close`. The main file's bytes are identical.
  - `TestOpenSqliteReaderMissingFile`: the error contains `no chronicle database at`.
  - `TestOpenSqliteReaderOldSchema`: create a raw database whose `entries` table lacks the timestamp columns. The error contains `predates chronicle 1.8.1`.
  - `TestMarkdownReaderSearchesWhileLockHeld` (unix file): hold `LOCK_EX` on `.lock`. `OpenMarkdownReader` plus `SearchEntries` returns within 2 seconds.
  - `TestConfigOpenReader` (config package): a markdown config with a missing data dir returns a reader and creates no folder. A sqlite config with a missing database returns the `no chronicle database at` error.
- [ ] **Step 2: Run the tests to confirm they fail.** Run `go test ./internal/storage/ ./internal/config/ -count=1`. Expected: build failure.
- [ ] **Step 3: Implement** `Reader`, the two openers, and `OpenReader`.
- [ ] **Step 4: Run the tests.** Run `go test ./... -count=1 -race`. Expected: PASS, with every package compiling against the new `Storage` shape.
- [ ] **Step 5: Commit.** Run `git add` on the files listed above, then `git commit -m "feat(storage): add read-only openers and Reader interface"`.

### Task 4: Stream endpoint

**Files:**
- Modify: `internal/web/server.go` (extract the filter parsing; add `PollInterval`, `StopStreams` and the route)
- Create: `internal/web/stream.go`
- Test: `internal/web/stream_test.go`

**Interfaces:**
- Consumes: `storage.Reader`, `Feed` (Tasks 1–3).
- Produces:
  - `Options.PollInterval time.Duration`. Zero means `defaultPollInterval = time.Second`.
  - `func (s *Server) StopStreams()`. It is safe to call more than once, and it ends every open stream.
  - Route `GET /api/stream`, registered in every mode.
  - Unexported `filterFromQuery(r *http.Request) (*storage.SearchFilter, error)`, which `listEntries` and the stream share. It takes the same `q`, `tag`, `since` and `until` rules and error messages as today's `listEntries`.

**Rules:**
- An invalid filter gets the same 400 JSON error as `/api/entries`.
- Headers: `Content-Type: text/event-stream`, `Cache-Control: no-cache`.
- Call `http.NewResponseController(w).SetWriteDeadline(time.Time{})` before the first write, which lifts the server's 30-second `WriteTimeout` for this response, and flush after every event.
- Open the feed, write `retry: 3000`, then `event: ready` with `data: {}`. On each tick, `Poll` and write one `entry` event per entry.
- On a `Poll` error, return. The browser reconnects after 3 seconds and the page shows it was interrupted.
- Exit when the request context ends or `StopStreams` is called.

- [ ] **Step 1: Write the failing tests.** Use a real markdown store in `t.TempDir()` and `PollInterval: 20 * time.Millisecond`. A test helper reads `event:`/`data:` pairs with a timeout.
  - `TestStreamSendsReadyBeforeEntries`: connect. The first event is `ready`. Then `CreateEntry`, and the next event is `entry`, whose decoded `storage.Entry` has the same ID and message.
  - `TestStreamAppliesQueryFilters`: connect with `?tag=keep&q=ship`. Create entries tagged `drop` and `keep` (message "ship it"). The first `entry` event is the `keep` one.
  - `TestStreamRejectsBadFilter`: `?since=garbage` returns 400 with `{"error":"invalid since date"}`.
  - `TestStreamSurvivesWriteTimeout`: use `httptest.NewUnstartedServer`, set `Config.WriteTimeout = 200 * time.Millisecond`, and `Start`. Connect, wait 500 ms, create an entry. It arrives.
  - `TestStreamEndsWhenServerStops`: connect and read `ready`, then call `app.StopStreams()`. The client read hits EOF within 1 second.
  - `TestStreamServesTwoClients`: two connections both get the same new entry.
- [ ] **Step 2: Run the tests to confirm they fail.** Run `go test ./internal/web/ -run TestStream -count=1`. Expected: FAIL.
- [ ] **Step 3: Implement** `stream.go`, the `filterFromQuery` extraction, the `Server` fields (poll interval, a done channel with a `sync.Once`) and the route.
- [ ] **Step 4: Run the tests.** Run `go test ./internal/web/ -count=1 -race`. Expected: PASS, including the existing tests.
- [ ] **Step 5: Commit.** Run `git add internal/web/server.go internal/web/stream.go internal/web/stream_test.go`, then `git commit -m "feat(web): stream new entries over server-sent events"`.

### Task 5: Read-only web mode and `chronicle web --read-only`

**Files:**
- Modify: `internal/web/server.go`, `internal/cli/web.go`, `README.md`
- Create: `internal/e2e/main_test.go`, `internal/e2e/web_test.go` (both `//go:build !windows`)
- Test: `internal/web/server_test.go`

**Interfaces:**
- Consumes: `config.OpenReader` (Task 3), `StopStreams` and `PollInterval` (Task 4).
- Produces:
  - `func NewReadOnly(reader storage.Reader, options ...Options) (*Server, error)`. `New(store storage.Storage, options ...Options)` keeps its signature.
  - `Server` holds `reader storage.Reader` and `writer storage.Storage`. In read-only mode `writer` is nil, and the write routes (`POST /api/entries`, `PUT` and `DELETE /api/entries/{id}`) aren't registered, so the mux answers them with 405.
  - `GET /{$}` serves `index.html` with `<html lang="en">` replaced by `<html lang="en" data-mode="read-only">` or `data-mode="full"`. The bytes are built once in the constructor.
  - CLI: a `--read-only` flag, and the `--address` default becomes `127.0.0.1:1066`. Read-only mode opens `cfg.OpenReader()` and uses `NewReadOnly`. Both modes call `server.RegisterOnShutdown(app.StopStreams)`. The startup line stays `Chronicle web interface: <url>`.
  - e2e helpers in `internal/e2e`:
    - `TestMain` builds the binary once with `go build -o <tmp>/chronicle github.com/harper/chronicle`.
    - `newEnv(t, backend string) []string` makes temp `HOME`, `XDG_DATA_HOME` and `XDG_CONFIG_HOME` and writes `{"backend":"<backend>"}`.
    - `runChronicle(t, env, args...) string`.
    - `startChronicle(t, env, args...) (*exec.Cmd, *bufio.Reader)`. It reads stdout, and its cleanup sends `os.Interrupt` and waits.

- [ ] **Step 1: Write the failing server tests:**
  - `TestReadOnlyServerRefusesWrites`: `POST /api/entries`, `PUT /api/entries/x` and `DELETE /api/entries/x` each return 405.
  - `TestReadOnlyServerServesReads`: `GET /api/entries`, `/api/stats` and `/api/health` each return 200.
  - `TestIndexMarksMode`: the `GET /` body contains `data-mode="read-only"` from `NewReadOnly` and `data-mode="full"` from `New`.
- [ ] **Step 2: Write the failing e2e test.** `TestReadOnlyWebStreamsAndRefusesWrites` runs over a table of `markdown` and `sqlite`. For sqlite, run `chronicle add "seed"` first so the database exists. For markdown, start with no data dir at all.
  - Start `chronicle web --read-only --address 127.0.0.1:0 --open=false` and parse the URL from the first stdout line.
  - Open `GET /api/stream` and wait for `ready`. Run `chronicle add "e2e live entry" --tag e2e`. An `entry` event with that message arrives within 3 seconds.
  - POST, PUT and DELETE return 405.
  - After `os.Interrupt`, the process exits with status 0 within 3 seconds, even with the stream still open.
  - Keep the whole `internal/e2e` package under about 30 seconds, binary build included. The pre-commit hook runs `go test -race -count=1 -timeout=60s ./...`.
- [ ] **Step 3: Run the tests to confirm they fail.** Run `go test ./internal/web/ ./internal/e2e/ -count=1`. Expected: FAIL.
- [ ] **Step 4: Implement** the server split, `NewReadOnly`, the index handler, and the CLI flag, default and shutdown hook.
- [ ] **Step 5: Update the README** "Web Interface" section: the default URL `http://127.0.0.1:1066`, a `chronicle web --read-only` example with one line on what it hides and refuses, and a note that the page updates live.
- [ ] **Step 6: Run the tests.** Run `go test ./... -count=1 -race`. Expected: PASS.
- [ ] **Step 7: Commit.** Run `git add` on the files above, then `git commit -m "feat(web): add read-only mode and move default port to 1066"`.

### Task 6: New look and themes

**Files:**
- Modify: `internal/web/assets/app.css` (rewrite in readable form), `internal/web/assets/index.html`, `internal/web/assets/app.js` (theme button only), `Makefile`, `.github/workflows/ci.yml`
- Create: `internal/web/assets/theme.js`, `internal/web/jstest/theme.test.js`
- Test: `internal/web/server_test.go`

**Interfaces:**
- Produces:
  - `theme.js` defines `globalThis.ChronicleTheme = { normalize(stored), next(mode), apply(mode, root) }`:
    - `normalize` returns `auto`, `light` or `dark`, and anything unknown becomes `auto`.
    - `next` cycles `auto` → `light` → `dark` → `auto`.
    - `apply` sets `root.dataset.theme` for `light` and `dark` and removes it for `auto`.
    - When `document` exists, it applies `normalize(localStorage.getItem('chronicle-theme'))` at load, wrapped in try/catch.
  - `index.html` loads `<script src="/theme.js"></script>`, with no `defer`, before the stylesheet link, so a stored theme applies before first paint.
  - CSS tokens are defined once with `light-dark()` under `:root { color-scheme: light dark }`, with `:root[data-theme=light] { color-scheme: light }` and `:root[data-theme=dark] { color-scheme: dark }`.
  - Mode classes: `.full-only` is hidden under `:root[data-mode=read-only]`, and `.read-only-only` is hidden otherwise. In read-only mode, `.full-only` covers the composer, the edit dialog, the entry action buttons and the full hero text. `.read-only-only` covers the READ ONLY badge and the "LIVE FEED" / "Agent activity" header.
  - Elements Task 7 drives:
    - `#live-status`, with `data-state` set to `connecting`, `live` or `reconnecting` and text set to match.
    - `.entry-card[data-new]`, which shows the NEW tag and tint.
    - `#theme-toggle`, whose `aria-label` and `title` read `Theme: Auto`, `Theme: Light` or `Theme: Dark`.
  - A `chronicle_` wordmark in monospace, with the `_` in `--accent`, replaces the "C" tile.
  - `make test-js` runs `node --test internal/web/jstest/*.test.js`. `make ci` becomes `fmt lint test test-js integration-test`. CI's `test` job gains a step that runs the same command.

- [ ] **Step 1: Write the failing tests:**
  - `theme.test.js` uses `node:test` and `require('../assets/theme.js')`:
    - `normalize(null)`, `normalize('nope')` and `normalize('auto')` all return `'auto'`.
    - `next('auto')` is `'light'`, `next('light')` is `'dark'`, and `next('dark')` is `'auto'`.
    - `apply('dark', root)` sets `root.dataset.theme` to `'dark'`, and `apply('auto', root)` deletes it. Use a plain `{dataset: {}}` object as the root.
  - Go, `TestStylesheetUsesApprovedLook`: the embedded `app.css` contains every hex value in the palette table, and contains `light-dark(`. It contains none of `Georgia`, `serif;`, `#f3f0e8` or `#e35d35`. Search for `serif;` with a leading space or comma so `sans-serif` doesn't match.
  - Go, `TestIndexLoadsThemeBeforeStylesheet`: in `index.html`, `<script src="/theme.js"></script>` comes before `href="/app.css"`.
- [ ] **Step 2: Run the tests to confirm they fail.** Run `node --test internal/web/jstest/theme.test.js`, then `go test ./internal/web/ -run 'TestStylesheet|TestIndexLoads' -count=1`. Expected: both FAIL.
- [ ] **Step 3: Implement** `theme.js`, the new `app.css` (layout A as in the mockup, palette from the constraints, today's mobile breakpoint at 760px and reduced-motion rule kept), the `index.html` markup, and the cycling theme button in `app.js`.
- [ ] **Step 4: Run the tests.** Run `make test-js`, then `go test ./internal/web/ -count=1`. Expected: PASS.
- [ ] **Step 5: Update the README.** In the "Web Interface" section, replace "dark mode" with one line: the page follows the system's light or dark setting, and the theme button cycles Auto, Light and Dark. Also add `make test-js` to "Development".
- [ ] **Step 6: Commit.** Run `git add` on the files above plus `README.md`, then `git commit -m "feat(web): restyle with light and dark themes that follow the system"`.

### Task 7: Live page behavior

**Files:**
- Modify: `internal/web/assets/app.js`, `internal/web/assets/index.html` (load `timeline.js` with `defer`, before `app.js`)
- Create: `internal/web/assets/timeline.js`, `internal/web/jstest/timeline.test.js`

**Interfaces:**
- Consumes: the stream contract (Task 4), `data-mode` (Task 5), and the elements and classes from Task 6.
- Produces: `globalThis.ChronicleTimeline = { insertionIndex(entries, entry), contains(entries, id) }`. Entries are sorted newest first. `insertionIndex` returns the index of the first entry whose timestamp is earlier than `entry.timestamp`, so entries with equal timestamps keep arrival order, or `entries.length` if there is none.

**Rules for `app.js`:**
- One `filterParams()` builds the query for both `/api/entries` and `/api/stream`.
- Open `new EventSource('/api/stream?' + filterParams())` at load. Close it and open a new one whenever the filters change.
- Keep today's list load at startup for a fast first paint. On every `ready`, the first one included, set `#live-status` to `live` and reload the list once. The feed opens before `ready` is sent, so a list loaded after `ready` plus the stream covers every entry. The startup load alone could miss one written between that load and the feed opening. On `error`, set the marker to `reconnecting`.
- On `entry`, skip the entry if `contains` is true. Otherwise insert it into `state.entries` and the DOM at `insertionIndex`, build the card with the existing template and `textContent` only, set `data-new`, and add 1 to the total count.
- Keep the content on screen in place. If the inserted card lands above the top of the viewport and the page is scrolled down, `window.scrollBy(0, height of the inserted card)`.
- NEW marks: when an entry arrives while the page is visible, clear `data-new` after 10 seconds. If the page is hidden, start that 10 seconds at the next `visibilitychange` to visible.
- In read-only mode (`document.documentElement.dataset.mode === 'read-only'`), don't attach the composer, edit or delete handlers.

- [ ] **Step 1: Write the failing test.** `timeline.test.js` takes newest-first lists of `{id, timestamp}`:
  - `insertionIndex([], e)` is 0.
  - A newer entry gets 0, and an older one gets `length`.
  - An entry between two neighbors gets the index between them.
  - With equal timestamps, the new entry goes after the existing equal ones.
  - `contains` returns true and false correctly.
- [ ] **Step 2: Run the test to confirm it fails.** Run `node --test internal/web/jstest/timeline.test.js`. Expected: FAIL.
- [ ] **Step 3: Implement** `timeline.js` and the `app.js` stream client following the rules above.
- [ ] **Step 4: Run the tests.** Run `make test-js` and `go test ./internal/web/ -count=1`. Expected: PASS.
- [ ] **Step 5: Smoke-check in a browser.** Build with `go build -o /private/tmp/claude-501/-Users-harper-Public-src-personal-suite-chronicle/0d39a7ba-0435-4382-9cf2-659909b45492/scratchpad/chronicle-live .`. Run it with temp `HOME` and `XDG_*` as `web --read-only --address 127.0.0.1:0 --open=false`. Open the page with agent-browser, run `chronicle add` with the same environment, and confirm with `snapshot` that the entry appeared with NEW. Task 9 does the full check.
- [ ] **Step 6: Commit.** Run `git add` on the files above, then `git commit -m "feat(web): insert streamed entries live with new markers"`.

### Task 8: `chronicle tail`

**Files:**
- Create: `internal/cli/tail.go`, `internal/cli/tail_test.go`, `internal/e2e/tail_test.go` (`//go:build !windows`)
- Modify: `internal/cli/root_test.go` (`TestIsKnownCommand` gains `tail`), `README.md`

**Interfaces:**
- Consumes: `config.OpenReader`, `Reader`, `Feed` (Task 3), and the e2e helpers (Task 5).
- Produces:
  - `func formatTailEntry(entry storage.Entry, now time.Time, home string) string`.
  - `func runTail(ctx context.Context, out io.Writer, reader storage.Reader, limit int, tags []string, interval time.Duration) error`.
  - Command `tail`, with `--limit/-n` (int, default 10; matches `list`) and `--tag/-t` (repeatable, `StringArrayVarP` like `search`). It calls `runTail` with `time.Second` under `signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)` and returns nil when the context ends.

**Rules:**
- `runTail` opens the feed first, then prints history: if `limit > 0`, it calls `SearchEntries(&SearchFilter{Tags: tags}, limit)` and prints the results oldest first. `-n 0` prints no history. It then polls, and it skips any ID it has already printed, because the feed opens before the history query and the two can overlap.
- Format:
  - The time is `entry.Timestamp.In(now.Location())`. Use `15:04:05` for the same day as `now`, `Jan 2 15:04:05` for the same year, and `2006-01-02 15:04:05` otherwise.
  - The first line is the time, two spaces, the message's first line, then `  #tag #tag` if there are tags.
  - The message's later lines each go on their own line, indented by `len(time) + 2` spaces.
  - The working directory goes last, on its own line, with the same indent and with `home` shortened to `~`. Exactly `home` becomes `~`, a `home + "/"` prefix becomes `~/`, and any other path stays as it is. With no working directory, that line is left out.
  - Color the time and path faint and the tags cyan with `fatih/color`, which turns itself off when output isn't a terminal or `NO_COLOR` is set. Tests set `color.NoColor = true`.

- [ ] **Step 1: Write the failing unit tests** with `now := time.Date(2026, 11, 11, 18, 0, 0, 0, time.Local)` and `home := "/home/h"`:
  - `TestFormatTailEntryToday`: an entry at 16:04:43 local, tags `spaceport` and `kata`, working directory `/home/h/Public/src/2389/spaceport`, gives exactly `"16:04:43  started spaceport kata backlog work  #spaceport #kata\n          ~/Public/src/2389/spaceport\n"`.
  - `TestFormatTailEntryEarlierDayAndYear`: Nov 10 gives the prefix `"Nov 10 16:04:43  "`, and 2025-03-01 gives `"2025-03-01 16:04:43  "`, each with a matching indent on the path line.
  - `TestFormatTailEntryMultilineNoTagsNoDir`: the message `"one\ntwo"` with no tags and no working directory gives `"16:04:43  one\n          two\n"`.
  - `TestFormatTailEntryHomeExactlyAndOutside`: a working directory of `/home/h` prints `~`, and `/srv/x` prints `/srv/x`.
  - `TestFormatTailEntryConvertsToLocalTime`: an entry at `time.Date(2026, 11, 11, 16, 4, 43, 0, time.UTC)` prints the time from `.In(time.Local)`. Build the expected string with the same conversion, so the test passes in any zone.
  - `TestRunTailPrintsHistoryThenFollows`: use a real markdown store in a temp dir with 3 entries, `limit` 2, an interval of 20 ms, and output through an `io.Pipe` read line by line. The first lines are entries 2 and 3, oldest first. Then `CreateEntry`, and its line arrives within 2 seconds. Cancel the context, and `runTail` returns nil.
  - `TestRunTailFiltersByTag`: with `tags = ["keep"]`, entries tagged `drop` never print.
- [ ] **Step 2: Write the failing e2e test.** `TestTailFollowsAnotherProcess` runs over a table of `markdown` and `sqlite`, seeding sqlite with one `add`. Start `chronicle tail -n 0`, then run `chronicle add "e2e tail entry" --tag e2e`. The tail stdout contains `e2e tail entry  #e2e` within 3 seconds. After `os.Interrupt`, it exits 0.
- [ ] **Step 3: Run the tests to confirm they fail.** Run `go test ./internal/cli/ ./internal/e2e/ -count=1`. Expected: FAIL.
- [ ] **Step 4: Implement** `tail.go`.
- [ ] **Step 5: Update the README.** Add a "Tail" section with usage and the sample output from the spec. Add a note that a quick-add message can't start with a subcommand name, so use `chronicle add "tail ..."`.
- [ ] **Step 6: Run the tests.** Run `go test ./... -count=1 -race`. Expected: PASS.
- [ ] **Step 7: Commit.** Run `git add` on the files above, then `git commit -m "feat(cli): add tail command that follows new entries"`.

### Task 9: Browser check and final gates

**Files:** none new. Fix anything this task finds in the task that owns it, and commit the fix with a `fix:` message.

- [ ] **Step 1: Set up.** Build the binary into the scratchpad. Run `agent-browser doctor` and record the Chrome for Testing version. Start `web --read-only --address 127.0.0.1:0 --open=false` with temp `HOME` and `XDG_*`, seeded with 30 entries through `chronicle add` using that environment. Use your own agent-browser session name, and close it by name when done.
- [ ] **Step 2: Check the look.** With `set media light`, then `set media dark`, take screenshots at 1440×1000 and 390×844. Check that:
  - The page follows the system setting, and the palette matches the mockup.
  - There's no serif type, no cream, and no "C" tile.
  - Read-only mode shows no composer, edit or delete buttons, or dialog.
- [ ] **Step 3: Check the theme button.** It cycles Auto → Light → Dark → Auto, the label reads `Theme: …` to match, and a reload keeps the choice.
- [ ] **Step 4: Check live behavior.**
  - An entry written with `chronicle add` appears within about 2 seconds with NEW, and NEW clears after about 10 seconds.
  - Scroll halfway down, add an entry, and confirm by screenshot that the content on screen didn't move.
  - A filter on tag X hides a new entry tagged Y.
  - Two tabs both receive a new entry.
- [ ] **Step 5: Check a restart.** Stop the server. The marker shows reconnecting. Add an entry, then restart the server on the same port with `--address 127.0.0.1:<port>`. The marker returns to live, and the entry written during the outage appears.
- [ ] **Step 6: Check HTML in a message.** Add an entry with the message `<img src=x onerror=alert(1)>`. It shows as literal text.
- [ ] **Step 7: Check full mode.** `chronicle web` without the flag still adds, edits and deletes, and an entry added from the CLI streams in.
- [ ] **Step 8: Real-data smoke test (read only).** Run `chronicle web --read-only --address 127.0.0.1:0 --open=false` and `chronicle tail -n 5` against Harper's real config. Both are read-only, and no entry gets added. Check that the 4,400+ entries load, that the legacy short-ID file names read fine, and that the real data folder's file count and newest modified time are the same before and after.
- [ ] **Step 9: Run the gates.** `make ci` must exit 0, and so must `go test -race -count=1 ./...`. Report any warnings in the output.
- [ ] **Step 10: Commit the evidence notes.** Add a short "Verification" section to the end of the design doc: the Chrome version, the commands run, and what each check showed. Then `git commit -m "docs: record live view verification"`.
