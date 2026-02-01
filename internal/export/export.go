// ABOUTME: Export functionality for chronicle entries
// ABOUTME: Supports markdown, YAML, and JSON export formats

package export

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/harper/chronicle/internal/storage"
	"gopkg.in/yaml.v3"
)

// ExportData represents the export format structure.
type ExportData struct {
	Version    string        `json:"version" yaml:"version"`
	ExportedAt string        `json:"exported_at" yaml:"exported_at"`
	Tool       string        `json:"tool" yaml:"tool"`
	Entries    []ExportEntry `json:"entries" yaml:"entries"`
}

// ExportEntry represents an entry in the export format.
type ExportEntry struct {
	ID               string   `json:"id" yaml:"id"`
	Timestamp        string   `json:"timestamp" yaml:"timestamp"`
	Message          string   `json:"message" yaml:"message"`
	Tags             []string `json:"tags,omitempty" yaml:"tags,omitempty"`
	Hostname         string   `json:"hostname,omitempty" yaml:"hostname,omitempty"`
	Username         string   `json:"username,omitempty" yaml:"username,omitempty"`
	WorkingDirectory string   `json:"working_directory,omitempty" yaml:"working_directory,omitempty"`
}

// ToMarkdown exports entries as markdown.
func ToMarkdown(entries []storage.Entry) (string, error) {
	var sb strings.Builder

	// Header
	sb.WriteString("# Chronicle Export\n\n")
	sb.WriteString(fmt.Sprintf("Generated: %s\n\n", time.Now().Format(time.RFC3339)))
	sb.WriteString("---\n\n")

	if len(entries) == 0 {
		sb.WriteString("No entries to export.\n")
		return sb.String(), nil
	}

	// Group entries by date
	byDate := make(map[string][]storage.Entry)
	var dates []string

	for _, entry := range entries {
		date := entry.Timestamp.Format("2006-01-02")
		if _, ok := byDate[date]; !ok {
			dates = append(dates, date)
		}
		byDate[date] = append(byDate[date], entry)
	}

	// Sort dates descending
	sort.Sort(sort.Reverse(sort.StringSlice(dates)))

	for _, date := range dates {
		sb.WriteString(fmt.Sprintf("## %s\n\n", date))

		dateEntries := byDate[date]
		// Sort entries by timestamp descending within the day
		sort.Slice(dateEntries, func(i, j int) bool {
			return dateEntries[i].Timestamp.After(dateEntries[j].Timestamp)
		})

		for _, entry := range dateEntries {
			timeStr := entry.Timestamp.Format("15:04:05")
			sb.WriteString(fmt.Sprintf("### %s - %s\n", timeStr, entry.Message))
			sb.WriteString(fmt.Sprintf("- **ID**: %s\n", entry.ID))

			if len(entry.Tags) > 0 {
				sb.WriteString(fmt.Sprintf("- **Tags**: %s\n", strings.Join(entry.Tags, ", ")))
			}

			if entry.Hostname != "" {
				sb.WriteString(fmt.Sprintf("- **Host**: %s\n", entry.Hostname))
			}
			if entry.Username != "" {
				sb.WriteString(fmt.Sprintf("- **User**: %s\n", entry.Username))
			}
			if entry.WorkingDirectory != "" {
				sb.WriteString(fmt.Sprintf("- **Directory**: %s\n", entry.WorkingDirectory))
			}

			sb.WriteString("\n")
		}
	}

	return sb.String(), nil
}

// ToYAML exports entries as YAML.
func ToYAML(entries []storage.Entry) (string, error) {
	data := buildExportData(entries)

	out, err := yaml.Marshal(data)
	if err != nil {
		return "", fmt.Errorf("marshal yaml: %w", err)
	}

	return string(out), nil
}

// ToJSON exports entries as JSON.
func ToJSON(entries []storage.Entry) (string, error) {
	data := buildExportData(entries)

	out, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal json: %w", err)
	}

	return string(out), nil
}

// buildExportData creates the common export structure.
func buildExportData(entries []storage.Entry) ExportData {
	exportEntries := make([]ExportEntry, len(entries))

	for i, entry := range entries {
		exportEntries[i] = ExportEntry{
			ID:               entry.ID,
			Timestamp:        entry.Timestamp.Format(time.RFC3339),
			Message:          entry.Message,
			Tags:             entry.Tags,
			Hostname:         entry.Hostname,
			Username:         entry.Username,
			WorkingDirectory: entry.WorkingDirectory,
		}
	}

	return ExportData{
		Version:    "1.0",
		ExportedAt: time.Now().Format(time.RFC3339),
		Tool:       "chronicle",
		Entries:    exportEntries,
	}
}
