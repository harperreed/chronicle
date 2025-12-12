//go:build sqlite_fts5

// ABOUTME: MCP tool implementations for chronicle
// ABOUTME: Provides low-level and high-level tools for logging and querying
package mcp

import (
	"context"
	"fmt"
	"os"
	"strings"

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

// SearchEntriesInput defines the input for search_entries tool.
type SearchEntriesInput struct {
	Text  string   `json:"text,omitempty" jsonschema:"Text to search for in entries"`
	Tags  []string `json:"tags,omitempty" jsonschema:"Filter by tags"`
	Since string   `json:"since,omitempty" jsonschema:"Start date/time (e.g. '2025-01-01' or 'yesterday')"`
	Until string   `json:"until,omitempty" jsonschema:"End date/time"`
	Limit int      `json:"limit,omitempty" jsonschema:"Maximum results (default 20)"`
}

// RememberThisInput defines input for remember_this tool.
type RememberThisInput struct {
	Activity string `json:"activity" jsonschema:"The activity or information to remember" jsonschema_extras:"required=true"`
	Context  string `json:"context,omitempty" jsonschema:"Why this matters or additional context"`
}

// WhatWasIDoingInput defines input for what_was_i_doing tool.
type WhatWasIDoingInput struct {
	Timeframe string `json:"timeframe,omitempty" jsonschema:"Timeframe to search (today, yesterday, this week, last N hours),default=today"`
}

// WhatWasIDoingOutput provides narrative summary.
type WhatWasIDoingOutput struct {
	Summary string      `json:"summary"`
	Entries []EntryData `json:"entries"`
}

// FindWhenIInput defines input for find_when_i tool.
type FindWhenIInput struct {
	What string `json:"what" jsonschema:"Description of the activity to find" jsonschema_extras:"required=true"`
}

// registerTools adds all MCP tools to the server.
func (s *Server) registerTools() {
	// add_entry tool
	addEntryTool := &mcp.Tool{
		Name:        "add_entry",
		Description: "Log a timestamped entry to chronicle. Use this proactively when you notice the user accomplished something significant (deployed code, fixed a bug, made a decision, solved a problem, completed a task) even if they don't explicitly ask you to log it. Also use when they explicitly request logging. Logging important moments helps them recall their work later.",
	}
	mcp.AddTool(s.mcpServer, addEntryTool, s.handleAddEntry)

	// list_entries tool
	listEntriesTool := &mcp.Tool{
		Name:        "list_entries",
		Description: "Retrieve recent chronicle entries. Use this to answer questions like 'what did I do today/recently' or 'show my recent work'.",
	}
	mcp.AddTool(s.mcpServer, listEntriesTool, s.handleListEntries)

	// search_entries tool
	searchEntriesTool := &mcp.Tool{
		Name:        "search_entries",
		Description: "Search chronicle history by text, tags, or date range. Use this when the user wants to find specific past activities or recall when something happened.",
	}
	mcp.AddTool(s.mcpServer, searchEntriesTool, s.handleSearchEntries)

	// remember_this tool
	rememberThisTool := &mcp.Tool{
		Name:        "remember_this",
		Description: "Proactively log important information the user shares about their work, decisions, or progress. Use this when you notice the user accomplished something worth tracking, even if they don't explicitly ask to log it. Automatically suggests relevant tags based on context.",
	}
	mcp.AddTool(s.mcpServer, rememberThisTool, s.handleRememberThis)

	// what_was_i_doing tool
	whatWasIDoingTool := &mcp.Tool{
		Name:        "what_was_i_doing",
		Description: "Recall the user's recent activities and context. Use this at the start of conversations to understand what they've been working on, or when they ask 'what was I doing' or 'where did I leave off'.",
	}
	mcp.AddTool(s.mcpServer, whatWasIDoingTool, s.handleWhatWasIDoing)

	// find_when_i tool
	findWhenITool := &mcp.Tool{
		Name:        "find_when_i",
		Description: "Find when the user did something specific. Use this to answer questions like 'when did I deploy X' or 'when did I fix that bug'.",
	}
	mcp.AddTool(s.mcpServer, findWhenITool, s.handleFindWhenI)
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
		Content: []mcp.Content{
			&mcp.TextContent{
				Text: fmt.Sprintf("Found %d matching entries", len(outputEntries)),
			},
		},
	}

	return result, output, nil
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

// handleWhatWasIDoing implements the what_was_i_doing tool.
func (s *Server) handleWhatWasIDoing(ctx context.Context, req *mcp.CallToolRequest, input WhatWasIDoingInput) (*mcp.CallToolResult, WhatWasIDoingOutput, error) {
	// TODO: Parse input.Timeframe and set date filters
	// For now, just search recent entries regardless of timeframe
	_ = input.Timeframe // silence unused warning until timeframe parsing is implemented

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

// handleFindWhenI implements the find_when_i tool.
func (s *Server) handleFindWhenI(ctx context.Context, req *mcp.CallToolRequest, input FindWhenIInput) (*mcp.CallToolResult, ListEntriesOutput, error) {
	// Search using the description
	searchInput := SearchEntriesInput{
		Text:  input.What,
		Limit: 10,
	}

	return s.handleSearchEntries(ctx, req, searchInput)
}
