// ABOUTME: Chronicle CLI - Entry point for timestamped logging tool
// ABOUTME: Initializes CLI and routes commands
package main

import (
	"fmt"
	"os"

	"github.com/harper/chronicle/internal/cli"
)

func main() {
	if err := cli.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
