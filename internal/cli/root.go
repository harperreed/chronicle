// ABOUTME: Root command definition and CLI setup
// ABOUTME: Handles global flags and command initialization
package cli

import (
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "chronicle",
	Short: "Timestamped logging tool",
	Long:  `Chronicle logs timestamped messages with metadata to SQLite and optional project log files.`,
}

func Execute() error {
	// If first arg is not a known subcommand, inject "add"
	if len(os.Args) > 1 {
		arg := os.Args[1]
		// Check if it's not a flag and not a known command
		if len(arg) > 0 && arg[0] != '-' {
			isCommand := false
			for _, cmd := range rootCmd.Commands() {
				if cmd.Name() == arg || cmd.HasAlias(arg) {
					isCommand = true
					break
				}
			}
			// If not a command, inject "add"
			if !isCommand {
				os.Args = append([]string{os.Args[0], "add"}, os.Args[1:]...)
			}
		}
	}
	return rootCmd.Execute()
}

func init() {
	// Global flags can go here
}
