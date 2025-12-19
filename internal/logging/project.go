// ABOUTME: Project log file writing
// ABOUTME: Formats entries as markdown or JSON and appends to daily logs
package logging

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Entry represents a log entry for project logging.
type Entry struct {
	ID               string    `json:"id"`
	Timestamp        time.Time `json:"timestamp"`
	Message          string    `json:"message"`
	Hostname         string    `json:"hostname"`
	Username         string    `json:"username"`
	WorkingDirectory string    `json:"working_directory"`
	Tags             []string  `json:"tags"`
}

// WriteProjectLog appends entry to project log file.
func WriteProjectLog(logDir, format string, entry Entry) error {
	// Validate timestamp is not zero
	if entry.Timestamp.IsZero() {
		return fmt.Errorf("entry timestamp is zero")
	}

	// Clean the logDir path to prevent traversal
	logDir = filepath.Clean(logDir)

	// Create log directory if needed
	if err := os.MkdirAll(logDir, 0755); err != nil { //nolint:gosec // Standard directory permissions for project logs
		return err
	}

	// Determine log file name (one per day in local time)
	date := entry.Timestamp.Local().Format("2006-01-02")
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
	f, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644) //nolint:gosec // Standard file permissions for log files
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()

	_, err = f.WriteString(content)
	return err
}

func formatMarkdown(entry Entry) string {
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
