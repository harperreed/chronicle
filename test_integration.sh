#!/usr/bin/env bash
# ABOUTME: Integration test script for chronicle
# ABOUTME: Validates end-to-end workflows and command interactions

set -euo pipefail

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
NC='\033[0m'

echo "Running chronicle integration tests..."

# Build
echo "Building chronicle..."
go build -tags=sqlite_fts5 -o chronicle .

# Save the path to chronicle binary before changing directories
CHRONICLE_BIN="$(pwd)/chronicle"

# Setup temp directory
TEST_DIR=$(mktemp -d)
export HOME=$TEST_DIR
export XDG_DATA_HOME="$TEST_DIR/.local/share"
export XDG_CONFIG_HOME="$TEST_DIR/.config"

cleanup() {
  rm -rf "$TEST_DIR"
}
trap cleanup EXIT

# Test 1: Add entry
echo -n "Test 1: Add entry... "
"$CHRONICLE_BIN" add "test entry 1" --tag test
if [ $? -eq 0 ]; then
  echo -e "${GREEN}PASS${NC}"
else
  echo -e "${RED}FAIL${NC}"
  exit 1
fi

# Test 2: Add without explicit command
echo -n "Test 2: Add with shorthand... "
"$CHRONICLE_BIN" "test entry 2" --tag work
if [ $? -eq 0 ]; then
  echo -e "${GREEN}PASS${NC}"
else
  echo -e "${RED}FAIL${NC}"
  exit 1
fi

# Test 3: List entries
echo -n "Test 3: List entries... "
OUTPUT=$("$CHRONICLE_BIN" list)
if echo "$OUTPUT" | grep -q "test entry 1" && echo "$OUTPUT" | grep -q "test entry 2"; then
  echo -e "${GREEN}PASS${NC}"
else
  echo -e "${RED}FAIL${NC}"
  echo "Output: $OUTPUT"
  exit 1
fi

# Test 4: Search by text
echo -n "Test 4: Search by text... "
OUTPUT=$("$CHRONICLE_BIN" search "entry 1")
if echo "$OUTPUT" | grep -q "test entry 1"; then
  echo -e "${GREEN}PASS${NC}"
else
  echo -e "${RED}FAIL${NC}"
  exit 1
fi

# Test 5: Search by tag
echo -n "Test 5: Search by tag... "
OUTPUT=$("$CHRONICLE_BIN" search --tag work)
if echo "$OUTPUT" | grep -q "test entry 2"; then
  echo -e "${GREEN}PASS${NC}"
else
  echo -e "${RED}FAIL${NC}"
  exit 1
fi

# Test 6: JSON output
echo -n "Test 6: JSON output... "
OUTPUT=$("$CHRONICLE_BIN" list --json)
if echo "$OUTPUT" | grep -q '"Message"' && echo "$OUTPUT" | grep -q '"Tags"'; then
  echo -e "${GREEN}PASS${NC}"
else
  echo -e "${RED}FAIL${NC}"
  exit 1
fi

# Test 7: Project logging
echo -n "Test 7: Project logging... "
PROJECT_DIR="$TEST_DIR/test-project"
mkdir -p "$PROJECT_DIR/src"
cat > "$PROJECT_DIR/.chronicle" << EOF
local_logging = true
log_dir = "logs"
log_format = "markdown"
EOF

cd "$PROJECT_DIR/src"
"$CHRONICLE_BIN" "project entry" --tag project
if [ -f "$PROJECT_DIR/logs/$(date +%Y-%m-%d).log" ]; then
  echo -e "${GREEN}PASS${NC}"
else
  echo -e "${RED}FAIL${NC}"
  exit 1
fi

echo ""
echo -e "${GREEN}All integration tests passed!${NC}"
