// ABOUTME: List command for displaying recent entries
// ABOUTME: Supports table and JSON output formats
package cli

import (
	"encoding/json"
	"fmt"
	"path/filepath"

	"github.com/harper/chronicle/internal/config"
	"github.com/harper/chronicle/internal/db"
	"github.com/spf13/cobra"
)

var (
	listLimit      int
	listJSONOutput bool
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List recent entries",
	RunE: func(cmd *cobra.Command, args []string) error {
		// Get database path
		dataHome := config.GetDataHome()
		dbPath := filepath.Join(dataHome, "chronicle", "chronicle.db")

		// Open database
		database, err := db.InitDB(dbPath)
		if err != nil {
			return fmt.Errorf("failed to open database: %w", err)
		}
		defer database.Close()

		// List entries
		entries, err := db.ListEntries(database, listLimit)
		if err != nil {
			return fmt.Errorf("failed to list entries: %w", err)
		}

		if listJSONOutput {
			data, err := json.MarshalIndent(entries, "", "  ")
			if err != nil {
				return fmt.Errorf("failed to marshal JSON: %w", err)
			}
			fmt.Println(string(data))
		} else {
			// Print table
			fmt.Println("ID\tTimestamp\t\t\tTags\t\tMessage")
			fmt.Println("--\t---------\t\t\t----\t\t-------")
			for _, entry := range entries {
				tagsStr := ""
				if len(entry.Tags) > 0 {
					tagsStr = fmt.Sprintf("%v", entry.Tags)
				}
				timestamp := entry.Timestamp.Format("2006-01-02 15:04:05")
				fmt.Printf("%d\t%s\t%s\t%s\n", entry.ID, timestamp, tagsStr, entry.Message)
			}
		}

		return nil
	},
}

func init() {
	listCmd.Flags().IntVarP(&listLimit, "limit", "n", 20, "Number of entries to show")
	listCmd.Flags().BoolVar(&listJSONOutput, "json", false, "Output as JSON")
	rootCmd.AddCommand(listCmd)
}
