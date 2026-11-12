// ABOUTME: Tests for the server-sent events stream at /api/stream.
// ABOUTME: They run real HTTP servers over real stores and poll every 20 ms.

package web

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/harper/chronicle/internal/storage"
)

const (
	// streamTestPollInterval makes a new entry reach a client within a few milliseconds.
	streamTestPollInterval = 20 * time.Millisecond
	// streamTestWait bounds every wait on a stream. It is far above the poll interval, so a slow
	// machine or the race detector doesn't fail a test that works.
	streamTestWait = 5 * time.Second
	// getBodyTimeout bounds reading a plain response, which a working server finishes at once.
	getBodyTimeout = 2 * time.Second
)

// sseEvent is one event read from a stream.
type sseEvent struct {
	Name string
	Data string
}

// streamClient reads the lines of one open event stream.
type streamClient struct {
	response *http.Response
	lines    chan string
	cancel   context.CancelFunc
	// err holds why reading stopped, or nil for a clean end. It is valid once lines is closed.
	err error
}

// newMarkdownStreamStore returns an empty markdown store in a temp folder.
func newMarkdownStreamStore(t *testing.T) *storage.MarkdownStore {
	t.Helper()
	store, err := storage.NewMarkdownStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store
}

// newSqliteStreamStore returns an empty SQLite store in a temp folder.
func newSqliteStreamStore(t *testing.T) *storage.SqliteStore {
	t.Helper()
	store, err := storage.NewSqliteStore(filepath.Join(t.TempDir(), "chronicle.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store
}

// streamServer serves store over HTTP with a fast poll interval. A writeTimeout above zero sets
// the HTTP server's WriteTimeout. Cleanup stops the streams before it closes the HTTP server,
// because Close waits for every open request and a stream never ends on its own.
func streamServer(t *testing.T, store storage.Storage, writeTimeout time.Duration) (*Server, *httptest.Server) {
	t.Helper()
	app, err := New(store, Options{PollInterval: streamTestPollInterval})
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewUnstartedServer(app.Handler())
	server.Config.WriteTimeout = writeTimeout
	server.Start()
	t.Cleanup(func() {
		app.StopStreams()
		server.Close()
	})
	return app, server
}

// addTestEntry stores an entry stamped now and returns its ID.
func addTestEntry(t *testing.T, store storage.Storage, message string, tags ...string) string {
	t.Helper()
	id, err := store.CreateEntry(storage.Entry{Message: message, Tags: tags, Timestamp: time.Now()})
	if err != nil {
		t.Fatalf("create entry %q: %v", message, err)
	}
	return id
}

// getBody sends a GET and returns the status code and the whole body. A response that doesn't
// end within getBodyTimeout, as an event stream never does, fails the test instead of hanging it.
func getBody(t *testing.T, server *httptest.Server, path string) (int, string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), getBodyTimeout)
	defer cancel()
	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodGet, server.URL+path, nil)
	if err != nil {
		t.Fatal(err)
	}
	response, err := server.Client().Do(httpRequest)
	if err != nil {
		t.Fatalf("GET %s: %v", path, err)
	}
	defer func() { _ = response.Body.Close() }()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatalf("read the body of GET %s (status %d): %v", path, response.StatusCode, err)
	}
	return response.StatusCode, string(body)
}

// connectStream opens the stream at path, requires a 200 response and starts reading its lines.
// Cleanup closes the connection.
func connectStream(t *testing.T, server *httptest.Server, path string) *streamClient {
	t.Helper()
	// The timeout only stops a test that hangs; a working one never reaches it.
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodGet, server.URL+path, nil)
	if err != nil {
		cancel()
		t.Fatal(err)
	}
	response, err := server.Client().Do(httpRequest)
	if err != nil {
		cancel()
		t.Fatalf("connect to %s: %v", path, err)
	}
	t.Cleanup(func() {
		cancel()
		_ = response.Body.Close()
	})
	if response.StatusCode != http.StatusOK {
		t.Fatalf("GET %s status = %d, want 200", path, response.StatusCode)
	}

	client := &streamClient{response: response, lines: make(chan string), cancel: cancel}
	go func() {
		defer close(client.lines)
		scanner := bufio.NewScanner(response.Body)
		for scanner.Scan() {
			select {
			case client.lines <- scanner.Text():
			case <-ctx.Done():
				return
			}
		}
		client.err = scanner.Err()
	}()
	return client
}

// disconnect makes the client leave, as a closed browser tab does.
func (c *streamClient) disconnect() {
	c.cancel()
	_ = c.response.Body.Close()
}

// line returns the next line of the stream. It fails the test if the stream ends or nothing
// arrives in time.
func (c *streamClient) line(t *testing.T) string {
	t.Helper()
	select {
	case line, ok := <-c.lines:
		if !ok {
			t.Fatalf("the stream ended (%v) before the next line", c.err)
		}
		return line
	case <-time.After(streamTestWait):
		t.Fatalf("no line arrived within %s", streamTestWait)
		return ""
	}
}

// event returns the next event. Blocks that carry neither an event name nor data, such as the
// retry line, are skipped.
func (c *streamClient) event(t *testing.T) sseEvent {
	t.Helper()
	var event sseEvent
	for {
		line := c.line(t)
		switch {
		case line == "":
			if event != (sseEvent{}) {
				return event
			}
		case strings.HasPrefix(line, "event: "):
			event.Name = strings.TrimPrefix(line, "event: ")
		case strings.HasPrefix(line, "data: "):
			if event.Data != "" {
				event.Data += "\n"
			}
			event.Data += strings.TrimPrefix(line, "data: ")
		}
	}
}

// entry returns the next event, which must be an entry event, decoded.
func (c *streamClient) entry(t *testing.T) storage.Entry {
	t.Helper()
	event := c.event(t)
	if event.Name != "entry" {
		t.Fatalf("event = %+v, want an entry event", event)
	}
	var entry storage.Entry
	if err := json.Unmarshal([]byte(event.Data), &entry); err != nil {
		t.Fatalf("entry data %q is not an entry: %v", event.Data, err)
	}
	return entry
}

// ready requires the next event to be the ready event.
func (c *streamClient) ready(t *testing.T) {
	t.Helper()
	if event := c.event(t); event.Name != "ready" || event.Data != "{}" {
		t.Fatalf("event = %+v, want ready with {}", event)
	}
}

// expectEnd requires the server to end the stream cleanly within the given time, without
// sending anything more.
func (c *streamClient) expectEnd(t *testing.T, within time.Duration) {
	t.Helper()
	select {
	case line, ok := <-c.lines:
		if ok {
			t.Fatalf("the stream sent %q, want it to end", line)
		}
		if c.err != nil {
			t.Fatalf("the stream ended with %v, want a clean end", c.err)
		}
	case <-time.After(within):
		t.Fatalf("the stream was still open after %s", within)
	}
}

func TestPollIntervalDefaultsToOneSecond(t *testing.T) {
	store := newMarkdownStreamStore(t)
	for name, options := range map[string][]Options{
		"no options":    nil,
		"zero interval": {{}},
		"negative":      {{PollInterval: -time.Second}},
	} {
		app, err := New(store, options...)
		if err != nil {
			t.Fatal(err)
		}
		if app.pollInterval != time.Second {
			t.Errorf("%s: poll interval = %s, want 1s", name, app.pollInterval)
		}
	}

	app, err := New(store, Options{PollInterval: 250 * time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	if app.pollInterval != 250*time.Millisecond {
		t.Errorf("poll interval = %s, want the configured 250ms", app.pollInterval)
	}
}

func TestStreamSendsReadyBeforeEntries(t *testing.T) {
	store := newMarkdownStreamStore(t)
	addTestEntry(t, store, "already there")
	_, server := streamServer(t, store, 0)

	stream := connectStream(t, server, "/api/stream")
	stream.ready(t)

	id := addTestEntry(t, store, "first entry")
	entry := stream.entry(t)
	if entry.ID != id || entry.Message != "first entry" {
		t.Fatalf("entry = %+v, want ID %s and message %q", entry, id, "first entry")
	}
}

func TestStreamWireFormat(t *testing.T) {
	store := newMarkdownStreamStore(t)
	_, server := streamServer(t, store, 0)

	stream := connectStream(t, server, "/api/stream")
	if got := stream.response.Header.Get("Content-Type"); got != "text/event-stream" {
		t.Errorf("Content-Type = %q, want text/event-stream", got)
	}
	if got := stream.response.Header.Get("Cache-Control"); got != "no-cache" {
		t.Errorf("Cache-Control = %q, want no-cache", got)
	}
	for _, want := range []string{"retry: 3000", "", "event: ready", "data: {}", ""} {
		if got := stream.line(t); got != want {
			t.Fatalf("line = %q, want %q", got, want)
		}
	}

	addTestEntry(t, store, "wire check", "web")
	if got := stream.line(t); got != "event: entry" {
		t.Fatalf("line = %q, want %q", got, "event: entry")
	}
	data := stream.line(t)
	if got := stream.line(t); got != "" {
		t.Fatalf("the entry event ended with %q, want a blank line", got)
	}

	// The entry's JSON is the same text /api/entries gives for it.
	status, body := getBody(t, server, "/api/entries")
	if status != http.StatusOK {
		t.Fatalf("GET /api/entries status = %d", status)
	}
	var listed struct {
		Entries []json.RawMessage `json:"entries"`
	}
	if err := json.Unmarshal([]byte(body), &listed); err != nil || len(listed.Entries) != 1 {
		t.Fatalf("GET /api/entries body = %q, want one entry (%v)", body, err)
	}
	if want := "data: " + string(listed.Entries[0]); data != want {
		t.Fatalf("entry data line = %q, want %q", data, want)
	}
}

func TestStreamAppliesQueryFilters(t *testing.T) {
	store := newMarkdownStreamStore(t)
	_, server := streamServer(t, store, 0)

	stream := connectStream(t, server, "/api/stream?tag=keep&q=ship")
	stream.ready(t)

	addTestEntry(t, store, "ship it", "drop")
	addTestEntry(t, store, "unrelated", "keep")
	want := addTestEntry(t, store, "ship it", "keep")

	entry := stream.entry(t)
	if entry.ID != want {
		t.Fatalf("first entry = %+v, want the keep entry %s", entry, want)
	}
}

func TestStreamRejectsBadFilter(t *testing.T) {
	store := newMarkdownStreamStore(t)
	_, server := streamServer(t, store, 0)

	for query, wantBody := range map[string]string{
		"since=garbage": `{"error":"invalid since date"}`,
		"until=garbage": `{"error":"invalid until date"}`,
	} {
		status, body := getBody(t, server, "/api/stream?"+query)
		if status != http.StatusBadRequest || strings.TrimSpace(body) != wantBody {
			t.Errorf("GET /api/stream?%s = %d %q, want 400 %s", query, status, body, wantBody)
		}
	}
}

func TestStreamRejectsUnsearchableText(t *testing.T) {
	store := newSqliteStreamStore(t)
	_, server := streamServer(t, store, 0)

	// SQLite's full-text search rejects both: a lone quote, and a date it reads as a column filter.
	for _, text := range []string{`"2026`, "2026-11-11"} {
		t.Run(text, func(t *testing.T) {
			query := "?q=" + url.QueryEscape(text)
			wantStatus, wantBody := getBody(t, server, "/api/entries"+query)
			if wantStatus != http.StatusBadRequest {
				t.Fatalf("GET /api/entries%s status = %d, want the store to reject the text", query, wantStatus)
			}

			status, body := getBody(t, server, "/api/stream"+query)
			if status != wantStatus || body != wantBody {
				t.Errorf("GET /api/stream%s = %d %q, want %d %q", query, status, body, wantStatus, wantBody)
			}
		})
	}
}

func TestStreamSurvivesWriteTimeout(t *testing.T) {
	store := newMarkdownStreamStore(t)
	_, server := streamServer(t, store, 200*time.Millisecond)

	stream := connectStream(t, server, "/api/stream")
	stream.ready(t)

	time.Sleep(500 * time.Millisecond)
	id := addTestEntry(t, store, "after the write timeout")
	if entry := stream.entry(t); entry.ID != id {
		t.Fatalf("entry = %+v, want ID %s", entry, id)
	}
}

func TestStreamEndsWhenServerStops(t *testing.T) {
	store := newMarkdownStreamStore(t)
	app, server := streamServer(t, store, 0)

	stream := connectStream(t, server, "/api/stream")
	stream.ready(t)

	app.StopStreams()
	stream.expectEnd(t, time.Second)

	app.StopStreams() // A second call must be harmless.
}

func TestStreamEndsWhenClientDisconnects(t *testing.T) {
	store := newMarkdownStreamStore(t)
	app, err := New(store, Options{PollInterval: streamTestPollInterval})
	if err != nil {
		t.Fatal(err)
	}
	// Wrap the handler to see when a request returns. The stream is the only request.
	returned := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer close(returned)
		app.Handler().ServeHTTP(w, r)
	}))
	t.Cleanup(func() {
		app.StopStreams()
		server.Close()
	})

	stream := connectStream(t, server, "/api/stream")
	stream.ready(t)
	stream.disconnect()

	select {
	case <-returned:
	case <-time.After(streamTestWait):
		t.Fatalf("the handler was still running %s after the client left", streamTestWait)
	}
}

func TestStreamEndsWhenPollFails(t *testing.T) {
	store := newSqliteStreamStore(t)
	_, server := streamServer(t, store, 0)

	stream := connectStream(t, server, "/api/stream")
	stream.ready(t)

	// Polling a closed database fails.
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	stream.expectEnd(t, time.Second)
}

func TestStreamServesTwoClients(t *testing.T) {
	store := newMarkdownStreamStore(t)
	_, server := streamServer(t, store, 0)

	first := connectStream(t, server, "/api/stream")
	second := connectStream(t, server, "/api/stream")
	first.ready(t)
	second.ready(t)

	id := addTestEntry(t, store, "for both")
	for name, stream := range map[string]*streamClient{"first": first, "second": second} {
		if entry := stream.entry(t); entry.ID != id {
			t.Errorf("%s client got %+v, want ID %s", name, entry, id)
		}
	}
}
