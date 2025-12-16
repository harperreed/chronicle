// ABOUTME: Add command for creating new log entries
// ABOUTME: Handles message input and tag flags with optional sync
package cli

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/harper/chronicle/internal/config"
	"github.com/harper/chronicle/internal/db"
	"github.com/harper/chronicle/internal/logging"
	"github.com/harper/chronicle/internal/sync"
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

		// Get database path
		dataHome := config.GetDataHome()
		dbPath := filepath.Join(dataHome, "chronicle", "chronicle.db")

		// Open database
		database, err := db.InitDB(dbPath)
		if err != nil {
			return fmt.Errorf("failed to open database: %w", err)
		}
		defer func() {
			if closeErr := database.Close(); closeErr != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to close database: %v\n", closeErr)
			}
		}()

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

		// Update entry with the returned ID
		entry.ID = id

		// Fetch the specific entry we just created by ID to get its timestamp
		err = database.QueryRow("SELECT timestamp FROM entries WHERE id = ?", id).Scan(&entry.Timestamp)
		if err != nil {
			// If we can't get timestamp, use current time as fallback
			entry.Timestamp = time.Now()
		}

		fmt.Printf("Entry created (ID: %s)\n", id)

		// Queue for sync (secondary, non-blocking)
		if err := queueEntryToSync(cmd.Context(), database, entry); err != nil {
			log.Printf("warning: sync queue failed: %v", err)
		}

		// Check for project logging
		projectRoot, err := config.FindProjectRoot(workingDir)
		if err == nil && projectRoot != "" {
			chroniclePath := filepath.Join(projectRoot, ".chronicle")
			projectCfg, err := config.LoadProjectConfig(chroniclePath)
			if err == nil && projectCfg.LocalLogging {
				logDir := filepath.Join(projectRoot, projectCfg.LogDir)
				if err := logging.WriteProjectLog(logDir, projectCfg.LogFormat, entry); err != nil {
					fmt.Fprintf(os.Stderr, "Warning: failed to write project log: %v\n", err)
				} else {
					fmt.Printf("Project log updated: %s\n", logDir)
				}
			}
		}

		return nil
	},
}

func queueEntryToSync(ctx context.Context, appDB *sql.DB, entry db.Entry) error {
	cfg, err := sync.LoadConfig()
	if err != nil {
		return nil // No config, skip silently
	}

	if !cfg.IsConfigured() {
		return nil // Not configured, skip silently
	}

	syncer, err := sync.NewSyncer(cfg, appDB)
	if err != nil {
		return fmt.Errorf("create syncer: %w", err)
	}
	defer func() {
		if closeErr := syncer.Close(); closeErr != nil {
			log.Printf("warning: failed to close syncer: %v", closeErr)
		}
	}()

	if err := syncer.QueueEntryChange(ctx, entry, sync.OpUpsert); err != nil {
		return fmt.Errorf("queue entry: %w", err)
	}

	return nil
}

func init() {
	addCmd.Flags().StringArrayVarP(&tags, "tag", "t", []string{}, "Add tags to entry")
	rootCmd.AddCommand(addCmd)
}
