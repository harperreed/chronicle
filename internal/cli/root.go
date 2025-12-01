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
	Long: `
 ██████╗██╗  ██╗██████╗  ██████╗ ███╗   ██╗██╗ ██████╗██╗     ███████╗
██╔════╝██║  ██║██╔══██╗██╔═══██╗████╗  ██║██║██╔════╝██║     ██╔════╝
██║     ███████║██████╔╝██║   ██║██╔██╗ ██║██║██║     ██║     █████╗
██║     ██╔══██║██╔══██╗██║   ██║██║╚██╗██║██║██║     ██║     ██╔══╝
╚██████╗██║  ██║██║  ██║╚██████╔╝██║ ╚████║██║╚██████╗███████╗███████╗
 ╚═════╝╚═╝  ╚═╝╚═╝  ╚═╝ ╚═════╝ ╚═╝  ╚═══╝╚═╝ ╚═════╝╚══════╝╚══════╝

     📝 Timestamped logging for your development journey

Chronicle logs timestamped messages with metadata to SQLite and optional project log files.`,
}

func Execute() error {
	// If first arg is not a known subcommand, inject "add"
	if shouldInjectAddCommand() {
		os.Args = append([]string{os.Args[0], "add"}, os.Args[1:]...)
	}
	return rootCmd.Execute()
}

func shouldInjectAddCommand() bool {
	if len(os.Args) <= 1 {
		return false
	}

	arg := os.Args[1]
	// Check if it's a flag
	if len(arg) == 0 || arg[0] == '-' {
		return false
	}

	// Check if it's a known command
	return !isKnownCommand(arg)
}

func isKnownCommand(arg string) bool {
	for _, cmd := range rootCmd.Commands() {
		if cmd.Name() == arg || cmd.HasAlias(arg) {
			return true
		}
	}
	return false
}

func init() {
	// Global flags can go here
}
