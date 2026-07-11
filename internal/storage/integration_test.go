// ABOUTME: Integration tests for SQLite storage layer
// ABOUTME: Tests concurrent access and WAL mode behavior

package storage

import (
	"database/sql"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestSqliteStartupWaitsForConcurrentMigration(t *testing.T) {
	const childEnv = "CHRONICLE_SQLITE_LOCK_WAIT_CHILD"
	if os.Getenv(childEnv) == "1" {
		marker := os.Getenv("CHRONICLE_SQLITE_LOCK_WAIT_MARKER")
		if err := os.WriteFile(marker, []byte("started"), 0600); err != nil {
			t.Fatalf("signal lock-wait child start: %v", err)
		}

		store, err := NewSqliteStore(os.Getenv("CHRONICLE_SQLITE_LOCK_WAIT_DB"))
		if err != nil {
			t.Fatalf("open database after waiting for migration lock: %v", err)
		}
		if err := store.Close(); err != nil {
			t.Fatalf("close database after waiting for migration lock: %v", err)
		}
		return
	}

	dbPath := filepath.Join(t.TempDir(), "lock-wait.db")
	store, err := NewSqliteStore(dbPath)
	if err != nil {
		t.Fatalf("create lock-wait database: %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("close lock-wait database: %v", err)
	}

	holder, err := sql.Open("sqlite", sqliteDataSourceName(dbPath))
	if err != nil {
		t.Fatalf("open migration lock holder: %v", err)
	}
	defer holder.Close()
	holderTx, err := holder.Begin()
	if err != nil {
		t.Fatalf("acquire migration write lock: %v", err)
	}
	defer holderTx.Rollback()

	marker := filepath.Join(filepath.Dir(dbPath), "child-started")
	command := exec.Command(os.Args[0], "-test.run=^TestSqliteStartupWaitsForConcurrentMigration$")
	command.Env = append(
		os.Environ(),
		childEnv+"=1",
		"CHRONICLE_SQLITE_LOCK_WAIT_DB="+dbPath,
		"CHRONICLE_SQLITE_LOCK_WAIT_MARKER="+marker,
	)
	var output strings.Builder
	command.Stdout = &output
	command.Stderr = &output
	if err := command.Start(); err != nil {
		t.Fatalf("start lock-wait child: %v", err)
	}
	done := make(chan error, 1)
	go func() {
		done <- command.Wait()
	}()
	childWaited := false
	t.Cleanup(func() {
		if childWaited {
			return
		}
		_ = command.Process.Kill()
		<-done
	})

	markerDeadline := time.Now().Add(10 * time.Second)
	for {
		if _, err := os.Stat(marker); err == nil {
			break
		}
		if time.Now().After(markerDeadline) {
			t.Fatalf("timed out waiting for lock-wait child marker %q", marker)
		}
		time.Sleep(time.Millisecond)
	}

	// The constructor must remain pending while a competing immediate transaction
	// holds the migration lock beyond the minimum supported contention interval.
	const sustainedMigrationContention = 6 * time.Second
	select {
	case err := <-done:
		childWaited = true
		t.Fatalf("lock-wait child exited while migration lock was held: %v\n%s", err, output.String())
	case <-time.After(sustainedMigrationContention):
	}

	if err := holderTx.Commit(); err != nil {
		t.Fatalf("release migration write lock: %v", err)
	}
	select {
	case err := <-done:
		childWaited = true
		if err != nil {
			t.Fatalf("lock-wait child failed after lock release: %v\n%s", err, output.String())
		}
	case <-time.After(10 * time.Second):
		t.Fatal("lock-wait child did not finish after migration lock release")
	}
}

func TestSqliteMigrationAcrossProcesses(t *testing.T) {
	const childEnv = "CHRONICLE_SQLITE_MIGRATION_CHILD"
	if os.Getenv(childEnv) == "1" {
		marker := os.Getenv("CHRONICLE_SQLITE_MIGRATION_MARKER")
		deadline := time.Now().Add(10 * time.Second)
		for {
			if _, err := os.Stat(marker); err == nil {
				break
			}
			if time.Now().After(deadline) {
				t.Fatalf("timed out waiting for migration start marker %q", marker)
			}
			time.Sleep(time.Millisecond)
		}

		store, err := NewSqliteStore(os.Getenv("CHRONICLE_SQLITE_MIGRATION_DB"))
		if err != nil {
			t.Fatalf("open shared migration database: %v", err)
		}
		if err := store.Close(); err != nil {
			t.Fatalf("close shared migration database: %v", err)
		}
		return
	}

	dbPath := filepath.Join(t.TempDir(), "multiprocess.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open intermediate database: %v", err)
	}
	if _, err := db.Exec(`
		PRAGMA journal_mode=WAL;
		CREATE TABLE entries (
			rowid INTEGER PRIMARY KEY AUTOINCREMENT,
			id TEXT UNIQUE NOT NULL,
			timestamp DATETIME NOT NULL,
			timestamp_unix_nano INTEGER,
			message TEXT NOT NULL,
			hostname TEXT,
			username TEXT,
			working_directory TEXT,
			tags TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)
	`); err != nil {
		t.Fatalf("create intermediate schema: %v", err)
	}
	tx, err := db.Begin()
	if err != nil {
		t.Fatalf("begin intermediate seed: %v", err)
	}
	statement, err := tx.Prepare(`
		INSERT INTO entries(id, timestamp, timestamp_unix_nano, message, tags)
		VALUES(?, ?, ?, ?, ?)
	`)
	if err != nil {
		_ = tx.Rollback()
		t.Fatalf("prepare intermediate seed: %v", err)
	}
	baseTime := time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC)
	const entryCount = 20_000
	for i := 0; i < entryCount; i++ {
		timestamp := baseTime.Add(time.Duration(i) * time.Nanosecond)
		if _, err := statement.Exec(
			fmt.Sprintf("entry-%05d", i),
			formatTimestamp(timestamp),
			timestamp.UnixNano(),
			"multiprocess migration",
			`["migration"]`,
		); err != nil {
			_ = statement.Close()
			_ = tx.Rollback()
			t.Fatalf("seed intermediate entry %d: %v", i, err)
		}
	}
	if err := statement.Close(); err != nil {
		_ = tx.Rollback()
		t.Fatalf("close intermediate seed statement: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit intermediate seed: %v", err)
	}
	if _, err := db.Exec(`
		CREATE VIRTUAL TABLE entries_fts USING fts5(
			message,
			tags,
			content=entries,
			content_rowid=rowid
		);
		INSERT INTO entries_fts(entries_fts) VALUES('rebuild');
		CREATE TRIGGER entries_au AFTER UPDATE ON entries BEGIN
			INSERT INTO entries_fts(entries_fts, rowid, message, tags)
			VALUES('delete', old.rowid, old.message, old.tags);
			INSERT INTO entries_fts(rowid, message, tags)
			VALUES(new.rowid, new.message, new.tags);
		END;
	`); err != nil {
		t.Fatalf("create intermediate full-text schema: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close intermediate database: %v", err)
	}

	marker := filepath.Join(filepath.Dir(dbPath), "start")
	const processCount = 8
	type childProcess struct {
		command *exec.Cmd
		output  strings.Builder
	}
	children := make([]childProcess, processCount)
	for i := range children {
		children[i].command = exec.Command(os.Args[0], "-test.run=^TestSqliteMigrationAcrossProcesses$")
		children[i].command.Env = append(
			os.Environ(),
			childEnv+"=1",
			"CHRONICLE_SQLITE_MIGRATION_DB="+dbPath,
			"CHRONICLE_SQLITE_MIGRATION_MARKER="+marker,
		)
		children[i].command.Stdout = &children[i].output
		children[i].command.Stderr = &children[i].output
		if err := children[i].command.Start(); err != nil {
			t.Fatalf("start migration child %d: %v", i, err)
		}
	}
	if err := os.WriteFile(marker, []byte("start"), 0600); err != nil {
		t.Fatalf("release migration children: %v", err)
	}
	for i := range children {
		if err := children[i].command.Wait(); err != nil {
			t.Errorf("migration child %d failed: %v\n%s", i, err, children[i].output.String())
		}
	}
	if t.Failed() {
		return
	}

	store, err := NewSqliteStore(dbPath)
	if err != nil {
		t.Fatalf("reopen multiprocess migration database: %v", err)
	}
	var normalizedCount int
	if err := store.db.QueryRow(`
		SELECT COUNT(*) FROM entries
		WHERE timestamp_unix_seconds IS NOT NULL AND timestamp_nanosecond IS NOT NULL
	`).Scan(&normalizedCount); err != nil {
		_ = store.Close()
		t.Fatalf("count multiprocess timestamp backfill: %v", err)
	}
	if normalizedCount != entryCount {
		_ = store.Close()
		t.Fatalf("normalized entries = %d, want %d", normalizedCount, entryCount)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("close multiprocess verification store: %v", err)
	}
	reopened, err := NewSqliteStore(dbPath)
	if err != nil {
		t.Fatalf("second reopen after multiprocess migration: %v", err)
	}
	defer reopened.Close()
}

func TestWALConcurrentConnections(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "chronicle.db")

	const numConnections = 3
	const writesPerConnection = 5

	var wg sync.WaitGroup
	errors := make(chan error, numConnections*(writesPerConnection+1))

	for i := 0; i < numConnections; i++ {
		wg.Add(1)
		go func(connID int) {
			defer wg.Done()

			// Each goroutine opens an independent store within this process.
			store, err := NewSqliteStore(dbPath)
			if err != nil {
				errors <- err
				return
			}
			defer store.Close()

			// Perform writes
			for j := 0; j < writesPerConnection; j++ {
				entry := Entry{
					Timestamp: time.Now(),
					Message:   "test message",
					Hostname:  "test-host",
				}
				if _, err := store.CreateEntry(entry); err != nil {
					errors <- err
				}
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	// Collect any errors
	var errs []error
	for err := range errors {
		errs = append(errs, err)
	}

	if len(errs) > 0 {
		t.Errorf("concurrent connections produced %d errors, first: %v", len(errs), errs[0])
	}

	// Verify total entries
	store, err := NewSqliteStore(dbPath)
	if err != nil {
		t.Fatalf("failed to open store for verification: %v", err)
	}
	defer store.Close()

	entries, err := store.ListEntries(0)
	if err != nil {
		t.Fatalf("failed to list entries: %v", err)
	}

	expected := numConnections * writesPerConnection
	if len(entries) != expected {
		t.Errorf("expected %d entries, got %d", expected, len(entries))
	}
}

func TestSqliteConcurrentCRUDAndLastModified(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "chronicle.db")
	store, err := NewSqliteStore(dbPath)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	const iterations = 50
	ready := make(chan struct{})
	start := make(chan struct{})
	done := make(chan struct{})
	errs := make(chan error, 1)

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		close(ready)
		<-start
		for {
			select {
			case <-done:
				return
			default:
				_ = store.LastModified()
			}
		}
	}()

	<-ready
	go func() {
		defer wg.Done()
		defer close(done)
		close(start)
		for i := 0; i < iterations; i++ {
			entry := Entry{
				Timestamp: time.Now(),
				Message:   fmt.Sprintf("entry %d", i),
			}
			id, err := store.CreateEntry(entry)
			if err != nil {
				errs <- fmt.Errorf("create entry %d: %w", i, err)
				return
			}
			entry.ID = id
			entry.Message = fmt.Sprintf("updated entry %d", i)
			if err := store.UpdateEntry(entry); err != nil {
				errs <- fmt.Errorf("update entry %d: %w", i, err)
				return
			}
			if err := store.DeleteEntry(id); err != nil {
				errs <- fmt.Errorf("delete entry %d: %w", i, err)
				return
			}
		}
	}()

	wg.Wait()
	close(errs)
	for err := range errs {
		t.Error(err)
	}
}

func TestFTSSearchIntegration(t *testing.T) {
	store, err := NewSqliteStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	// Create entries with various messages
	entries := []Entry{
		{Timestamp: time.Now(), Message: "deployed production release v2.1.0"},
		{Timestamp: time.Now(), Message: "fixed critical bug in authentication"},
		{Timestamp: time.Now(), Message: "updated documentation for API"},
		{Timestamp: time.Now(), Message: "production database migration complete"},
		{Timestamp: time.Now(), Message: "new feature: user authentication flow"},
	}

	for _, e := range entries {
		if _, err := store.CreateEntry(e); err != nil {
			t.Fatalf("failed to create entry: %v", err)
		}
	}

	tests := []struct {
		searchText    string
		expectedCount int
	}{
		{"production", 2},
		{"authentication", 2},
		{"bug", 1},
		{"documentation", 1},
		{"nonexistent", 0},
	}

	for _, tt := range tests {
		t.Run(tt.searchText, func(t *testing.T) {
			filter := &SearchFilter{Text: tt.searchText}
			results, err := store.SearchEntries(filter, 100)
			if err != nil {
				t.Fatalf("search failed: %v", err)
			}
			if len(results) != tt.expectedCount {
				t.Errorf("expected %d results for %q, got %d", tt.expectedCount, tt.searchText, len(results))
			}
		})
	}
}

func TestResetAndVacuum(t *testing.T) {
	store, err := NewSqliteStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	// Create some entries
	for i := 0; i < 10; i++ {
		entry := Entry{
			Timestamp: time.Now(),
			Message:   "test message",
		}
		if _, err := store.CreateEntry(entry); err != nil {
			t.Fatalf("failed to create entry: %v", err)
		}
	}

	// Verify entries exist
	entries, _ := store.ListEntries(0)
	if len(entries) != 10 {
		t.Errorf("expected 10 entries, got %d", len(entries))
	}

	// Reset
	if err := store.Reset(); err != nil {
		t.Fatalf("reset failed: %v", err)
	}

	// Verify entries cleared
	entries, _ = store.ListEntries(0)
	if len(entries) != 0 {
		t.Errorf("expected 0 entries after reset, got %d", len(entries))
	}

	// Vacuum should work after reset
	if err := store.Vacuum(); err != nil {
		t.Errorf("vacuum failed after reset: %v", err)
	}
}

func TestSqliteReadsLegacyDriverTimestampFormats(t *testing.T) {
	store, err := NewSqliteStore(filepath.Join(t.TempDir(), "legacy.db"))
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	for _, test := range []struct {
		id   string
		raw  string
		want time.Time
	}{
		{
			id:   "named-zone",
			raw:  "2026-07-10 12:00:00.000000123 +0100 offset",
			want: time.Date(2026, 7, 10, 12, 0, 0, 123, time.FixedZone("expected", 60*60)),
		},
		{
			id:   "numeric-zone-name",
			raw:  "2026-07-10 12:00:00.123456789 +0100 +0100",
			want: time.Date(2026, 7, 10, 12, 0, 0, 123456789, time.FixedZone("expected", 60*60)),
		},
	} {
		t.Run(test.id, func(t *testing.T) {
			if _, err := store.db.Exec(
				"INSERT INTO entries(id, timestamp, message, hostname, username, working_directory, tags) VALUES(?, ?, ?, ?, ?, ?, ?)",
				test.id, test.raw, "legacy", "", "", "", "[]",
			); err != nil {
				t.Fatalf("failed to insert legacy row: %v", err)
			}

			got, err := store.GetEntry(test.id)
			if err != nil {
				t.Fatalf("failed to read legacy timestamp %q: %v", test.raw, err)
			}
			if !got.Timestamp.Equal(test.want) || got.Timestamp.Nanosecond() != test.want.Nanosecond() {
				t.Fatalf("timestamp = %v, want instant %v", got.Timestamp, test.want)
			}
			_, gotOffset := got.Timestamp.Zone()
			_, wantOffset := test.want.Zone()
			if gotOffset != wantOffset {
				t.Fatalf("offset = %d, want %d", gotOffset, wantOffset)
			}
		})
	}
}

func TestSqliteBackfillsAndOrdersTimestampInstants(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "legacy.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open legacy database: %v", err)
	}
	if _, err := db.Exec(`
		CREATE TABLE entries (
			rowid INTEGER PRIMARY KEY AUTOINCREMENT,
			id TEXT UNIQUE NOT NULL,
			timestamp DATETIME NOT NULL,
			timestamp_unix_nano INTEGER,
			message TEXT NOT NULL,
			hostname TEXT,
			username TEXT,
			working_directory TEXT,
			tags TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)
	`); err != nil {
		t.Fatalf("create legacy schema: %v", err)
	}
	for _, row := range []struct {
		id        string
		timestamp string
	}{
		{id: "legacy-future", timestamp: "2500-01-02 03:04:05.123456789 +0000 UTC"},
		{id: "legacy-late", timestamp: "2026-07-10 23:00:00.000000123 +0100 offset"},
		{id: "legacy-numeric", timestamp: "2026-07-10 12:00:00.123456789 +0100 +0100"},
		{id: "corrupt", timestamp: "not-a-time"},
	} {
		if _, err := db.Exec(
			"INSERT INTO entries(id, timestamp, timestamp_unix_nano, message, tags) VALUES(?, ?, ?, ?, ?)",
			row.id, row.timestamp, 42, row.id, "[]",
		); err != nil {
			t.Fatalf("insert legacy row %q: %v", row.id, err)
		}
	}
	if _, err := db.Exec(`
		CREATE VIRTUAL TABLE entries_fts USING fts5(
			message,
			tags,
			content=entries,
			content_rowid=rowid
		);
		INSERT INTO entries_fts(entries_fts) VALUES('rebuild');
		CREATE TRIGGER entries_au AFTER UPDATE ON entries BEGIN
			INSERT INTO entries_fts(entries_fts, rowid, message, tags)
			VALUES('delete', old.rowid, old.message, old.tags);
			INSERT INTO entries_fts(rowid, message, tags)
			VALUES(new.rowid, new.message, new.tags);
		END;
	`); err != nil {
		t.Fatalf("create legacy full-text schema: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close legacy database: %v", err)
	}

	store, err := NewSqliteStore(dbPath)
	if err != nil {
		t.Fatalf("migrate legacy database: %v", err)
	}
	assertScopedUpdateTrigger(t, store.db)
	plusOne := time.FixedZone("offset", 60*60)
	minusFive := time.FixedZone("offset", -5*60*60)
	for _, entry := range []Entry{
		{ID: "new-cross-offset", Timestamp: time.Date(2026, 7, 10, 18, 0, 0, 500000000, minusFive), Message: "new cross offset"},
		{ID: "new-early", Timestamp: time.Date(2026, 7, 10, 1, 0, 0, 0, plusOne), Message: "new early"},
	} {
		if _, err := store.CreateEntry(entry); err != nil {
			t.Fatalf("create entry %q: %v", entry.ID, err)
		}
	}

	wantOrder := []string{"legacy-future", "new-cross-offset", "legacy-late", "legacy-numeric", "new-early", "corrupt"}
	listed, err := store.ListEntries(0)
	if err != nil {
		t.Fatalf("list mixed timestamp rows: %v", err)
	}
	if len(listed) != len(wantOrder) {
		t.Fatalf("listed entries = %d, want %d", len(listed), len(wantOrder))
	}
	for i, wantID := range wantOrder {
		if listed[i].ID != wantID {
			t.Fatalf("listed entry %d = %q, want %q; full order: %+v", i, listed[i].ID, wantID, listed)
		}
	}

	since := time.Date(2026, 7, 10, 21, 30, 0, 0, time.UTC)
	sinceResults, err := store.SearchEntries(&SearchFilter{Since: &since}, 0)
	if err != nil {
		t.Fatalf("search mixed timestamps since boundary: %v", err)
	}
	if len(sinceResults) != 3 || sinceResults[0].ID != "legacy-future" || sinceResults[1].ID != "new-cross-offset" || sinceResults[2].ID != "legacy-late" {
		t.Fatalf("since results = %+v, want legacy-future, new-cross-offset, legacy-late", sinceResults)
	}

	until := time.Date(2026, 7, 10, 11, 30, 0, 0, time.UTC)
	untilResults, err := store.SearchEntries(&SearchFilter{Until: &until}, 0)
	if err != nil {
		t.Fatalf("search mixed timestamps until boundary: %v", err)
	}
	if len(untilResults) != 2 || untilResults[0].ID != "legacy-numeric" || untilResults[1].ID != "new-early" {
		t.Fatalf("until results = %+v, want legacy-numeric then new-early", untilResults)
	}

	var firstLegacySeconds, firstLegacyNanoseconds int64
	var corruptSeconds, corruptNanoseconds any
	if err := store.db.QueryRow(
		"SELECT timestamp_unix_seconds, timestamp_nanosecond FROM entries WHERE id = ?",
		"legacy-late",
	).Scan(&firstLegacySeconds, &firstLegacyNanoseconds); err != nil {
		t.Fatalf("read backfilled legacy key: %v", err)
	}
	if err := store.db.QueryRow(
		"SELECT timestamp_unix_seconds, timestamp_nanosecond FROM entries WHERE id = ?",
		"corrupt",
	).Scan(&corruptSeconds, &corruptNanoseconds); err != nil {
		t.Fatalf("read corrupt legacy key: %v", err)
	}
	if corruptSeconds != nil || corruptNanoseconds != nil {
		t.Fatalf("corrupt legacy keys = (%v, %v), want (NULL, NULL)", corruptSeconds, corruptNanoseconds)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("close migrated store: %v", err)
	}

	reopened, err := NewSqliteStore(dbPath)
	if err != nil {
		t.Fatalf("reopen migrated database: %v", err)
	}
	defer reopened.Close()
	assertScopedUpdateTrigger(t, reopened.db)
	var reopenedLegacySeconds, reopenedLegacyNanoseconds int64
	if err := reopened.db.QueryRow(
		"SELECT timestamp_unix_seconds, timestamp_nanosecond FROM entries WHERE id = ?",
		"legacy-late",
	).Scan(&reopenedLegacySeconds, &reopenedLegacyNanoseconds); err != nil {
		t.Fatalf("read legacy key after idempotent reopen: %v", err)
	}
	if reopenedLegacySeconds != firstLegacySeconds || reopenedLegacyNanoseconds != firstLegacyNanoseconds {
		t.Fatalf(
			"legacy keys after reopen = (%d, %d), want unchanged (%d, %d)",
			reopenedLegacySeconds,
			reopenedLegacyNanoseconds,
			firstLegacySeconds,
			firstLegacyNanoseconds,
		)
	}
}

func TestSqliteMigratesOldestSchemaTimestampKeys(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "oldest.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open oldest database: %v", err)
	}
	if _, err := db.Exec(`
		CREATE TABLE entries (
			rowid INTEGER PRIMARY KEY AUTOINCREMENT,
			id TEXT UNIQUE NOT NULL,
			timestamp DATETIME NOT NULL,
			message TEXT NOT NULL,
			hostname TEXT,
			username TEXT,
			working_directory TEXT,
			tags TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);
		INSERT INTO entries(id, timestamp, message, tags) VALUES
			('oldest-future', '2500-01-02 03:04:05.123456789 +0000 UTC', 'needle future', '["alpha"]'),
			('oldest-past', '1600-01-02 03:04:05.000000006 +0000 UTC', 'needle past', '["beta"]');
		CREATE VIRTUAL TABLE entries_fts USING fts5(
			message,
			tags,
			content=entries,
			content_rowid=rowid
		);
		INSERT INTO entries_fts(entries_fts) VALUES('rebuild');
		CREATE TRIGGER entries_au AFTER UPDATE ON entries BEGIN
			INSERT INTO entries_fts(entries_fts, rowid, message, tags)
			VALUES('delete', old.rowid, old.message, old.tags);
			INSERT INTO entries_fts(rowid, message, tags)
			VALUES(new.rowid, new.message, new.tags);
		END;
	`); err != nil {
		t.Fatalf("prepare oldest database: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close oldest database: %v", err)
	}

	store, err := NewSqliteStore(dbPath)
	if err != nil {
		t.Fatalf("migrate oldest database: %v", err)
	}
	assertScopedUpdateTrigger(t, store.db)

	columns := make(map[string]bool)
	rows, err := store.db.Query("PRAGMA table_info(entries)")
	if err != nil {
		t.Fatalf("inspect migrated oldest schema: %v", err)
	}
	for rows.Next() {
		var cid int
		var name, columnType string
		var notNull, primaryKey int
		var defaultValue any
		if err := rows.Scan(&cid, &name, &columnType, &notNull, &defaultValue, &primaryKey); err != nil {
			_ = rows.Close()
			t.Fatalf("scan migrated oldest schema: %v", err)
		}
		columns[name] = true
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		t.Fatalf("iterate migrated oldest schema: %v", err)
	}
	if err := rows.Close(); err != nil {
		t.Fatalf("close migrated oldest schema rows: %v", err)
	}
	if !columns["timestamp_unix_seconds"] || !columns["timestamp_nanosecond"] {
		t.Fatalf("migrated columns = %v, want both timestamp composite columns", columns)
	}
	var indexExists int
	if err := store.db.QueryRow(`
		SELECT EXISTS(
			SELECT 1 FROM sqlite_master
			WHERE type = 'index' AND name = 'idx_entries_timestamp_instant'
		)
	`).Scan(&indexExists); err != nil {
		t.Fatalf("inspect migrated timestamp index: %v", err)
	}
	if indexExists != 1 {
		t.Fatal("migrated timestamp composite index is missing")
	}

	var futureSeconds, futureNanosecond int64
	if err := store.db.QueryRow(
		"SELECT timestamp_unix_seconds, timestamp_nanosecond FROM entries WHERE id = 'oldest-future'",
	).Scan(&futureSeconds, &futureNanosecond); err != nil {
		t.Fatalf("read oldest backfill keys: %v", err)
	}
	listed, err := store.ListEntries(0)
	if err != nil {
		t.Fatalf("list migrated oldest entries: %v", err)
	}
	if len(listed) != 2 || listed[0].ID != "oldest-future" || listed[1].ID != "oldest-past" {
		t.Fatalf("migrated oldest order = %+v", listed)
	}
	since := time.Date(2400, 1, 1, 0, 0, 0, 0, time.UTC)
	sinceResults, err := store.SearchEntries(&SearchFilter{Since: &since}, 0)
	if err != nil || len(sinceResults) != 1 || sinceResults[0].ID != "oldest-future" {
		t.Fatalf("migrated oldest since results = %+v, err=%v", sinceResults, err)
	}
	until := time.Date(1700, 1, 1, 0, 0, 0, 0, time.UTC)
	untilResults, err := store.SearchEntries(&SearchFilter{Until: &until}, 0)
	if err != nil || len(untilResults) != 1 || untilResults[0].ID != "oldest-past" {
		t.Fatalf("migrated oldest until results = %+v, err=%v", untilResults, err)
	}
	ftsResults, err := store.SearchEntries(&SearchFilter{Text: "needle", Tags: []string{"alpha"}}, 0)
	if err != nil || len(ftsResults) != 1 || ftsResults[0].ID != "oldest-future" {
		t.Fatalf("migrated oldest FTS/tag results = %+v, err=%v", ftsResults, err)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("close migrated oldest store: %v", err)
	}

	reopened, err := NewSqliteStore(dbPath)
	if err != nil {
		t.Fatalf("reopen migrated oldest database: %v", err)
	}
	defer reopened.Close()
	var reopenedSeconds, reopenedNanosecond int64
	if err := reopened.db.QueryRow(
		"SELECT timestamp_unix_seconds, timestamp_nanosecond FROM entries WHERE id = 'oldest-future'",
	).Scan(&reopenedSeconds, &reopenedNanosecond); err != nil {
		t.Fatalf("read oldest keys after reopen: %v", err)
	}
	if reopenedSeconds != futureSeconds || reopenedNanosecond != futureNanosecond {
		t.Fatalf(
			"oldest keys after reopen = (%d, %d), want (%d, %d)",
			reopenedSeconds,
			reopenedNanosecond,
			futureSeconds,
			futureNanosecond,
		)
	}
	reopenedFTS, err := reopened.SearchEntries(&SearchFilter{Text: "needle", Tags: []string{"alpha"}}, 0)
	if err != nil || len(reopenedFTS) != 1 || reopenedFTS[0].ID != "oldest-future" {
		t.Fatalf("reopened oldest FTS/tag results = %+v, err=%v", reopenedFTS, err)
	}
}

func assertScopedUpdateTrigger(t *testing.T, db *sql.DB) {
	t.Helper()
	var definition string
	if err := db.QueryRow(`
		SELECT sql FROM sqlite_master
		WHERE type = 'trigger' AND name = 'entries_au'
	`).Scan(&definition); err != nil {
		t.Fatalf("read entries update trigger: %v", err)
	}
	if !strings.Contains(definition, "AFTER UPDATE OF message, tags") {
		t.Fatalf("entries update trigger was not scoped to FTS fields: %s", definition)
	}
}

func TestSqliteTimestampMigrationIsAtomicOnBackfillFailure(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "blocked.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open legacy database: %v", err)
	}
	if _, err := db.Exec(`
		CREATE TABLE entries (
			rowid INTEGER PRIMARY KEY AUTOINCREMENT,
			id TEXT UNIQUE NOT NULL,
			timestamp DATETIME NOT NULL,
			timestamp_unix_nano INTEGER,
			message TEXT NOT NULL,
			hostname TEXT,
			username TEXT,
			working_directory TEXT,
			tags TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);
		INSERT INTO entries(id, timestamp, message, tags)
		VALUES('blocked', '2026-07-10 12:00:00.000000123 +0100 offset', 'blocked', '[]');
		CREATE VIRTUAL TABLE entries_fts USING fts5(
			message,
			tags,
			content=entries,
			content_rowid=rowid
		);
		INSERT INTO entries_fts(entries_fts) VALUES('rebuild');
		CREATE TRIGGER entries_au AFTER UPDATE ON entries BEGIN
			INSERT INTO entries_fts(entries_fts, rowid, message, tags)
			VALUES('delete', old.rowid, old.message, old.tags);
			INSERT INTO entries_fts(rowid, message, tags)
			VALUES(new.rowid, new.message, new.tags);
		END;
		CREATE TRIGGER block_entry_updates BEFORE UPDATE ON entries BEGIN
			SELECT RAISE(ABORT, 'blocked backfill');
		END;
	`); err != nil {
		t.Fatalf("prepare blocked legacy database: %v", err)
	}
	wantSchema := readSQLiteSchema(t, db)
	if err := db.Close(); err != nil {
		t.Fatalf("close blocked legacy database: %v", err)
	}

	if store, err := NewSqliteStore(dbPath); err == nil {
		_ = store.Close()
		t.Fatal("timestamp migration succeeded despite blocked backfill")
	}

	checkDB, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("reopen blocked database: %v", err)
	}
	defer checkDB.Close()
	if gotSchema := readSQLiteSchema(t, checkDB); !reflect.DeepEqual(gotSchema, wantSchema) {
		t.Fatalf("failed migration changed schema:\ngot:  %#v\nwant: %#v", gotSchema, wantSchema)
	}
	rows, err := checkDB.Query("PRAGMA table_info(entries)")
	if err != nil {
		t.Fatalf("inspect entries schema: %v", err)
	}
	defer rows.Close()
	for rows.Next() {
		var cid int
		var name, columnType string
		var notNull, primaryKey int
		var defaultValue any
		if err := rows.Scan(&cid, &name, &columnType, &notNull, &defaultValue, &primaryKey); err != nil {
			t.Fatalf("scan entries schema: %v", err)
		}
		if name == "timestamp_unix_seconds" || name == "timestamp_nanosecond" {
			t.Fatalf("failed migration left %s column behind", name)
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate entries schema: %v", err)
	}
}

func readSQLiteSchema(t *testing.T, db *sql.DB) map[string]string {
	t.Helper()
	rows, err := db.Query(`
		SELECT type, name, COALESCE(sql, '')
		FROM sqlite_master
		WHERE name NOT LIKE 'sqlite_%'
		ORDER BY type, name
	`)
	if err != nil {
		t.Fatalf("read SQLite schema: %v", err)
	}
	defer rows.Close()

	schema := make(map[string]string)
	for rows.Next() {
		var objectType, name, definition string
		if err := rows.Scan(&objectType, &name, &definition); err != nil {
			t.Fatalf("scan SQLite schema: %v", err)
		}
		schema[objectType+":"+name] = definition
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate SQLite schema: %v", err)
	}
	return schema
}
