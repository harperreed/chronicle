// ABOUTME: Server-sent events stream that tells the browser about entries added after it connected.
// ABOUTME: Each connection polls its own feed, so a client sees every matching entry once.

package web

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/harper/chronicle/internal/storage"
)

// streamRetryMillis is how long, in milliseconds, the browser waits before it reconnects to a
// stream that has ended.
const streamRetryMillis = 3000

// streamEntries answers GET /api/stream. It takes the q, tag, since and until parameters of
// GET /api/entries. The response starts with "retry: 3000", then an event named ready once the
// feed is open, then one event named entry per new entry that matches, oldest first, whose data
// is the entry's JSON on one line.
//
// The stream ends when the client leaves, when StopStreams runs or when the feed fails. The
// browser then reports an error, which tells the page it was interrupted, and reconnects after
// the retry time.
func (s *Server) streamEntries(w http.ResponseWriter, r *http.Request) {
	filter, err := filterFromQuery(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	// SQLite reports a text search it can't parse only when the feed meets a new entry. The
	// stream would end at the next new entry, and every reconnect would end the same way, so the
	// page would never show an entry. Search once now, so that the request fails here the way
	// GET /api/entries fails.
	if _, err := s.store.SearchEntries(filter, 1); err != nil {
		writeError(w, http.StatusBadRequest, "could not search entries")
		return
	}
	feed, err := s.store.OpenFeed(filter)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not open entry feed")
		return
	}

	// The server's write timeout would cut the stream off, so lift it for this response.
	controller := http.NewResponseController(w)
	if err := controller.SetWriteDeadline(time.Time{}); err != nil {
		writeError(w, http.StatusInternalServerError, "streaming is not supported")
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	if _, err := fmt.Fprintf(w, "retry: %d\n\n", streamRetryMillis); err != nil {
		return
	}
	if err := writeEvent(w, controller, "ready", "{}"); err != nil {
		return
	}

	ticker := time.NewTicker(s.pollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case <-s.streamsDone:
			return
		case <-ticker.C:
			if err := sendNewEntries(w, controller, feed); err != nil {
				return
			}
		}
	}
}

// sendNewEntries polls the feed and writes an entry event for each entry it returns.
func sendNewEntries(w http.ResponseWriter, controller *http.ResponseController, feed storage.Feed) error {
	entries, err := feed.Poll()
	if err != nil {
		return err
	}
	for _, entry := range entries {
		data, err := json.Marshal(entry)
		if err != nil {
			return err
		}
		if err := writeEvent(w, controller, "entry", string(data)); err != nil {
			return err
		}
	}
	return nil
}

// writeEvent writes one server-sent event and flushes it, so the client gets it at once. The
// data must be a single line.
func writeEvent(w http.ResponseWriter, controller *http.ResponseController, name, data string) error {
	if _, err := fmt.Fprintf(w, "event: %s\ndata: %s\n\n", name, data); err != nil {
		return err
	}
	return controller.Flush()
}
