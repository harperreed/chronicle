// ABOUTME: Storage interface for chronicle data backends
// ABOUTME: Defines the contract that all storage implementations must satisfy

package storage

import "time"

// Reader defines the read side of chronicle data persistence. A viewer holds a Reader, so it
// has no way to change entries. OpenSqliteReader and OpenMarkdownReader open the two backends
// for reading only.
type Reader interface {
	// GetEntry retrieves an entry by ID.
	GetEntry(id string) (*Entry, error)

	// ListEntries returns entries ordered by timestamp descending.
	ListEntries(limit int) ([]Entry, error)

	// SearchEntries returns entries matching the filter.
	SearchEntries(filter *SearchFilter, limit int) ([]Entry, error)

	// OpenFeed returns a feed of the entries added after this call that match the filter.
	// A nil filter matches every entry.
	OpenFeed(filter *SearchFilter) (Feed, error)

	// Close releases resources held by the storage backend.
	Close() error
}

// Storage defines the interface for chronicle data persistence: everything a Reader does, plus
// the methods that change data.
// Implementations include SqliteStore and MarkdownStore.
type Storage interface {
	Reader

	// CreateEntry creates a new entry and returns its ID.
	CreateEntry(entry Entry) (string, error)

	// UpdateEntry updates an existing entry.
	UpdateEntry(entry Entry) error

	// DeleteEntry removes an entry by ID.
	DeleteEntry(id string) error

	// LastModified returns when the store was last modified.
	LastModified() time.Time

	// Vacuum optimizes the storage (no-op for some backends).
	Vacuum() error

	// Reset clears all data from the storage.
	Reset() error
}

// Feed reports the entries that appear in a store after the feed was opened.
type Feed interface {
	// Poll returns the entries that appeared since the previous call, or since the feed
	// opened, and that match the feed's filter. They come oldest first by Timestamp.
	// It returns nil when nothing is new. Callers must not call Poll from several
	// goroutines at once.
	Poll() ([]Entry, error)
}
