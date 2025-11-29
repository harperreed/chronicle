// ABOUTME: Project log file writing
// ABOUTME: Formats entries as markdown or JSON and appends to daily logs
package logging

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/harper/chronicle/internal/db"
)

// WriteProjectLog appends entry to project log file
func WriteProjectLog(logDir, format string, entry db.Entry) error {
	// Create log directory if needed
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return err
	}

	// Determine log file name (one per day)
	date := entry.Timestamp.Format("2006-01-02")
	logFile := filepath.Join(logDir, date+".log")

	// Format entry
	var content string
	switch format {
	case "json":
		data, err := json.Marshal(entry)
		if err != nil {
			return err
		}
		content = string(data) + "\n"
	case "markdown":
		fallthrough
	default:
		content = formatMarkdown(entry)
	}

	// Append to file
	f, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = f.WriteString(content)
	return err
}

func formatMarkdown(entry db.Entry) string {
	var sb strings.Builder

	timeStr := entry.Timestamp.Format("15:04:05")
	sb.WriteString(fmt.Sprintf("## %s - %s\n", timeStr, entry.Message))

	if len(entry.Tags) > 0 {
		sb.WriteString(fmt.Sprintf("- **Tags**: %s\n", strings.Join(entry.Tags, ", ")))
	}

	sb.WriteString(fmt.Sprintf("- **User**: %s@%s\n", entry.Username, entry.Hostname))
	sb.WriteString(fmt.Sprintf("- **Directory**: %s\n", entry.WorkingDirectory))
	sb.WriteString("\n")

	return sb.String()
}
