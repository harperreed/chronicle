// ABOUTME: Data migration between chronicle storage backends
// ABOUTME: Copies entries from source to destination storage

package storage

import "fmt"

// MigrateSummary holds counts of migrated entities.
type MigrateSummary struct {
	Entries int
}

// MigrateData copies all data from src to dst storage.
// It reads all entries from the source and creates them in the destination.
// The destination should be empty before calling this function.
func MigrateData(src, dst Storage) (*MigrateSummary, error) {
	summary := &MigrateSummary{}

	// List all entries (no limit)
	entries, err := src.ListEntries(0)
	if err != nil {
		return nil, fmt.Errorf("list source entries: %w", err)
	}

	for _, entry := range entries {
		if _, err := dst.CreateEntry(entry); err != nil {
			return nil, fmt.Errorf("create entry %q: %w", entry.ID, err)
		}
		summary.Entries++
	}

	return summary, nil
}
