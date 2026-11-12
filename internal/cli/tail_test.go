// ABOUTME: Tests for the tail command's entry formatting and its follow loop.
// ABOUTME: The follow tests read real markdown stores in temp folders and never run the root command.

package cli

import (
	"bufio"
	"context"
	"io"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/fatih/color"
	"github.com/harper/chronicle/internal/storage"
)

const (
	// tailTestInterval is how often the follow tests poll for new entries. It is short, so they run fast.
	tailTestInterval = 20 * time.Millisecond
	// tailTestWait is how long a follow test waits for a line that it expects.
	tailTestWait = 2 * time.Second
	// tailTestQuiet is how long a follow test waits to show that no line comes. It spans several polls.
	tailTestQuiet = 200 * time.Millisecond
)

// setTailTestColor turns colored output on or off for the test and restores the setting after it.
func setTailTestColor(t *testing.T, enabled bool) {
	t.Helper()
	previous := color.NoColor
	color.NoColor = !enabled
	t.Cleanup(func() { color.NoColor = previous })
}

func TestFormatTailEntryToday(t *testing.T) {
	setTailTestColor(t, false)
	now := time.Date(2026, 11, 11, 18, 0, 0, 0, time.Local)
	home := "/home/h"
	entry := storage.Entry{
		Timestamp:        time.Date(2026, 11, 11, 16, 4, 43, 0, time.Local),
		Message:          "started spaceport kata backlog work",
		WorkingDirectory: "/home/h/Public/src/2389/spaceport",
		Tags:             []string{"spaceport", "kata"},
	}

	want := "16:04:43  started spaceport kata backlog work  #spaceport #kata\n" +
		"          ~/Public/src/2389/spaceport\n"
	if got := formatTailEntry(entry, now, home); got != want {
		t.Errorf("formatTailEntry() = %q, want %q", got, want)
	}
}

func TestFormatTailEntryEarlierDayAndYear(t *testing.T) {
	setTailTestColor(t, false)
	now := time.Date(2026, 11, 11, 18, 0, 0, 0, time.Local)
	home := "/home/h"

	tests := []struct {
		name string
		at   time.Time
		// prefix is the time and the two spaces after it. The path line is indented by its width.
		prefix string
	}{
		{"earlier day of this year", time.Date(2026, 11, 10, 16, 4, 43, 0, time.Local), "Nov 10 16:04:43  "},
		{"day with one digit", time.Date(2026, 11, 9, 16, 4, 43, 0, time.Local), "Nov 9 16:04:43  "},
		{"earlier year", time.Date(2025, 3, 1, 16, 4, 43, 0, time.Local), "2025-03-01 16:04:43  "},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entry := storage.Entry{
				Timestamp:        tt.at,
				Message:          "worked",
				WorkingDirectory: "/home/h/proj",
				Tags:             []string{"x"},
			}

			want := tt.prefix + "worked  #x\n" + strings.Repeat(" ", len(tt.prefix)) + "~/proj\n"
			if got := formatTailEntry(entry, now, home); got != want {
				t.Errorf("formatTailEntry() = %q, want %q", got, want)
			}
		})
	}
}

func TestFormatTailEntryMultilineNoTagsNoDir(t *testing.T) {
	setTailTestColor(t, false)
	now := time.Date(2026, 11, 11, 18, 0, 0, 0, time.Local)
	entry := storage.Entry{
		Timestamp: time.Date(2026, 11, 11, 16, 4, 43, 0, time.Local),
		Message:   "one\ntwo",
	}

	want := "16:04:43  one\n          two\n"
	if got := formatTailEntry(entry, now, "/home/h"); got != want {
		t.Errorf("formatTailEntry() = %q, want %q", got, want)
	}
}

func TestFormatTailEntryHomeExactlyAndOutside(t *testing.T) {
	setTailTestColor(t, false)
	now := time.Date(2026, 11, 11, 18, 0, 0, 0, time.Local)
	home := "/home/h"

	tests := []struct {
		name             string
		workingDirectory string
		want             string
	}{
		{"home itself", "/home/h", "~"},
		{"outside home", "/srv/x", "/srv/x"},
		{"sibling folder that starts with the home folder's name", "/home/hank/x", "/home/hank/x"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entry := storage.Entry{
				Timestamp:        time.Date(2026, 11, 11, 16, 4, 43, 0, time.Local),
				Message:          "m",
				WorkingDirectory: tt.workingDirectory,
			}

			want := "16:04:43  m\n          " + tt.want + "\n"
			if got := formatTailEntry(entry, now, home); got != want {
				t.Errorf("formatTailEntry() = %q, want %q", got, want)
			}
		})
	}
}

func TestFormatTailEntryWithUnknownHomeKeepsPaths(t *testing.T) {
	setTailTestColor(t, false)
	now := time.Date(2026, 11, 11, 18, 0, 0, 0, time.Local)
	entry := storage.Entry{
		Timestamp:        time.Date(2026, 11, 11, 16, 4, 43, 0, time.Local),
		Message:          "m",
		WorkingDirectory: "/srv/x",
	}

	// An unknown home folder is the empty string. Every absolute path starts with "/", so it must
	// not count as being inside the home folder.
	want := "16:04:43  m\n          /srv/x\n"
	if got := formatTailEntry(entry, now, ""); got != want {
		t.Errorf("formatTailEntry() = %q, want %q", got, want)
	}
}

func TestFormatTailEntryConvertsToLocalTime(t *testing.T) {
	setTailTestColor(t, false)
	entry := storage.Entry{
		Timestamp: time.Date(2026, 11, 11, 16, 4, 43, 0, time.UTC),
		Message:   "from utc",
	}
	local := entry.Timestamp.In(time.Local)
	// A minute after the entry, so that both fall on the same local day in any zone.
	now := local.Add(time.Minute)

	wantPrefix := local.Format("15:04:05") + "  "
	if got := formatTailEntry(entry, now, "/home/h"); !strings.HasPrefix(got, wantPrefix) {
		t.Errorf("formatTailEntry() = %q, want it to start with %q", got, wantPrefix)
	}
}

// ansiEscape matches the color codes that fatih/color writes.
var ansiEscape = regexp.MustCompile("\x1b\\[[0-9;]*m")

func TestFormatTailEntryColorsTimePathAndTags(t *testing.T) {
	setTailTestColor(t, true)
	now := time.Date(2026, 11, 11, 18, 0, 0, 0, time.Local)
	entry := storage.Entry{
		Timestamp:        time.Date(2026, 11, 11, 16, 4, 43, 0, time.Local),
		Message:          "started work",
		WorkingDirectory: "/home/h/proj",
		Tags:             []string{"spaceport", "kata"},
	}

	got := formatTailEntry(entry, now, "/home/h")

	const faint, cyan = "\x1b[2m", "\x1b[36m"
	for _, want := range []string{faint + "16:04:43", cyan + "#spaceport #kata", faint + "~/proj"} {
		if !strings.Contains(got, want) {
			t.Errorf("formatTailEntry() = %q, want it to contain %q", got, want)
		}
	}
	// The color codes take no room: the indent matches the width of the time as it shows.
	wantText := "16:04:43  started work  #spaceport #kata\n          ~/proj\n"
	if text := ansiEscape.ReplaceAllString(got, ""); text != wantText {
		t.Errorf("formatTailEntry() without color codes = %q, want %q", text, wantText)
	}
}

// newTailTestStore returns a markdown store in a temp folder.
func newTailTestStore(t *testing.T) *storage.MarkdownStore {
	t.Helper()
	store, err := storage.NewMarkdownStore(t.TempDir())
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	return store
}

// addTailTestEntry writes an entry with the message, time and tags to the store.
func addTailTestEntry(t *testing.T, store storage.Storage, message string, at time.Time, tags ...string) {
	t.Helper()
	if _, err := store.CreateEntry(storage.Entry{Timestamp: at, Message: message, Tags: tags}); err != nil {
		t.Fatalf("failed to create entry %q: %v", message, err)
	}
}

// hookedReader calls feedOpened once the wrapped reader has opened a feed. From that moment on,
// entries that arrive in the store are new to runTail.
type hookedReader struct {
	storage.Reader
	feedOpened func()
}

func (r hookedReader) OpenFeed(filter *storage.SearchFilter) (storage.Feed, error) {
	feed, err := r.Reader.OpenFeed(filter)
	if err == nil {
		r.feedOpened()
	}
	return feed, err
}

// runningTail is a runTail call in the background whose output the test reads line by line.
type runningTail struct {
	lines  chan string
	done   chan struct{} // Closed once runTail has returned.
	err    error         // What runTail returned. Valid once done is closed.
	cancel context.CancelFunc
}

// startTail runs runTail over reader in the background. Cleanup stops it, so a failing test leaves
// nothing running.
func startTail(t *testing.T, reader storage.Reader, limit int, tags []string) *runningTail {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	pipeReader, pipeWriter := io.Pipe()
	tail := &runningTail{lines: make(chan string, 64), done: make(chan struct{}), cancel: cancel}

	go func() {
		defer close(tail.done)
		tail.err = runTail(ctx, pipeWriter, reader, limit, tags, tailTestInterval)
		_ = pipeWriter.Close()
	}()
	go func() {
		defer close(tail.lines)
		scanner := bufio.NewScanner(pipeReader)
		for scanner.Scan() {
			tail.lines <- scanner.Text()
		}
	}()
	t.Cleanup(func() {
		cancel()
		// Closing the read end fails a write that is waiting for a reader, so runTail can return.
		_ = pipeReader.Close()
		<-tail.done
	})
	return tail
}

// nextLine returns the next line runTail prints. It fails the test when no line comes in time.
func (tail *runningTail) nextLine(t *testing.T) string {
	t.Helper()
	select {
	case line, ok := <-tail.lines:
		if !ok {
			t.Fatal("runTail's output ended before the next line")
		}
		return line
	case <-time.After(tailTestWait):
		t.Fatalf("no line arrived within %s", tailTestWait)
		return ""
	}
}

// requireLineEndingWith requires the next line to end with the text.
func (tail *runningTail) requireLineEndingWith(t *testing.T, text string) {
	t.Helper()
	if line := tail.nextLine(t); !strings.HasSuffix(line, text) {
		t.Fatalf("next line = %q, want it to end with %q", line, text)
	}
}

// requireQuiet requires that no line comes during a few polls.
func (tail *runningTail) requireQuiet(t *testing.T) {
	t.Helper()
	select {
	case line := <-tail.lines:
		t.Fatalf("unexpected line %q", line)
	case <-time.After(tailTestQuiet):
	}
}

// stop ends runTail by canceling its context and returns what it returned. It fails the test when
// runTail does not return in time.
func (tail *runningTail) stop(t *testing.T) error {
	t.Helper()
	tail.cancel()
	select {
	case <-tail.done:
		return tail.err
	case <-time.After(tailTestWait):
		t.Fatalf("runTail did not return within %s of the cancel", tailTestWait)
		return nil
	}
}

func TestRunTailPrintsHistoryThenFollows(t *testing.T) {
	setTailTestColor(t, false)
	store := newTailTestStore(t)
	base := time.Now().Add(-time.Hour)
	addTailTestEntry(t, store, "entry one", base)
	addTailTestEntry(t, store, "entry two", base.Add(time.Minute))
	addTailTestEntry(t, store, "entry three", base.Add(2*time.Minute))

	tail := startTail(t, store, 2, nil)
	// The history shows the newest two entries, oldest first.
	tail.requireLineEndingWith(t, "  entry two")
	tail.requireLineEndingWith(t, "  entry three")

	addTailTestEntry(t, store, "entry four", time.Now())
	tail.requireLineEndingWith(t, "  entry four")

	if err := tail.stop(t); err != nil {
		t.Fatalf("runTail() = %v, want nil once the context ends", err)
	}
}

func TestRunTailWithZeroLimitPrintsNoHistory(t *testing.T) {
	setTailTestColor(t, false)
	store := newTailTestStore(t)
	addTailTestEntry(t, store, "old entry", time.Now().Add(-time.Hour))
	feedOpened := make(chan struct{})
	reader := hookedReader{Reader: store, feedOpened: func() { close(feedOpened) }}

	tail := startTail(t, reader, 0, nil)
	// The entry that arrives once the feed is open must be the first thing printed.
	select {
	case <-feedOpened:
	case <-time.After(tailTestWait):
		t.Fatal("runTail did not open its feed")
	}
	addTailTestEntry(t, store, "new entry", time.Now())
	tail.requireLineEndingWith(t, "  new entry")
}

func TestRunTailFiltersByTag(t *testing.T) {
	setTailTestColor(t, false)
	store := newTailTestStore(t)
	base := time.Now().Add(-time.Hour)
	addTailTestEntry(t, store, "old dropped", base, "drop")
	addTailTestEntry(t, store, "old kept", base.Add(time.Minute), "keep")

	tail := startTail(t, store, 10, []string{"keep"})
	tail.requireLineEndingWith(t, "  old kept  #keep")

	// The dropped entry comes first, so a filter that let it through would print it first.
	addTailTestEntry(t, store, "new dropped", time.Now(), "drop")
	addTailTestEntry(t, store, "new kept", time.Now(), "keep")
	tail.requireLineEndingWith(t, "  new kept  #keep")
	tail.requireQuiet(t)
}

func TestRunTailPrintsAnEntryOnceWhenHistoryAndFeedOverlap(t *testing.T) {
	setTailTestColor(t, false)
	store := newTailTestStore(t)
	addTailTestEntry(t, store, "before", time.Now().Add(-time.Hour))
	// Another process adds an entry after runTail opened its feed and before it reads the history,
	// so the entry is in both.
	reader := hookedReader{Reader: store, feedOpened: func() {
		entry := storage.Entry{Timestamp: time.Now(), Message: "raced"}
		if _, err := store.CreateEntry(entry); err != nil {
			t.Errorf("failed to create the racing entry: %v", err)
		}
	}}

	tail := startTail(t, reader, 10, nil)
	tail.requireLineEndingWith(t, "  before")
	tail.requireLineEndingWith(t, "  raced")

	// A second line for "raced" would come before this one.
	addTailTestEntry(t, store, "after", time.Now())
	tail.requireLineEndingWith(t, "  after")
	tail.requireQuiet(t)
}
