# Chronicle Makefile

.PHONY: help build test clean install lint fmt run-mcp dev-db

# Default target
help:
	@echo "Chronicle - Timestamped logging tool"
	@echo ""
	@echo "Available targets:"
	@echo "  make build      - Build the chronicle binary"
	@echo "  make test       - Run all tests"
	@echo "  make install    - Install chronicle to GOPATH/bin"
	@echo "  make clean      - Remove built binaries"
	@echo "  make lint       - Run linter"
	@echo "  make fmt        - Format code"
	@echo "  make run-mcp    - Run the MCP server"
	@echo "  make dev-db     - Show development database location"
	@echo "  make release    - Create a release build"

# Build the binary with sqlite_fts5 support
build:
	@echo "Building chronicle..."
	go build -tags sqlite_fts5 -o chronicle .
	@echo "✓ Built successfully: ./chronicle"

# Run all tests
test:
	@echo "Running tests..."
	go test -tags sqlite_fts5 -v ./...

# Run tests with coverage
test-coverage:
	@echo "Running tests with coverage..."
	go test -tags sqlite_fts5 -coverprofile=coverage.out -covermode=atomic ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "✓ Coverage report: coverage.html"

# Install to GOPATH/bin
install:
	@echo "Installing chronicle..."
	go install -tags sqlite_fts5 .
	@echo "✓ Installed to $(shell go env GOPATH)/bin/chronicle"

# Clean built binaries
clean:
	@echo "Cleaning..."
	rm -f chronicle
	rm -f coverage.out coverage.html
	rm -rf dist/
	@echo "✓ Clean complete"

# Run linter
lint:
	@echo "Running linter..."
	golangci-lint run --timeout=10m --build-tags=sqlite_fts5

# Format code
fmt:
	@echo "Formatting code..."
	go fmt ./...
	@echo "✓ Formatted"

# Run the MCP server
run-mcp: build
	@echo "Starting MCP server..."
	./chronicle mcp

# Show development database location
dev-db:
	@echo "Database location:"
	@echo "  Default: ~/.local/share/chronicle/chronicle.db"
	@echo "  Override with: CHRONICLE_DB_PATH=/path/to/db"

# Create a release build with goreleaser
release:
	@echo "Creating release build..."
	goreleaser release --snapshot --clean
	@echo "✓ Release builds in dist/"

# Quick development workflow
dev: fmt lint test build
	@echo "✓ Development checks complete"

# Integration test
integration-test: build
	@echo "Running integration tests..."
	./test_mcp.sh
	@echo "✓ Integration tests passed"

# Run all checks (CI equivalent)
ci: fmt lint test integration-test
	@echo "✓ All CI checks passed"
