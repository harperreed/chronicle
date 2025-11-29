// ABOUTME: Tests for project log file writing
// ABOUTME: Validates log entry formatting and file operations
package logging

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/harper/chronicle/internal/db"
)

func TestWriteProjectLog(t *testing.T) {
	tmpDir := t.TempDir()
	logDir := filepath.Join(tmpDir, "logs")

	entry := db.Entry{
		Timestamp:        time.Date(2025, 11, 29, 14, 30, 0, 0, time.UTC),
		Message:          "test message",
		Hostname:         "testhost",
		Username:         "testuser",
		WorkingDirectory: "/test/dir",
		Tags:             []string{"work", "test"},
	}

	err := WriteProjectLog(logDir, "markdown", entry)
	if err != nil {
		t.Fatalf("WriteProjectLog failed: %v", err)
	}

	// Verify log file was created
	logFile := filepath.Join(logDir, "2025-11-29.log")
	if _, err := os.Stat(logFile); os.IsNotExist(err) {
		t.Fatal("log file was not created")
	}

	// Verify content
	content, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}

	expectedContent := `## 14:30:00 - test message
- **Tags**: work, test
- **User**: testuser@testhost
- **Directory**: /test/dir

`
	if string(content) != expectedContent {
		t.Errorf("got:\n%s\nwant:\n%s", string(content), expectedContent)
	}
}

func TestWriteProjectLogJSON(t *testing.T) {
	tmpDir := t.TempDir()
	logDir := filepath.Join(tmpDir, "logs")

	entry := db.Entry{
		Timestamp:        time.Date(2025, 11, 29, 14, 30, 0, 0, time.UTC),
		Message:          "test message",
		Hostname:         "testhost",
		Username:         "testuser",
		WorkingDirectory: "/test/dir",
		Tags:             []string{"work"},
	}

	err := WriteProjectLog(logDir, "json", entry)
	if err != nil {
		t.Fatalf("WriteProjectLog failed: %v", err)
	}

	// Verify content is valid JSON
	logFile := filepath.Join(logDir, "2025-11-29.log")
	content, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}

	// Should contain JSON fields
	contentStr := string(content)
	if !strings.Contains(contentStr, `"Message"`) || !strings.Contains(contentStr, `"Tags"`) {
		t.Errorf("JSON output missing expected fields: %s", contentStr)
	}
}

func TestWriteProjectLogMultipleEntries(t *testing.T) {
	tmpDir := t.TempDir()
	logDir := filepath.Join(tmpDir, "logs")

	entry1 := db.Entry{
		Timestamp:        time.Date(2025, 11, 29, 10, 0, 0, 0, time.UTC),
		Message:          "first entry",
		Hostname:         "testhost",
		Username:         "testuser",
		WorkingDirectory: "/test/dir",
		Tags:             []string{"tag1"},
	}

	entry2 := db.Entry{
		Timestamp:        time.Date(2025, 11, 29, 15, 0, 0, 0, time.UTC),
		Message:          "second entry",
		Hostname:         "testhost",
		Username:         "testuser",
		WorkingDirectory: "/test/dir",
		Tags:             []string{"tag2"},
	}

	// Write both entries
	err := WriteProjectLog(logDir, "markdown", entry1)
	if err != nil {
		t.Fatalf("WriteProjectLog failed: %v", err)
	}

	err = WriteProjectLog(logDir, "markdown", entry2)
	if err != nil {
		t.Fatalf("WriteProjectLog failed: %v", err)
	}

	// Verify both entries are in the log file
	logFile := filepath.Join(logDir, "2025-11-29.log")
	content, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}

	contentStr := string(content)
	if !strings.Contains(contentStr, "first entry") || !strings.Contains(contentStr, "second entry") {
		t.Errorf("log file should contain both entries: %s", contentStr)
	}
}
