// ABOUTME: Tests for MCP resource handlers
// ABOUTME: Validates resource handlers return correct data structures

package mcp

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/harper/chronicle/internal/config"
	"github.com/harper/chronicle/internal/storage"
	gomcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

func connectTestClient(t *testing.T, mcpServer *gomcp.Server) *gomcp.ClientSession {
	t.Helper()

	serverTransport, clientTransport := gomcp.NewInMemoryTransports()
	serverSession, err := mcpServer.Connect(context.Background(), serverTransport, nil)
	if err != nil {
		t.Fatalf("failed to connect MCP server: %v", err)
	}

	client := gomcp.NewClient(&gomcp.Implementation{Name: "chronicle-test-client", Version: "0.0.0"}, nil)
	clientSession, err := client.Connect(context.Background(), clientTransport, nil)
	if err != nil {
		_ = serverSession.Close()
		t.Fatalf("failed to connect MCP client: %v", err)
	}

	t.Cleanup(func() {
		_ = clientSession.Close()
		_ = serverSession.Close()
	})

	return clientSession
}

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

	now := time.Now()
	startOfToday := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	entries := []storage.Entry{
		{Timestamp: startOfToday.Add(-time.Nanosecond), Message: "yesterday's task"},
		{Timestamp: startOfToday, Message: "today starts"},
		{Timestamp: startOfToday.AddDate(0, 0, 1).Add(-time.Nanosecond), Message: "today ends"},
		{Timestamp: startOfToday.AddDate(0, 0, 1), Message: "tomorrow's task"},
	}
	for _, entry := range entries {
		if _, err := store.CreateEntry(entry); err != nil {
			t.Fatalf("failed to create test entry: %v", err)
		}
	}

	t.Run("returns only the current local calendar day as markdown", func(t *testing.T) {
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

		for _, bullet := range []string{"- **00:00:00**: today starts", "- **23:59:59**: today ends"} {
			if !strings.Contains(content.Text, bullet) {
				t.Errorf("expected summary to contain timestamped bullet %q", bullet)
			}
		}
		for _, message := range []string{"yesterday's task", "tomorrow's task"} {
			if strings.Contains(content.Text, message) {
				t.Errorf("expected summary to exclude %q", message)
			}
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

	t.Run("reports no config from an unconfigured directory", func(t *testing.T) {
		t.Chdir(t.TempDir())

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

		var contextData struct {
			HasProjectConfig bool                  `json:"has_project_config"`
			ProjectRoot      string                `json:"project_root"`
			Config           *config.ProjectConfig `json:"config"`
			Message          string                `json:"message"`
		}
		if err := json.Unmarshal([]byte(content.Text), &contextData); err != nil {
			t.Fatalf("failed to parse JSON: %v", err)
		}
		if contextData.HasProjectConfig {
			t.Error("expected has_project_config to be false")
		}
		if contextData.ProjectRoot != "" || contextData.Config != nil {
			t.Errorf("expected no project details, got root %q and config %#v", contextData.ProjectRoot, contextData.Config)
		}
		if contextData.Message != "No .chronicle project configuration found in current directory tree" {
			t.Errorf("unexpected message: %q", contextData.Message)
		}
	})

	t.Run("returns parsed config from the production .chronicle file", func(t *testing.T) {
		projectRoot := t.TempDir()
		configPath := filepath.Join(projectRoot, ".chronicle")
		configContent := "local_logging = true\nlog_dir = \"project-logs\"\nlog_format = \"json\"\n"
		if err := os.WriteFile(configPath, []byte(configContent), 0o600); err != nil {
			t.Fatalf("failed to write project config: %v", err)
		}
		nestedDir := filepath.Join(projectRoot, "nested", "directory")
		if err := os.MkdirAll(nestedDir, 0o755); err != nil {
			t.Fatalf("failed to create nested directory: %v", err)
		}
		t.Chdir(nestedDir)

		result, err := server.handleProjectContext(context.Background(), nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		var contextData struct {
			HasProjectConfig bool                  `json:"has_project_config"`
			ProjectRoot      string                `json:"project_root"`
			Config           *config.ProjectConfig `json:"config"`
			Message          string                `json:"message"`
		}
		if err := json.Unmarshal([]byte(result.Contents[0].Text), &contextData); err != nil {
			t.Fatalf("failed to parse JSON: %v", err)
		}
		if !contextData.HasProjectConfig {
			t.Error("expected has_project_config to be true")
		}
		if contextData.ProjectRoot != projectRoot {
			t.Errorf("expected project root %q, got %q", projectRoot, contextData.ProjectRoot)
		}
		if contextData.Config == nil {
			t.Fatal("expected parsed project config")
		}
		if !contextData.Config.LocalLogging || contextData.Config.LogDir != "project-logs" || contextData.Config.LogFormat != "json" {
			t.Errorf("unexpected parsed config: %#v", contextData.Config)
		}
		if contextData.Message != "Project-specific chronicle configuration found" {
			t.Errorf("unexpected message: %q", contextData.Message)
		}
	})

	t.Run("reports a discovered but malformed config without parsed details", func(t *testing.T) {
		projectRoot := t.TempDir()
		if err := os.WriteFile(filepath.Join(projectRoot, ".chronicle"), []byte("not valid = [toml"), 0o600); err != nil {
			t.Fatalf("failed to write malformed project config: %v", err)
		}
		t.Chdir(projectRoot)

		result, err := server.handleProjectContext(context.Background(), nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		var contextData struct {
			HasProjectConfig bool                  `json:"has_project_config"`
			ProjectRoot      string                `json:"project_root"`
			Config           *config.ProjectConfig `json:"config"`
			Message          string                `json:"message"`
		}
		if err := json.Unmarshal([]byte(result.Contents[0].Text), &contextData); err != nil {
			t.Fatalf("failed to parse JSON: %v", err)
		}
		if !contextData.HasProjectConfig || contextData.ProjectRoot != projectRoot {
			t.Errorf("expected discovered project root %q, got %#v", projectRoot, contextData)
		}
		if contextData.Config != nil {
			t.Errorf("expected no parsed config, got %#v", contextData.Config)
		}
		if !strings.HasPrefix(contextData.Message, "Failed to load .chronicle project configuration: ") {
			t.Errorf("expected descriptive config error, got %q", contextData.Message)
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

func TestRegisteredResourceDescriptors(t *testing.T) {
	server, store := newTestServer(t)
	defer store.Close()
	server.registerResources()
	clientSession := connectTestClient(t, server.mcpServer)

	result, err := clientSession.ListResources(context.Background(), nil)
	if err != nil {
		t.Fatalf("failed to list registered resources: %v", err)
	}
	if len(result.Resources) != 4 {
		t.Fatalf("expected 4 resources, got %d", len(result.Resources))
	}

	descriptions := make(map[string]string, len(result.Resources))
	for _, resource := range result.Resources {
		descriptions[resource.URI] = resource.Description
	}

	want := map[string]string{
		"chronicle://recent-activity": "Last 10 chronicle entries with full metadata",
		"chronicle://tags":            "Tag usage counts keyed by tag",
		"chronicle://today-summary":   "Today's entries as timestamped message bullets",
		"chronicle://project-context": "Current directory's project root and .chronicle configuration",
	}
	for uri, description := range want {
		if descriptions[uri] != description {
			t.Errorf("resource %s: expected description %q, got %q", uri, description, descriptions[uri])
		}
	}
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
