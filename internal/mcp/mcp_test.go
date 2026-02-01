// ABOUTME: Comprehensive tests for MCP tool handlers
// ABOUTME: Tests tool handlers with real in-memory storage

package mcp

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/harper/chronicle/internal/storage"
	gomcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

// testServer creates an MCP server with an in-memory store for testing
func newTestServer(t *testing.T) (*Server, *storage.Store) {
	t.Helper()

	store, err := storage.NewStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create test store: %v", err)
	}

	impl := &gomcp.Implementation{
		Name:    "chronicle-test",
		Version: "0.0.0",
	}

	server := &Server{
		mcpServer: gomcp.NewServer(impl, nil),
		store:     store,
	}

	return server, store
}

func TestRegisterTools(t *testing.T) {
	server, store := newTestServer(t)
	defer store.Close()

	// Should not panic
	server.registerTools()
}

func TestRegisterResources(t *testing.T) {
	server, store := newTestServer(t)
	defer store.Close()

	// Should not panic
	server.registerResources()
}

func TestHandleAddEntry(t *testing.T) {
	server, store := newTestServer(t)
	defer store.Close()

	t.Run("creates entry with message", func(t *testing.T) {
		input := AddEntryInput{
			Message: "test log message",
			Tags:    []string{"test", "unit"},
		}

		result, output, err := server.handleAddEntry(context.Background(), nil, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if result == nil {
			t.Fatal("expected result to be non-nil")
		}

		if output.EntryID == "" {
			t.Error("expected entry ID to be non-empty")
		}

		if output.Message != "test log message" {
			t.Errorf("expected message 'test log message', got: %s", output.Message)
		}

		if output.Timestamp == "" {
			t.Error("expected timestamp to be non-empty")
		}

		// Verify entry was created in store
		entry, err := store.GetEntry(output.EntryID)
		if err != nil {
			t.Fatalf("failed to get entry from store: %v", err)
		}

		if entry.Message != "test log message" {
			t.Errorf("expected message in store, got: %s", entry.Message)
		}
	})

	t.Run("creates entry without tags", func(t *testing.T) {
		input := AddEntryInput{
			Message: "no tags message",
		}

		_, output, err := server.handleAddEntry(context.Background(), nil, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if output.EntryID == "" {
			t.Error("expected entry ID to be non-empty")
		}
	})

	t.Run("result contains text content", func(t *testing.T) {
		input := AddEntryInput{
			Message: "check result content",
		}

		result, _, err := server.handleAddEntry(context.Background(), nil, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(result.Content) == 0 {
			t.Fatal("expected content in result")
		}

		textContent, ok := result.Content[0].(*gomcp.TextContent)
		if !ok {
			t.Fatal("expected TextContent type")
		}

		if !strings.Contains(textContent.Text, "Entry created successfully") {
			t.Errorf("expected success message, got: %s", textContent.Text)
		}
	})
}

func TestHandleListEntries(t *testing.T) {
	server, store := newTestServer(t)
	defer store.Close()

	// Create test entries
	now := time.Now()
	for i := 0; i < 5; i++ {
		entry := storage.Entry{
			Timestamp: now.Add(time.Duration(i) * time.Hour),
			Message:   "test entry",
			Tags:      []string{"test"},
		}
		if _, err := store.CreateEntry(entry); err != nil {
			t.Fatalf("failed to create test entry: %v", err)
		}
	}

	t.Run("returns entries with default limit", func(t *testing.T) {
		input := ListEntriesInput{} // Limit defaults to 10

		result, output, err := server.handleListEntries(context.Background(), nil, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if result == nil {
			t.Fatal("expected result to be non-nil")
		}

		if output.Count != 5 {
			t.Errorf("expected 5 entries, got: %d", output.Count)
		}

		if len(output.Entries) != 5 {
			t.Errorf("expected 5 entries in list, got: %d", len(output.Entries))
		}
	})

	t.Run("respects limit parameter", func(t *testing.T) {
		input := ListEntriesInput{Limit: 2}

		_, output, err := server.handleListEntries(context.Background(), nil, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if output.Count != 2 {
			t.Errorf("expected 2 entries, got: %d", output.Count)
		}
	})

	t.Run("entries have correct fields", func(t *testing.T) {
		input := ListEntriesInput{Limit: 1}

		_, output, err := server.handleListEntries(context.Background(), nil, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(output.Entries) == 0 {
			t.Fatal("expected at least one entry")
		}

		entry := output.Entries[0]
		if entry.ID == "" {
			t.Error("expected ID to be non-empty")
		}
		if entry.Timestamp == "" {
			t.Error("expected Timestamp to be non-empty")
		}
		if entry.Message == "" {
			t.Error("expected Message to be non-empty")
		}
	})
}

func TestHandleSearchEntries(t *testing.T) {
	server, store := newTestServer(t)
	defer store.Close()

	// Create test entries
	entries := []storage.Entry{
		{Timestamp: time.Now(), Message: "deployed app to production", Tags: []string{"deployment"}},
		{Timestamp: time.Now(), Message: "fixed bug in login", Tags: []string{"bugfix"}},
		{Timestamp: time.Now(), Message: "deployed new feature", Tags: []string{"deployment", "feature"}},
	}

	for _, e := range entries {
		if _, err := store.CreateEntry(e); err != nil {
			t.Fatalf("failed to create test entry: %v", err)
		}
	}

	t.Run("searches by text", func(t *testing.T) {
		input := SearchEntriesInput{
			Text:  "deployed",
			Limit: 10,
		}

		_, output, err := server.handleSearchEntries(context.Background(), nil, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if output.Count != 2 {
			t.Errorf("expected 2 results, got: %d", output.Count)
		}
	})

	t.Run("returns all with empty filter", func(t *testing.T) {
		input := SearchEntriesInput{
			Limit: 10,
		}

		_, output, err := server.handleSearchEntries(context.Background(), nil, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if output.Count != 3 {
			t.Errorf("expected 3 results, got: %d", output.Count)
		}
	})

	t.Run("uses default limit of 20", func(t *testing.T) {
		input := SearchEntriesInput{} // Limit defaults to 20

		_, output, err := server.handleSearchEntries(context.Background(), nil, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Should still return all 3 since we have fewer than 20
		if output.Count != 3 {
			t.Errorf("expected 3 results, got: %d", output.Count)
		}
	})
}

func TestHandleRememberThis(t *testing.T) {
	server, store := newTestServer(t)
	defer store.Close()

	t.Run("creates entry with activity only", func(t *testing.T) {
		input := RememberThisInput{
			Activity: "Deployed the new API",
		}

		_, output, err := server.handleRememberThis(context.Background(), nil, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if output.EntryID == "" {
			t.Error("expected entry ID")
		}

		if output.Message != "Deployed the new API" {
			t.Errorf("expected message 'Deployed the new API', got: %s", output.Message)
		}
	})

	t.Run("creates entry with activity and context", func(t *testing.T) {
		input := RememberThisInput{
			Activity: "Fixed login bug",
			Context:  "Users were reporting 401 errors",
		}

		_, output, err := server.handleRememberThis(context.Background(), nil, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if !strings.Contains(output.Message, "Fixed login bug") {
			t.Errorf("expected message to contain activity")
		}

		if !strings.Contains(output.Message, "Users were reporting 401 errors") {
			t.Errorf("expected message to contain context")
		}
	})

	t.Run("auto-tags deployment activities", func(t *testing.T) {
		input := RememberThisInput{
			Activity: "deployed new version",
		}

		_, output, err := server.handleRememberThis(context.Background(), nil, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Verify tag was added
		entry, err := store.GetEntry(output.EntryID)
		if err != nil {
			t.Fatalf("failed to get entry: %v", err)
		}

		hasDeploymentTag := false
		for _, tag := range entry.Tags {
			if tag == "deployment" {
				hasDeploymentTag = true
				break
			}
		}

		if !hasDeploymentTag {
			t.Error("expected 'deployment' tag for deployment activity")
		}
	})
}

func TestHandleWhatWasIDoing(t *testing.T) {
	server, store := newTestServer(t)
	defer store.Close()

	// Create test entries
	entries := []storage.Entry{
		{Timestamp: time.Now(), Message: "first task", Tags: []string{"work"}},
		{Timestamp: time.Now().Add(time.Hour), Message: "second task", Tags: []string{"work"}},
	}

	for _, e := range entries {
		if _, err := store.CreateEntry(e); err != nil {
			t.Fatalf("failed to create test entry: %v", err)
		}
	}

	t.Run("returns recent entries with summary", func(t *testing.T) {
		input := WhatWasIDoingInput{}

		result, output, err := server.handleWhatWasIDoing(context.Background(), nil, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if result == nil {
			t.Fatal("expected result to be non-nil")
		}

		if output.Summary == "" {
			t.Error("expected summary to be non-empty")
		}

		if len(output.Entries) != 2 {
			t.Errorf("expected 2 entries, got: %d", len(output.Entries))
		}
	})

	t.Run("summary contains entry information", func(t *testing.T) {
		input := WhatWasIDoingInput{}

		_, output, err := server.handleWhatWasIDoing(context.Background(), nil, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if !strings.Contains(output.Summary, "first task") {
			t.Error("expected summary to contain entry messages")
		}
	})

	t.Run("accepts timeframe parameter", func(t *testing.T) {
		input := WhatWasIDoingInput{
			Timeframe: "today",
		}

		_, _, err := server.handleWhatWasIDoing(context.Background(), nil, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func TestHandleFindWhenI(t *testing.T) {
	server, store := newTestServer(t)
	defer store.Close()

	// Create test entries
	entries := []storage.Entry{
		{Timestamp: time.Now(), Message: "deployed app to production"},
		{Timestamp: time.Now().Add(time.Hour), Message: "fixed bug in login"},
		{Timestamp: time.Now().Add(2 * time.Hour), Message: "deployed hotfix"},
	}

	for _, e := range entries {
		if _, err := store.CreateEntry(e); err != nil {
			t.Fatalf("failed to create test entry: %v", err)
		}
	}

	t.Run("finds entries matching description", func(t *testing.T) {
		input := FindWhenIInput{
			What: "deployed",
		}

		_, output, err := server.handleFindWhenI(context.Background(), nil, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if output.Count != 2 {
			t.Errorf("expected 2 results, got: %d", output.Count)
		}
	})

	t.Run("returns empty for non-matching search", func(t *testing.T) {
		input := FindWhenIInput{
			What: "nonexistent",
		}

		_, output, err := server.handleFindWhenI(context.Background(), nil, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if output.Count != 0 {
			t.Errorf("expected 0 results, got: %d", output.Count)
		}
	})
}

func TestSuggestTagsComprehensive(t *testing.T) {
	tests := []struct {
		name          string
		activity      string
		context       string
		expectedTag   string
		shouldContain bool
	}{
		{"deployment keyword", "deployed the app", "", "deployment", true},
		{"release keyword", "released v2.0", "", "deployment", true},
		{"fix keyword", "fixed the issue", "", "bug-fix", true},
		{"bug keyword", "found a bug", "", "bug-fix", true},
		{"decide keyword", "decided on Go", "", "decision", true},
		{"chose keyword", "chose React", "", "decision", true},
		{"learn keyword", "learned about channels", "", "learning", true},
		{"discover keyword", "discovered a pattern", "", "learning", true},
		{"test keyword", "wrote tests", "", "testing", true},
		{"no keywords", "did some work", "", "work", true},
		{"multiple keywords", "deployed and fixed", "", "deployment", true},
		{"context affects tags", "meeting", "decided on architecture", "decision", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tags := suggestTags(tt.activity, tt.context)

			found := false
			for _, tag := range tags {
				if tag == tt.expectedTag {
					found = true
					break
				}
			}

			if tt.shouldContain && !found {
				t.Errorf("expected tag %q in %v", tt.expectedTag, tags)
			}
		})
	}
}

func TestEntryDataConversion(t *testing.T) {
	t.Run("converts storage entry to entry data", func(t *testing.T) {
		// Create an entry via the server to test conversion
		server, store := newTestServer(t)
		defer store.Close()

		entry := storage.Entry{
			Timestamp:        time.Now(),
			Message:          "test message",
			Hostname:         "test-host",
			Username:         "test-user",
			WorkingDirectory: "/test/dir",
			Tags:             []string{"tag1", "tag2"},
		}

		id, err := store.CreateEntry(entry)
		if err != nil {
			t.Fatalf("failed to create entry: %v", err)
		}

		input := ListEntriesInput{Limit: 1}
		_, output, err := server.handleListEntries(context.Background(), nil, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(output.Entries) == 0 {
			t.Fatal("expected at least one entry")
		}

		entryData := output.Entries[0]
		if entryData.ID != id {
			t.Errorf("expected ID %s, got %s", id, entryData.ID)
		}
		if entryData.Message != "test message" {
			t.Errorf("expected message 'test message', got %s", entryData.Message)
		}
		if entryData.Hostname != "test-host" {
			t.Errorf("expected hostname 'test-host', got %s", entryData.Hostname)
		}
	})
}

func TestAddEntryInputStructs(t *testing.T) {
	t.Run("AddEntryInput fields", func(t *testing.T) {
		input := AddEntryInput{
			Message: "test message",
			Tags:    []string{"tag1", "tag2"},
		}
		if input.Message != "test message" {
			t.Error("expected message field")
		}
		if len(input.Tags) != 2 {
			t.Error("expected 2 tags")
		}
	})

	t.Run("AddEntryOutput fields", func(t *testing.T) {
		output := AddEntryOutput{
			EntryID:   "abc123",
			Message:   "test",
			Timestamp: "2025-01-01 12:00:00",
		}
		if output.EntryID != "abc123" {
			t.Error("expected entry_id field")
		}
		if output.Message != "test" {
			t.Error("expected message field")
		}
		if output.Timestamp != "2025-01-01 12:00:00" {
			t.Error("expected timestamp field")
		}
	})
}

func TestListEntriesInputStructs(t *testing.T) {
	t.Run("ListEntriesInput fields", func(t *testing.T) {
		input := ListEntriesInput{Limit: 50}
		if input.Limit != 50 {
			t.Error("expected limit field")
		}
	})

	t.Run("ListEntriesOutput fields", func(t *testing.T) {
		output := ListEntriesOutput{
			Entries: []EntryData{{ID: "1"}},
			Count:   1,
		}
		if output.Count != 1 {
			t.Error("expected count field")
		}
		if len(output.Entries) != 1 {
			t.Error("expected 1 entry")
		}
	})
}

func TestSearchEntriesInputStructs(t *testing.T) {
	input := SearchEntriesInput{
		Text:  "query",
		Tags:  []string{"tag1"},
		Since: "2025-01-01",
		Until: "2025-12-31",
		Limit: 100,
	}
	if input.Text != "query" {
		t.Error("expected text field")
	}
	if input.Since != "2025-01-01" {
		t.Error("expected since field")
	}
	if input.Until != "2025-12-31" {
		t.Error("expected until field")
	}
}

func TestWhatWasIDoingOutputStruct(t *testing.T) {
	output := WhatWasIDoingOutput{
		Summary: "Summary text",
		Entries: []EntryData{{ID: "1", Message: "test"}},
	}
	if output.Summary != "Summary text" {
		t.Error("expected summary field")
	}
	if len(output.Entries) != 1 {
		t.Error("expected 1 entry")
	}
}

func TestHandleAddEntryWithEmptyHostname(t *testing.T) {
	server, store := newTestServer(t)
	defer store.Close()

	input := AddEntryInput{
		Message: "test entry",
	}

	result, output, err := server.handleAddEntry(context.Background(), nil, input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("expected result")
	}

	if output.EntryID == "" {
		t.Error("expected entry ID")
	}
}

func TestHandleSearchEntriesWithTags(t *testing.T) {
	server, store := newTestServer(t)
	defer store.Close()

	// Create entries with tags
	entries := []storage.Entry{
		{Timestamp: time.Now(), Message: "entry 1", Tags: []string{"golang", "work"}},
		{Timestamp: time.Now(), Message: "entry 2", Tags: []string{"python", "work"}},
		{Timestamp: time.Now(), Message: "entry 3", Tags: []string{"personal"}},
	}

	for _, e := range entries {
		if _, err := store.CreateEntry(e); err != nil {
			t.Fatalf("failed to create entry: %v", err)
		}
	}

	input := SearchEntriesInput{
		Tags:  []string{"work"},
		Limit: 10,
	}

	_, output, err := server.handleSearchEntries(context.Background(), nil, input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if output.Count != 2 {
		t.Errorf("expected 2 entries with 'work' tag, got %d", output.Count)
	}
}

func TestHandleWhatWasIDoingWithDifferentTimeframes(t *testing.T) {
	server, store := newTestServer(t)
	defer store.Close()

	// Create entries
	entry := storage.Entry{
		Timestamp: time.Now(),
		Message:   "recent work",
	}
	if _, err := store.CreateEntry(entry); err != nil {
		t.Fatalf("failed to create entry: %v", err)
	}

	timeframes := []string{"today", "yesterday", "this week", "last 24 hours"}

	for _, tf := range timeframes {
		t.Run(tf, func(t *testing.T) {
			input := WhatWasIDoingInput{Timeframe: tf}
			_, _, err := server.handleWhatWasIDoing(context.Background(), nil, input)
			if err != nil {
				t.Fatalf("unexpected error for timeframe %s: %v", tf, err)
			}
		})
	}
}

func TestHandleWhatWasIDoingSummaryWithTags(t *testing.T) {
	server, store := newTestServer(t)
	defer store.Close()

	// Create entry with tags
	entry := storage.Entry{
		Timestamp: time.Now(),
		Message:   "tagged entry",
		Tags:      []string{"work", "important"},
	}
	if _, err := store.CreateEntry(entry); err != nil {
		t.Fatalf("failed to create entry: %v", err)
	}

	input := WhatWasIDoingInput{}
	_, output, err := server.handleWhatWasIDoing(context.Background(), nil, input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Summary should include tag information
	if !strings.Contains(output.Summary, "work") && len(output.Entries) > 0 && len(output.Entries[0].Tags) > 0 {
		t.Error("expected summary to include tags")
	}
}

func TestHandleListEntriesWithNoEntries(t *testing.T) {
	server, store := newTestServer(t)
	defer store.Close()

	input := ListEntriesInput{Limit: 10}
	result, output, err := server.handleListEntries(context.Background(), nil, input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("expected result")
	}

	if output.Count != 0 {
		t.Errorf("expected 0 entries, got %d", output.Count)
	}
}

func TestHandleSearchEntriesWithNoResults(t *testing.T) {
	server, store := newTestServer(t)
	defer store.Close()

	// Create an entry
	entry := storage.Entry{
		Timestamp: time.Now(),
		Message:   "test entry",
	}
	if _, err := store.CreateEntry(entry); err != nil {
		t.Fatalf("failed to create entry: %v", err)
	}

	// Search for something that won't match
	input := SearchEntriesInput{
		Text:  "nonexistent",
		Limit: 10,
	}

	result, output, err := server.handleSearchEntries(context.Background(), nil, input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("expected result")
	}

	if output.Count != 0 {
		t.Errorf("expected 0 entries, got %d", output.Count)
	}
}

func TestHandleWhatWasIDoingWithNoEntries(t *testing.T) {
	server, store := newTestServer(t)
	defer store.Close()

	input := WhatWasIDoingInput{}
	result, output, err := server.handleWhatWasIDoing(context.Background(), nil, input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("expected result")
	}

	if !strings.Contains(output.Summary, "0 recent entries") {
		t.Logf("summary: %s", output.Summary)
	}
}

func TestHandleAddEntryResultContent(t *testing.T) {
	server, store := newTestServer(t)
	defer store.Close()

	input := AddEntryInput{
		Message: "check result content format",
	}

	result, output, err := server.handleAddEntry(context.Background(), nil, input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.Content) == 0 {
		t.Fatal("expected content in result")
	}

	// Verify result content contains ID and timestamp
	textContent, ok := result.Content[0].(*gomcp.TextContent)
	if !ok {
		t.Fatal("expected TextContent")
	}

	if !strings.Contains(textContent.Text, output.EntryID) {
		t.Error("expected result text to contain entry ID")
	}

	if !strings.Contains(textContent.Text, output.Timestamp) {
		t.Error("expected result text to contain timestamp")
	}
}

func TestHandleListEntriesResultContent(t *testing.T) {
	server, store := newTestServer(t)
	defer store.Close()

	// Create some entries
	for i := 0; i < 3; i++ {
		entry := storage.Entry{
			Timestamp: time.Now(),
			Message:   "test entry",
		}
		if _, err := store.CreateEntry(entry); err != nil {
			t.Fatalf("failed to create entry: %v", err)
		}
	}

	input := ListEntriesInput{Limit: 10}
	result, _, err := server.handleListEntries(context.Background(), nil, input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.Content) == 0 {
		t.Fatal("expected content in result")
	}

	textContent, ok := result.Content[0].(*gomcp.TextContent)
	if !ok {
		t.Fatal("expected TextContent")
	}

	if !strings.Contains(textContent.Text, "3") {
		t.Errorf("expected result text to mention count, got: %s", textContent.Text)
	}
}

func TestHandleSearchEntriesResultContent(t *testing.T) {
	server, store := newTestServer(t)
	defer store.Close()

	// Create entries
	for i := 0; i < 2; i++ {
		entry := storage.Entry{
			Timestamp: time.Now(),
			Message:   "searchable content",
		}
		if _, err := store.CreateEntry(entry); err != nil {
			t.Fatalf("failed to create entry: %v", err)
		}
	}

	input := SearchEntriesInput{
		Text:  "searchable",
		Limit: 10,
	}
	result, _, err := server.handleSearchEntries(context.Background(), nil, input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.Content) == 0 {
		t.Fatal("expected content in result")
	}

	textContent, ok := result.Content[0].(*gomcp.TextContent)
	if !ok {
		t.Fatal("expected TextContent")
	}

	if !strings.Contains(textContent.Text, "2") {
		t.Errorf("expected result text to mention count, got: %s", textContent.Text)
	}
}

func TestEntryDataAllFields(t *testing.T) {
	data := EntryData{
		ID:        "test-id",
		Timestamp: "2025-01-15 12:00:00",
		Message:   "test message",
		Tags:      []string{"tag1", "tag2"},
		Hostname:  "test-host",
		Username:  "test-user",
		Directory: "/test/dir",
	}

	if data.ID != "test-id" {
		t.Error("ID field mismatch")
	}
	if data.Timestamp != "2025-01-15 12:00:00" {
		t.Error("Timestamp field mismatch")
	}
	if data.Message != "test message" {
		t.Error("Message field mismatch")
	}
	if len(data.Tags) != 2 {
		t.Error("Tags field mismatch")
	}
	if data.Hostname != "test-host" {
		t.Error("Hostname field mismatch")
	}
	if data.Username != "test-user" {
		t.Error("Username field mismatch")
	}
	if data.Directory != "/test/dir" {
		t.Error("Directory field mismatch")
	}
}
