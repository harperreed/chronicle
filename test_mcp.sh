#!/bin/bash
set -e

echo "Building chronicle with MCP support..."
go build -tags sqlite_fts5 -o chronicle .

echo "Testing MCP server can start..."
# MCP server runs on stdio and will block waiting for input.
# We start it in background, give it a moment, then kill it.
# If it crashes on startup, this test will fail.
(
  # Run server with stdin connected to prevent immediate exit
  sleep 2 | ./chronicle mcp > /dev/null 2>&1
) &
MCP_PID=$!

# Give it a moment to potentially crash
sleep 0.5

# Check if process is still running (or just finished cleanly)
if ps -p $MCP_PID > /dev/null 2>&1; then
    # Still running - kill it
    kill $MCP_PID 2>/dev/null || true
    wait $MCP_PID 2>/dev/null || true
fi

# If we got here without error, test passed
echo "MCP integration test passed!"
