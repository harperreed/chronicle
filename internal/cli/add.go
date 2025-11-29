// ABOUTME: Add command for creating new log entries
// ABOUTME: Handles message input and tag flags
package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

var (
	tags []string
)

var addCmd = &cobra.Command{
	Use:   "add [message]",
	Short: "Add a log entry",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		message := args[0]
		fmt.Printf("Adding: %s (tags: %v)\n", message, tags)
	},
}

func init() {
	addCmd.Flags().StringArrayVarP(&tags, "tag", "t", []string{}, "Add tags to entry")
	rootCmd.AddCommand(addCmd)
}
