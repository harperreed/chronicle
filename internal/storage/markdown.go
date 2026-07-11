// ABOUTME: Markdown file-based storage backend for chronicle entries
// ABOUTME: Implements Storage interface using mdstore library with date-organized directories

package storage

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/harperreed/mdstore"
	"gopkg.in/yaml.v3"
)

// MarkdownStore provides file-based storage for chronicle entries using markdown files.
// Entries are organized in date-based directories: <dataDir>/YYYY/MM/DD/<slug>-<id-digest>.md.
type MarkdownStore struct {
	dataDir        string
	lastModifiedMu sync.RWMutex
	lastModified   time.Time
}

// Compile-time check that MarkdownStore implements Storage.
var _ Storage = (*MarkdownStore)(nil)

var (
	errMarkdownEntryNotFound      = errors.New("entry not found")
	errMarkdownEntryAlreadyExists = errors.New("entry already exists")
)

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

// entryFileName generates a filename for an entry: <slug>-<id-digest>.md.
func entryFileName(message string, id string) string {
	// Take first few words for the slug, keep it short
	slug := mdstore.Slugify(truncateForSlug(message))
	return slug + "-" + entryIDFileComponent(id) + ".md"
}

// entryIDFileComponent hashes the full ID into a fixed-length lowercase filename component.
func entryIDFileComponent(id string) string {
	digest := sha256.Sum256([]byte(id))
	return hex.EncodeToString(digest[:])
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
		Timestamp:        mdstore.FormatTime(entry.Timestamp),
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

// moveEntryFile publishes content at a new path without overwriting an existing file.
// The old path remains authoritative unless both publishing and old-path removal succeed.
func moveEntryFile(oldPath, newPath string, content []byte) error {
	if err := mdstore.EnsureDir(filepath.Dir(newPath)); err != nil {
		return fmt.Errorf("create updated entry directory: %w", err)
	}

	tmp, err := os.CreateTemp(filepath.Dir(newPath), ".tmp-*")
	if err != nil {
		return fmt.Errorf("create updated entry temporary file: %w", err)
	}
	tmpPath := tmp.Name()
	tmpClosed := false
	defer func() {
		if !tmpClosed {
			_ = tmp.Close()
		}
		_ = os.Remove(tmpPath)
	}()

	if _, err := tmp.Write(content); err != nil {
		return fmt.Errorf("write updated entry temporary file: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		return fmt.Errorf("sync updated entry temporary file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close updated entry temporary file: %w", err)
	}
	tmpClosed = true

	if err := os.Link(tmpPath, newPath); err != nil {
		return fmt.Errorf("publish updated entry: %w", err)
	}
	if err := os.Remove(tmpPath); err != nil {
		rollbackErr := os.Remove(newPath)
		return errors.Join(
			fmt.Errorf("remove updated entry temporary file: %w", err),
			wrapRollbackError(rollbackErr),
		)
	}
	if err := os.Remove(oldPath); err != nil {
		rollbackErr := os.Remove(newPath)
		return errors.Join(
			fmt.Errorf("remove previous entry file: %w", err),
			wrapRollbackError(rollbackErr),
		)
	}
	return nil
}

func wrapRollbackError(err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("rollback updated entry: %w", err)
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
	hasSuppliedID := entry.ID != ""
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

	err = mdstore.WithLock(s.dataDir, func() error {
		if hasSuppliedID {
			if checkErr := s.checkEntryIDAvailable(entry.ID); checkErr != nil {
				return checkErr
			}
		}
		return mdstore.AtomicWrite(filePath, []byte(content))
	})
	if err != nil {
		return "", fmt.Errorf("write entry file: %w", err)
	}

	s.markModified()
	return entry.ID, nil
}

// checkEntryIDAvailable strictly inspects every markdown file before a supplied ID is created.
func (s *MarkdownStore) checkEntryIDAvailable(id string) error {
	err := filepath.Walk(s.dataDir, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return fmt.Errorf("inspect %s: %w", path, walkErr)
		}
		if info.IsDir() || !strings.HasSuffix(path, ".md") {
			return nil
		}

		entry, parseErr := parseEntryFile(path)
		if parseErr != nil {
			return fmt.Errorf("inspect entry file %s: %w", path, parseErr)
		}
		if entry.ID == id {
			return fmt.Errorf("%w: %s", errMarkdownEntryAlreadyExists, id)
		}
		return nil
	})
	if err == nil || errors.Is(err, errMarkdownEntryAlreadyExists) {
		return err
	}
	return fmt.Errorf("check entry ID uniqueness: %w", err)
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

	content, err := renderEntry(&entry)
	if err != nil {
		return fmt.Errorf("render entry: %w", err)
	}

	// Determine new path based on potentially changed timestamp
	dir := s.entryDirPath(entry.Timestamp)
	fileName := entryFileName(entry.Message, entry.ID)
	newPath := filepath.Join(dir, fileName)

	return mdstore.WithLock(s.dataDir, func() error {
		oldPath, err := s.findEntryFile(entry.ID)
		if err != nil {
			return err
		}

		if oldPath == newPath {
			if err := mdstore.AtomicWrite(newPath, []byte(content)); err != nil {
				return fmt.Errorf("write updated entry: %w", err)
			}
		} else {
			if err := moveEntryFile(oldPath, newPath, []byte(content)); err != nil {
				return fmt.Errorf("move updated entry: %w", err)
			}
			s.cleanEmptyDirs(filepath.Dir(oldPath))
		}

		s.markModified()
		return nil
	})
}

// DeleteEntry removes an entry by ID.
func (s *MarkdownStore) DeleteEntry(id string) error {
	return mdstore.WithLock(s.dataDir, func() error {
		path, err := s.findEntryFile(id)
		if err != nil {
			return err
		}
		if err := os.Remove(path); err != nil {
			return fmt.Errorf("delete entry file: %w", err)
		}

		// Clean up empty directories
		s.cleanEmptyDirs(filepath.Dir(path))

		s.markModified()
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

	matchCount := 0
	for _, entry := range allEntries {
		if matchesFilter(entry, filter) {
			matchCount++
		}
	}
	if matchCount == 0 {
		return nil, nil
	}

	// Apply filters with exact capacity while preserving nil for no matches.
	filtered := make([]Entry, 0, matchCount)
	for _, entry := range allEntries {
		if matchesFilter(entry, filter) {
			filtered = append(filtered, entry)
		}
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
	s.lastModifiedMu.RLock()
	defer s.lastModifiedMu.RUnlock()
	return s.lastModified
}

// markModified records a successful mutation.
func (s *MarkdownStore) markModified() {
	s.lastModifiedMu.Lock()
	s.lastModified = time.Now()
	s.lastModifiedMu.Unlock()
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

		s.markModified()
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

		// New filenames contain the full ID digest; legacy filenames contain the short ID.
		idDigest := entryIDFileComponent(id)
		shortID := id
		if len(shortID) > 8 {
			shortID = shortID[:8]
		}
		fileName := filepath.Base(path)
		if !strings.Contains(fileName, idDigest) && !strings.Contains(fileName, shortID) {
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
		return "", fmt.Errorf("%w: %s", errMarkdownEntryNotFound, id)
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
