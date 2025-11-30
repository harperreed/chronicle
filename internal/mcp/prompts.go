//go:build sqlite_fts5

// ABOUTME: MCP prompt definitions for chronicle
// ABOUTME: Provides static context to AI assistants about chronicle capabilities
package mcp

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// registerPrompts adds static prompts to the MCP server.
func (s *Server) registerPrompts() {
	prompt := &mcp.Prompt{
		Name:        "chronicle-getting-started",
		Description: "Introduction to chronicle and how AI assistants should use it",
	}

	handler := func(ctx context.Context, req *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
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
						Text: content,
					},
				},
			},
		}

		return result, nil
	}

	s.mcpServer.AddPrompt(prompt, handler)
}
