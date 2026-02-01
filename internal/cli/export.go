// ABOUTME: Export command for backing up chronicle entries
// ABOUTME: Supports markdown, yaml, and json output formats
package cli

import (
	"fmt"
	"os"

	"github.com/harper/chronicle/internal/export"
	"github.com/harper/chronicle/internal/storage"
	"github.com/spf13/cobra"
)

var (
	exportFormat string
	exportOutput string
)

var exportCmd = &cobra.Command{
	Use:   "export",
	Short: "Export entries to file",
	Long: `Export chronicle entries to various formats.

Supported formats:
  markdown - Human-readable markdown document
  yaml     - YAML format for import/export
  json     - JSON format for programmatic use

Examples:
  chronicle export --format=markdown > backup.md
  chronicle export --format=yaml -o backup.yaml
  chronicle export --format=json`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Get storage
		store, err := storage.NewStore(storage.DefaultPath())
		if err != nil {
			return fmt.Errorf("failed to open storage: %w", err)
		}
		defer func() { _ = store.Close() }()

		// Get all entries (no limit)
		entries, err := store.ListEntries(0)
		if err != nil {
			return fmt.Errorf("failed to list entries: %w", err)
		}

		// Export based on format
		var output string
		switch exportFormat {
		case "markdown", "md":
			output, err = export.ToMarkdown(entries)
		case "yaml", "yml":
			output, err = export.ToYAML(entries)
		case "json":
			output, err = export.ToJSON(entries)
		default:
			return fmt.Errorf("unknown format: %s (use markdown, yaml, or json)", exportFormat)
		}

		if err != nil {
			return fmt.Errorf("export failed: %w", err)
		}

		// Write output
		if exportOutput != "" && exportOutput != "-" {
			// #nosec G306 -- Export files need to be readable by user
			if err := os.WriteFile(exportOutput, []byte(output), 0644); err != nil {
				return fmt.Errorf("failed to write file: %w", err)
			}
			fmt.Fprintf(os.Stderr, "Exported %d entries to %s\n", len(entries), exportOutput)
		} else {
			fmt.Print(output)
		}

		return nil
	},
}

func init() {
	exportCmd.Flags().StringVarP(&exportFormat, "format", "f", "yaml", "Output format (markdown, yaml, json)")
	exportCmd.Flags().StringVarP(&exportOutput, "output", "o", "", "Output file (default: stdout)")
	rootCmd.AddCommand(exportCmd)
}
