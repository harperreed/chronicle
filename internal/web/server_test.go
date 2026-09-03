package web

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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
