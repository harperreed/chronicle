// ABOUTME: Poll-based feed of entries added to a markdown store after the feed opened
// ABOUTME: Only reads the day folders: it never takes the write lock and never creates anything

package storage

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// markdownRecentFolderWindow is how long a day folder stays under close watch after its last
// change. Inside the window the feed lists the folder on every poll even when its modified
// time matches the recorded one, because a file system with a coarse clock gives two changes
// made close together the same time.
const markdownRecentFolderWindow = 2 * time.Second

// markdownFeed is a Feed over the day folders of a markdown store.
//
// A writer publishes an entry by renaming a finished temp file into place, so a .md file is
// complete from the moment it appears. The feed lists folders and reads files. It never takes
// the store's lock and never creates anything.
type markdownFeed struct {
	dataDir string
	filter  *SearchFilter

	// folderModTimes holds each day folder's modified time as seen just before the feed last
	// listed the folder. The feed lists the folder again only after that time changes.
	folderModTimes map[string]time.Time
	// seenPaths holds the entry files the feed has read, plus the ones that already failed
	// to parse when it opened.
	seenPaths map[string]bool
	// seenIDs holds the IDs of the entries read so far. An edit can rename a file but keeps
	// the ID, so a known ID is not a new entry.
	seenIDs map[string]bool
}

// Compile-time check that markdownFeed implements Feed.
var _ Feed = (*markdownFeed)(nil)

// OpenFeed returns a feed of the entries added to the store after this call that match the
// filter. A nil filter matches every entry. Opening reads every entry file once.
func (s *MarkdownStore) OpenFeed(filter *SearchFilter) (Feed, error) {
	feed, err := newMarkdownFeed(s.dataDir, filter)
	if err != nil {
		return nil, err
	}
	return feed, nil
}

// newMarkdownFeed opens a feed on dataDir. A data directory that doesn't exist yet counts
// as an empty store, and the feed reports its entries once they appear.
func newMarkdownFeed(dataDir string, filter *SearchFilter) (*markdownFeed, error) {
	feed := &markdownFeed{
		dataDir:        dataDir,
		filter:         filter,
		folderModTimes: make(map[string]time.Time),
		seenPaths:      make(map[string]bool),
		seenIDs:        make(map[string]bool),
	}
	if _, err := feed.scan(true); err != nil {
		return nil, err
	}
	return feed, nil
}

// Poll returns the entries in files the feed hasn't read before, oldest first. It returns
// nil when there are none.
func (f *markdownFeed) Poll() ([]Entry, error) {
	return f.scan(false)
}

// scan reads the new files in every day folder that may have changed. While opening, it only
// records what is there and returns nothing, because files that exist at that point are not new.
func (f *markdownFeed) scan(opening bool) ([]Entry, error) {
	folders, err := dayFolders(f.dataDir)
	if err != nil {
		return nil, err
	}

	var fresh []Entry
	for _, folder := range folders {
		for _, path := range f.unreadEntryFiles(folder) {
			entry, isNew := f.readEntryFile(path, opening)
			if isNew && !opening && matchesFilter(entry, f.filter) {
				fresh = append(fresh, entry)
			}
		}
	}

	sort.SliceStable(fresh, func(i, j int) bool {
		return fresh[i].Timestamp.Before(fresh[j].Timestamp)
	})
	return fresh, nil
}

// unreadEntryFiles lists a day folder and returns the paths of its .md files that the feed
// hasn't read yet. It lists nothing when the folder's modified time is unchanged since the
// last listing and old enough that no writer could still be changing it.
func (f *markdownFeed) unreadEntryFiles(folder string) []string {
	info, err := os.Stat(folder)
	if err != nil {
		// The folder is gone or unreadable. Nothing is recorded, so the next poll looks again.
		return nil
	}
	modTime := info.ModTime()
	recorded, listed := f.folderModTimes[folder]
	if listed && modTime.Equal(recorded) && time.Since(modTime) > markdownRecentFolderWindow {
		return nil
	}

	files, err := os.ReadDir(folder)
	if err != nil {
		return nil
	}
	// Record the time seen before the listing. A file that lands during or after the listing
	// changes the folder's time, so the next poll lists the folder again.
	f.folderModTimes[folder] = modTime

	var paths []string
	for _, file := range files {
		path := filepath.Join(folder, file.Name())
		if !file.IsDir() && strings.HasSuffix(file.Name(), ".md") && !f.seenPaths[path] {
			paths = append(paths, path)
		}
	}
	return paths
}

// readEntryFile parses a file the feed hasn't read before. It reports whether the file holds
// a new entry: one whose ID the feed hasn't seen.
func (f *markdownFeed) readEntryFile(path string, opening bool) (Entry, bool) {
	entry, err := parseEntryFile(path)
	if err != nil {
		// The file may be half written. Leave it unrecorded so the next listing of its folder
		// tries it again, unless it was already there when the feed opened.
		if opening {
			f.seenPaths[path] = true
		}
		return Entry{}, false
	}

	f.seenPaths[path] = true
	if f.seenIDs[entry.ID] {
		return Entry{}, false // An edit that renamed the file.
	}
	f.seenIDs[entry.ID] = true
	return *entry, true
}

// dayFolders returns every <dataDir>/YYYY/MM/DD folder. Anything else under the data
// directory is skipped. A data directory that doesn't exist holds no folders. Below the data
// directory, a folder that vanishes or can't be read is skipped, as loadAllEntries skips it.
func dayFolders(dataDir string) ([]string, error) {
	years, err := digitFolders(dataDir, 4)
	if err != nil {
		return nil, fmt.Errorf("read data directory: %w", err)
	}

	var days []string
	for _, year := range years {
		months, err := digitFolders(year, 2)
		if err != nil {
			continue
		}
		for _, month := range months {
			monthDays, err := digitFolders(month, 2)
			if err != nil {
				continue
			}
			days = append(days, monthDays...)
		}
	}
	return days, nil
}

// digitFolders returns the folders directly inside dir whose names are exactly width digits.
// A missing dir has none.
func digitFolders(dir string, width int) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	folders := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() && isDigits(entry.Name(), width) {
			folders = append(folders, filepath.Join(dir, entry.Name()))
		}
	}
	return folders, nil
}

// isDigits reports whether name is exactly width ASCII digits.
func isDigits(name string, width int) bool {
	if len(name) != width {
		return false
	}
	for i := 0; i < len(name); i++ {
		if name[i] < '0' || name[i] > '9' {
			return false
		}
	}
	return true
}
