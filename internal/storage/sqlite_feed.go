// ABOUTME: Poll-based feed of entries added to a SQLite store after the feed opened.
// ABOUTME: Runs plain SELECT statements only, so it never takes the write lock and works on a read-only connection.

package storage

import (
	"database/sql"
	"fmt"
	"sort"
)

// sqliteFeed is a Feed over the entries table of a SQLite store.
//
// SQLite commits one writer at a time, and AUTOINCREMENT gives every new row a higher rowid than
// any row before it, so rowids rise in commit order. Every row with a lower rowid than one the
// feed has seen is therefore already committed, and the feed only has to remember the highest
// rowid it has read up to. It runs plain SELECT statements and never opens a transaction, so it
// never takes the write lock, and it holds no read open between polls, so it never holds back a
// WAL checkpoint.
type sqliteFeed struct {
	db     *sql.DB
	filter *SearchFilter
	// lastRowID is the highest rowid the feed has read up to, whether or not that row's entry
	// matched the filter, so a row that fails the filter is never read twice.
	lastRowID int64
}

// Compile-time check that sqliteFeed implements Feed.
var _ Feed = (*sqliteFeed)(nil)

// OpenFeed returns a feed of the entries added to the store after this call that match the
// filter. A nil filter matches every entry.
func (s *SqliteStore) OpenFeed(filter *SearchFilter) (Feed, error) {
	newest, err := newestRowID(s.db)
	if err != nil {
		return nil, err
	}
	return &sqliteFeed{db: s.db, filter: filter, lastRowID: newest}, nil
}

// Poll returns the matching entries in the rows added since the previous call, oldest first by
// Timestamp. It returns nil when there are none.
func (f *sqliteFeed) Poll() ([]Entry, error) {
	// Select only up to the newest rowid read here. A row committed after this statement has a
	// higher rowid, so the next poll finds it, and no poll misses a row.
	newest, err := newestRowID(f.db)
	if err != nil {
		return nil, err
	}
	if newest <= f.lastRowID {
		return nil, nil
	}

	query, args := sqliteEntryQuery(f.filter, []string{"e.rowid > ?", "e.rowid <= ?"}, f.lastRowID, newest)
	rows, err := f.db.Query(query+" ORDER BY e.rowid", args...)
	if err != nil {
		return nil, fmt.Errorf("query new entries: %w", err)
	}
	defer func() { _ = rows.Close() }()

	fresh, err := scanEntryRows(rows)
	if err != nil {
		// The cursor stays where it was, so the next poll reads the same rows again.
		return nil, err
	}
	f.lastRowID = newest

	// An entry can carry any timestamp, so the order of the rows isn't always the order of the
	// timestamps. The sort is stable, so entries with equal timestamps stay in row order.
	sort.SliceStable(fresh, func(i, j int) bool {
		return fresh[i].Timestamp.Before(fresh[j].Timestamp)
	})
	return fresh, nil
}

// newestRowID returns the highest rowid in the entries table, or 0 when the table is empty.
func newestRowID(db *sql.DB) (int64, error) {
	var rowID int64
	if err := db.QueryRow("SELECT COALESCE(MAX(rowid), 0) FROM entries").Scan(&rowID); err != nil {
		return 0, fmt.Errorf("read newest row: %w", err)
	}
	return rowID, nil
}
