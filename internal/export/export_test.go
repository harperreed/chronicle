// ABOUTME: Tests for export functionality
// ABOUTME: Validates markdown, YAML, and JSON export formats

package export

import (
	"strings"
	"testing"
	"time"

	"github.com/harper/chronicle/internal/storage"
)

func TestMarkdownExport(t *testing.T) {
	entries := []storage.Entry{
		{
			ID:               "abc123",
			Timestamp:        time.Date(2026, 1, 31, 14, 32, 15, 0, time.UTC),
			Message:          "deployed v2.1.0",
			Tags:             []string{"deployment", "production"},
			Hostname:         "MacBook-Pro",
			Username:         "harper",
			WorkingDirectory: "/home/harper/app",
		},
		{
			ID:               "def456",
			Timestamp:        time.Date(2026, 1, 31, 10, 15, 0, 0, time.UTC),
			Message:          "fixed login bug",
			Tags:             []string{"bugfix"},
			Hostname:         "MacBook-Pro",
			Username:         "harper",
			WorkingDirectory: "/home/harper/app",
		},
	}

	output, err := ToMarkdown(entries)
	if err != nil {
		t.Fatalf("ToMarkdown failed: %v", err)
	}

	lines := strings.Split(output, "\n")
	if len(lines) < 3 {
		t.Fatalf("markdown output is missing its generated timestamp: %q", output)
	}
	generatedAt := strings.TrimPrefix(lines[2], "Generated: ")
	if generatedAt == lines[2] {
		t.Fatalf("generated timestamp line has an unexpected format: %q", lines[2])
	}
	if _, err := time.Parse(time.RFC3339, generatedAt); err != nil {
		t.Fatalf("generated timestamp is not RFC3339: %q: %v", generatedAt, err)
	}
	lines[2] = "Generated: <timestamp>"

	expected := `# Chronicle Export

Generated: <timestamp>

---

## 2026-01-31

### 14:32:15 - deployed v2.1.0
- **ID**: abc123
- **Tags**: deployment, production
- **Host**: MacBook-Pro
- **User**: harper
- **Directory**: /home/harper/app

### 10:15:00 - fixed login bug
- **ID**: def456
- **Tags**: bugfix
- **Host**: MacBook-Pro
- **User**: harper
- **Directory**: /home/harper/app

`

	if normalized := strings.Join(lines, "\n"); normalized != expected {
		t.Errorf("markdown output mismatch:\ngot:\n%s\nwant:\n%s", normalized, expected)
	}
}

func TestYAMLExport(t *testing.T) {
	entries := []storage.Entry{
		{
			ID:               "abc123",
			Timestamp:        time.Date(2026, 1, 31, 14, 32, 15, 0, time.UTC),
			Message:          "deployed v2.1.0",
			Tags:             []string{"deployment", "production"},
			Hostname:         "MacBook-Pro",
			Username:         "harper",
			WorkingDirectory: "/home/harper/app",
		},
	}

	output, err := ToYAML(entries)
	if err != nil {
		t.Fatalf("ToYAML failed: %v", err)
	}

	// Check version header
	if !strings.Contains(output, "version: \"1.0\"") {
		t.Error("expected version header")
	}
	if !strings.Contains(output, "tool: chronicle") {
		t.Error("expected tool identifier")
	}
	if !strings.Contains(output, "exported_at:") {
		t.Error("expected exported_at timestamp")
	}

	// Check entry content
	if !strings.Contains(output, "id: abc123") {
		t.Error("expected entry ID")
	}
	if !strings.Contains(output, "message: deployed v2.1.0") {
		t.Error("expected entry message")
	}
	if !strings.Contains(output, "- deployment") {
		t.Error("expected deployment tag")
	}
}

func TestJSONExport(t *testing.T) {
	entries := []storage.Entry{
		{
			ID:               "abc123",
			Timestamp:        time.Date(2026, 1, 31, 14, 32, 15, 0, time.UTC),
			Message:          "deployed v2.1.0",
			Tags:             []string{"deployment", "production"},
			Hostname:         "MacBook-Pro",
			Username:         "harper",
			WorkingDirectory: "/home/harper/app",
		},
	}

	output, err := ToJSON(entries)
	if err != nil {
		t.Fatalf("ToJSON failed: %v", err)
	}

	// Check structure
	if !strings.Contains(output, "\"version\": \"1.0\"") {
		t.Error("expected version")
	}
	if !strings.Contains(output, "\"tool\": \"chronicle\"") {
		t.Error("expected tool identifier")
	}
	if !strings.Contains(output, "\"entries\":") {
		t.Error("expected entries array")
	}

	// Check entry content
	if !strings.Contains(output, "\"id\": \"abc123\"") {
		t.Error("expected entry ID")
	}
	if !strings.Contains(output, "\"message\": \"deployed v2.1.0\"") {
		t.Error("expected entry message")
	}
}

func TestEmptyExport(t *testing.T) {
	var entries []storage.Entry

	// Markdown should still have header
	md, err := ToMarkdown(entries)
	if err != nil {
		t.Fatalf("ToMarkdown failed: %v", err)
	}
	if !strings.Contains(md, "# Chronicle Export") {
		t.Error("expected markdown header even with no entries")
	}

	// YAML should still have header
	yaml, err := ToYAML(entries)
	if err != nil {
		t.Fatalf("ToYAML failed: %v", err)
	}
	if !strings.Contains(yaml, "version:") {
		t.Error("expected YAML header even with no entries")
	}

	// JSON should still have structure
	json, err := ToJSON(entries)
	if err != nil {
		t.Fatalf("ToJSON failed: %v", err)
	}
	if !strings.Contains(json, "\"entries\":") {
		t.Error("expected JSON structure even with no entries")
	}
}
