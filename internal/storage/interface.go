// ABOUTME: Storage interface for chronicle data backends
// ABOUTME: Defines the contract that all storage implementations must satisfy

package storage

import "time"

// Storage defines the interface for chronicle data persistence.
// Implementations include SqliteStore and MarkdownStore.
type Storage interface {
	// CreateEntry creates a new entry and returns its ID.
	CreateEntry(entry Entry) (string, error)

	// GetEntry retrieves an entry by ID.
	GetEntry(id string) (*Entry, error)

	// UpdateEntry updates an existing entry.
	UpdateEntry(entry Entry) error

	// DeleteEntry removes an entry by ID.
	DeleteEntry(id string) error

	// ListEntries returns entries ordered by timestamp descending.
	ListEntries(limit int) ([]Entry, error)

	// SearchEntries returns entries matching the filter.
	SearchEntries(filter *SearchFilter, limit int) ([]Entry, error)

	// LastModified returns when the store was last modified.
	LastModified() time.Time

	// Vacuum optimizes the storage (no-op for some backends).
	Vacuum() error

	// Reset clears all data from the storage.
	Reset() error

	// Close releases resources held by the storage backend.
	Close() error
}
