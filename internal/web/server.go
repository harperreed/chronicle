// ABOUTME: HTTP API and embedded browser interface for Chronicle.
// ABOUTME: Exposes safe CRUD and search operations over the configured storage backend.
package web

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/harper/chronicle/internal/storage"
)

const maxBodySize = 1 << 20

// Server serves Chronicle's web application and API.
type Server struct {
	store       storage.Storage
	app         http.Handler
	allowRemote bool
}

// Options controls web server security behavior.
type Options struct {
	// AllowRemote permits non-loopback Host headers. Keep this false unless an
	// authenticated reverse proxy protects Chronicle.
	AllowRemote bool
}

// New creates a web server backed by store. By default, only loopback requests
// are accepted to protect the unauthenticated local API from DNS rebinding.
func New(store storage.Storage, options ...Options) (*Server, error) {
	assets, err := fs.Sub(assetsFS, "assets")
	if err != nil {
		return nil, fmt.Errorf("load web assets: %w", err)
	}
	var settings Options
	if len(options) > 0 {
		settings = options[0]
	}
	s := &Server{store: store, app: http.FileServer(http.FS(assets)), allowRemote: settings.AllowRemote}
	return s, nil
}

// Handler returns the complete web application handler.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", s.health)
	mux.HandleFunc("GET /api/entries", s.listEntries)
	mux.HandleFunc("POST /api/entries", s.createEntry)
	mux.HandleFunc("GET /api/entries/{id}", s.getEntry)
	mux.HandleFunc("PUT /api/entries/{id}", s.updateEntry)
	mux.HandleFunc("DELETE /api/entries/{id}", s.deleteEntry)
	mux.HandleFunc("GET /api/stats", s.stats)
	mux.Handle("GET /", s.app)
	return securityHeaders(mux, s.allowRemote)
}

type entryInput struct {
	Message   string     `json:"message"`
	Tags      []string   `json:"tags"`
	Timestamp *time.Time `json:"timestamp,omitempty"`
}

type apiError struct {
	Error string `json:"error"`
}

func (s *Server) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) listEntries(w http.ResponseWriter, r *http.Request) {
	limit := 100
	if value := r.URL.Query().Get("limit"); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil || parsed < 1 || parsed > 500 {
			writeError(w, http.StatusBadRequest, "limit must be between 1 and 500")
			return
		}
		limit = parsed
	}
	filter := &storage.SearchFilter{Text: strings.TrimSpace(r.URL.Query().Get("q")), Tags: queryTags(r)}
	var err error
	if value := r.URL.Query().Get("since"); value != "" {
		filter.Since, err = parseDate(value, false)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid since date")
			return
		}
	}
	if value := r.URL.Query().Get("until"); value != "" {
		filter.Until, err = parseDate(value, true)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid until date")
			return
		}
	}
	entries, err := s.store.SearchEntries(filter, limit)
	if err != nil {
		writeError(w, http.StatusBadRequest, "could not search entries")
		return
	}
	if entries == nil {
		entries = []storage.Entry{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"entries": entries, "count": len(entries)})
}

func (s *Server) createEntry(w http.ResponseWriter, r *http.Request) {
	var input entryInput
	if !decodeJSON(w, r, &input) || !validateInput(w, &input) {
		return
	}
	hostname, _ := os.Hostname()
	workingDirectory, _ := os.Getwd()
	username := os.Getenv("USER")
	if username == "" {
		username = os.Getenv("USERNAME")
	}
	timestamp := time.Now()
	if input.Timestamp != nil {
		timestamp = *input.Timestamp
	}
	entry := storage.Entry{
		Timestamp: timestamp, Message: strings.TrimSpace(input.Message), Tags: normalizeTags(input.Tags),
		Hostname: hostname, Username: username, WorkingDirectory: workingDirectory,
	}
	id, err := s.store.CreateEntry(entry)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not create entry")
		return
	}
	entry.ID = id
	writeJSON(w, http.StatusCreated, entry)
}

func (s *Server) getEntry(w http.ResponseWriter, r *http.Request) {
	entry, err := s.store.GetEntry(r.PathValue("id"))
	if err != nil {
		writeStorageError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, entry)
}

func (s *Server) updateEntry(w http.ResponseWriter, r *http.Request) {
	entry, err := s.store.GetEntry(r.PathValue("id"))
	if err != nil {
		writeStorageError(w, err)
		return
	}
	var input entryInput
	if !decodeJSON(w, r, &input) || !validateInput(w, &input) {
		return
	}
	entry.Message = strings.TrimSpace(input.Message)
	entry.Tags = normalizeTags(input.Tags)
	if input.Timestamp != nil {
		entry.Timestamp = *input.Timestamp
	}
	if err := s.store.UpdateEntry(*entry); err != nil {
		writeStorageError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, entry)
}

func (s *Server) deleteEntry(w http.ResponseWriter, r *http.Request) {
	if err := s.store.DeleteEntry(r.PathValue("id")); err != nil {
		writeStorageError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) stats(w http.ResponseWriter, _ *http.Request) {
	entries, err := s.store.ListEntries(0)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not load statistics")
		return
	}
	counts := make(map[string]int)
	for _, entry := range entries {
		for _, tag := range entry.Tags {
			counts[tag]++
		}
	}
	type tagCount struct {
		Name  string `json:"name"`
		Count int    `json:"count"`
	}
	tags := make([]tagCount, 0, len(counts))
	for name, count := range counts {
		tags = append(tags, tagCount{Name: name, Count: count})
	}
	sort.Slice(tags, func(i, j int) bool {
		if tags[i].Count == tags[j].Count {
			return tags[i].Name < tags[j].Name
		}
		return tags[i].Count > tags[j].Count
	})
	writeJSON(w, http.StatusOK, map[string]any{"total": len(entries), "tags": tags})
}

func decodeJSON(w http.ResponseWriter, r *http.Request, target any) bool {
	if contentType := r.Header.Get("Content-Type"); !strings.HasPrefix(contentType, "application/json") {
		writeError(w, http.StatusUnsupportedMediaType, "Content-Type must be application/json")
		return false
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxBodySize))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return false
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		writeError(w, http.StatusBadRequest, "body must contain one JSON object")
		return false
	}
	return true
}

func validateInput(w http.ResponseWriter, input *entryInput) bool {
	message := strings.TrimSpace(input.Message)
	if message == "" {
		writeError(w, http.StatusUnprocessableEntity, "message is required")
		return false
	}
	if len(message) > 10000 {
		writeError(w, http.StatusUnprocessableEntity, "message must be 10,000 characters or fewer")
		return false
	}
	if len(input.Tags) > 50 {
		writeError(w, http.StatusUnprocessableEntity, "no more than 50 tags are allowed")
		return false
	}
	return true
}

func normalizeTags(values []string) []string {
	seen := make(map[string]bool)
	tags := make([]string, 0, len(values))
	for _, value := range values {
		tag := strings.TrimSpace(strings.TrimPrefix(value, "#"))
		if tag != "" && len(tag) <= 64 && !seen[tag] {
			seen[tag] = true
			tags = append(tags, tag)
		}
	}
	return tags
}

func queryTags(r *http.Request) []string {
	var values []string
	for _, value := range r.URL.Query()["tag"] {
		values = append(values, strings.Split(value, ",")...)
	}
	return normalizeTags(values)
}

func parseDate(value string, endOfDay bool) (*time.Time, error) {
	if date, err := time.Parse("2006-01-02", value); err == nil {
		if endOfDay {
			date = date.Add(24*time.Hour - time.Nanosecond)
		}
		return &date, nil
	}
	date, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return nil, err
	}
	return &date, nil
}

func writeStorageError(w http.ResponseWriter, err error) {
	if strings.Contains(err.Error(), "not found") {
		writeError(w, http.StatusNotFound, "entry not found")
		return
	}
	writeError(w, http.StatusInternalServerError, "storage operation failed")
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, apiError{Error: message})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func securityHeaders(next http.Handler, allowRemote bool) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Security-Policy", "default-src 'self'; style-src 'self'; script-src 'self'; img-src 'self' data:; connect-src 'self'; frame-ancestors 'none'")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		if !allowRemote && !isLoopbackHost(r.Host) {
			writeError(w, http.StatusForbidden, "remote requests are not allowed")
			return
		}
		if !isSameOrigin(r) {
			writeError(w, http.StatusForbidden, "cross-origin requests are not allowed")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func isLoopbackHost(hostPort string) bool {
	host := hostPort
	if parsedHost, _, err := net.SplitHostPort(hostPort); err == nil {
		host = parsedHost
	}
	host = strings.Trim(host, "[]")
	return strings.EqualFold(host, "localhost") || net.ParseIP(host).IsLoopback()
}

func isSameOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true
	}
	parsed, err := url.Parse(origin)
	if err != nil || parsed.Host == "" {
		return false
	}
	return strings.EqualFold(parsed.Host, r.Host)
}
