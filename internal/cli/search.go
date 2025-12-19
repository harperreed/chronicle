// ABOUTME: Search command for querying entries
// ABOUTME: Supports text search, tags, and date ranges
package cli

import (
	"encoding/json"
	"fmt"

	"github.com/araddon/dateparse"
	"github.com/harper/chronicle/internal/charm"
	"github.com/spf13/cobra"
)

var (
	searchTags       []string
	searchSince      string
	searchUntil      string
	searchLimit      int
	searchJSONOutput bool
)

var searchCmd = &cobra.Command{
	Use:   "search [text]",
	Short: "Search entries",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		// Get Charm client
		client, err := charm.GetClient()
		if err != nil {
			return fmt.Errorf("failed to connect to Charm: %w", err)
		}

		// Build search filter
		filter := &charm.SearchFilter{
			Tags: searchTags,
		}

		if len(args) > 0 {
			filter.Text = args[0]
		}

		// Parse dates
		if searchSince != "" {
			since, err := dateparse.ParseAny(searchSince)
			if err != nil {
				return fmt.Errorf("invalid --since date: %w", err)
			}
			filter.Since = &since
		}

		if searchUntil != "" {
			until, err := dateparse.ParseAny(searchUntil)
			if err != nil {
				return fmt.Errorf("invalid --until date: %w", err)
			}
			filter.Until = &until
		}

		// Search
		entries, err := client.SearchEntries(filter, searchLimit)
		if err != nil {
			return fmt.Errorf("failed to search entries: %w", err)
		}

		// Output
		if searchJSONOutput {
			data, err := json.MarshalIndent(entries, "", "  ")
			if err != nil {
				return fmt.Errorf("failed to marshal JSON: %w", err)
			}
			fmt.Println(string(data))
		} else {
			fmt.Println("ID\tTimestamp\t\t\tTags\t\tMessage")
			fmt.Println("--\t---------\t\t\t----\t\t-------")
			for _, entry := range entries {
				tagsStr := ""
				if len(entry.Tags) > 0 {
					tagsStr = fmt.Sprintf("%v", entry.Tags)
				}
				timestamp := entry.Timestamp.Format("2006-01-02 15:04:05")
				fmt.Printf("%s\t%s\t%s\t%s\n", entry.ID, timestamp, tagsStr, entry.Message)
			}
		}

		return nil
	},
}

func init() {
	searchCmd.Flags().StringArrayVarP(&searchTags, "tag", "t", []string{}, "Filter by tags")
	searchCmd.Flags().StringVar(&searchSince, "since", "", "Start date (natural language or ISO)")
	searchCmd.Flags().StringVar(&searchUntil, "until", "", "End date (natural language or ISO)")
	searchCmd.Flags().IntVarP(&searchLimit, "limit", "n", 100, "Maximum results")
	searchCmd.Flags().BoolVar(&searchJSONOutput, "json", false, "Output as JSON")
	rootCmd.AddCommand(searchCmd)
}
