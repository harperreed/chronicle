// ABOUTME: Markdown file-based storage backend for chronicle entries
// ABOUTME: Implements Storage interface using mdstore library with date-organized directories

package storage

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/harperreed/mdstore"
	"gopkg.in/yaml.v3"
)

// MarkdownStore provides file-based storage for chronicle entries using markdown files.
// Entries are organized in date-based directories: <dataDir>/YYYY/MM/DD/<slug>-<shortid>.md.
type MarkdownStore struct {
	dataDir      string
	lastModified time.Time
}

// Compile-time check that MarkdownStore implements Storage.
var _ Storage = (*MarkdownStore)(nil)

// NewMarkdownStore creates a new markdown-backed store rooted at dataDir.
func NewMarkdownStore(dataDir string) (*MarkdownStore, error) {
	if err := mdstore.EnsureDir(dataDir); err != nil {
		return nil, fmt.Errorf("create data directory: %w", err)
	}
	return &MarkdownStore{
		dataDir:      dataDir,
		lastModified: time.Now(),
	}, nil
}

// Close releases resources. For MarkdownStore this is a no-op.
func (s *MarkdownStore) Close() error {
	return nil
}

// entryFrontmatter holds the YAML frontmatter of an entry file.
type entryFrontmatter struct {
	ID               string   `yaml:"id"`
	Timestamp        string   `yaml:"timestamp"`
	Hostname         string   `yaml:"hostname,omitempty"`
	Username         string   `yaml:"username,omitempty"`
	WorkingDirectory string   `yaml:"working_directory,omitempty"`
	Tags             []string `yaml:"tags,omitempty"`
}

// entryDirPath returns the date-based directory path for an entry's timestamp.
func (s *MarkdownStore) entryDirPath(t time.Time) string {
	return filepath.Join(s.dataDir, t.Format("2006"), t.Format("01"), t.Format("02"))
}

// entryFileName generates a filename for an entry: <slug>-<shortid>.md.
func entryFileName(message string, id string) string {
	// Take first few words for the slug, keep it short
	slug := mdstore.Slugify(truncateForSlug(message))
	shortID := id
	if len(shortID) > 8 {
		shortID = shortID[:8]
	}
	return slug + "-" + shortID + ".md"
}

// truncateForSlug limits message text to a reasonable slug length.
func truncateForSlug(message string) string {
	words := strings.Fields(message)
	if len(words) > 5 {
		words = words[:5]
	}
	result := strings.Join(words, " ")
	if len(result) > 60 {
		result = result[:60]
	}
	return result
}

// renderEntry renders a complete entry file (frontmatter + message body).
func renderEntry(entry *Entry) (string, error) {
	fm := entryFrontmatter{
		ID:               entry.ID,
		Timestamp:        mdstore.FormatTime(entry.Timestamp.UTC()),
		Hostname:         entry.Hostname,
		Username:         entry.Username,
		WorkingDirectory: entry.WorkingDirectory,
		Tags:             entry.Tags,
	}

	content, err := mdstore.RenderFrontmatter(&fm, "\n"+entry.Message+"\n")
	if err != nil {
		return "", fmt.Errorf("render entry frontmatter: %w", err)
	}
	return content, nil
}

// parseEntryFile reads and parses an entry markdown file.
func parseEntryFile(path string) (*Entry, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	yamlStr, body := mdstore.ParseFrontmatter(string(data))
	if yamlStr == "" {
		return nil, fmt.Errorf("no frontmatter found in %s", path)
	}

	var fm entryFrontmatter
	if err := yaml.Unmarshal([]byte(yamlStr), &fm); err != nil {
		return nil, fmt.Errorf("parse frontmatter in %s: %w", path, err)
	}

	timestamp, err := mdstore.ParseTime(fm.Timestamp)
	if err != nil {
		return nil, fmt.Errorf("parse timestamp in %s: %w", path, err)
	}

	return &Entry{
		ID:               fm.ID,
		Timestamp:        timestamp,
		Message:          strings.TrimSpace(body),
		Hostname:         fm.Hostname,
		Username:         fm.Username,
		WorkingDirectory: fm.WorkingDirectory,
		Tags:             fm.Tags,
	}, nil
}

// CreateEntry creates a new entry and returns its ID.
func (s *MarkdownStore) CreateEntry(entry Entry) (string, error) {
	if entry.ID == "" {
		entry.ID = uuid.New().String()
	}

	if entry.Timestamp.IsZero() {
		entry.Timestamp = time.Now()
	}

	dir := s.entryDirPath(entry.Timestamp)
	fileName := entryFileName(entry.Message, entry.ID)
	filePath := filepath.Join(dir, fileName)

	content, err := renderEntry(&entry)
	if err != nil {
		return "", fmt.Errorf("render entry: %w", err)
	}

	var writeErr error
	err = mdstore.WithLock(s.dataDir, func() error {
		writeErr = mdstore.AtomicWrite(filePath, []byte(content))
		return writeErr
	})
	if err != nil {
		return "", fmt.Errorf("write entry file: %w", err)
	}

	s.lastModified = time.Now()
	return entry.ID, nil
}

// GetEntry retrieves an entry by ID.
func (s *MarkdownStore) GetEntry(id string) (*Entry, error) {
	path, err := s.findEntryFile(id)
	if err != nil {
		return nil, err
	}

	return parseEntryFile(path)
}

// UpdateEntry updates an existing entry.
func (s *MarkdownStore) UpdateEntry(entry Entry) error {
	if entry.ID == "" {
		return fmt.Errorf("entry ID required")
	}

	oldPath, err := s.findEntryFile(entry.ID)
	if err != nil {
		return err
	}

	content, err := renderEntry(&entry)
	if err != nil {
		return fmt.Errorf("render entry: %w", err)
	}

	// Determine new path based on potentially changed timestamp
	dir := s.entryDirPath(entry.Timestamp)
	fileName := entryFileName(entry.Message, entry.ID)
	newPath := filepath.Join(dir, fileName)

	return mdstore.WithLock(s.dataDir, func() error {
		if err := mdstore.AtomicWrite(newPath, []byte(content)); err != nil {
			return fmt.Errorf("write updated entry: %w", err)
		}

		// Remove old file if it moved
		if oldPath != newPath {
			_ = os.Remove(oldPath)
			// Clean up empty directories
			s.cleanEmptyDirs(filepath.Dir(oldPath))
		}

		s.lastModified = time.Now()
		return nil
	})
}

// DeleteEntry removes an entry by ID.
func (s *MarkdownStore) DeleteEntry(id string) error {
	path, err := s.findEntryFile(id)
	if err != nil {
		return err
	}

	return mdstore.WithLock(s.dataDir, func() error {
		if err := os.Remove(path); err != nil {
			return fmt.Errorf("delete entry file: %w", err)
		}

		// Clean up empty directories
		s.cleanEmptyDirs(filepath.Dir(path))

		s.lastModified = time.Now()
		return nil
	})
}

// ListEntries returns entries ordered by timestamp descending.
func (s *MarkdownStore) ListEntries(limit int) ([]Entry, error) {
	return s.SearchEntries(nil, limit)
}

// SearchEntries returns entries matching the filter.
func (s *MarkdownStore) SearchEntries(filter *SearchFilter, limit int) ([]Entry, error) {
	allEntries, err := s.loadAllEntries()
	if err != nil {
		return nil, err
	}

	// Apply filters
	var filtered []Entry
	for _, entry := range allEntries {
		if !matchesFilter(entry, filter) {
			continue
		}
		filtered = append(filtered, entry)
	}

	// Sort by timestamp descending
	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].Timestamp.After(filtered[j].Timestamp)
	})

	// Apply limit
	if limit > 0 && len(filtered) > limit {
		filtered = filtered[:limit]
	}

	return filtered, nil
}

// LastModified returns when the store was last modified.
func (s *MarkdownStore) LastModified() time.Time {
	return s.lastModified
}

// Vacuum is a no-op for MarkdownStore.
func (s *MarkdownStore) Vacuum() error {
	return nil
}

// Reset clears all data from the store.
func (s *MarkdownStore) Reset() error {
	return mdstore.WithLock(s.dataDir, func() error {
		entries, err := os.ReadDir(s.dataDir)
		if err != nil {
			return fmt.Errorf("read data directory: %w", err)
		}

		for _, entry := range entries {
			// Skip lock file
			if entry.Name() == ".lock" {
				continue
			}
			path := filepath.Join(s.dataDir, entry.Name())
			if err := os.RemoveAll(path); err != nil {
				return fmt.Errorf("remove %s: %w", path, err)
			}
		}

		s.lastModified = time.Now()
		return nil
	})
}

// findEntryFile locates the file for an entry by its ID.
// Walks the entire directory tree looking for the file.
func (s *MarkdownStore) findEntryFile(id string) (string, error) {
	var foundPath string

	err := filepath.Walk(s.dataDir, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return nil //nolint:nilerr // intentionally skip walk errors to continue traversal
		}
		if info.IsDir() || !strings.HasSuffix(path, ".md") {
			return nil
		}

		// Quick check: the filename should contain the short ID
		shortID := id
		if len(shortID) > 8 {
			shortID = shortID[:8]
		}
		if !strings.Contains(filepath.Base(path), shortID) {
			return nil
		}

		// Read and verify full ID
		entry, parseErr := parseEntryFile(path)
		if parseErr != nil {
			return nil //nolint:nilerr // skip unparseable files
		}
		if entry.ID == id {
			foundPath = path
			return filepath.SkipAll
		}
		return nil
	})

	if err != nil {
		return "", fmt.Errorf("search for entry: %w", err)
	}

	if foundPath == "" {
		return "", fmt.Errorf("entry not found: %s", id)
	}

	return foundPath, nil
}

// loadAllEntries reads all entry files from the data directory.
func (s *MarkdownStore) loadAllEntries() ([]Entry, error) {
	var entries []Entry

	err := filepath.Walk(s.dataDir, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return nil //nolint:nilerr // intentionally skip walk errors to continue traversal
		}
		if info.IsDir() || !strings.HasSuffix(path, ".md") {
			return nil
		}

		entry, parseErr := parseEntryFile(path)
		if parseErr != nil {
			return nil //nolint:nilerr // skip malformed files
		}

		entries = append(entries, *entry)
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("walk data directory: %w", err)
	}

	return entries, nil
}

// matchesFilter checks whether an entry matches the given search filter.
// For text search in markdown mode, uses case-insensitive string containment
// instead of FTS5 matching.
func matchesFilter(entry Entry, filter *SearchFilter) bool {
	if filter == nil {
		return true
	}

	// Text search: case-insensitive containment on message and tags
	if filter.Text != "" {
		textLower := strings.ToLower(filter.Text)
		messageLower := strings.ToLower(entry.Message)
		tagsStr := strings.ToLower(strings.Join(entry.Tags, " "))

		if !strings.Contains(messageLower, textLower) && !strings.Contains(tagsStr, textLower) {
			return false
		}
	}

	// Date range: since
	if filter.Since != nil {
		if entry.Timestamp.Before(*filter.Since) {
			return false
		}
	}

	// Date range: until
	if filter.Until != nil {
		if entry.Timestamp.After(*filter.Until) {
			return false
		}
	}

	// Tag filter (OR logic, matching BBS pattern)
	if len(filter.Tags) > 0 {
		hasMatchingTag := false
		for _, filterTag := range filter.Tags {
			for _, entryTag := range entry.Tags {
				if entryTag == filterTag {
					hasMatchingTag = true
					break
				}
			}
			if hasMatchingTag {
				break
			}
		}
		if !hasMatchingTag {
			return false
		}
	}

	return true
}

// cleanEmptyDirs removes empty parent directories up to the data directory root.
func (s *MarkdownStore) cleanEmptyDirs(dir string) {
	for dir != s.dataDir {
		entries, err := os.ReadDir(dir)
		if err != nil || len(entries) > 0 {
			return
		}
		_ = os.Remove(dir)
		dir = filepath.Dir(dir)
	}
}
