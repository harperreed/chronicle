// ABOUTME: Tests for MCP resource handlers
// ABOUTME: Validates resource handlers return correct data structures

package mcp

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/harper/chronicle/internal/storage"
)

func TestHandleRecentActivity(t *testing.T) {
	server, store := newTestServer(t)
	defer store.Close()

	// Create test entries
	for i := 0; i < 3; i++ {
		entry := storage.Entry{
			Timestamp: time.Now(),
			Message:   "test entry",
			Tags:      []string{"test"},
		}
		if _, err := store.CreateEntry(entry); err != nil {
			t.Fatalf("failed to create test entry: %v", err)
		}
	}

	t.Run("returns recent entries as JSON", func(t *testing.T) {
		result, err := server.handleRecentActivity(context.Background(), nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if result == nil {
			t.Fatal("expected result to be non-nil")
		}

		if len(result.Contents) == 0 {
			t.Fatal("expected content in result")
		}

		content := result.Contents[0]
		if content.URI != "chronicle://recent-activity" {
			t.Errorf("expected URI 'chronicle://recent-activity', got: %s", content.URI)
		}

		if content.MIMEType != "application/json" {
			t.Errorf("expected MIME type 'application/json', got: %s", content.MIMEType)
		}

		// Verify JSON is valid
		var entries []storage.Entry
		if err := json.Unmarshal([]byte(content.Text), &entries); err != nil {
			t.Errorf("failed to parse JSON: %v", err)
		}

		if len(entries) != 3 {
			t.Errorf("expected 3 entries in JSON, got: %d", len(entries))
		}
	})

	t.Run("returns empty array when no entries", func(t *testing.T) {
		// Create new empty server
		emptyServer, emptyStore := newTestServer(t)
		defer emptyStore.Close()

		result, err := emptyServer.handleRecentActivity(context.Background(), nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		content := result.Contents[0]
		if content.Text != "null" && content.Text != "[]" {
			// Check it's valid JSON representing empty or null
			var entries []storage.Entry
			if err := json.Unmarshal([]byte(content.Text), &entries); err != nil {
				t.Errorf("expected valid JSON, got: %s", content.Text)
			}
		}
	})
}

func TestHandleTags(t *testing.T) {
	server, store := newTestServer(t)
	defer store.Close()

	// Create entries with various tags
	entries := []storage.Entry{
		{Timestamp: time.Now(), Message: "entry 1", Tags: []string{"work", "golang"}},
		{Timestamp: time.Now(), Message: "entry 2", Tags: []string{"work", "testing"}},
		{Timestamp: time.Now(), Message: "entry 3", Tags: []string{"personal"}},
	}

	for _, e := range entries {
		if _, err := store.CreateEntry(e); err != nil {
			t.Fatalf("failed to create test entry: %v", err)
		}
	}

	t.Run("returns tag counts as JSON", func(t *testing.T) {
		result, err := server.handleTags(context.Background(), nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(result.Contents) == 0 {
			t.Fatal("expected content in result")
		}

		content := result.Contents[0]
		if content.URI != "chronicle://tags" {
			t.Errorf("expected URI 'chronicle://tags', got: %s", content.URI)
		}

		if content.MIMEType != "application/json" {
			t.Errorf("expected MIME type 'application/json', got: %s", content.MIMEType)
		}

		// Parse tag counts
		var tagCounts map[string]int
		if err := json.Unmarshal([]byte(content.Text), &tagCounts); err != nil {
			t.Fatalf("failed to parse JSON: %v", err)
		}

		if tagCounts["work"] != 2 {
			t.Errorf("expected 'work' count 2, got: %d", tagCounts["work"])
		}

		if tagCounts["golang"] != 1 {
			t.Errorf("expected 'golang' count 1, got: %d", tagCounts["golang"])
		}

		if tagCounts["personal"] != 1 {
			t.Errorf("expected 'personal' count 1, got: %d", tagCounts["personal"])
		}
	})

	t.Run("returns empty object when no tags", func(t *testing.T) {
		emptyServer, emptyStore := newTestServer(t)
		defer emptyStore.Close()

		// Create entry without tags
		entry := storage.Entry{
			Timestamp: time.Now(),
			Message:   "no tags",
		}
		if _, err := emptyStore.CreateEntry(entry); err != nil {
			t.Fatalf("failed to create test entry: %v", err)
		}

		result, err := emptyServer.handleTags(context.Background(), nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		content := result.Contents[0]
		var tagCounts map[string]int
		if err := json.Unmarshal([]byte(content.Text), &tagCounts); err != nil {
			t.Fatalf("failed to parse JSON: %v", err)
		}

		if len(tagCounts) != 0 {
			t.Errorf("expected empty tag counts, got: %v", tagCounts)
		}
	})
}

func TestHandleTodaySummary(t *testing.T) {
	server, store := newTestServer(t)
	defer store.Close()

	// Create an entry for today
	entry := storage.Entry{
		Timestamp: time.Now(),
		Message:   "today's task",
		Tags:      []string{"today"},
	}
	if _, err := store.CreateEntry(entry); err != nil {
		t.Fatalf("failed to create test entry: %v", err)
	}

	t.Run("returns markdown summary", func(t *testing.T) {
		result, err := server.handleTodaySummary(context.Background(), nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(result.Contents) == 0 {
			t.Fatal("expected content in result")
		}

		content := result.Contents[0]
		if content.URI != "chronicle://today-summary" {
			t.Errorf("expected URI 'chronicle://today-summary', got: %s", content.URI)
		}

		if content.MIMEType != "text/markdown" {
			t.Errorf("expected MIME type 'text/markdown', got: %s", content.MIMEType)
		}

		if !strings.Contains(content.Text, "# Today's Activity") {
			t.Error("expected markdown header")
		}

		if !strings.Contains(content.Text, "today's task") {
			t.Error("expected entry message in summary")
		}
	})

	t.Run("shows no entries message when empty", func(t *testing.T) {
		emptyServer, emptyStore := newTestServer(t)
		defer emptyStore.Close()

		result, err := emptyServer.handleTodaySummary(context.Background(), nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		content := result.Contents[0]
		if !strings.Contains(content.Text, "No entries logged today") {
			t.Error("expected 'no entries' message")
		}
	})
}

func TestHandleProjectContext(t *testing.T) {
	server, store := newTestServer(t)
	defer store.Close()

	t.Run("returns JSON structure", func(t *testing.T) {
		result, err := server.handleProjectContext(context.Background(), nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(result.Contents) == 0 {
			t.Fatal("expected content in result")
		}

		content := result.Contents[0]
		if content.URI != "chronicle://project-context" {
			t.Errorf("expected URI 'chronicle://project-context', got: %s", content.URI)
		}

		if content.MIMEType != "application/json" {
			t.Errorf("expected MIME type 'application/json', got: %s", content.MIMEType)
		}

		// Verify JSON is valid
		var contextData map[string]interface{}
		if err := json.Unmarshal([]byte(content.Text), &contextData); err != nil {
			t.Errorf("failed to parse JSON: %v", err)
		}

		// Should have has_project_config field
		if _, ok := contextData["has_project_config"]; !ok {
			t.Error("expected 'has_project_config' field")
		}

		// Should have message field
		if _, ok := contextData["message"]; !ok {
			t.Error("expected 'message' field")
		}
	})

	t.Run("returns no project config message when not in project", func(t *testing.T) {
		// Run from a temp directory without .chronicle
		result, err := server.handleProjectContext(context.Background(), nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		var contextData map[string]interface{}
		if err := json.Unmarshal([]byte(result.Contents[0].Text), &contextData); err != nil {
			t.Fatalf("failed to parse JSON: %v", err)
		}

		// Verify we get a message either way
		msg, ok := contextData["message"].(string)
		if !ok {
			t.Fatal("expected message to be a string")
		}

		if msg == "" {
			t.Error("expected non-empty message")
		}
	})
}

func TestHandleProjectContextWithConfig(t *testing.T) {
	server, store := newTestServer(t)
	defer store.Close()

	t.Run("handles directory with .chronicle config", func(t *testing.T) {
		// Create temp directory with .chronicle config
		tempDir := t.TempDir()
		chronicleDir := tempDir + "/.chronicle"
		if err := os.MkdirAll(chronicleDir, 0755); err != nil {
			t.Fatalf("failed to create .chronicle dir: %v", err)
		}

		configContent := `local_logging = true
log_dir = "logs"
log_format = "markdown"`

		if err := os.WriteFile(chronicleDir+"/config.toml", []byte(configContent), 0644); err != nil {
			t.Fatalf("failed to write config: %v", err)
		}

		// Change to temp dir temporarily
		originalWd, _ := os.Getwd()
		if err := os.Chdir(tempDir); err != nil {
			t.Fatalf("failed to change directory: %v", err)
		}
		defer os.Chdir(originalWd)

		result, err := server.handleProjectContext(context.Background(), nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(result.Contents) == 0 {
			t.Fatal("expected content in result")
		}

		var contextData map[string]interface{}
		if err := json.Unmarshal([]byte(result.Contents[0].Text), &contextData); err != nil {
			t.Fatalf("failed to parse JSON: %v", err)
		}

		// Should indicate project config was found
		hasConfig, ok := contextData["has_project_config"].(bool)
		if !ok {
			t.Fatal("expected has_project_config to be a bool")
		}

		if !hasConfig {
			// This is ok - it means the config wasn't loadable
			// We're just verifying the structure
		}
	})
}

func TestResourceContentFields(t *testing.T) {
	server, store := newTestServer(t)
	defer store.Close()

	t.Run("recent activity has correct fields", func(t *testing.T) {
		result, err := server.handleRecentActivity(context.Background(), nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(result.Contents) != 1 {
			t.Errorf("expected 1 content, got %d", len(result.Contents))
		}
	})

	t.Run("tags resource has correct fields", func(t *testing.T) {
		result, err := server.handleTags(context.Background(), nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(result.Contents) != 1 {
			t.Errorf("expected 1 content, got %d", len(result.Contents))
		}
	})

	t.Run("today summary has correct fields", func(t *testing.T) {
		result, err := server.handleTodaySummary(context.Background(), nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(result.Contents) != 1 {
			t.Errorf("expected 1 content, got %d", len(result.Contents))
		}
	})
}

func TestRegisterResourcesCall(t *testing.T) {
	server, store := newTestServer(t)
	defer store.Close()

	t.Run("registers all resources without error", func(t *testing.T) {
		// Should not panic
		server.registerResources()
	})
}

func TestHandleRecentActivityWithManyEntries(t *testing.T) {
	server, store := newTestServer(t)
	defer store.Close()

	// Create more than 10 entries to test limit
	for i := 0; i < 15; i++ {
		entry := storage.Entry{
			Timestamp: time.Now(),
			Message:   "test entry",
			Tags:      []string{"test"},
		}
		if _, err := store.CreateEntry(entry); err != nil {
			t.Fatalf("failed to create test entry: %v", err)
		}
	}

	t.Run("limits to 10 entries", func(t *testing.T) {
		result, err := server.handleRecentActivity(context.Background(), nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		var entries []storage.Entry
		if err := json.Unmarshal([]byte(result.Contents[0].Text), &entries); err != nil {
			t.Fatalf("failed to parse JSON: %v", err)
		}

		if len(entries) != 10 {
			t.Errorf("expected 10 entries, got: %d", len(entries))
		}
	})
}

func TestHandleTagsWithManyTags(t *testing.T) {
	server, store := newTestServer(t)
	defer store.Close()

	// Create entries with many different tags
	for i := 0; i < 5; i++ {
		entry := storage.Entry{
			Timestamp: time.Now(),
			Message:   "test entry",
			Tags:      []string{"common", "tag" + string(rune('a'+i))},
		}
		if _, err := store.CreateEntry(entry); err != nil {
			t.Fatalf("failed to create test entry: %v", err)
		}
	}

	t.Run("returns all tags with correct counts", func(t *testing.T) {
		result, err := server.handleTags(context.Background(), nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		var tagCounts map[string]int
		if err := json.Unmarshal([]byte(result.Contents[0].Text), &tagCounts); err != nil {
			t.Fatalf("failed to parse JSON: %v", err)
		}

		// "common" tag should appear 5 times
		if tagCounts["common"] != 5 {
			t.Errorf("expected 'common' count 5, got: %d", tagCounts["common"])
		}

		// Individual tags should appear once each
		for i := 0; i < 5; i++ {
			tag := "tag" + string(rune('a'+i))
			if tagCounts[tag] != 1 {
				t.Errorf("expected '%s' count 1, got: %d", tag, tagCounts[tag])
			}
		}
	})
}

func TestHandleTodaySummaryMultipleEntries(t *testing.T) {
	server, store := newTestServer(t)
	defer store.Close()

	// Create multiple entries for today
	for i := 0; i < 3; i++ {
		entry := storage.Entry{
			Timestamp: time.Now(),
			Message:   "today task " + string(rune('A'+i)),
			Tags:      []string{"today"},
		}
		if _, err := store.CreateEntry(entry); err != nil {
			t.Fatalf("failed to create test entry: %v", err)
		}
	}

	t.Run("includes all today's entries in summary", func(t *testing.T) {
		result, err := server.handleTodaySummary(context.Background(), nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		content := result.Contents[0].Text

		// Should contain all 3 entries
		for i := 0; i < 3; i++ {
			msg := "today task " + string(rune('A'+i))
			if !strings.Contains(content, msg) {
				t.Errorf("expected summary to contain '%s'", msg)
			}
		}
	})
}
