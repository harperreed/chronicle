// ABOUTME: Root command definition and CLI setup
// ABOUTME: Handles global flags and command initialization
package cli

import (
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "chronicle",
	Short: "Timestamped logging tool",
	Long:  `Chronicle logs timestamped messages with metadata to SQLite and optional project log files.`,
}

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	// Global flags can go here
}
