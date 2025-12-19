// ABOUTME: Add command for creating new log entries
// ABOUTME: Handles message input and tag flags with automatic Charm sync
package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/harper/chronicle/internal/charm"
	"github.com/harper/chronicle/internal/config"
	"github.com/harper/chronicle/internal/logging"
	"github.com/spf13/cobra"
)

const (
	unknownValue = "unknown"
)

var (
	tags []string
)

var addCmd = &cobra.Command{
	Use:     "add [message]",
	Aliases: []string{"a"},
	Short:   "Add a log entry",
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		message := args[0]

		// Validate message is not empty
		if message == "" {
			return fmt.Errorf("message cannot be empty")
		}

		// Get Charm client
		client, err := charm.GetClient()
		if err != nil {
			return fmt.Errorf("failed to connect to Charm: %w", err)
		}

		// Get metadata
		hostname, err := os.Hostname()
		if err != nil {
			hostname = unknownValue
		}
		username := os.Getenv("USER")
		if username == "" {
			username = unknownValue
		}
		workingDir, err := os.Getwd()
		if err != nil {
			workingDir = unknownValue
		}

		// Create entry (set timestamp now for project logging)
		now := time.Now()
		entry := charm.Entry{
			Timestamp:        now,
			Message:          message,
			Hostname:         hostname,
			Username:         username,
			WorkingDirectory: workingDir,
			Tags:             tags,
		}

		id, err := client.CreateEntry(entry)
		if err != nil {
			return fmt.Errorf("failed to create entry: %w", err)
		}

		fmt.Printf("Entry created (ID: %s)\n", id)

		// Check for project logging
		projectRoot, err := config.FindProjectRoot(workingDir)
		if err == nil && projectRoot != "" {
			chroniclePath := filepath.Join(projectRoot, ".chronicle")
			projectCfg, err := config.LoadProjectConfig(chroniclePath)
			if err == nil && projectCfg.LocalLogging {
				logDir := filepath.Join(projectRoot, projectCfg.LogDir)
				// Convert charm.Entry to logging.Entry for project logging
				logEntry := logging.Entry{
					ID:               id,
					Timestamp:        now,
					Message:          entry.Message,
					Hostname:         entry.Hostname,
					Username:         entry.Username,
					WorkingDirectory: entry.WorkingDirectory,
					Tags:             entry.Tags,
				}
				if err := logging.WriteProjectLog(logDir, projectCfg.LogFormat, logEntry); err != nil {
					fmt.Fprintf(os.Stderr, "Warning: failed to write project log: %v\n", err)
				} else {
					fmt.Printf("Project log updated: %s\n", logDir)
				}
			}
		}

		return nil
	},
}

func init() {
	addCmd.Flags().StringArrayVarP(&tags, "tag", "t", []string{}, "Add tags to entry")
	rootCmd.AddCommand(addCmd)
}
