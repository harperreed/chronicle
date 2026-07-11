# ABOUTME: Defines Chronicle's local build, test, install, and release commands.
# ABOUTME: Provides the developer workflows mirrored by continuous integration.

.PHONY: help build test test-coverage install clean lint fmt run-mcp dev-db release dev integration-test ci

INSTALL_DIR = $(shell gobin="$$(go env GOBIN)"; if [ -n "$$gobin" ]; then printf '%s' "$$gobin"; else gopath="$$(go env GOPATH)"; printf '%s/bin' "$${gopath%%:*}"; fi)

# Default target
help:
	@echo "Chronicle - Timestamped logging tool"
	@echo ""
	@echo "Available targets:"
	@echo "  make build      - Build the chronicle binary"
	@echo "  make test       - Run all tests"
	@echo "  make install    - Install chronicle to the configured Go binary directory"
	@echo "  make clean      - Remove built binaries"
	@echo "  make lint       - Run linter"
	@echo "  make fmt        - Format code"
	@echo "  make run-mcp    - Run the MCP server"
	@echo "  make dev-db     - Show development database location"
	@echo "  make release    - Create a release build"

# Build the binary
build:
	@echo "Building chronicle..."
	go build -o chronicle .
	@echo "✓ Built successfully: ./chronicle"

# Run all tests
test:
	@echo "Running tests..."
	go test -v ./...

# Run tests with coverage
test-coverage:
	@echo "Running tests with coverage..."
	go test -coverprofile=coverage.out -covermode=atomic ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "✓ Coverage report: coverage.html"

# Install to the configured Go binary directory
install:
	@echo "Installing chronicle..."
	go install .
	@echo "✓ Installed to $(INSTALL_DIR)/chronicle"

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
	golangci-lint run --timeout=10m

# Format code
fmt:
	@echo "Formatting code..."
	go fmt ./...
	@echo "✓ Formatted"

# Run the MCP server
run-mcp: build
	@echo "Starting MCP server..."
	./chronicle mcp

# Show development data location
dev-db:
	@echo "Data location:"
	@echo "  Default: ~/.local/share/chronicle/"
	@echo "  Config:  ~/.config/chronicle/config.json"
	@echo "  Override data_dir in config.json or run: chronicle setup"

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
