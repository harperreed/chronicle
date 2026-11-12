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

// tailedEntryLine is what tail's first line for the entry that the test adds contains: the
// message, two spaces and the tag.
const tailedEntryLine = "e2e tail entry  #e2e"

func TestTailFollowsAnotherProcess(t *testing.T) {
	for _, backend := range []string{"markdown", "sqlite"} {
		t.Run(backend, func(t *testing.T) {
			env := newEnv(t, backend)
			// The seed gives tail a line of history to print. It also gives SQLite the database that
			// tail needs, because tail doesn't create one.
			runChronicle(t, env, "add", "seed")

			cmd, stdout := startChronicle(t, env, "tail", "-n", "1")
			// Tail opens its feed before it prints the history, and an entry is new to it only if it
			// arrives after that. So once the seed's line is here, the add below can't be too early.
			if line := readLine(t, stdout, startupTimeout); !strings.Contains(line, "seed") {
				t.Fatalf("first line of tail's output = %q, want the seed entry", line)
			}

			runChronicle(t, env, "add", "e2e tail entry", "--tag", "e2e")
			waitForLine(t, stdout, tailedEntryLine, entryTimeout)

			if err := cmd.Process.Signal(os.Interrupt); err != nil {
				t.Fatalf("interrupt chronicle tail: %v", err)
			}
			if code := waitForExit(t, cmd, shutdownTimeout); code != 0 {
				t.Fatalf("chronicle tail exited with status %d (%s), want 0", code, cmd.ProcessState)
			}
		})
	}
}

// waitForLine reads lines from r until one contains text. Other lines are skipped, such as the
// line that shows the seed entry's working directory. It fails the test when no such line arrives
// within the given time.
func waitForLine(t *testing.T, r *bufio.Reader, text string, within time.Duration) {
	t.Helper()
	deadline := time.Now().Add(within)
	for {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			t.Fatalf("no line containing %q arrived within %s", text, within)
		}
		if strings.Contains(readLine(t, r, remaining), text) {
			return
		}
	}
}
