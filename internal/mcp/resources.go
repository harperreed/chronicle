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
	"time"

	"github.com/harper/chronicle/internal/charm"
	"github.com/harper/chronicle/internal/config"
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
	entries, err := s.client.ListEntries(10)
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
	// Get all entries and count tags
	entries, err := s.client.ListEntries(0) // 0 = no limit
	if err != nil {
		return nil, fmt.Errorf("failed to list entries: %w", err)
	}

	// Count tag occurrences
	tagCounts := make(map[string]int)
	for _, entry := range entries {
		for _, tag := range entry.Tags {
			tagCounts[tag]++
		}
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
	// Get entries from today
	now := time.Now()
	startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	filter := &charm.SearchFilter{
		Since: &startOfDay,
	}

	entries, err := s.client.SearchEntries(filter, 0) // 0 = no limit
	if err != nil {
		return nil, fmt.Errorf("failed to search entries: %w", err)
	}

	var summary strings.Builder
	summary.WriteString("# Today's Activity\n\n")

	if len(entries) == 0 {
		summary.WriteString("No entries logged today yet.\n")
	} else {
		for _, entry := range entries {
			summary.WriteString(fmt.Sprintf("- **%s**: %s\n",
				entry.Timestamp.Format("15:04:05"),
				entry.Message))
		}
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
