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
	Message string   `json:"message" jsonschema:"The message to log" jsonschema_extras:"required=true"`
	Tags    []string `json:"tags,omitempty" jsonschema:"Optional tags to categorize the entry"`
}

// AddEntryOutput defines the output for add_entry tool.
type AddEntryOutput struct {
	EntryID   int64  `json:"entry_id" jsonschema:"The ID of the created entry"`
	Message   string `json:"message" jsonschema:"The logged message"`
	Timestamp string `json:"timestamp" jsonschema:"When the entry was created"`
}

// ListEntriesInput defines the input for list_entries tool.
type ListEntriesInput struct {
	Limit int `json:"limit,omitempty" jsonschema:"Maximum number of entries to return (default 10)"`
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

// registerTools adds all MCP tools to the server.
func (s *Server) registerTools() {
	// add_entry tool
	addEntryTool := &mcp.Tool{
		Name:        "add_entry",
		Description: "Log a timestamped entry to chronicle. Use this when the user explicitly asks to log, track, or record something they did or are doing.",
	}
	mcp.AddTool(s.mcpServer, addEntryTool, s.handleAddEntry)

	// list_entries tool
	listEntriesTool := &mcp.Tool{
		Name:        "list_entries",
		Description: "Retrieve recent chronicle entries. Use this to answer questions like 'what did I do today/recently' or 'show my recent work'.",
	}
	mcp.AddTool(s.mcpServer, listEntriesTool, s.handleListEntries)
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
		Content: []mcp.Content{
			&mcp.TextContent{
				Text: fmt.Sprintf("Entry created successfully (ID: %d) at %s", id, timestamp),
			},
		},
	}

	return result, output, nil
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
		Content: []mcp.Content{
			&mcp.TextContent{
				Text: fmt.Sprintf("Retrieved %d recent entries", len(outputEntries)),
			},
		},
	}

	return result, output, nil
}
