// ABOUTME: End-to-end test of read-only `chronicle web` as a real process over real storage.
// ABOUTME: A separate `chronicle add` process writes an entry while the page's stream is open.

//go:build !windows

package e2e

import (
	"bufio"
	"context"
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"
)

const (
	// webStartupPrefix starts the first line that `chronicle web` prints. The URL follows it.
	webStartupPrefix = "Chronicle web interface: "
	// startupTimeout bounds the wait for a new process to print its first line, and for a stream
	// to open.
	startupTimeout = 10 * time.Second
	// entryTimeout is how long a new entry may take to reach an open stream. The server polls the
	// store once a second.
	entryTimeout = 3 * time.Second
	// shutdownTimeout is how long `chronicle web` may take to exit after an interrupt.
	shutdownTimeout = 3 * time.Second
)

// sseEvent is one event of a server-sent events stream.
type sseEvent struct {
	name string
	data string
}

// eventStream reads the events of one server-sent events response.
type eventStream struct {
	events chan sseEvent
	// err holds why reading stopped, or nil for a clean end. It is valid once events is closed.
	err error
}

// openEventStream connects to the stream at url and starts reading its events. Cleanup closes the
// connection.
func openEventStream(t *testing.T, url string) *eventStream {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		t.Fatal(err)
	}

	// The response has no end to wait for, so only the wait for its headers has a time limit.
	connectTimer := time.AfterFunc(startupTimeout, cancel)
	response, err := http.DefaultClient.Do(httpRequest)
	connectTimer.Stop()
	if err != nil {
		t.Fatalf("connect to %s: %v", url, err)
	}
	t.Cleanup(func() { _ = response.Body.Close() })
	if response.StatusCode != http.StatusOK {
		t.Fatalf("GET %s status = %d, want 200", url, response.StatusCode)
	}

	stream := &eventStream{events: make(chan sseEvent)}
	go func() {
		defer close(stream.events)
		scanner := bufio.NewScanner(response.Body)
		var event sseEvent
		for scanner.Scan() {
			switch line := scanner.Text(); {
			case line == "":
				// A block with neither a name nor data, such as the retry line, is not an event.
				if event != (sseEvent{}) {
					select {
					case stream.events <- event:
					case <-ctx.Done():
						return
					}
				}
				event = sseEvent{}
			case strings.HasPrefix(line, "event: "):
				event.name = strings.TrimPrefix(line, "event: ")
			case strings.HasPrefix(line, "data: "):
				event.data = strings.TrimPrefix(line, "data: ")
			}
		}
		stream.err = scanner.Err()
	}()
	return stream
}

// next returns the next event. It fails the test when none arrives within the given time or the
// stream ends first.
func (s *eventStream) next(t *testing.T, within time.Duration) sseEvent {
	t.Helper()
	select {
	case event, ok := <-s.events:
		if !ok {
			t.Fatalf("the stream ended (%v) before the next event", s.err)
		}
		return event
	case <-time.After(within):
		t.Fatalf("no event arrived within %s", within)
		return sseEvent{}
	}
}

// requireCleanEnd requires the stream to be over within the given time and to have ended the way
// a server ends it when it finishes a response, not by cutting the connection.
func (s *eventStream) requireCleanEnd(t *testing.T, within time.Duration) {
	t.Helper()
	select {
	case event, ok := <-s.events:
		if ok {
			t.Fatalf("the stream sent %+v, want it to end", event)
		}
		if s.err != nil {
			t.Fatalf("the stream ended with %v, want a clean end", s.err)
		}
	case <-time.After(within):
		t.Fatalf("the stream was still open %s after the server exited", within)
	}
}

func TestReadOnlyWebStreamsAndRefusesWrites(t *testing.T) {
	backends := []struct {
		backend string
		// seed adds an entry first. SQLite needs it, because the read-only viewer doesn't create
		// the database. Markdown needs nothing: the viewer starts on a data folder that isn't
		// there yet and must notice it appear.
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

			cmd, stdout := startChronicle(t, env, "web", "--read-only", "--address", "127.0.0.1:0", "--open=false")
			firstLine := readLine(t, stdout, startupTimeout)
			baseURL, found := strings.CutPrefix(firstLine, webStartupPrefix)
			if !found {
				t.Fatalf("first line of output = %q, want it to start with %q", firstLine, webStartupPrefix)
			}

			stream := openEventStream(t, baseURL+"/api/stream")
			if event := stream.next(t, startupTimeout); event.name != "ready" {
				t.Fatalf("first event = %+v, want ready", event)
			}

			// A separate process writes, as an agent does.
			runChronicle(t, env, "add", "e2e live entry", "--tag", "e2e")
			event := stream.next(t, entryTimeout)
			if event.name != "entry" {
				t.Fatalf("event = %+v, want an entry event", event)
			}
			var entry struct {
				Message string   `json:"message"`
				Tags    []string `json:"tags"`
			}
			if err := json.Unmarshal([]byte(event.data), &entry); err != nil {
				t.Fatalf("entry data %q is not an entry: %v", event.data, err)
			}
			if entry.Message != "e2e live entry" || len(entry.Tags) != 1 || entry.Tags[0] != "e2e" {
				t.Fatalf("entry = %+v, want the message and tag that add wrote", entry)
			}

			requireWritesRefused(t, baseURL)

			// The stream is still open. A graceful stop ends it.
			if err := cmd.Process.Signal(os.Interrupt); err != nil {
				t.Fatalf("interrupt chronicle web: %v", err)
			}
			if code := waitForExit(t, cmd, shutdownTimeout); code != 0 {
				t.Fatalf("chronicle web exited with status %d (%s), want 0", code, cmd.ProcessState)
			}
			stream.requireCleanEnd(t, time.Second)
		})
	}
}

// requireWritesRefused requires the server at baseURL to answer each write request with 405.
func requireWritesRefused(t *testing.T, baseURL string) {
	t.Helper()
	client := &http.Client{Timeout: 5 * time.Second}
	writes := []struct {
		method string
		path   string
		body   string
	}{
		{http.MethodPost, "/api/entries", `{"message":"nope"}`},
		{http.MethodPut, "/api/entries/x", `{"message":"nope"}`},
		{http.MethodDelete, "/api/entries/x", ""},
	}
	for _, write := range writes {
		httpRequest, err := http.NewRequest(write.method, baseURL+write.path, strings.NewReader(write.body))
		if err != nil {
			t.Fatal(err)
		}
		if write.body != "" {
			httpRequest.Header.Set("Content-Type", "application/json")
		}
		response, err := client.Do(httpRequest)
		if err != nil {
			t.Fatalf("%s %s: %v", write.method, write.path, err)
		}
		_ = response.Body.Close()
		if response.StatusCode != http.StatusMethodNotAllowed {
			t.Errorf("%s %s status = %d, want 405", write.method, write.path, response.StatusCode)
		}
	}
}
