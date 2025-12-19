// ABOUTME: Entry model and CRUD operations for chronicle entries
// ABOUTME: Uses type-prefixed keys (entry:uuid) with denormalized tags
package charm

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/dgraph-io/badger/v3"
	"github.com/google/uuid"
)

// Entry represents a chronicle log entry.
type Entry struct {
	ID               string    `json:"id"`
	Timestamp        time.Time `json:"timestamp"`
	Message          string    `json:"message"`
	Hostname         string    `json:"hostname"`
	Username         string    `json:"username"`
	WorkingDirectory string    `json:"working_directory"`
	Tags             []string  `json:"tags"`
}

// entryKey returns the KV key for an entry.
func entryKey(id string) []byte {
	return []byte(EntryPrefix + id)
}

// CreateEntry creates a new entry and returns its ID.
func (c *Client) CreateEntry(entry Entry) (string, error) {
	// Generate UUID if not provided
	if entry.ID == "" {
		entry.ID = uuid.New().String()
	}

	// Set timestamp if not provided
	if entry.Timestamp.IsZero() {
		entry.Timestamp = time.Now()
	}

	key := entryKey(entry.ID)
	if err := c.SetJSON(key, entry); err != nil {
		return "", fmt.Errorf("create entry: %w", err)
	}

	return entry.ID, nil
}

// GetEntry retrieves an entry by ID.
func (c *Client) GetEntry(id string) (*Entry, error) {
	key := entryKey(id)
	var entry Entry
	if err := c.GetJSON(key, &entry); err != nil {
		return nil, fmt.Errorf("get entry: %w", err)
	}
	return &entry, nil
}

// UpdateEntry updates an existing entry.
func (c *Client) UpdateEntry(entry Entry) error {
	if entry.ID == "" {
		return fmt.Errorf("entry ID required")
	}
	key := entryKey(entry.ID)
	if err := c.SetJSON(key, entry); err != nil {
		return fmt.Errorf("update entry: %w", err)
	}
	return nil
}

// DeleteEntry removes an entry by ID.
func (c *Client) DeleteEntry(id string) error {
	key := entryKey(id)
	if err := c.Delete(key); err != nil {
		return fmt.Errorf("delete entry: %w", err)
	}
	return nil
}

// ListEntries returns entries, ordered by timestamp descending.
func (c *Client) ListEntries(limit int) ([]Entry, error) {
	return c.SearchEntries(nil, limit)
}

// SearchFilter defines search criteria.
type SearchFilter struct {
	Text  string
	Tags  []string
	Since *time.Time
	Until *time.Time
}

// SearchEntries returns entries matching the filter.
func (c *Client) SearchEntries(filter *SearchFilter, limit int) ([]Entry, error) {
	var entries []Entry

	err := c.View(func(txn *badger.Txn) error {
		it := txn.NewIterator(badger.DefaultIteratorOptions)
		defer it.Close()

		prefix := []byte(EntryPrefix)
		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			item := it.Item()
			err := item.Value(func(val []byte) error {
				var entry Entry
				if err := json.Unmarshal(val, &entry); err != nil {
					// Skip invalid entries (corrupted data) - intentionally ignoring error
					return nil //nolint:nilerr
				}

				if matchesFilter(&entry, filter) {
					entries = append(entries, entry)
				}
				return nil
			})
			if err != nil {
				return err
			}
		}
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("search entries: %w", err)
	}

	// Sort by timestamp descending
	sortEntriesByTimestamp(entries)

	// Apply limit
	if limit > 0 && len(entries) > limit {
		entries = entries[:limit]
	}

	return entries, nil
}

// matchesFilter checks if an entry matches the search filter.
func matchesFilter(entry *Entry, filter *SearchFilter) bool {
	if filter == nil {
		return true
	}

	// Text search (case-insensitive substring match)
	if filter.Text != "" {
		text := strings.ToLower(filter.Text)
		message := strings.ToLower(entry.Message)
		if !strings.Contains(message, text) {
			return false
		}
	}

	// Tag filter (entry must have at least one matching tag)
	if len(filter.Tags) > 0 {
		if !hasAnyTag(entry.Tags, filter.Tags) {
			return false
		}
	}

	// Date range filter
	if filter.Since != nil && entry.Timestamp.Before(*filter.Since) {
		return false
	}
	if filter.Until != nil && entry.Timestamp.After(*filter.Until) {
		return false
	}

	return true
}

// hasAnyTag checks if entryTags contains any of filterTags.
func hasAnyTag(entryTags, filterTags []string) bool {
	tagSet := make(map[string]bool)
	for _, t := range entryTags {
		tagSet[strings.ToLower(t)] = true
	}
	for _, t := range filterTags {
		if tagSet[strings.ToLower(t)] {
			return true
		}
	}
	return false
}

// sortEntriesByTimestamp sorts entries by timestamp descending (most recent first).
func sortEntriesByTimestamp(entries []Entry) {
	// Simple insertion sort (entries are usually already mostly sorted)
	for i := 1; i < len(entries); i++ {
		for j := i; j > 0 && entries[j].Timestamp.After(entries[j-1].Timestamp); j-- {
			entries[j], entries[j-1] = entries[j-1], entries[j]
		}
	}
}
