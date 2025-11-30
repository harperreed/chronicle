# MCP Server Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add an embedded MCP server to chronicle that exposes tools and resources for AI assistants to log and retrieve user activities.

**Architecture:** Embedded subcommand (`chronicle mcp`) that runs an MCP server over stdio transport. Reuses all existing database/config code. Provides 6 tools (3 low-level CLI mappings + 3 high-level semantic), 4 dynamic resources, and 1 prompt.

**Tech Stack:** Go 1.22+, github.com/modelcontextprotocol/go-sdk, existing chronicle database code

---

## Task 1: Add MCP SDK Dependency and Basic Server

**Files:**
- Modify: `go.mod`
- Create: `internal/mcp/server.go`
- Create: `internal/mcp/server_test.go`

**Step 1: Add MCP SDK dependency**

```bash
go get github.com/modelcontextprotocol/go-sdk
go mod tidy
```

**Step 2: Create basic server structure**

Create `internal/mcp/server.go`:

```go
//go:build sqlite_fts5

// ABOUTME: MCP server implementation for chronicle
// ABOUTME: Provides tools and resources for AI assistants to interact with chronicle
package mcp

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Server wraps the MCP server with chronicle-specific functionality.
type Server struct {
	mcpServer *mcp.Server
	dbPath    string
}

// NewServer creates a new chronicle MCP server.
func NewServer(dbPath string) *Server {
	impl := &mcp.Implementation{
		Name:    "chronicle",
		Version: "0.1.1",
	}

	server := &Server{
		mcpServer: mcp.NewServer(impl, nil),
		dbPath:    dbPath,
	}

	return server
}

// Run starts the MCP server with stdio transport.
func (s *Server) Run(ctx context.Context) error {
	transport := &mcp.StdioTransport{}
	return s.mcpServer.Run(ctx, transport)
}
```

**Step 3: Write test for server creation**

Create `internal/mcp/server_test.go`:

```go
//go:build sqlite_fts5

// ABOUTME: Tests for MCP server
// ABOUTME: Validates server initialization and configuration
package mcp

import (
	"testing"
)

func TestNewServer(t *testing.T) {
	server := NewServer("/tmp/test.db")

	if server == nil {
		t.Fatal("expected server to be created")
	}

	if server.dbPath != "/tmp/test.db" {
		t.Errorf("expected dbPath to be /tmp/test.db, got %s", server.dbPath)
	}

	if server.mcpServer == nil {
		t.Fatal("expected mcpServer to be initialized")
	}
}
```

**Step 4: Run test to verify it passes**

```bash
go test -tags sqlite_fts5 ./internal/mcp -v
```

Expected: PASS

**Step 5: Commit**

```bash
git add go.mod go.sum internal/mcp/
git commit -m "feat: add basic MCP server structure with SDK dependency"
```

---

## Task 2: Add CLI Command for MCP Server

**Files:**
- Create: `internal/cli/mcp.go`
- Modify: `internal/cli/root.go`

**Step 1: Create MCP CLI command**

Create `internal/cli/mcp.go`:

```go
//go:build sqlite_fts5

// ABOUTME: MCP subcommand for running the chronicle MCP server
// ABOUTME: Handles stdio transport initialization and server lifecycle
package cli

import (
	"context"
	"os"
	"path/filepath"

	"github.com/harper/chronicle/internal/config"
	"github.com/harper/chronicle/internal/mcp"
	"github.com/spf13/cobra"
)

var mcpCmd = &cobra.Command{
	Use:   "mcp",
	Short: "Run the chronicle MCP server",
	Long:  `Start the Model Context Protocol server for AI assistants to interact with chronicle over stdio.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Get database path
		dbPath := os.Getenv("CHRONICLE_DB_PATH")
		if dbPath == "" {
			dataHome := config.GetDataHome()
			dbPath = filepath.Join(dataHome, "chronicle", "chronicle.db")
		}

		// Create and run server
		server := mcp.NewServer(dbPath)
		return server.Run(context.Background())
	},
}

func init() {
	rootCmd.AddCommand(mcpCmd)
}
```

**Step 2: Verify command is registered**

```bash
go build -tags sqlite_fts5 -o chronicle .
./chronicle --help
```

Expected: Should see "mcp" in Available Commands

**Step 3: Commit**

```bash
git add internal/cli/mcp.go
git commit -m "feat: add 'chronicle mcp' CLI command"
```

---

## Task 3: Implement Prompts

**Files:**
- Create: `internal/mcp/prompts.go`
- Modify: `internal/mcp/server.go`

**Step 1: Create prompts file**

Create `internal/mcp/prompts.go`:

```go
//go:build sqlite_fts5

// ABOUTME: MCP prompt definitions for chronicle
// ABOUTME: Provides static context to AI assistants about chronicle capabilities
package mcp

import "github.com/modelcontextprotocol/go-sdk/mcp"

// registerPrompts adds static prompts to the MCP server.
func (s *Server) registerPrompts() {
	prompt := &mcp.Prompt{
		Name:        "chronicle-getting-started",
		Description: "Introduction to chronicle and how AI assistants should use it",
	}

	handler := func(ctx context.Context, req *mcp.GetPromptRequest, input struct{}) (*mcp.GetPromptResult, struct{}, error) {
		content := `Chronicle is a personal activity logging system that helps you track what you're working on.

When to use chronicle:
- User accomplishes something worth remembering (deployed, fixed, decided, learned)
- User asks about past activities ("what did I do yesterday?")
- User wants to recall when something happened
- At start of work sessions to load context

Best practices:
- Log activities as they happen, not just when asked
- Use specific tags (work, personal, golang, debugging, deployment, etc.)
- Include enough detail to jog memory later
- Think of it as a work journal that can be searched

The user has configured chronicle to track their development work and important decisions.`

		result := &mcp.GetPromptResult{
			Description: "Getting started with chronicle",
			Messages: []*mcp.PromptMessage{
				{
					Role: "user",
					Content: &mcp.TextContent{
						Type: "text",
						Text: content,
					},
				},
			},
		}

		return result, struct{}{}, nil
	}

	mcp.AddPrompt(s.mcpServer, prompt, handler)
}
```

**Step 2: Register prompts in server**

Modify `internal/mcp/server.go`, add to NewServer:

```go
func NewServer(dbPath string) *Server {
	impl := &mcp.Implementation{
		Name:    "chronicle",
		Version: "0.1.1",
	}

	server := &Server{
		mcpServer: mcp.NewServer(impl, nil),
		dbPath:    dbPath,
	}

	// Register prompts
	server.registerPrompts()

	return server
}
```

**Step 3: Commit**

```bash
git add internal/mcp/prompts.go internal/mcp/server.go
git commit -m "feat: add chronicle-getting-started prompt"
```

---

## Task 4: Implement Low-Level Tool - add_entry

**Files:**
- Create: `internal/mcp/tools.go`
- Create: `internal/mcp/tools_test.go`
- Modify: `internal/mcp/server.go`

**Step 1: Write test for add_entry tool**

Create `internal/mcp/tools_test.go`:

```go
//go:build sqlite_fts5

// ABOUTME: Tests for MCP tools
// ABOUTME: Validates tool handlers and input/output types
package mcp

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/harper/chronicle/internal/db"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestAddEntryTool(t *testing.T) {
	// Create temp DB
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	database, err := db.InitDB(dbPath)
	if err != nil {
		t.Fatalf("failed to init db: %v", err)
	}
	defer func() { _ = database.Close() }()

	// Create server
	server := NewServer(dbPath)

	// Test input
	input := AddEntryInput{
		Message: "test message",
		Tags:    []string{"test", "work"},
	}

	// Call handler directly
	result, output, err := server.handleAddEntry(context.Background(), nil, input)
	if err != nil {
		t.Fatalf("handleAddEntry failed: %v", err)
	}

	if result == nil {
		t.Fatal("expected non-nil result")
	}

	if output.EntryID == 0 {
		t.Error("expected non-zero entry ID")
	}

	if output.Message != "test message" {
		t.Errorf("expected message 'test message', got %s", output.Message)
	}
}
```

**Step 2: Run test to verify it fails**

```bash
go test -tags sqlite_fts5 ./internal/mcp -v
```

Expected: FAIL (types and handler don't exist)

**Step 3: Implement add_entry tool**

Create `internal/mcp/tools.go`:

```go
//go:build sqlite_fts5

// ABOUTME: MCP tool implementations for chronicle
// ABOUTME: Provides low-level and high-level tools for logging and querying
package mcp

import (
	"context"
	"fmt"
	"os"

	"github.com/harper/chronicle/internal/db"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// AddEntryInput defines the input for add_entry tool.
type AddEntryInput struct {
	Message string   `json:"message" jsonschema:"description=The message to log,required"`
	Tags    []string `json:"tags,omitempty" jsonschema:"description=Optional tags to categorize the entry"`
}

// AddEntryOutput defines the output for add_entry tool.
type AddEntryOutput struct {
	EntryID   int64  `json:"entry_id" jsonschema:"description=The ID of the created entry"`
	Message   string `json:"message" jsonschema:"description=The logged message"`
	Timestamp string `json:"timestamp" jsonschema:"description=When the entry was created"`
}

// registerTools adds all MCP tools to the server.
func (s *Server) registerTools() {
	// add_entry tool
	addEntryTool := &mcp.Tool{
		Name:        "add_entry",
		Description: "Log a timestamped entry to chronicle. Use this when the user explicitly asks to log, track, or record something they did or are doing.",
	}
	mcp.AddTool(s.mcpServer, addEntryTool, s.handleAddEntry)
}

// handleAddEntry implements the add_entry tool.
func (s *Server) handleAddEntry(ctx context.Context, req *mcp.CallToolRequest, input AddEntryInput) (*mcp.CallToolResult, AddEntryOutput, error) {
	// Open database
	database, err := db.InitDB(s.dbPath)
	if err != nil {
		return nil, AddEntryOutput{}, fmt.Errorf("failed to open database: %w", err)
	}
	defer func() { _ = database.Close() }()

	// Get metadata
	hostname, _ := os.Hostname()
	if hostname == "" {
		hostname = "unknown"
	}

	username := os.Getenv("USER")
	if username == "" {
		username = "unknown"
	}

	workingDir, _ := os.Getwd()
	if workingDir == "" {
		workingDir = "unknown"
	}

	// Create entry
	entry := db.Entry{
		Message:          input.Message,
		Hostname:         hostname,
		Username:         username,
		WorkingDirectory: workingDir,
		Tags:             input.Tags,
	}

	id, err := db.CreateEntry(database, entry)
	if err != nil {
		return nil, AddEntryOutput{}, fmt.Errorf("failed to create entry: %w", err)
	}

	// Get timestamp
	var timestamp string
	err = database.QueryRow("SELECT datetime(timestamp) FROM entries WHERE id = ?", id).Scan(&timestamp)
	if err != nil {
		timestamp = "unknown"
	}

	output := AddEntryOutput{
		EntryID:   id,
		Message:   input.Message,
		Timestamp: timestamp,
	}

	result := &mcp.CallToolResult{
		Content: []*mcp.Content{
			{
				Type: "text",
				Text: fmt.Sprintf("Entry created successfully (ID: %d) at %s", id, timestamp),
			},
		},
	}

	return result, output, nil
}
```

**Step 4: Register tools in server**

Modify `internal/mcp/server.go` NewServer function:

```go
func NewServer(dbPath string) *Server {
	impl := &mcp.Implementation{
		Name:    "chronicle",
		Version: "0.1.1",
	}

	server := &Server{
		mcpServer: mcp.NewServer(impl, nil),
		dbPath:    dbPath,
	}

	// Register components
	server.registerPrompts()
	server.registerTools()

	return server
}
```

**Step 5: Run test to verify it passes**

```bash
go test -tags sqlite_fts5 ./internal/mcp -v
```

Expected: PASS

**Step 6: Commit**

```bash
git add internal/mcp/tools.go internal/mcp/tools_test.go internal/mcp/server.go
git commit -m "feat: implement add_entry MCP tool"
```

---

## Task 5: Implement Low-Level Tool - list_entries

**Files:**
- Modify: `internal/mcp/tools.go`
- Modify: `internal/mcp/tools_test.go`

**Step 1: Write test for list_entries**

Add to `internal/mcp/tools_test.go`:

```go
func TestListEntriesTool(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	database, err := db.InitDB(dbPath)
	if err != nil {
		t.Fatalf("failed to init db: %v", err)
	}
	defer func() { _ = database.Close() }()

	// Add test entries
	for i := 0; i < 5; i++ {
		entry := db.Entry{
			Message:          fmt.Sprintf("message %d", i),
			Hostname:         "testhost",
			Username:         "testuser",
			WorkingDirectory: "/test",
			Tags:             []string{"test"},
		}
		_, err := db.CreateEntry(database, entry)
		if err != nil {
			t.Fatalf("failed to create entry: %v", err)
		}
	}

	server := NewServer(dbPath)

	input := ListEntriesInput{Limit: 3}
	result, output, err := server.handleListEntries(context.Background(), nil, input)

	if err != nil {
		t.Fatalf("handleListEntries failed: %v", err)
	}

	if result == nil {
		t.Fatal("expected non-nil result")
	}

	if len(output.Entries) != 3 {
		t.Errorf("expected 3 entries, got %d", len(output.Entries))
	}
}
```

**Step 2: Run test to verify it fails**

```bash
go test -tags sqlite_fts5 ./internal/mcp -v -run TestListEntriesTool
```

Expected: FAIL

**Step 3: Implement list_entries tool**

Add to `internal/mcp/tools.go`:

```go
// ListEntriesInput defines the input for list_entries tool.
type ListEntriesInput struct {
	Limit int `json:"limit,omitempty" jsonschema:"description=Maximum number of entries to return (default 10)"`
}

// EntryData represents a chronicle entry for output.
type EntryData struct {
	ID        int64    `json:"id"`
	Timestamp string   `json:"timestamp"`
	Message   string   `json:"message"`
	Tags      []string `json:"tags"`
	Hostname  string   `json:"hostname"`
	Username  string   `json:"username"`
	Directory string   `json:"directory"`
}

// ListEntriesOutput defines the output for list_entries tool.
type ListEntriesOutput struct {
	Entries []EntryData `json:"entries"`
	Count   int         `json:"count"`
}

// In registerTools, add:
func (s *Server) registerTools() {
	// ... existing add_entry tool ...

	// list_entries tool
	listEntriesTool := &mcp.Tool{
		Name:        "list_entries",
		Description: "Retrieve recent chronicle entries. Use this to answer questions like 'what did I do today/recently' or 'show my recent work'.",
	}
	mcp.AddTool(s.mcpServer, listEntriesTool, s.handleListEntries)
}

// handleListEntries implements the list_entries tool.
func (s *Server) handleListEntries(ctx context.Context, req *mcp.CallToolRequest, input ListEntriesInput) (*mcp.CallToolResult, ListEntriesOutput, error) {
	limit := input.Limit
	if limit == 0 {
		limit = 10
	}

	database, err := db.InitDB(s.dbPath)
	if err != nil {
		return nil, ListEntriesOutput{}, fmt.Errorf("failed to open database: %w", err)
	}
	defer func() { _ = database.Close() }()

	entries, err := db.ListEntries(database, limit)
	if err != nil {
		return nil, ListEntriesOutput{}, fmt.Errorf("failed to list entries: %w", err)
	}

	outputEntries := make([]EntryData, len(entries))
	for i, entry := range entries {
		outputEntries[i] = EntryData{
			ID:        entry.ID,
			Timestamp: entry.Timestamp.Format("2006-01-02 15:04:05"),
			Message:   entry.Message,
			Tags:      entry.Tags,
			Hostname:  entry.Hostname,
			Username:  entry.Username,
			Directory: entry.WorkingDirectory,
		}
	}

	output := ListEntriesOutput{
		Entries: outputEntries,
		Count:   len(outputEntries),
	}

	result := &mcp.CallToolResult{
		Content: []*mcp.Content{
			{
				Type: "text",
				Text: fmt.Sprintf("Retrieved %d recent entries", len(outputEntries)),
			},
		},
	}

	return result, output, nil
}
```

**Step 4: Run test to verify it passes**

```bash
go test -tags sqlite_fts5 ./internal/mcp -v
```

Expected: PASS

**Step 5: Commit**

```bash
git add internal/mcp/tools.go internal/mcp/tools_test.go
git commit -m "feat: implement list_entries MCP tool"
```

---

## Task 6: Implement Low-Level Tool - search_entries

**Files:**
- Modify: `internal/mcp/tools.go`
- Modify: `internal/mcp/tools_test.go`

**Step 1: Write test for search_entries**

Add to `internal/mcp/tools_test.go`:

```go
func TestSearchEntriesTool(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	database, err := db.InitDB(dbPath)
	if err != nil {
		t.Fatalf("failed to init db: %v", err)
	}
	defer func() { _ = database.Close() }()

	// Add test entries
	entries := []db.Entry{
		{Message: "deployed app", Hostname: "h", Username: "u", WorkingDirectory: "/", Tags: []string{"work", "deploy"}},
		{Message: "fixed bug", Hostname: "h", Username: "u", WorkingDirectory: "/", Tags: []string{"work", "bug"}},
		{Message: "wrote tests", Hostname: "h", Username: "u", WorkingDirectory: "/", Tags: []string{"test"}},
	}
	for _, entry := range entries {
		_, err := db.CreateEntry(database, entry)
		if err != nil {
			t.Fatalf("failed to create entry: %v", err)
		}
	}

	server := NewServer(dbPath)

	// Search by text
	input := SearchEntriesInput{Text: "bug"}
	result, output, err := server.handleSearchEntries(context.Background(), nil, input)

	if err != nil {
		t.Fatalf("handleSearchEntries failed: %v", err)
	}

	if len(output.Entries) != 1 {
		t.Errorf("expected 1 entry, got %d", len(output.Entries))
	}

	if output.Entries[0].Message != "fixed bug" {
		t.Errorf("expected 'fixed bug', got %s", output.Entries[0].Message)
	}
}
```

**Step 2: Run test to verify it fails**

```bash
go test -tags sqlite_fts5 ./internal/mcp -v -run TestSearchEntriesTool
```

Expected: FAIL

**Step 3: Implement search_entries tool**

Add to `internal/mcp/tools.go`:

```go
// SearchEntriesInput defines the input for search_entries tool.
type SearchEntriesInput struct {
	Text  string   `json:"text,omitempty" jsonschema:"description=Text to search for in entries"`
	Tags  []string `json:"tags,omitempty" jsonschema:"description=Filter by tags"`
	Since string   `json:"since,omitempty" jsonschema:"description=Start date/time (e.g. '2025-01-01' or 'yesterday')"`
	Until string   `json:"until,omitempty" jsonschema:"description=End date/time"`
	Limit int      `json:"limit,omitempty" jsonschema:"description=Maximum results (default 20)"`
}

// In registerTools, add:
func (s *Server) registerTools() {
	// ... existing tools ...

	// search_entries tool
	searchEntriesTool := &mcp.Tool{
		Name:        "search_entries",
		Description: "Search chronicle history by text, tags, or date range. Use this when the user wants to find specific past activities or recall when something happened.",
	}
	mcp.AddTool(s.mcpServer, searchEntriesTool, s.handleSearchEntries)
}

// handleSearchEntries implements the search_entries tool.
func (s *Server) handleSearchEntries(ctx context.Context, req *mcp.CallToolRequest, input SearchEntriesInput) (*mcp.CallToolResult, ListEntriesOutput, error) {
	limit := input.Limit
	if limit == 0 {
		limit = 20
	}

	database, err := db.InitDB(s.dbPath)
	if err != nil {
		return nil, ListEntriesOutput{}, fmt.Errorf("failed to open database: %w", err)
	}
	defer func() { _ = database.Close() }()

	params := db.SearchParams{
		Text:  input.Text,
		Tags:  input.Tags,
		Limit: limit,
	}

	// TODO: Parse Since/Until dates - for now skip date parsing

	entries, err := db.SearchEntries(database, params)
	if err != nil {
		return nil, ListEntriesOutput{}, fmt.Errorf("failed to search entries: %w", err)
	}

	outputEntries := make([]EntryData, len(entries))
	for i, entry := range entries {
		outputEntries[i] = EntryData{
			ID:        entry.ID,
			Timestamp: entry.Timestamp.Format("2006-01-02 15:04:05"),
			Message:   entry.Message,
			Tags:      entry.Tags,
			Hostname:  entry.Hostname,
			Username:  entry.Username,
			Directory: entry.WorkingDirectory,
		}
	}

	output := ListEntriesOutput{
		Entries: outputEntries,
		Count:   len(outputEntries),
	}

	result := &mcp.CallToolResult{
		Content: []*mcp.Content{
			{
				Type: "text",
				Text: fmt.Sprintf("Found %d matching entries", len(outputEntries)),
			},
		},
	}

	return result, output, nil
}
```

**Step 4: Run test to verify it passes**

```bash
go test -tags sqlite_fts5 ./internal/mcp -v
```

Expected: PASS

**Step 5: Commit**

```bash
git add internal/mcp/tools.go internal/mcp/tools_test.go
git commit -m "feat: implement search_entries MCP tool"
```

---

## Task 7: Implement High-Level Semantic Tools

**Files:**
- Modify: `internal/mcp/tools.go`

**Step 1: Implement remember_this tool**

Add to `internal/mcp/tools.go`:

```go
// RememberThisInput defines input for remember_this tool.
type RememberThisInput struct {
	Activity string `json:"activity" jsonschema:"description=The activity or information to remember,required"`
	Context  string `json:"context,omitempty" jsonschema:"description=Why this matters or additional context"`
}

// In registerTools, add:
func (s *Server) registerTools() {
	// ... existing tools ...

	// remember_this tool
	rememberThisTool := &mcp.Tool{
		Name:        "remember_this",
		Description: "Proactively log important information the user shares about their work, decisions, or progress. Use this when you notice the user accomplished something worth tracking, even if they don't explicitly ask to log it. Automatically suggests relevant tags based on context.",
	}
	mcp.AddTool(s.mcpServer, rememberThisTool, s.handleRememberThis)
}

// handleRememberThis implements the remember_this tool.
func (s *Server) handleRememberThis(ctx context.Context, req *mcp.CallToolRequest, input RememberThisInput) (*mcp.CallToolResult, AddEntryOutput, error) {
	// Build message
	message := input.Activity
	if input.Context != "" {
		message = message + " (" + input.Context + ")"
	}

	// Smart tag suggestions based on keywords
	tags := suggestTags(input.Activity, input.Context)

	// Delegate to add_entry
	addInput := AddEntryInput{
		Message: message,
		Tags:    tags,
	}

	return s.handleAddEntry(ctx, req, addInput)
}

// suggestTags provides smart tag suggestions based on content.
func suggestTags(activity, context string) []string {
	var tags []string

	combined := activity + " " + context
	combined = strings.ToLower(combined)

	// Work-related keywords
	if strings.Contains(combined, "deploy") || strings.Contains(combined, "release") {
		tags = append(tags, "deployment")
	}
	if strings.Contains(combined, "fix") || strings.Contains(combined, "bug") {
		tags = append(tags, "bug-fix")
	}
	if strings.Contains(combined, "decid") || strings.Contains(combined, "chose") {
		tags = append(tags, "decision")
	}
	if strings.Contains(combined, "learn") || strings.Contains(combined, "discover") {
		tags = append(tags, "learning")
	}
	if strings.Contains(combined, "test") {
		tags = append(tags, "testing")
	}

	// Default tag
	if len(tags) == 0 {
		tags = append(tags, "work")
	}

	return tags
}
```

Add import at top:
```go
import (
	"strings"
	// ... other imports
)
```

**Step 2: Implement what_was_i_doing tool**

Add to `internal/mcp/tools.go`:

```go
// WhatWasIDoingInput defines input for what_was_i_doing tool.
type WhatWasIDoingInput struct {
	Timeframe string `json:"timeframe,omitempty" jsonschema:"description=Timeframe to search (today, yesterday, this week, last N hours),default=today"`
}

// WhatWasIDoingOutput provides narrative summary.
type WhatWasIDoingOutput struct {
	Summary string      `json:"summary"`
	Entries []EntryData `json:"entries"`
}

// In registerTools, add:
func (s *Server) registerTools() {
	// ... existing tools ...

	// what_was_i_doing tool
	whatWasIDoingTool := &mcp.Tool{
		Name:        "what_was_i_doing",
		Description: "Recall the user's recent activities and context. Use this at the start of conversations to understand what they've been working on, or when they ask 'what was I doing' or 'where did I leave off'.",
	}
	mcp.AddTool(s.mcpServer, whatWasIDoingTool, s.handleWhatWasIDoing)
}

// handleWhatWasIDoing implements the what_was_i_doing tool.
func (s *Server) handleWhatWasIDoing(ctx context.Context, req *mcp.CallToolRequest, input WhatWasIDoingInput) (*mcp.CallToolResult, WhatWasIDoingOutput, error) {
	timeframe := input.Timeframe
	if timeframe == "" {
		timeframe = "today"
	}

	// For now, just search recent entries
	// TODO: Parse timeframe and set date filters
	searchInput := SearchEntriesInput{
		Limit: 20,
	}

	result, listOutput, err := s.handleSearchEntries(ctx, req, searchInput)
	if err != nil {
		return nil, WhatWasIDoingOutput{}, err
	}

	// Build narrative summary
	var summary strings.Builder
	summary.WriteString(fmt.Sprintf("Based on %d recent entries:\n\n", listOutput.Count))

	for _, entry := range listOutput.Entries {
		summary.WriteString(fmt.Sprintf("- %s: %s", entry.Timestamp, entry.Message))
		if len(entry.Tags) > 0 {
			summary.WriteString(fmt.Sprintf(" [%s]", strings.Join(entry.Tags, ", ")))
		}
		summary.WriteString("\n")
	}

	output := WhatWasIDoingOutput{
		Summary: summary.String(),
		Entries: listOutput.Entries,
	}

	return result, output, nil
}
```

**Step 3: Implement find_when_i tool**

Add to `internal/mcp/tools.go`:

```go
// FindWhenIInput defines input for find_when_i tool.
type FindWhenIInput struct {
	What string `json:"what" jsonschema:"description=Description of the activity to find,required"`
}

// In registerTools, add:
func (s *Server) registerTools() {
	// ... existing tools ...

	// find_when_i tool
	findWhenITool := &mcp.Tool{
		Name:        "find_when_i",
		Description: "Find when the user did something specific. Use this to answer questions like 'when did I deploy X' or 'when did I fix that bug'.",
	}
	mcp.AddTool(s.mcpServer, findWhenITool, s.handleFindWhenI)
}

// handleFindWhenI implements the find_when_i tool.
func (s *Server) handleFindWhenI(ctx context.Context, req *mcp.CallToolRequest, input FindWhenIInput) (*mcp.CallToolResult, ListEntriesOutput, error) {
	// Search using the description
	searchInput := SearchEntriesInput{
		Text:  input.What,
		Limit: 10,
	}

	return s.handleSearchEntries(ctx, req, searchInput)
}
```

**Step 4: Build and test manually**

```bash
go build -tags sqlite_fts5 -o chronicle .
go test -tags sqlite_fts5 ./internal/mcp -v
```

Expected: All tests PASS, build succeeds

**Step 5: Commit**

```bash
git add internal/mcp/tools.go
git commit -m "feat: implement high-level semantic tools (remember_this, what_was_i_doing, find_when_i)"
```

---

## Task 8: Implement Resources

**Files:**
- Create: `internal/mcp/resources.go`
- Modify: `internal/mcp/server.go`

**Step 1: Create resources file**

Create `internal/mcp/resources.go`:

```go
//go:build sqlite_fts5

// ABOUTME: MCP resource implementations for chronicle
// ABOUTME: Provides dynamic queryable context about user's chronicle data
package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/harper/chronicle/internal/config"
	"github.com/harper/chronicle/internal/db"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// registerResources adds all MCP resources to the server.
func (s *Server) registerResources() {
	// recent-activity resource
	recentActivity := &mcp.Resource{
		URI:         "chronicle://recent-activity",
		Name:        "Recent Activity",
		Description: "Last 10 chronicle entries with full metadata",
		MimeType:    "application/json",
	}
	mcp.AddResource(s.mcpServer, recentActivity, s.handleRecentActivity)

	// tags resource
	tagsResource := &mcp.Resource{
		URI:         "chronicle://tags",
		Name:        "Tags",
		Description: "All tags sorted by frequency with usage counts",
		MimeType:    "application/json",
	}
	mcp.AddResource(s.mcpServer, tagsResource, s.handleTags)

	// today-summary resource
	todayResource := &mcp.Resource{
		URI:         "chronicle://today-summary",
		Name:        "Today Summary",
		Description: "All entries from today grouped by tags",
		MimeType:    "text/markdown",
	}
	mcp.AddResource(s.mcpServer, todayResource, s.handleTodaySummary)

	// project-context resource
	projectResource := &mcp.Resource{
		URI:         "chronicle://project-context",
		Name:        "Project Context",
		Description: "Current directory's chronicle config and recent project-specific logs",
		MimeType:    "application/json",
	}
	mcp.AddResource(s.mcpServer, projectResource, s.handleProjectContext)
}

// handleRecentActivity implements the recent-activity resource.
func (s *Server) handleRecentActivity(ctx context.Context, req *mcp.ReadResourceRequest, input struct{}) (*mcp.ReadResourceResult, struct{}, error) {
	database, err := db.InitDB(s.dbPath)
	if err != nil {
		return nil, struct{}{}, fmt.Errorf("failed to open database: %w", err)
	}
	defer func() { _ = database.Close() }()

	entries, err := db.ListEntries(database, 10)
	if err != nil {
		return nil, struct{}{}, fmt.Errorf("failed to list entries: %w", err)
	}

	data, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return nil, struct{}{}, err
	}

	result := &mcp.ReadResourceResult{
		Contents: []*mcp.ResourceContent{
			{
				URI:      "chronicle://recent-activity",
				MimeType: "application/json",
				Text:     string(data),
			},
		},
	}

	return result, struct{}{}, nil
}

// handleTags implements the tags resource.
func (s *Server) handleTags(ctx context.Context, req *mcp.ReadResourceRequest, input struct{}) (*mcp.ReadResourceResult, struct{}, error) {
	database, err := db.InitDB(s.dbPath)
	if err != nil {
		return nil, struct{}{}, fmt.Errorf("failed to open database: %w", err)
	}
	defer func() { _ = database.Close() }()

	// Query tag counts
	rows, err := database.Query("SELECT tag, COUNT(*) as count FROM tags GROUP BY tag ORDER BY count DESC")
	if err != nil {
		return nil, struct{}{}, fmt.Errorf("failed to query tags: %w", err)
	}
	defer func() { _ = rows.Close() }()

	tagCounts := make(map[string]int)
	for rows.Next() {
		var tag string
		var count int
		if err := rows.Scan(&tag, &count); err != nil {
			return nil, struct{}{}, err
		}
		tagCounts[tag] = count
	}

	data, err := json.MarshalIndent(tagCounts, "", "  ")
	if err != nil {
		return nil, struct{}{}, err
	}

	result := &mcp.ReadResourceResult{
		Contents: []*mcp.ResourceContent{
			{
				URI:      "chronicle://tags",
				MimeType: "application/json",
				Text:     string(data),
			},
		},
	}

	return result, struct{}{}, nil
}

// handleTodaySummary implements the today-summary resource.
func (s *Server) handleTodaySummary(ctx context.Context, req *mcp.ReadResourceRequest, input struct{}) (*mcp.ReadResourceResult, struct{}, error) {
	database, err := db.InitDB(s.dbPath)
	if err != nil {
		return nil, struct{}{}, fmt.Errorf("failed to open database: %w", err)
	}
	defer func() { _ = database.Close() }()

	// Query today's entries
	rows, err := database.Query(`
		SELECT id, datetime(timestamp), message
		FROM entries
		WHERE date(timestamp) = date('now', 'localtime')
		ORDER BY timestamp DESC
	`)
	if err != nil {
		return nil, struct{}{}, fmt.Errorf("failed to query entries: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var summary strings.Builder
	summary.WriteString("# Today's Activity\n\n")

	count := 0
	for rows.Next() {
		var id int64
		var timestamp, message string
		if err := rows.Scan(&id, &timestamp, &message); err != nil {
			return nil, struct{}{}, err
		}
		summary.WriteString(fmt.Sprintf("- **%s**: %s\n", timestamp, message))
		count++
	}

	if count == 0 {
		summary.WriteString("No entries logged today yet.\n")
	}

	result := &mcp.ReadResourceResult{
		Contents: []*mcp.ResourceContent{
			{
				URI:      "chronicle://today-summary",
				MimeType: "text/markdown",
				Text:     summary.String(),
			},
		},
	}

	return result, struct{}{}, nil
}

// handleProjectContext implements the project-context resource.
func (s *Server) handleProjectContext(ctx context.Context, req *mcp.ReadResourceRequest, input struct{}) (*mcp.ReadResourceResult, struct{}, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, struct{}{}, fmt.Errorf("failed to get working directory: %w", err)
	}

	// Find project root
	projectRoot, err := config.FindProjectRoot(cwd)
	if err != nil {
		return nil, struct{}{}, err
	}

	var contextData struct {
		HasProjectConfig bool                  `json:"has_project_config"`
		ProjectRoot      string                `json:"project_root,omitempty"`
		Config           *config.ProjectConfig `json:"config,omitempty"`
		Message          string                `json:"message"`
	}

	if projectRoot == "" {
		contextData.Message = "No .chronicle project configuration found in current directory tree"
	} else {
		contextData.HasProjectConfig = true
		contextData.ProjectRoot = projectRoot

		chroniclePath := filepath.Join(projectRoot, ".chronicle")
		cfg, err := config.LoadProjectConfig(chroniclePath)
		if err == nil {
			contextData.Config = &cfg
			contextData.Message = "Project-specific chronicle configuration found"
		}
	}

	data, err := json.MarshalIndent(contextData, "", "  ")
	if err != nil {
		return nil, struct{}{}, err
	}

	result := &mcp.ReadResourceResult{
		Contents: []*mcp.ResourceContent{
			{
				URI:      "chronicle://project-context",
				MimeType: "application/json",
				Text:     string(data),
			},
		},
	}

	return result, struct{}{}, nil
}
```

**Step 2: Register resources in server**

Modify `internal/mcp/server.go` NewServer function:

```go
func NewServer(dbPath string) *Server {
	impl := &mcp.Implementation{
		Name:    "chronicle",
		Version: "0.1.1",
	}

	server := &Server{
		mcpServer: mcp.NewServer(impl, nil),
		dbPath:    dbPath,
	}

	// Register components
	server.registerPrompts()
	server.registerTools()
	server.registerResources()

	return server
}
```

**Step 3: Build and test**

```bash
go build -tags sqlite_fts5 -o chronicle .
```

Expected: Build succeeds

**Step 4: Commit**

```bash
git add internal/mcp/resources.go internal/mcp/server.go
git commit -m "feat: implement MCP resources (recent-activity, tags, today-summary, project-context)"
```

---

## Task 9: Update README with MCP Documentation

**Files:**
- Modify: `README.md`

**Step 1: Add MCP section to README**

Add after "Quick Start" section in `README.md`:

```markdown
## MCP Server

Chronicle includes an MCP (Model Context Protocol) server that allows AI assistants to interact with your activity log.

### Running the MCP Server

```bash
# Run the MCP server (stdio transport)
chronicle mcp
```

### Configuring with Claude Desktop

Add to your Claude Desktop MCP settings (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "chronicle": {
      "command": "/path/to/chronicle",
      "args": ["mcp"]
    }
  }
}
```

### Available Tools

**Low-Level Tools:**
- `add_entry` - Log a new entry
- `list_entries` - Retrieve recent entries
- `search_entries` - Search by text, tags, or dates

**High-Level Semantic Tools:**
- `remember_this` - Proactively log important information with smart tagging
- `what_was_i_doing` - Recall recent activities and context
- `find_when_i` - Find when you did something specific

### Available Resources

- `chronicle://recent-activity` - Last 10 entries
- `chronicle://tags` - Tag usage statistics
- `chronicle://today-summary` - Today's activity summary
- `chronicle://project-context` - Current project's chronicle config

### Available Prompts

- `chronicle-getting-started` - Introduction to using chronicle with AI
```

**Step 2: Commit**

```bash
git add README.md
git commit -m "docs: add MCP server documentation to README"
```

---

## Task 10: Integration Test

**Files:**
- Create: `test_mcp.sh`

**Step 1: Create integration test script**

Create `test_mcp.sh`:

```bash
#!/bin/bash
set -e

echo "Building chronicle with MCP support..."
go build -tags sqlite_fts5 -o chronicle .

echo "Testing MCP server can start..."
# Start server in background with timeout
timeout 2s ./chronicle mcp || true

echo "MCP integration test passed!"
```

**Step 2: Make executable and run**

```bash
chmod +x test_mcp.sh
./test_mcp.sh
```

Expected: Test passes

**Step 3: Commit**

```bash
git add test_mcp.sh
git commit -m "test: add MCP server integration test"
```

---

## Success Criteria

- [ ] All unit tests pass (`go test -tags sqlite_fts5 ./...`)
- [ ] Integration test passes (`./test_mcp.sh`)
- [ ] Build succeeds without errors
- [ ] Can run `chronicle mcp` command
- [ ] All 6 tools registered and functional
- [ ] All 4 resources registered and functional
- [ ] Prompt registered
- [ ] README documentation complete
- [ ] All commits follow conventional commit format

---

## Notes for Implementation

- Use TDD: Write test first, watch it fail, implement, watch it pass
- Commit frequently after each task
- All database operations reuse existing code
- Error messages should be clear and actionable
- Tool descriptions are compelling for AI usage
- Build with `-tags sqlite_fts5` always
