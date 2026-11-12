# Live Read-Only View: Follow-ups

Date: 2026-11-12. Branch: `feat/live-view`. This file collects what the build's reviews left open, so it outlasts the session that built the feature.

## Waiting on Harper

1. **Decision 7 changed during the build.** The design doc keeps the approved text and adds a dated note describing the built behavior:
   - Hiding the page restarts the 10 seconds when it's shown again.
   - Entries a reconnect brings in are marked NEW.
   - A hidden page closes its stream and catches up when shown.

   Because of that last point, "New entries reach every open page within about a second" (design "The stream"; README) now holds only for visible pages.
2. **Merge and push.** Local `main` has two kola commits (the original web UI) that `origin/main` lacks. This branch builds on them.
3. **Evidence screenshots.** `docs/plans/2026-11-11-live-read-only-view-evidence/` holds six screenshots. They show only seeded test entries, but they also show `harper@disaster` and a scratch path. Decide whether those belong in a public repo before pushing.
4. **LIVE label contrast.** In the light theme, the approved green `#16a34a` on the light background measures 3.15:1 for the small LIVE text, below WCAG AA. Fixing it means adding a darker green used for text only.

## Known weaknesses (shipped as is)

- **SQLite read-only:**
  - Opening a database read-only leaves SQLite's own `-wal` and `-shm` sidecar files behind.
  - A database in a folder the viewer can't write to won't open, failing with `attempt to write a readonly database`.
- **Tag counts** don't follow the stream. They refresh on a filter change, a reconnect and a page load.
- **Full reads on markdown:**
  - Every filter change and every return to the tab reloads stats, which reads every entry file (about 0.1 s today, growing with the log).
  - Opening a page reads the store about four times.
- **Long-open tabs:** the list never shrinks on a page left open for days.
- **Failed loads:** after a failed or timed-out list load, the page shows only the entries that arrived in the meantime and doesn't retry.
- **Unescaped characters in `tail`:** control characters print as escapes, but Unicode format characters (bidi overrides, zero-width spaces) still print raw.
- **Screen readers:** new entries are no longer announced (`aria-live="off"`).
- **Browser coverage:**
  - Checked only in Chrome for Testing 151. Safari, Firefox and a real Retina display are untested.
  - `light-dark()` needs a 2024-or-later browser.

## Problems found that predate this branch (not fixed here)

- **Order-dependent cli tests:**
  - Under `-shuffle`, `TestInstallSkillCommandExecution` replays stale root-command arguments and can reach the real data home.
  - `TestIsKnownCommandCases/recognizes_help_command` fails in narrow `-run` selections.
  - Default order, which the hook and CI use, is safe.
- **Raw terminal output:** `chronicle list` and `chronicle search` print entry text raw to the terminal, escape sequences included.
- **Unescaped database path:** `sqliteDataSourceName` appends `?_txlock…` to an unescaped path, so `NewSqliteStore` fails on paths that contain `?`.
- **Over-broad ignore rule:** `.gitignore`'s bare `chronicle` line ignores any path segment named `chronicle`, such as `.claude/skills/chronicle/`. `/chronicle` would match only the binary.
- **Lint version gap:** CI pins golangci-lint v2.7.2, while the pre-commit hook runs 2.13.2.

## Minor findings the final review said can wait

- **Storage**
  - A day folder that can't be listed when a feed opens reports all its entries as new once it recovers.
  - Files with an empty `id:` share one seen ID.
  - The SQLite tests' `assertPolls` treats an empty slice as nil.
  - No test checks that the cursor stays put after a failed `Poll`.
  - A test variable shadows `newestRowID`.
  - The lock-holding helper is duplicated in the tests.
  - No reader test covers `GetEntry` or text and tag search on the `mode=ro` connection.
- **Stream**
  - The zero write deadline lets a client that stops reading block its handler.
  - `Poll` errors leave no server log.
  - A request that arrives after `StopStreams` still opens a feed.
  - HEAD requests hold `/api/stream` open.
- **Web server**
  - Read-only mode is stored twice: the `mode` argument and `writer == nil`.
  - `readOnlyTestServer` never closes its stores.
- **Page code**
  - The theme storage key appears in both theme.js and app.js.
  - A test comment names a person.
  - app.css has a redundant `scroll-behavior` rule.
  - There is no `@supports` fallback for `light-dark()`.
  - `overflow-anchor: none` lets a tag-panel rewrap move a phone reader's place.
- **Page behavior**
  - A backdated entry is appended at the bottom when more entries remain unloaded.
  - A rejected search retries every 3 s and shows RECONNECTING.
  - The read-only CSS test's regex is loose.
  - No automated test drives app.js's stream handling or the two scroll fixes. The scroll arithmetic could move into timeline.js.
  - The gap-fill flag isn't tied to the `ready` that set it.
  - An entry that a concurrent list load already included gets no NEW mark.
  - A failed stats load shows "—" for the total.
  - `timeline.js:52` lost a space.
- **tail**
  - The local-time test can't fail on a UTC machine. A `time.FixedZone` twin would fix that.
  - SIGTERM exit 0 has only been checked by hand.
  - The set of printed IDs grows with every entry printed.
  - The markdown e2e starts with the data folder already present.
- **Docs**
  - The design's Gates row says 36 JavaScript tests (now 45).
  - The Verification section doesn't record the final fix wave's browser checks:
    - six tabs froze on the old build and eight load on the new one;
    - a hidden tab holds no stream;
    - the NEW mark shows on return.
  - The NEW pill's height was measured only in Chrome at 1440 px.
