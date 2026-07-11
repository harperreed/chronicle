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
- User asks about recent activities ("what did I do recently?")
- User wants to recall when something happened
- At start of work sessions to load context

Best practices:
- Log activities as they happen, not just when asked
- Use specific tags (work, personal, golang, debugging, deployment, etc.)
- Include enough detail to jog memory later
- Think of it as a work journal that can be searched

Chronicle uses the configured storage on this machine. Storage location, sharing, and synchronization depend on that configuration.
Do not assume a particular setup. Use the available tools and resources, and report configuration or storage errors plainly.`

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
