// ABOUTME: Tests for MCP tools
// ABOUTME: Validates tool type definitions and helper functions
package mcp

import (
	"context"
	"strings"
	"testing"

	gomcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestListEntriesDescriptionPromisesOnlyRecentActivity(t *testing.T) {
	impl := &gomcp.Implementation{
		Name:    "chronicle-test",
		Version: "0.0.0",
	}
	mcpServer := gomcp.NewServer(impl, nil)
	server := &Server{mcpServer: mcpServer}
	server.registerTools()
	clientSession := connectTestClient(t, mcpServer)

	result, err := clientSession.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatalf("failed to list tools: %v", err)
	}

	for _, tool := range result.Tools {
		if tool.Name != "list_entries" {
			continue
		}
		if !strings.Contains(tool.Description, "what did I do recently") {
			t.Errorf("list_entries description does not describe recent activity: %q", tool.Description)
		}
		if strings.Contains(tool.Description, "today") {
			t.Errorf("list_entries description promises unsupported date filtering: %q", tool.Description)
		}
		return
	}

	t.Fatal("list_entries tool was not registered")
}

func TestSuggestTags(t *testing.T) {
	tests := []struct {
		activity string
		context  string
		expected []string
	}{
		{"deployed the app", "", []string{"deployment"}},
		{"fixed a bug", "", []string{"bug-fix"}},
		{"decided to use Go", "", []string{"decision"}},
		{"learned about channels", "", []string{"learning"}},
		{"wrote some tests", "", []string{"testing"}},
		{"random work", "", []string{"work"}}, // default
		{"deployed and fixed bug", "", []string{"deployment", "bug-fix"}},
	}

	for _, tt := range tests {
		t.Run(tt.activity, func(t *testing.T) {
			result := suggestTags(tt.activity, tt.context)

			// Check that all expected tags are present
			for _, exp := range tt.expected {
				found := false
				for _, got := range result {
					if got == exp {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("expected tag %q not found in %v", exp, result)
				}
			}
		})
	}
}

func TestRememberThisInput(t *testing.T) {
	input := RememberThisInput{
		Activity: "Deployed v2.0",
		Context:  "Major milestone",
	}

	if input.Activity != "Deployed v2.0" {
		t.Error("expected activity field")
	}
	if input.Context != "Major milestone" {
		t.Error("expected context field")
	}
}

func TestWhatWasIDoingInput(t *testing.T) {
	input := WhatWasIDoingInput{
		Timeframe: "today",
	}
	if input.Timeframe != "today" {
		t.Error("expected timeframe field")
	}
}

func TestFindWhenIInput(t *testing.T) {
	input := FindWhenIInput{
		What: "deployed the app",
	}
	if input.What != "deployed the app" {
		t.Error("expected what field")
	}
}

func TestEntryData(t *testing.T) {
	entry := EntryData{
		ID:        "123",
		Timestamp: "2025-01-01 12:00:00",
		Message:   "test",
		Tags:      []string{"work"},
		Hostname:  "host",
		Username:  "user",
		Directory: "/home/user",
	}

	if entry.ID != "123" {
		t.Error("expected id field")
	}
	if entry.Message != "test" {
		t.Error("expected message field")
	}
	if len(entry.Tags) != 1 || entry.Tags[0] != "work" {
		t.Error("expected tags field")
	}
}
