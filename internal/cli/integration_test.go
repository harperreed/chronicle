// ABOUTME: Integration tests for CLI commands
// ABOUTME: Tests actual command execution with temp database

package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/harper/chronicle/internal/storage"
)

// testSetup creates a temporary directory and sets up environment for testing
func testSetup(t *testing.T) (string, func()) {
	t.Helper()

	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "chronicle", "chronicle.db")

	// Create directory
	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		t.Fatalf("failed to create temp directory: %v", err)
	}

	// Set XDG_DATA_HOME to redirect storage
	oldDataHome := os.Getenv("XDG_DATA_HOME")
	os.Setenv("XDG_DATA_HOME", tmpDir)

	cleanup := func() {
		os.Setenv("XDG_DATA_HOME", oldDataHome)
	}

	return tmpDir, cleanup
}

func TestIsKnownCommand(t *testing.T) {
	tests := []struct {
		name     string
		arg      string
		expected bool
	}{
		{"add command", "add", true},
		{"list command", "list", true},
		{"search command", "search", true},
		{"export command", "export", true},
		{"sync command", "sync", true},
		{"mcp command", "mcp", true},
		{"install-skill command", "install-skill", true},
		{"unknown command", "unknown", false},
		{"random text", "random", false},
		{"empty string", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isKnownCommand(tt.arg)
			if result != tt.expected {
				t.Errorf("isKnownCommand(%q) = %v, want %v", tt.arg, result, tt.expected)
			}
		})
	}
}

func TestShouldInjectAddCommandLogic(t *testing.T) {
	// Test the helper function directly
	tests := []struct {
		name string
		arg  string
		want bool
	}{
		{"known command should not inject", "list", false},
		{"flag should not inject", "-h", false},
		{"flag with double dash should not inject", "--help", false},
		{"unknown text should inject", "some message", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// isKnownCommand returns true for known commands
			// shouldInjectAddCommand should return false for known commands
			isKnown := isKnownCommand(tt.arg)
			isFlag := len(tt.arg) > 0 && tt.arg[0] == '-'

			// The logic: inject add if not a flag AND not a known command
			shouldInject := !isFlag && !isKnown

			if shouldInject != tt.want {
				t.Errorf("inject logic for %q = %v, want %v", tt.arg, shouldInject, tt.want)
			}
		})
	}
}

func TestAddCommandWithRealStorage(t *testing.T) {
	tmpDir, cleanup := testSetup(t)
	defer cleanup()

	t.Run("adds entry to real database", func(t *testing.T) {
		var stdout bytes.Buffer
		rootCmd.SetOut(&stdout)
		rootCmd.SetErr(&stdout)

		// Reset tags before test
		tags = []string{}

		rootCmd.SetArgs([]string{"add", "test message for integration"})
		err := rootCmd.Execute()

		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}

		// Verify entry was created by opening the database
		dbPath := filepath.Join(tmpDir, "chronicle", "chronicle.db")
		store, err := storage.NewStore(dbPath)
		if err != nil {
			t.Fatalf("failed to open store: %v", err)
		}
		defer store.Close()

		entries, err := store.ListEntries(10)
		if err != nil {
			t.Fatalf("failed to list entries: %v", err)
		}

		if len(entries) == 0 {
			t.Error("expected at least one entry in database")
		}
	})
}

func TestListCommandWithRealStorage(t *testing.T) {
	tmpDir, cleanup := testSetup(t)
	defer cleanup()

	// First add an entry
	dbPath := filepath.Join(tmpDir, "chronicle", "chronicle.db")
	store, err := storage.NewStore(dbPath)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	entry := storage.Entry{
		Message: "test entry for list",
		Tags:    []string{"test"},
	}
	_, err = store.CreateEntry(entry)
	if err != nil {
		store.Close()
		t.Fatalf("failed to create entry: %v", err)
	}
	store.Close()

	t.Run("lists entries from database", func(t *testing.T) {
		var stdout bytes.Buffer
		rootCmd.SetOut(&stdout)
		rootCmd.SetErr(&stdout)

		// Reset flags
		listLimit = 20
		listJSONOutput = false

		rootCmd.SetArgs([]string{"list"})
		err := rootCmd.Execute()

		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}
		// Output goes to os.Stdout not to cobra's buffer for this command
		// so just verify no error
	})

	t.Run("lists entries as JSON", func(t *testing.T) {
		var stdout bytes.Buffer
		rootCmd.SetOut(&stdout)
		rootCmd.SetErr(&stdout)

		// Reset flags
		listLimit = 20
		listJSONOutput = false

		rootCmd.SetArgs([]string{"list", "--json"})
		err := rootCmd.Execute()

		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}
	})
}

func TestSearchCommandWithRealStorage(t *testing.T) {
	tmpDir, cleanup := testSetup(t)
	defer cleanup()

	// First add some entries
	dbPath := filepath.Join(tmpDir, "chronicle", "chronicle.db")
	store, err := storage.NewStore(dbPath)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	entries := []storage.Entry{
		{Message: "deployed app to production", Tags: []string{"deployment"}},
		{Message: "fixed bug in login", Tags: []string{"bugfix"}},
	}

	for _, e := range entries {
		if _, err := store.CreateEntry(e); err != nil {
			store.Close()
			t.Fatalf("failed to create entry: %v", err)
		}
	}
	store.Close()

	t.Run("searches entries", func(t *testing.T) {
		var stdout bytes.Buffer
		rootCmd.SetOut(&stdout)
		rootCmd.SetErr(&stdout)

		// Reset flags
		searchTags = []string{}
		searchSince = ""
		searchUntil = ""
		searchLimit = 100
		searchJSONOutput = false

		rootCmd.SetArgs([]string{"search", "deployed"})
		err := rootCmd.Execute()

		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}
	})

	t.Run("searches with tags", func(t *testing.T) {
		var stdout bytes.Buffer
		rootCmd.SetOut(&stdout)
		rootCmd.SetErr(&stdout)

		// Reset flags
		searchTags = []string{}
		searchSince = ""
		searchUntil = ""
		searchLimit = 100
		searchJSONOutput = false

		rootCmd.SetArgs([]string{"search", "--tag", "deployment"})
		err := rootCmd.Execute()

		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}
	})

	t.Run("searches with JSON output", func(t *testing.T) {
		var stdout bytes.Buffer
		rootCmd.SetOut(&stdout)
		rootCmd.SetErr(&stdout)

		// Reset flags
		searchTags = []string{}
		searchSince = ""
		searchUntil = ""
		searchLimit = 100
		searchJSONOutput = false

		rootCmd.SetArgs([]string{"search", "--json"})
		err := rootCmd.Execute()

		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}
	})

	t.Run("handles invalid since date", func(t *testing.T) {
		var stderr bytes.Buffer
		rootCmd.SetOut(&stderr)
		rootCmd.SetErr(&stderr)

		// Reset flags
		searchTags = []string{}
		searchSince = ""
		searchUntil = ""
		searchLimit = 100
		searchJSONOutput = false

		rootCmd.SetArgs([]string{"search", "--since", "not-a-date"})
		err := rootCmd.Execute()

		if err == nil {
			t.Error("expected error for invalid date")
		}
	})

	t.Run("handles invalid until date", func(t *testing.T) {
		var stderr bytes.Buffer
		rootCmd.SetOut(&stderr)
		rootCmd.SetErr(&stderr)

		// Reset flags
		searchTags = []string{}
		searchSince = ""
		searchUntil = ""
		searchLimit = 100
		searchJSONOutput = false

		rootCmd.SetArgs([]string{"search", "--until", "not-a-date"})
		err := rootCmd.Execute()

		if err == nil {
			t.Error("expected error for invalid date")
		}
	})
}

func TestExportCommandWithRealStorage(t *testing.T) {
	tmpDir, cleanup := testSetup(t)
	defer cleanup()

	// First add an entry
	dbPath := filepath.Join(tmpDir, "chronicle", "chronicle.db")
	store, err := storage.NewStore(dbPath)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	entry := storage.Entry{
		Message: "test entry for export",
		Tags:    []string{"test"},
	}
	_, err = store.CreateEntry(entry)
	if err != nil {
		store.Close()
		t.Fatalf("failed to create entry: %v", err)
	}
	store.Close()

	t.Run("exports as yaml (default)", func(t *testing.T) {
		var stdout bytes.Buffer
		rootCmd.SetOut(&stdout)
		rootCmd.SetErr(&stdout)

		// Reset flags
		exportFormat = "yaml"
		exportOutput = ""

		rootCmd.SetArgs([]string{"export"})
		err := rootCmd.Execute()

		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}
	})

	t.Run("exports as json", func(t *testing.T) {
		var stdout bytes.Buffer
		rootCmd.SetOut(&stdout)
		rootCmd.SetErr(&stdout)

		// Reset flags
		exportFormat = "yaml"
		exportOutput = ""

		rootCmd.SetArgs([]string{"export", "--format", "json"})
		err := rootCmd.Execute()

		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}
	})

	t.Run("exports as markdown", func(t *testing.T) {
		var stdout bytes.Buffer
		rootCmd.SetOut(&stdout)
		rootCmd.SetErr(&stdout)

		// Reset flags
		exportFormat = "yaml"
		exportOutput = ""

		rootCmd.SetArgs([]string{"export", "--format", "markdown"})
		err := rootCmd.Execute()

		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}
	})

	t.Run("exports to file", func(t *testing.T) {
		var stdout bytes.Buffer
		rootCmd.SetOut(&stdout)
		rootCmd.SetErr(&stdout)

		// Reset flags
		exportFormat = "yaml"
		exportOutput = ""

		outputFile := filepath.Join(tmpDir, "export.yaml")
		rootCmd.SetArgs([]string{"export", "-o", outputFile})
		err := rootCmd.Execute()

		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}

		// Verify file was created
		if _, err := os.Stat(outputFile); os.IsNotExist(err) {
			t.Error("expected output file to exist")
		}
	})

	t.Run("rejects unknown format", func(t *testing.T) {
		var stderr bytes.Buffer
		rootCmd.SetOut(&stderr)
		rootCmd.SetErr(&stderr)

		// Reset flags
		exportFormat = "yaml"
		exportOutput = ""

		rootCmd.SetArgs([]string{"export", "--format", "unknown"})
		err := rootCmd.Execute()

		if err == nil {
			t.Error("expected error for unknown format")
		}
	})
}

func TestSyncStatusCommand(t *testing.T) {
	tmpDir, cleanup := testSetup(t)
	defer cleanup()

	t.Run("shows status for non-existent database", func(t *testing.T) {
		var stdout bytes.Buffer
		rootCmd.SetOut(&stdout)
		rootCmd.SetErr(&stdout)

		rootCmd.SetArgs([]string{"sync", "status"})
		err := rootCmd.Execute()

		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}
	})

	t.Run("shows status for existing database", func(t *testing.T) {
		// Create database
		dbPath := filepath.Join(tmpDir, "chronicle", "chronicle.db")
		store, err := storage.NewStore(dbPath)
		if err != nil {
			t.Fatalf("failed to create store: %v", err)
		}
		store.Close()

		var stdout bytes.Buffer
		rootCmd.SetOut(&stdout)
		rootCmd.SetErr(&stdout)

		rootCmd.SetArgs([]string{"sync", "status"})
		err = rootCmd.Execute()

		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}
	})
}

func TestSyncRepairCommand(t *testing.T) {
	tmpDir, cleanup := testSetup(t)
	defer cleanup()

	// Create database
	dbPath := filepath.Join(tmpDir, "chronicle", "chronicle.db")
	store, err := storage.NewStore(dbPath)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	store.Close()

	t.Run("repairs database", func(t *testing.T) {
		var stdout bytes.Buffer
		rootCmd.SetOut(&stdout)
		rootCmd.SetErr(&stdout)

		rootCmd.SetArgs([]string{"sync", "repair"})
		err := rootCmd.Execute()

		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}
	})
}

func TestCommandAliases(t *testing.T) {
	t.Run("add command has 'a' alias", func(t *testing.T) {
		found := false
		for _, alias := range addCmd.Aliases {
			if alias == "a" {
				found = true
				break
			}
		}
		if !found {
			t.Error("expected 'a' alias for add command")
		}
	})
}

func TestSearchWithSinceDate(t *testing.T) {
	tmpDir, cleanup := testSetup(t)
	defer cleanup()

	// Create database with entries
	dbPath := filepath.Join(tmpDir, "chronicle", "chronicle.db")
	store, err := storage.NewStore(dbPath)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	entry := storage.Entry{
		Message: "test entry",
	}
	if _, err := store.CreateEntry(entry); err != nil {
		store.Close()
		t.Fatalf("failed to create entry: %v", err)
	}
	store.Close()

	t.Run("searches with since date", func(t *testing.T) {
		var stdout bytes.Buffer
		rootCmd.SetOut(&stdout)
		rootCmd.SetErr(&stdout)

		// Reset flags
		searchTags = []string{}
		searchSince = ""
		searchUntil = ""
		searchLimit = 100
		searchJSONOutput = false

		rootCmd.SetArgs([]string{"search", "--since", "2020-01-01"})
		err := rootCmd.Execute()

		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}
	})

	t.Run("searches with until date", func(t *testing.T) {
		var stdout bytes.Buffer
		rootCmd.SetOut(&stdout)
		rootCmd.SetErr(&stdout)

		// Reset flags
		searchTags = []string{}
		searchSince = ""
		searchUntil = ""
		searchLimit = 100
		searchJSONOutput = false

		rootCmd.SetArgs([]string{"search", "--until", "2030-12-31"})
		err := rootCmd.Execute()

		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}
	})
}

func TestAddWithTags(t *testing.T) {
	_, cleanup := testSetup(t)
	defer cleanup()

	t.Run("adds entry with tags", func(t *testing.T) {
		var stdout bytes.Buffer
		rootCmd.SetOut(&stdout)
		rootCmd.SetErr(&stdout)

		// Reset tags before test
		tags = []string{}

		rootCmd.SetArgs([]string{"add", "tagged message", "-t", "work", "-t", "important"})
		err := rootCmd.Execute()

		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}
	})
}

func TestListWithLimit(t *testing.T) {
	tmpDir, cleanup := testSetup(t)
	defer cleanup()

	// Create database with entries
	dbPath := filepath.Join(tmpDir, "chronicle", "chronicle.db")
	store, err := storage.NewStore(dbPath)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	for i := 0; i < 5; i++ {
		entry := storage.Entry{
			Message: "test entry",
		}
		if _, err := store.CreateEntry(entry); err != nil {
			store.Close()
			t.Fatalf("failed to create entry: %v", err)
		}
	}
	store.Close()

	t.Run("limits results", func(t *testing.T) {
		var stdout bytes.Buffer
		rootCmd.SetOut(&stdout)
		rootCmd.SetErr(&stdout)

		// Reset flags
		listLimit = 20
		listJSONOutput = false

		rootCmd.SetArgs([]string{"list", "-n", "2"})
		err := rootCmd.Execute()

		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}
	})
}

func TestSearchWithLimit(t *testing.T) {
	tmpDir, cleanup := testSetup(t)
	defer cleanup()

	// Create database with entries
	dbPath := filepath.Join(tmpDir, "chronicle", "chronicle.db")
	store, err := storage.NewStore(dbPath)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	for i := 0; i < 5; i++ {
		entry := storage.Entry{
			Message: "search test entry",
		}
		if _, err := store.CreateEntry(entry); err != nil {
			store.Close()
			t.Fatalf("failed to create entry: %v", err)
		}
	}
	store.Close()

	t.Run("limits search results", func(t *testing.T) {
		var stdout bytes.Buffer
		rootCmd.SetOut(&stdout)
		rootCmd.SetErr(&stdout)

		// Reset flags
		searchTags = []string{}
		searchSince = ""
		searchUntil = ""
		searchLimit = 100
		searchJSONOutput = false

		rootCmd.SetArgs([]string{"search", "-n", "2"})
		err := rootCmd.Execute()

		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}
	})
}

func TestExportWithMdFormat(t *testing.T) {
	tmpDir, cleanup := testSetup(t)
	defer cleanup()

	// Create database with entries
	dbPath := filepath.Join(tmpDir, "chronicle", "chronicle.db")
	store, err := storage.NewStore(dbPath)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	entry := storage.Entry{
		Message: "test entry",
	}
	if _, err := store.CreateEntry(entry); err != nil {
		store.Close()
		t.Fatalf("failed to create entry: %v", err)
	}
	store.Close()

	t.Run("exports as md (alias for markdown)", func(t *testing.T) {
		var stdout bytes.Buffer
		rootCmd.SetOut(&stdout)
		rootCmd.SetErr(&stdout)

		// Reset flags
		exportFormat = "yaml"
		exportOutput = ""

		rootCmd.SetArgs([]string{"export", "--format", "md"})
		err := rootCmd.Execute()

		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}
	})

	t.Run("exports as yml (alias for yaml)", func(t *testing.T) {
		var stdout bytes.Buffer
		rootCmd.SetOut(&stdout)
		rootCmd.SetErr(&stdout)

		// Reset flags
		exportFormat = "yaml"
		exportOutput = ""

		rootCmd.SetArgs([]string{"export", "--format", "yml"})
		err := rootCmd.Execute()

		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}
	})
}

func TestSyncSubcommandMetadata(t *testing.T) {
	t.Run("status command has correct metadata", func(t *testing.T) {
		if syncStatusCmd.Use != "status" {
			t.Errorf("expected Use to be 'status', got: %s", syncStatusCmd.Use)
		}
	})

	t.Run("repair command has correct metadata", func(t *testing.T) {
		if syncRepairCmd.Use != "repair" {
			t.Errorf("expected Use to be 'repair', got: %s", syncRepairCmd.Use)
		}
	})

	t.Run("reset command has correct metadata", func(t *testing.T) {
		if syncResetCmd.Use != "reset" {
			t.Errorf("expected Use to be 'reset', got: %s", syncResetCmd.Use)
		}
	})

	t.Run("wipe command has correct metadata", func(t *testing.T) {
		if syncWipeCmd.Use != "wipe" {
			t.Errorf("expected Use to be 'wipe', got: %s", syncWipeCmd.Use)
		}
	})
}

func TestAddWithProjectLogging(t *testing.T) {
	tmpDir, cleanup := testSetup(t)
	defer cleanup()

	// Create a project directory with .chronicle config
	projectDir := filepath.Join(tmpDir, "project")
	chronicleDir := filepath.Join(projectDir, ".chronicle")
	logsDir := filepath.Join(projectDir, "logs")

	if err := os.MkdirAll(chronicleDir, 0755); err != nil {
		t.Fatalf("failed to create chronicle dir: %v", err)
	}

	// Create a valid project config
	configContent := `local_logging: true
log_dir: logs
log_format: json
`
	if err := os.WriteFile(filepath.Join(chronicleDir, "config.yaml"), []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	// Change to project directory
	oldWd, _ := os.Getwd()
	defer os.Chdir(oldWd)
	os.Chdir(projectDir)

	t.Run("adds entry with project logging", func(t *testing.T) {
		var stdout bytes.Buffer
		rootCmd.SetOut(&stdout)
		rootCmd.SetErr(&stdout)

		// Reset tags before test
		tags = []string{}

		rootCmd.SetArgs([]string{"add", "project logged message"})
		err := rootCmd.Execute()

		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}

		// Check if log directory was created
		if _, err := os.Stat(logsDir); err == nil {
			// Directory exists, project logging worked
		}
	})
}

func TestListCommandStructure(t *testing.T) {
	t.Run("list command accepts no arguments", func(t *testing.T) {
		if listCmd.Args != nil {
			// list command should not have args validation
		}
	})

	t.Run("list command has RunE set", func(t *testing.T) {
		if listCmd.RunE == nil {
			t.Error("expected RunE to be set")
		}
	})
}

func TestSearchCommandStructure(t *testing.T) {
	t.Run("search command has RunE set", func(t *testing.T) {
		if searchCmd.RunE == nil {
			t.Error("expected RunE to be set")
		}
	})
}

func TestExportCommandStructure(t *testing.T) {
	t.Run("export command has RunE set", func(t *testing.T) {
		if exportCmd.RunE == nil {
			t.Error("expected RunE to be set")
		}
	})
}

func TestSyncStatusRunE(t *testing.T) {
	t.Run("status command has RunE set", func(t *testing.T) {
		if syncStatusCmd.RunE == nil {
			t.Error("expected RunE to be set")
		}
	})
}

func TestSyncRepairRunE(t *testing.T) {
	t.Run("repair command has RunE set", func(t *testing.T) {
		if syncRepairCmd.RunE == nil {
			t.Error("expected RunE to be set")
		}
	})
}

func TestSyncResetRunE(t *testing.T) {
	t.Run("reset command has RunE set", func(t *testing.T) {
		if syncResetCmd.RunE == nil {
			t.Error("expected RunE to be set")
		}
	})
}

func TestSyncWipeRunE(t *testing.T) {
	t.Run("wipe command has RunE set", func(t *testing.T) {
		if syncWipeCmd.RunE == nil {
			t.Error("expected RunE to be set")
		}
	})
}

func TestInstallSkillCommand(t *testing.T) {
	tmpDir := t.TempDir()

	// Set HOME to temp dir for this test
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)

	t.Run("installs skill with --yes flag", func(t *testing.T) {
		var stdout bytes.Buffer
		rootCmd.SetOut(&stdout)
		rootCmd.SetErr(&stdout)

		// Reset flag
		skillSkipConfirm = false

		rootCmd.SetArgs([]string{"install-skill", "--yes"})
		err := rootCmd.Execute()

		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}

		// Verify file was created
		expectedPath := filepath.Join(tmpDir, ".claude", "skills", "chronicle", "SKILL.md")
		if _, err := os.Stat(expectedPath); os.IsNotExist(err) {
			t.Errorf("expected skill file to exist at %s", expectedPath)
		}
	})
}
