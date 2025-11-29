// ABOUTME: Add command for creating new log entries
// ABOUTME: Handles message input and tag flags
package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/harper/chronicle/internal/config"
	"github.com/harper/chronicle/internal/db"
	"github.com/spf13/cobra"
)

var (
	tags []string
)

var addCmd = &cobra.Command{
	Use:   "add [message]",
	Short: "Add a log entry",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		message := args[0]

		// Get database path
		dataHome := config.GetDataHome()
		dbPath := filepath.Join(dataHome, "chronicle", "chronicle.db")

		// Open database
		database, err := db.InitDB(dbPath)
		if err != nil {
			return fmt.Errorf("failed to open database: %w", err)
		}
		defer database.Close()

		// Get metadata
		hostname, err := os.Hostname()
		if err != nil {
			hostname = "unknown"
		}
		username := os.Getenv("USER")
		if username == "" {
			username = "unknown"
		}
		workingDir, err := os.Getwd()
		if err != nil {
			workingDir = "unknown"
		}

		// Create entry
		entry := db.Entry{
			Message:          message,
			Hostname:         hostname,
			Username:         username,
			WorkingDirectory: workingDir,
			Tags:             tags,
		}

		id, err := db.CreateEntry(database, entry)
		if err != nil {
			return fmt.Errorf("failed to create entry: %w", err)
		}

		fmt.Printf("Entry created (ID: %d)\n", id)
		return nil
	},
}

func init() {
	addCmd.Flags().StringArrayVarP(&tags, "tag", "t", []string{}, "Add tags to entry")
	rootCmd.AddCommand(addCmd)
}
