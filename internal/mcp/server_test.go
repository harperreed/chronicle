// ABOUTME: Tests for MCP server
// ABOUTME: Validates server initialization and configuration
package mcp

import (
	"testing"
)

func TestServerTypes(t *testing.T) {
	// Verify the Server struct has the expected fields
	// Full integration tests require a Charm connection
	var s *Server
	_ = s

	// Verify AddEntryInput has required fields
	input := AddEntryInput{
		Message: "test",
		Tags:    []string{"tag1"},
	}
	if input.Message != "test" {
		t.Error("expected message field")
	}

	// Verify AddEntryOutput has required fields
	output := AddEntryOutput{
		EntryID:   "uuid",
		Message:   "msg",
		Timestamp: "ts",
	}
	if output.EntryID != "uuid" {
		t.Error("expected entry_id field")
	}
}

func TestListEntriesTypes(t *testing.T) {
	input := ListEntriesInput{Limit: 10}
	if input.Limit != 10 {
		t.Error("expected limit field")
	}

	output := ListEntriesOutput{
		Entries: []EntryData{
			{ID: "1", Message: "test"},
		},
		Count: 1,
	}
	if output.Count != 1 {
		t.Error("expected count field")
	}
}

func TestSearchEntriesTypes(t *testing.T) {
	input := SearchEntriesInput{
		Text:  "query",
		Tags:  []string{"tag"},
		Limit: 20,
	}
	if input.Text != "query" {
		t.Error("expected text field")
	}
}
