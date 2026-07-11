// ABOUTME: Tests for project log file writing
// ABOUTME: Validates log entry formatting and file operations
package logging

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestWriteProjectLog(t *testing.T) {
	tmpDir := t.TempDir()
	logDir := filepath.Join(tmpDir, "logs")

	entry := Entry{
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
	content, err := os.ReadFile(logFile) //nolint:gosec // Reading test file
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

	entry := Entry{
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
	content, err := os.ReadFile(logFile) //nolint:gosec // Reading test file
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}

	// Should contain JSON fields
	contentStr := string(content)
	if !strings.Contains(contentStr, `"message"`) || !strings.Contains(contentStr, `"tags"`) {
		t.Errorf("JSON output missing expected fields: %s", contentStr)
	}
}

func TestWriteProjectLogMultipleEntries(t *testing.T) {
	tmpDir := t.TempDir()
	logDir := filepath.Join(tmpDir, "logs")

	entry1 := Entry{
		Timestamp:        time.Date(2025, 11, 29, 10, 0, 0, 0, time.UTC),
		Message:          "first entry",
		Hostname:         "testhost",
		Username:         "testuser",
		WorkingDirectory: "/test/dir",
		Tags:             []string{"tag1"},
	}

	entry2 := Entry{
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
	content, err := os.ReadFile(logFile) //nolint:gosec // Reading test file
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}

	contentStr := string(content)
	if !strings.Contains(contentStr, "first entry") || !strings.Contains(contentStr, "second entry") {
		t.Errorf("log file should contain both entries: %s", contentStr)
	}
}

func TestWriteProjectLogRejectsZeroTimestamp(t *testing.T) {
	err := WriteProjectLog(t.TempDir(), "markdown", Entry{})
	if err == nil {
		t.Fatal("WriteProjectLog should reject a zero timestamp")
	}
	if err.Error() != "entry timestamp is zero" {
		t.Fatalf("WriteProjectLog returned %q, want %q", err, "entry timestamp is zero")
	}
}

func TestWriteProjectLogReturnsMkdirAllError(t *testing.T) {
	parentFile := filepath.Join(t.TempDir(), "parent-file")
	if err := os.WriteFile(parentFile, []byte("not a directory"), 0600); err != nil {
		t.Fatalf("failed to create parent file: %v", err)
	}

	logDir := filepath.Join(parentFile, "logs")
	entry := Entry{Timestamp: time.Date(2025, 11, 29, 14, 30, 0, 0, time.Local)}
	err := WriteProjectLog(logDir, "markdown", entry)
	if err == nil {
		t.Fatal("WriteProjectLog should return an error when its parent path is a file")
	}
	var pathErr *os.PathError
	if !errors.As(err, &pathErr) || pathErr.Path != parentFile {
		t.Fatalf("WriteProjectLog error = %v, want a path error for %q", err, parentFile)
	}
}

func TestWriteProjectLogReturnsOpenFileError(t *testing.T) {
	logDir := t.TempDir()
	logFile := filepath.Join(logDir, "2025-11-29.log")
	if err := os.Mkdir(logFile, 0755); err != nil {
		t.Fatalf("failed to create directory at log file path: %v", err)
	}

	entry := Entry{Timestamp: time.Date(2025, 11, 29, 14, 30, 0, 0, time.Local)}
	err := WriteProjectLog(logDir, "markdown", entry)
	if err == nil {
		t.Fatal("WriteProjectLog should return an error when the log file path is a directory")
	}
	var pathErr *os.PathError
	if !errors.As(err, &pathErr) || pathErr.Path != logFile {
		t.Fatalf("WriteProjectLog error = %v, want a path error for %q", err, logFile)
	}
}

func TestWriteProjectLogReturnsJSONMarshalError(t *testing.T) {
	entry := Entry{Timestamp: time.Date(10000, 1, 1, 0, 0, 0, 0, time.UTC)}
	err := WriteProjectLog(t.TempDir(), "json", entry)
	if err == nil {
		t.Fatal("WriteProjectLog should return an error for a timestamp JSON cannot encode")
	}
	if !strings.Contains(err.Error(), "year outside of range") {
		t.Fatalf("WriteProjectLog returned unexpected JSON error: %v", err)
	}
}
