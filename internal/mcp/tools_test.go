// ABOUTME: Tests for MCP tools
// ABOUTME: Validates tool type definitions and helper functions
package mcp

import (
	"testing"
)

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
