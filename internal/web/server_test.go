package web

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/harper/chronicle/internal/storage"
)

func testServer(t *testing.T) (*httptest.Server, storage.Storage) {
	t.Helper()
	store, err := storage.NewSqliteStore(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	app, err := New(store)
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(app.Handler())
	t.Cleanup(func() {
		server.Close()
		_ = store.Close()
	})
	return server, store
}

func request(t *testing.T, client *http.Client, method, url string, body any) *http.Response {
	t.Helper()
	var data []byte
	if body != nil {
		var err error
		data, err = json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
	}
	req, err := http.NewRequest(method, url, bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	response, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	return response
}

func TestEntryCRUD(t *testing.T) {
	server, _ := testServer(t)
	created := request(t, server.Client(), http.MethodPost, server.URL+"/api/entries", map[string]any{
		"message": " Shipped the web UI ", "tags": []string{"web", "web", "#release"},
	})
	defer func() { _ = created.Body.Close() }()
	if created.StatusCode != http.StatusCreated {
		t.Fatalf("create status = %d", created.StatusCode)
	}
	var entry storage.Entry
	if err := json.NewDecoder(created.Body).Decode(&entry); err != nil {
		t.Fatal(err)
	}
	if entry.ID == "" || entry.Message != "Shipped the web UI" || len(entry.Tags) != 2 {
		t.Fatalf("unexpected entry: %#v", entry)
	}

	listed := request(t, server.Client(), http.MethodGet, server.URL+"/api/entries?tag=web", nil)
	defer func() { _ = listed.Body.Close() }()
	var result struct {
		Entries []storage.Entry `json:"entries"`
		Count   int             `json:"count"`
	}
	if err := json.NewDecoder(listed.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	if result.Count != 1 {
		t.Fatalf("count = %d", result.Count)
	}

	updated := request(t, server.Client(), http.MethodPut, server.URL+"/api/entries/"+entry.ID, map[string]any{"message": "Updated", "tags": []string{"done"}})
	_ = updated.Body.Close()
	if updated.StatusCode != http.StatusOK {
		t.Fatalf("update status = %d", updated.StatusCode)
	}

	deleted := request(t, server.Client(), http.MethodDelete, server.URL+"/api/entries/"+entry.ID, nil)
	_ = deleted.Body.Close()
	if deleted.StatusCode != http.StatusNoContent {
		t.Fatalf("delete status = %d", deleted.StatusCode)
	}

	missing := request(t, server.Client(), http.MethodGet, server.URL+"/api/entries/"+entry.ID, nil)
	_ = missing.Body.Close()
	if missing.StatusCode != http.StatusNotFound {
		t.Fatalf("missing status = %d", missing.StatusCode)
	}
}

func TestValidationAndDateFiltering(t *testing.T) {
	server, store := testServer(t)
	_, err := store.CreateEntry(storage.Entry{Message: "old", Timestamp: time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)})
	if err != nil {
		t.Fatal(err)
	}

	invalid := request(t, server.Client(), http.MethodPost, server.URL+"/api/entries", map[string]any{"message": "  "})
	_ = invalid.Body.Close()
	if invalid.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("invalid status = %d", invalid.StatusCode)
	}

	badType := request(t, server.Client(), http.MethodPost, server.URL+"/api/entries", nil)
	_ = badType.Body.Close()
	if badType.StatusCode != http.StatusUnsupportedMediaType {
		t.Fatalf("content type status = %d", badType.StatusCode)
	}

	filtered := request(t, server.Client(), http.MethodGet, server.URL+"/api/entries?since=2025-01-01", nil)
	defer func() { _ = filtered.Body.Close() }()
	var result struct {
		Count int `json:"count"`
	}
	if err := json.NewDecoder(filtered.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	if result.Count != 0 {
		t.Fatalf("filtered count = %d", result.Count)
	}
}

func TestRejectsRemoteHostAndCrossOriginRequests(t *testing.T) {
	store, err := storage.NewSqliteStore(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = store.Close() }()
	app, err := New(store)
	if err != nil {
		t.Fatal(err)
	}

	remote := httptest.NewRequest(http.MethodGet, "http://malicious.example/api/entries", nil)
	remote.Host = "malicious.example"
	remoteResponse := httptest.NewRecorder()
	app.Handler().ServeHTTP(remoteResponse, remote)
	if remoteResponse.Code != http.StatusForbidden {
		t.Fatalf("remote Host status = %d", remoteResponse.Code)
	}

	crossOrigin := httptest.NewRequest(http.MethodGet, "http://localhost/api/entries", nil)
	crossOrigin.Header.Set("Origin", "https://malicious.example")
	crossOriginResponse := httptest.NewRecorder()
	app.Handler().ServeHTTP(crossOriginResponse, crossOrigin)
	if crossOriginResponse.Code != http.StatusForbidden {
		t.Fatalf("cross-origin status = %d", crossOriginResponse.Code)
	}
}

// readOnlyTestServer serves a markdown folder through NewReadOnly. The server holds a store that
// OpenMarkdownReader opened on the folder, as the web command does. The test gets a second store
// on the same folder, and that store is the only way to change data.
func readOnlyTestServer(t *testing.T) (*httptest.Server, storage.Storage) {
	t.Helper()
	dataDir := t.TempDir()
	writer, err := storage.NewMarkdownStore(dataDir)
	if err != nil {
		t.Fatal(err)
	}
	reader, err := storage.OpenMarkdownReader(dataDir)
	if err != nil {
		t.Fatal(err)
	}
	app, err := NewReadOnly(reader, Options{PollInterval: streamTestPollInterval})
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(app.Handler())
	t.Cleanup(func() {
		app.StopStreams()
		server.Close()
	})
	return server, writer
}

func TestReadOnlyServerRefusesWrites(t *testing.T) {
	server, writer := readOnlyTestServer(t)
	id := addTestEntry(t, writer, "keep me", "keep")

	changed := map[string]any{"message": "changed", "tags": []string{"changed"}}
	writes := []struct {
		method string
		path   string
		body   any
	}{
		{http.MethodPost, "/api/entries", changed},
		{http.MethodPut, "/api/entries/x", changed},
		{http.MethodDelete, "/api/entries/x", nil},
		// A route that existed would change or delete this entry.
		{http.MethodPut, "/api/entries/" + id, changed},
		{http.MethodDelete, "/api/entries/" + id, nil},
	}
	for _, write := range writes {
		t.Run(write.method+" "+write.path, func(t *testing.T) {
			response := request(t, server.Client(), write.method, server.URL+write.path, write.body)
			_ = response.Body.Close()
			if response.StatusCode != http.StatusMethodNotAllowed {
				t.Fatalf("status = %d, want 405", response.StatusCode)
			}
		})
	}

	entries, err := writer.ListEntries(0)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].ID != id || entries[0].Message != "keep me" {
		t.Fatalf("entries after the refused writes = %+v, want the one untouched entry %s", entries, id)
	}
}

func TestReadOnlyServerServesReads(t *testing.T) {
	server, writer := readOnlyTestServer(t)
	id := addTestEntry(t, writer, "read me", "work")

	for _, path := range []string{"/api/health", "/api/entries", "/api/entries/" + id, "/api/stats", "/app.css"} {
		t.Run(path, func(t *testing.T) {
			status, body := getBody(t, server, path)
			if status != http.StatusOK {
				t.Fatalf("GET %s status = %d, want 200 (body %q)", path, status, body)
			}
		})
	}

	_, body := getBody(t, server, "/api/entries")
	if !strings.Contains(body, "read me") {
		t.Fatalf("GET /api/entries body = %q, want it to hold the entry the writer added", body)
	}
}

func TestReadOnlyServerStreamsNewEntries(t *testing.T) {
	server, writer := readOnlyTestServer(t)

	stream := connectStream(t, server, "/api/stream")
	stream.ready(t)

	id := addTestEntry(t, writer, "written while watching")
	if entry := stream.entry(t); entry.ID != id || entry.Message != "written while watching" {
		t.Fatalf("entry = %+v, want ID %s", entry, id)
	}
}

func TestIndexMarksMode(t *testing.T) {
	store := newMarkdownStreamStore(t)
	full, err := New(store)
	if err != nil {
		t.Fatal(err)
	}
	readOnly, err := NewReadOnly(store)
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name    string
		app     *Server
		want    string
		wantNot string
	}{
		{"read-only", readOnly, `<html lang="en" data-mode="read-only">`, `data-mode="full"`},
		{"full", full, `<html lang="en" data-mode="full">`, `data-mode="read-only"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			tt.app.Handler().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "http://localhost/", nil))
			if recorder.Code != http.StatusOK {
				t.Fatalf("GET / status = %d, want 200", recorder.Code)
			}
			if got := recorder.Header().Get("Content-Type"); !strings.HasPrefix(got, "text/html") {
				t.Errorf("Content-Type = %q, want text/html", got)
			}
			body := recorder.Body.String()
			if !strings.Contains(body, tt.want) {
				t.Errorf("GET / body has no %s", tt.want)
			}
			if strings.Contains(body, tt.wantNot) {
				t.Errorf("GET / body has %s, want only the %s mode", tt.wantNot, tt.name)
			}
		})
	}
}

func TestMarkIndexNeedsHTMLTag(t *testing.T) {
	// A page without the tag would reach the browser unmarked, and a read-only server would
	// serve a page that doesn't know it is read-only. Refuse it when the server is built.
	if _, err := markIndex([]byte("<!doctype html><html><body></body></html>"), modeReadOnly); err == nil {
		t.Fatal("markIndex accepted a page with no <html lang=\"en\"> tag")
	}
}

func TestWebAssetsStatsAndSecurityHeaders(t *testing.T) {
	server, store := testServer(t)
	_, _ = store.CreateEntry(storage.Entry{Message: "one", Tags: []string{"work"}})
	_, _ = store.CreateEntry(storage.Entry{Message: "two", Tags: []string{"work", "go"}})

	page := request(t, server.Client(), http.MethodGet, server.URL+"/", nil)
	defer func() { _ = page.Body.Close() }()
	if page.StatusCode != http.StatusOK {
		t.Fatalf("page status = %d", page.StatusCode)
	}
	if page.Header.Get("Content-Security-Policy") == "" {
		t.Fatal("missing CSP header")
	}

	stats := request(t, server.Client(), http.MethodGet, server.URL+"/api/stats", nil)
	defer func() { _ = stats.Body.Close() }()
	var result struct {
		Total int `json:"total"`
		Tags  []struct {
			Name  string `json:"name"`
			Count int    `json:"count"`
		} `json:"tags"`
	}
	if err := json.NewDecoder(stats.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	if result.Total != 2 || len(result.Tags) != 2 || result.Tags[0].Name != "work" || result.Tags[0].Count != 2 {
		t.Fatalf("unexpected stats: %#v", result)
	}
}

// embeddedAsset returns the text of a file embedded in the web assets.
func embeddedAsset(t *testing.T, name string) string {
	t.Helper()
	data, err := assetsFS.ReadFile("assets/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// serifFamily matches the serif generic font family. A plain substring search can't find it,
// because "sans-serif" holds "serif", so it looks for "serif" after a space, comma or colon.
var serifFamily = regexp.MustCompile(`[\s,:]serif\b`)

func TestStylesheetUsesApprovedLook(t *testing.T) {
	css := strings.ToLower(embeddedAsset(t, "app.css"))

	// The approved palette, light and dark. The dark accent and the live marker share one green.
	palette := []string{
		"#f8fafc", "#0b0f14", // Background.
		"#ffffff", "#111821", // Surface.
		"#f1f5f9", "#17202b", // Surface 2.
		"#0f172a", "#e5edf5", // Ink.
		"#64748b", "#8a99ab", // Muted.
		"#e2e8f0", "#1f2a37", // Line.
		"#2563eb", "#4ade80", // Accent.
		"#16a34a", // Live marker in the light theme.
	}
	for _, hex := range palette {
		if !strings.Contains(css, hex) {
			t.Errorf("app.css has no %s from the approved palette", hex)
		}
	}
	if !strings.Contains(css, "light-dark(") {
		t.Error("app.css has no light-dark(), so its colors can't follow the system setting")
	}

	// The look Harper rejected: serif type, a cream background and a coral accent.
	for _, rejected := range []string{"georgia", "ui-serif", "#f3f0e8", "#e35d35"} {
		if strings.Contains(css, rejected) {
			t.Errorf("app.css still has %s from the rejected look", rejected)
		}
	}
	if match := serifFamily.FindString(css); match != "" {
		t.Errorf("app.css still names the serif font family (found %q)", match)
	}
}

func TestIndexLoadsThemeBeforeStylesheet(t *testing.T) {
	page := embeddedAsset(t, "index.html")
	const themeScript = `<script src="/theme.js"></script>`
	const stylesheet = `href="/app.css"`

	themeAt := strings.Index(page, themeScript)
	if themeAt < 0 {
		t.Fatalf("index.html has no %s; it can't have defer, so a stored theme applies before the first paint", themeScript)
	}
	stylesheetAt := strings.Index(page, stylesheet)
	if stylesheetAt < 0 {
		t.Fatalf("index.html has no %s", stylesheet)
	}
	if themeAt > stylesheetAt {
		t.Errorf("index.html loads the theme script at byte %d, after the stylesheet at byte %d", themeAt, stylesheetAt)
	}
}

func TestIndexLoadsTimelineBeforeApp(t *testing.T) {
	page := embeddedAsset(t, "index.html")
	const timelineScript = `<script src="/timeline.js" defer></script>`
	const appScript = `<script src="/app.js" defer></script>`

	timelineAt := strings.Index(page, timelineScript)
	if timelineAt < 0 {
		t.Fatalf("index.html has no %s", timelineScript)
	}
	appAt := strings.Index(page, appScript)
	if appAt < 0 {
		t.Fatalf("index.html has no %s", appScript)
	}
	// Deferred scripts run in document order, and app.js uses ChronicleTimeline as soon as it starts.
	if timelineAt > appAt {
		t.Errorf("index.html loads timeline.js at byte %d, after app.js at byte %d", timelineAt, appAt)
	}
}

// localAssetRef matches the src and href attributes of index.html that name a file the server holds.
var localAssetRef = regexp.MustCompile(`(?:src|href)="(/[^"#?]+)"`)

func TestIndexReferencesServedAssets(t *testing.T) {
	server, _ := testServer(t)
	refs := localAssetRef.FindAllStringSubmatch(embeddedAsset(t, "index.html"), -1)
	if len(refs) == 0 {
		t.Fatal("index.html names no stylesheet or script")
	}
	for _, ref := range refs {
		t.Run(ref[1], func(t *testing.T) {
			if status, _ := getBody(t, server, ref[1]); status != http.StatusOK {
				t.Fatalf("GET %s status = %d, want 200", ref[1], status)
			}
		})
	}
}

// hidesFullOnlyInReadOnly matches a stylesheet rule that hides .full-only on a read-only page. It
// lets the attribute value carry quotes, as a formatter would write it.
var hidesFullOnlyInReadOnly = regexp.MustCompile(`:root\[data-mode=["']?read-only["']?\]\s+\.full-only[^{}]*\{[^{}]*display:\s*none`)

// classAttribute matches the class attribute of an opening tag.
var classAttribute = regexp.MustCompile(`\sclass="([^"]*)"`)

func TestReadOnlyPageHidesWriteControls(t *testing.T) {
	// The server marks a read-only page (TestIndexMarksMode). Two more things have to hold from
	// there: the stylesheet hides whatever the markup calls full-only, and the markup calls every
	// control that writes full-only. If either breaks, a read-only page shows its composer, its
	// edit dialog or its Edit and Delete buttons.
	if css := embeddedAsset(t, "app.css"); !hidesFullOnlyInReadOnly.MatchString(css) {
		t.Error("app.css has no rule that hides .full-only under :root[data-mode=read-only]")
	}

	page := embeddedAsset(t, "index.html")
	writeControls := []struct {
		name string
		tag  *regexp.Regexp
	}{
		{"composer section", regexp.MustCompile(`<section[^>]*\sclass="[^"]*\bcomposer\b[^"]*"[^>]*>`)},
		{"edit dialog", regexp.MustCompile(`<dialog[^>]*\sid="edit-dialog"[^>]*>`)},
		{"entry actions", regexp.MustCompile(`<div[^>]*\sclass="[^"]*\bentry-actions\b[^"]*"[^>]*>`)},
	}
	for _, control := range writeControls {
		t.Run(control.name, func(t *testing.T) {
			tag := control.tag.FindString(page)
			if tag == "" {
				t.Fatalf("index.html has no tag that matches %s", control.tag)
			}
			classes := classAttribute.FindStringSubmatch(tag)
			if classes == nil || !slices.Contains(strings.Fields(classes[1]), "full-only") {
				t.Errorf("%s has no full-only class, so a read-only page would show it", tag)
			}
		})
	}
}
