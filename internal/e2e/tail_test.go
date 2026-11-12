// ABOUTME: End-to-end test of `chronicle tail` as a real process over real storage.
// ABOUTME: A separate `chronicle add` process writes an entry while tail follows the store.

//go:build !windows

package e2e

import (
	"bufio"
	"os"
	"strings"
	"testing"
	"time"
)

const (
	// tailedEntryLine is what the first line of tail's output for the entry that these tests add
	// contains: the message, two spaces and the tag.
	tailedEntryLine = "e2e tail entry  #e2e"
	// maxTailAdds is how many entries addUntilTailPrints adds at most.
	maxTailAdds = 5
)

func TestTailFollowsAnotherProcess(t *testing.T) {
	backends := []struct {
		backend string
		// seed adds an entry first. SQLite needs it, because tail doesn't create the database.
		// Markdown needs nothing: tail starts on a data folder that isn't there yet and must
		// notice it appear.
		seed bool
	}{
		{"markdown", false},
		{"sqlite", true},
	}
	for _, tt := range backends {
		t.Run(tt.backend, func(t *testing.T) {
			env := newEnv(t, tt.backend)
			if tt.seed {
				runChronicle(t, env, "add", "seed")
			}

			// -n 0 prints no history, so the first line is the entry that the add below writes.
			cmd, stdout := startChronicle(t, env, "tail", "-n", "0")
			line := addUntilTailPrints(t, env, stdout)
			if !strings.Contains(line, tailedEntryLine) {
				t.Fatalf("first line of tail's output = %q, want it to contain %q", line, tailedEntryLine)
			}

			if err := cmd.Process.Signal(os.Interrupt); err != nil {
				t.Fatalf("interrupt chronicle tail: %v", err)
			}
			if code := waitForExit(t, cmd, shutdownTimeout); code != 0 {
				t.Fatalf("chronicle tail exited with status %d (%s), want 0", code, cmd.ProcessState)
			}
		})
	}
}

// addUntilTailPrints runs `chronicle add` until the tail process prints a line, and returns that
// line. Tail prints only the entries added after it opened its feed, and nothing it prints says
// when that is, so an entry that an add writes too early is never printed. Adding again makes the
// test wait as long as tail's startup takes, with no fixed sleep. Each add gets entryTimeout to
// show up, the time the web stream gets.
func addUntilTailPrints(t *testing.T, env []string, stdout *bufio.Reader) string {
	t.Helper()
	lines := make(chan string, 64)
	go func() {
		defer close(lines)
		for {
			line, err := stdout.ReadString('\n')
			if err != nil {
				return
			}
			lines <- strings.TrimRight(line, "\r\n")
		}
	}()

	for range maxTailAdds {
		runChronicle(t, env, "add", "e2e tail entry", "--tag", "e2e")
		select {
		case line, ok := <-lines:
			if !ok {
				t.Fatal("tail's output ended before it printed a line")
			}
			return line
		case <-time.After(entryTimeout):
		}
	}
	t.Fatalf("tail printed nothing within %s of each of %d adds", entryTimeout, maxTailAdds)
	return ""
}
