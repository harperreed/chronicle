#!/usr/bin/env bash
# ABOUTME: MCP stdio integration smoke test for Chronicle
# ABOUTME: Isolates state and verifies healthy startup plus real startup failure detection

set -euo pipefail

echo "Building chronicle with MCP support..."
go build -o chronicle .

TEST_DIR=$(mktemp -d)
export HOME="$TEST_DIR/home"
export XDG_CONFIG_HOME="$TEST_DIR/config"
export XDG_DATA_HOME="$TEST_DIR/data"
mkdir -p "$HOME" "$XDG_CONFIG_HOME" "$XDG_DATA_HOME"

MCP_INPUT=""
MCP_STDERR=""
MCP_PID=""
MCP_STATUS=0
MCP_FD_OPEN=0

close_mcp_input() {
	if [ "$MCP_FD_OPEN" -eq 1 ]; then
		exec 3>&-
		MCP_FD_OPEN=0
	fi
}

start_mcp() {
	local probe=$1
	MCP_INPUT="$TEST_DIR/$probe.stdin"
	MCP_STDERR="$TEST_DIR/$probe.stderr"
	mkfifo "$MCP_INPUT"
	exec 3<>"$MCP_INPUT"
	MCP_FD_OPEN=1
	./chronicle mcp <&3 >/dev/null 2>"$MCP_STDERR" &
	MCP_PID=$!
}

wait_for_mcp_exit() {
	MCP_STATUS=0
	wait "$MCP_PID" 2>/dev/null || MCP_STATUS=$?
	MCP_PID=""
	close_mcp_input
}

stop_mcp() {
	if [ -n "$MCP_PID" ]; then
		if kill -0 "$MCP_PID" 2>/dev/null; then
			kill "$MCP_PID" 2>/dev/null || true
		fi
		wait "$MCP_PID" 2>/dev/null || true
		MCP_PID=""
	fi
	close_mcp_input
}

cleanup() {
	stop_mcp
	rm -rf "$TEST_DIR"
}
trap cleanup EXIT

echo "Testing MCP server can start..."
# Keep stdin open without sending protocol input so a healthy stdio server waits.
start_mcp healthy

# Give it a moment to potentially crash
sleep 0.5

# A process that exited before this check failed to start, even if its parent shell survived.
if ! kill -0 "$MCP_PID" 2>/dev/null; then
	wait_for_mcp_exit
	echo "MCP server exited before startup check (status $MCP_STATUS)." >&2
	if [ -s "$MCP_STDERR" ]; then
		cat "$MCP_STDERR" >&2
	fi
	exit 1
fi

stop_mcp

echo "Testing MCP startup failure detection..."
CONFIG_PATH="$XDG_CONFIG_HOME/chronicle/config.json"
rm -f "$CONFIG_PATH"
mkdir -p "$CONFIG_PATH"
start_mcp invalid-config

# Invalid config should fail promptly; bound the probe so it cannot hang.
for _ in {1..20}; do
	if ! kill -0 "$MCP_PID" 2>/dev/null; then
		break
	fi
	sleep 0.05
done

if kill -0 "$MCP_PID" 2>/dev/null; then
	stop_mcp
	echo "MCP server remained running with an invalid config path." >&2
	exit 1
fi

wait_for_mcp_exit
if [ "$MCP_STATUS" -eq 0 ]; then
	echo "MCP server exited successfully despite an invalid config path." >&2
	exit 1
fi
if ! grep -Fq "config.json: is a directory" "$MCP_STDERR"; then
	echo "MCP server failed without the expected invalid-config error." >&2
	cat "$MCP_STDERR" >&2
	exit 1
fi

echo "MCP integration test passed!"
