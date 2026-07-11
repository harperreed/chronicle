// ABOUTME: Tests for MCP prompt handlers
// ABOUTME: Validates prompt registration and content

package mcp

import (
	"context"
	"strings"
	"testing"

	gomcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestRegisterPrompts(t *testing.T) {
	impl := &gomcp.Implementation{
		Name:    "chronicle-test",
		Version: "0.0.0",
	}
	mcpServer := gomcp.NewServer(impl, nil)
	server := &Server{mcpServer: mcpServer}
	server.registerPrompts()
	clientSession := connectTestClient(t, mcpServer)

	listResult, err := clientSession.ListPrompts(context.Background(), nil)
	if err != nil {
		t.Fatalf("failed to list prompts: %v", err)
	}
	if len(listResult.Prompts) != 1 {
		t.Fatalf("expected 1 prompt, got %d", len(listResult.Prompts))
	}
	prompt := listResult.Prompts[0]
	if prompt.Name != "chronicle-getting-started" {
		t.Errorf("unexpected prompt name: %q", prompt.Name)
	}
	if prompt.Description != "Introduction to chronicle and how AI assistants should use it" {
		t.Errorf("unexpected prompt description: %q", prompt.Description)
	}

	result, err := clientSession.GetPrompt(context.Background(), &gomcp.GetPromptParams{Name: prompt.Name})
	if err != nil {
		t.Fatalf("failed to get registered prompt: %v", err)
	}
	if result.Description != "Getting started with chronicle" {
		t.Errorf("unexpected result description: %q", result.Description)
	}
	if len(result.Messages) != 1 || result.Messages[0].Role != "user" {
		t.Fatalf("unexpected prompt messages: %#v", result.Messages)
	}
	textContent, ok := result.Messages[0].Content.(*gomcp.TextContent)
	if !ok {
		t.Fatalf("expected TextContent, got %T", result.Messages[0].Content)
	}

	for _, phrase := range []string{
		"Chronicle is a personal activity logging system",
		"When to use chronicle",
		"what did I do recently?",
		"Best practices",
		"configured storage",
		"Log activities as they happen",
		"Use specific tags",
		"Include enough detail",
	} {
		if !strings.Contains(textContent.Text, phrase) {
			t.Errorf("expected prompt content to contain %q", phrase)
		}
	}
	for _, unsupportedClaim := range []string{"what did I do yesterday?", "Charm Cloud", "syncs automatically", "The user has configured"} {
		if strings.Contains(textContent.Text, unsupportedClaim) {
			t.Errorf("prompt contains unsupported claim %q", unsupportedClaim)
		}
	}
}
