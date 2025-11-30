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
		Content: []mcp.Content{
			&mcp.TextContent{
				Text: fmt.Sprintf("Entry created successfully (ID: %d) at %s", id, timestamp),
			},
		},
	}

	return result, output, nil
}
