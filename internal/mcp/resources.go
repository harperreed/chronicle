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
		MIMEType:    "application/json",
	}
	s.mcpServer.AddResource(recentActivity, s.handleRecentActivity)

	// tags resource
	tagsResource := &mcp.Resource{
		URI:         "chronicle://tags",
		Name:        "Tags",
		Description: "All tags sorted by frequency with usage counts",
		MIMEType:    "application/json",
	}
	s.mcpServer.AddResource(tagsResource, s.handleTags)

	// today-summary resource
	todayResource := &mcp.Resource{
		URI:         "chronicle://today-summary",
		Name:        "Today Summary",
		Description: "All entries from today grouped by tags",
		MIMEType:    "text/markdown",
	}
	s.mcpServer.AddResource(todayResource, s.handleTodaySummary)

	// project-context resource
	projectResource := &mcp.Resource{
		URI:         "chronicle://project-context",
		Name:        "Project Context",
		Description: "Current directory's chronicle config and recent project-specific logs",
		MIMEType:    "application/json",
	}
	s.mcpServer.AddResource(projectResource, s.handleProjectContext)
}

// handleRecentActivity implements the recent-activity resource.
func (s *Server) handleRecentActivity(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
	database, err := db.InitDB(s.dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}
	defer func() { _ = database.Close() }()

	entries, err := db.ListEntries(database, 10)
	if err != nil {
		return nil, fmt.Errorf("failed to list entries: %w", err)
	}

	data, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return nil, err
	}

	result := &mcp.ReadResourceResult{
		Contents: []*mcp.ResourceContents{
			{
				URI:      "chronicle://recent-activity",
				MIMEType: "application/json",
				Text:     string(data),
			},
		},
	}

	return result, nil
}

// handleTags implements the tags resource.
func (s *Server) handleTags(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
	database, err := db.InitDB(s.dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}
	defer func() { _ = database.Close() }()

	// Query tag counts
	rows, err := database.Query("SELECT tag, COUNT(*) as count FROM tags GROUP BY tag ORDER BY count DESC")
	if err != nil {
		return nil, fmt.Errorf("failed to query tags: %w", err)
	}
	defer func() { _ = rows.Close() }()

	tagCounts := make(map[string]int)
	for rows.Next() {
		var tag string
		var count int
		if err := rows.Scan(&tag, &count); err != nil {
			return nil, err
		}
		tagCounts[tag] = count
	}

	data, err := json.MarshalIndent(tagCounts, "", "  ")
	if err != nil {
		return nil, err
	}

	result := &mcp.ReadResourceResult{
		Contents: []*mcp.ResourceContents{
			{
				URI:      "chronicle://tags",
				MIMEType: "application/json",
				Text:     string(data),
			},
		},
	}

	return result, nil
}

// handleTodaySummary implements the today-summary resource.
func (s *Server) handleTodaySummary(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
	database, err := db.InitDB(s.dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
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
		return nil, fmt.Errorf("failed to query entries: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var summary strings.Builder
	summary.WriteString("# Today's Activity\n\n")

	count := 0
	for rows.Next() {
		var id int64
		var timestamp, message string
		if err := rows.Scan(&id, &timestamp, &message); err != nil {
			return nil, err
		}
		summary.WriteString(fmt.Sprintf("- **%s**: %s\n", timestamp, message))
		count++
	}

	if count == 0 {
		summary.WriteString("No entries logged today yet.\n")
	}

	result := &mcp.ReadResourceResult{
		Contents: []*mcp.ResourceContents{
			{
				URI:      "chronicle://today-summary",
				MIMEType: "text/markdown",
				Text:     summary.String(),
			},
		},
	}

	return result, nil
}

// handleProjectContext implements the project-context resource.
func (s *Server) handleProjectContext(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("failed to get working directory: %w", err)
	}

	// Find project root
	projectRoot, err := config.FindProjectRoot(cwd)
	if err != nil {
		return nil, err
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
			contextData.Config = cfg
			contextData.Message = "Project-specific chronicle configuration found"
		}
	}

	data, err := json.MarshalIndent(contextData, "", "  ")
	if err != nil {
		return nil, err
	}

	result := &mcp.ReadResourceResult{
		Contents: []*mcp.ResourceContents{
			{
				URI:      "chronicle://project-context",
				MIMEType: "application/json",
				Text:     string(data),
			},
		},
	}

	return result, nil
}
