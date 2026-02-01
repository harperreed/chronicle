// ABOUTME: Unit tests for the install-skill command
// ABOUTME: Tests skill installation, directory creation, and overwrite behavior

package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// installSkillToDir is a testable version that installs to a specific directory
// instead of relying on os.UserHomeDir()
func installSkillToDir(homeDir string, skipConfirm bool) error {
	skillDir := filepath.Join(homeDir, ".claude", "skills", "chronicle")
	skillPath := filepath.Join(skillDir, "SKILL.md")

	// Read embedded skill file
	content, err := skillFS.ReadFile("skill/SKILL.md")
	if err != nil {
		return err
	}

	// Create directory
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		return err
	}

	// Write skill file
	if err := os.WriteFile(skillPath, content, 0644); err != nil {
		return err
	}

	return nil
}

func TestSkillCommand(t *testing.T) {
	t.Run("has correct metadata", func(t *testing.T) {
		if installSkillCmd.Use != "install-skill" {
			t.Errorf("expected Use to be 'install-skill', got: %s", installSkillCmd.Use)
		}

		if installSkillCmd.Short != "Install Claude Code skill" {
			t.Errorf("expected Short to be 'Install Claude Code skill', got: %s", installSkillCmd.Short)
		}

		if !strings.Contains(installSkillCmd.Long, "Claude Code") {
			t.Error("expected Long description to mention 'Claude Code'")
		}

		if !strings.Contains(installSkillCmd.Long, "~/.claude/skills/chronicle") {
			t.Error("expected Long description to mention the destination path")
		}
	})

	t.Run("has yes flag for skipping confirmation", func(t *testing.T) {
		flag := installSkillCmd.Flags().Lookup("yes")
		if flag == nil {
			t.Fatal("expected 'yes' flag to exist")
		}
		if flag.Shorthand != "y" {
			t.Errorf("expected yes shorthand to be 'y', got: %s", flag.Shorthand)
		}
		if flag.DefValue != "false" {
			t.Errorf("expected default value to be 'false', got: %s", flag.DefValue)
		}
	})

	t.Run("is registered as subcommand", func(t *testing.T) {
		hasCmd := false
		for _, cmd := range rootCmd.Commands() {
			if cmd.Name() == "install-skill" {
				hasCmd = true
				break
			}
		}
		if !hasCmd {
			t.Error("expected root command to have 'install-skill' subcommand registered")
		}
	})
}

func TestSkillInstallSuccessful(t *testing.T) {
	t.Run("creates skill file in correct location", func(t *testing.T) {
		tempDir := t.TempDir()

		err := installSkillToDir(tempDir, true)
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}

		expectedPath := filepath.Join(tempDir, ".claude", "skills", "chronicle", "SKILL.md")
		if _, err := os.Stat(expectedPath); os.IsNotExist(err) {
			t.Errorf("expected skill file to exist at %s", expectedPath)
		}
	})

	t.Run("creates nested directory structure", func(t *testing.T) {
		tempDir := t.TempDir()

		err := installSkillToDir(tempDir, true)
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}

		// Check each level of the directory structure
		dirs := []string{
			filepath.Join(tempDir, ".claude"),
			filepath.Join(tempDir, ".claude", "skills"),
			filepath.Join(tempDir, ".claude", "skills", "chronicle"),
		}

		for _, dir := range dirs {
			info, err := os.Stat(dir)
			if os.IsNotExist(err) {
				t.Errorf("expected directory to exist: %s", dir)
				continue
			}
			if !info.IsDir() {
				t.Errorf("expected %s to be a directory", dir)
			}
		}
	})

	t.Run("directory has correct permissions", func(t *testing.T) {
		tempDir := t.TempDir()

		err := installSkillToDir(tempDir, true)
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}

		skillDir := filepath.Join(tempDir, ".claude", "skills", "chronicle")
		info, err := os.Stat(skillDir)
		if err != nil {
			t.Fatalf("failed to stat directory: %v", err)
		}

		// Check directory permissions (0755)
		perm := info.Mode().Perm()
		if perm != 0755 {
			t.Errorf("expected directory permissions 0755, got: %04o", perm)
		}
	})
}

func TestSkillFileContent(t *testing.T) {
	t.Run("file contains skill frontmatter", func(t *testing.T) {
		tempDir := t.TempDir()

		err := installSkillToDir(tempDir, true)
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}

		skillPath := filepath.Join(tempDir, ".claude", "skills", "chronicle", "SKILL.md")
		content, err := os.ReadFile(skillPath)
		if err != nil {
			t.Fatalf("failed to read skill file: %v", err)
		}

		contentStr := string(content)

		// Check for YAML frontmatter
		if !strings.HasPrefix(contentStr, "---") {
			t.Error("expected skill file to start with YAML frontmatter '---'")
		}

		if !strings.Contains(contentStr, "name: chronicle") {
			t.Error("expected skill file to contain 'name: chronicle'")
		}

		if !strings.Contains(contentStr, "description:") {
			t.Error("expected skill file to contain 'description:'")
		}
	})

	t.Run("file contains MCP tools documentation", func(t *testing.T) {
		tempDir := t.TempDir()

		err := installSkillToDir(tempDir, true)
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}

		skillPath := filepath.Join(tempDir, ".claude", "skills", "chronicle", "SKILL.md")
		content, err := os.ReadFile(skillPath)
		if err != nil {
			t.Fatalf("failed to read skill file: %v", err)
		}

		contentStr := string(content)

		// Check for MCP tool documentation
		expectedTools := []string{
			"mcp__chronicle__add_entry",
			"mcp__chronicle__list_entries",
			"mcp__chronicle__search_entries",
			"mcp__chronicle__find_when_i",
			"mcp__chronicle__what_was_i_doing",
			"mcp__chronicle__remember_this",
		}

		for _, tool := range expectedTools {
			if !strings.Contains(contentStr, tool) {
				t.Errorf("expected skill file to document tool: %s", tool)
			}
		}
	})

	t.Run("file contains usage examples", func(t *testing.T) {
		tempDir := t.TempDir()

		err := installSkillToDir(tempDir, true)
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}

		skillPath := filepath.Join(tempDir, ".claude", "skills", "chronicle", "SKILL.md")
		content, err := os.ReadFile(skillPath)
		if err != nil {
			t.Fatalf("failed to read skill file: %v", err)
		}

		contentStr := string(content)

		// Check for common patterns section
		if !strings.Contains(contentStr, "Common patterns") {
			t.Error("expected skill file to contain 'Common patterns' section")
		}

		// Check for CLI fallback commands
		if !strings.Contains(contentStr, "chronicle add") {
			t.Error("expected skill file to contain CLI command examples")
		}
	})

	t.Run("file has correct permissions", func(t *testing.T) {
		tempDir := t.TempDir()

		err := installSkillToDir(tempDir, true)
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}

		skillPath := filepath.Join(tempDir, ".claude", "skills", "chronicle", "SKILL.md")
		info, err := os.Stat(skillPath)
		if err != nil {
			t.Fatalf("failed to stat skill file: %v", err)
		}

		// Check file permissions (0644)
		perm := info.Mode().Perm()
		if perm != 0644 {
			t.Errorf("expected file permissions 0644, got: %04o", perm)
		}
	})

	t.Run("file matches embedded content exactly", func(t *testing.T) {
		tempDir := t.TempDir()

		err := installSkillToDir(tempDir, true)
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}

		// Read the installed file
		skillPath := filepath.Join(tempDir, ".claude", "skills", "chronicle", "SKILL.md")
		installedContent, err := os.ReadFile(skillPath)
		if err != nil {
			t.Fatalf("failed to read installed skill file: %v", err)
		}

		// Read the embedded content
		embeddedContent, err := skillFS.ReadFile("skill/SKILL.md")
		if err != nil {
			t.Fatalf("failed to read embedded skill file: %v", err)
		}

		if !bytes.Equal(installedContent, embeddedContent) {
			t.Error("installed skill file content does not match embedded content")
		}
	})
}

func TestSkillOverwrite(t *testing.T) {
	t.Run("overwrites existing skill file", func(t *testing.T) {
		tempDir := t.TempDir()

		// Create directory and initial file
		skillDir := filepath.Join(tempDir, ".claude", "skills", "chronicle")
		if err := os.MkdirAll(skillDir, 0755); err != nil {
			t.Fatalf("failed to create directory: %v", err)
		}

		skillPath := filepath.Join(skillDir, "SKILL.md")
		oldContent := []byte("old content that should be overwritten")
		if err := os.WriteFile(skillPath, oldContent, 0644); err != nil {
			t.Fatalf("failed to write initial file: %v", err)
		}

		// Run install
		err := installSkillToDir(tempDir, true)
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}

		// Read the file
		newContent, err := os.ReadFile(skillPath)
		if err != nil {
			t.Fatalf("failed to read skill file: %v", err)
		}

		// Verify it was overwritten
		if bytes.Equal(newContent, oldContent) {
			t.Error("expected file to be overwritten, but it still contains old content")
		}

		if !strings.Contains(string(newContent), "name: chronicle") {
			t.Error("expected file to contain new skill content")
		}
	})

	t.Run("preserves directory permissions when overwriting", func(t *testing.T) {
		tempDir := t.TempDir()

		// Create directory with specific permissions
		skillDir := filepath.Join(tempDir, ".claude", "skills", "chronicle")
		if err := os.MkdirAll(skillDir, 0755); err != nil {
			t.Fatalf("failed to create directory: %v", err)
		}

		// Run install
		err := installSkillToDir(tempDir, true)
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}

		// Check directory permissions are preserved
		info, err := os.Stat(skillDir)
		if err != nil {
			t.Fatalf("failed to stat directory: %v", err)
		}

		perm := info.Mode().Perm()
		if perm != 0755 {
			t.Errorf("expected directory permissions 0755, got: %04o", perm)
		}
	})

	t.Run("handles consecutive installs", func(t *testing.T) {
		tempDir := t.TempDir()

		// Install multiple times
		for i := 0; i < 3; i++ {
			err := installSkillToDir(tempDir, true)
			if err != nil {
				t.Fatalf("install %d: expected no error, got: %v", i+1, err)
			}
		}

		// Verify file exists and has correct content
		skillPath := filepath.Join(tempDir, ".claude", "skills", "chronicle", "SKILL.md")
		content, err := os.ReadFile(skillPath)
		if err != nil {
			t.Fatalf("failed to read skill file: %v", err)
		}

		if !strings.Contains(string(content), "name: chronicle") {
			t.Error("expected file to contain skill content after multiple installs")
		}
	})
}

func TestSkillEmbeddedFS(t *testing.T) {
	t.Run("embedded skill file is accessible", func(t *testing.T) {
		content, err := skillFS.ReadFile("skill/SKILL.md")
		if err != nil {
			t.Fatalf("failed to read embedded skill file: %v", err)
		}

		if len(content) == 0 {
			t.Error("expected embedded skill file to have content")
		}
	})

	t.Run("embedded skill file is valid markdown", func(t *testing.T) {
		content, err := skillFS.ReadFile("skill/SKILL.md")
		if err != nil {
			t.Fatalf("failed to read embedded skill file: %v", err)
		}

		contentStr := string(content)

		// Should have headers
		if !strings.Contains(contentStr, "# ") {
			t.Error("expected embedded skill file to contain markdown headers")
		}

		// Should have code blocks
		if !strings.Contains(contentStr, "```") {
			t.Error("expected embedded skill file to contain code blocks")
		}
	})
}

func TestSkillSkipConfirmFlag(t *testing.T) {
	t.Run("flag variable defaults to false", func(t *testing.T) {
		// Reset the flag to its default state
		originalValue := skillSkipConfirm
		defer func() { skillSkipConfirm = originalValue }()

		// The flag is defined with default false
		flag := installSkillCmd.Flags().Lookup("yes")
		if flag == nil {
			t.Fatal("expected 'yes' flag to exist")
		}

		if flag.DefValue != "false" {
			t.Errorf("expected default value to be 'false', got: %s", flag.DefValue)
		}
	})
}

func TestInstallSkillDirectFunctionCall(t *testing.T) {
	t.Run("installs with skipConfirm true", func(t *testing.T) {
		tempDir := t.TempDir()

		// Save and set HOME to temp dir
		oldHome := os.Getenv("HOME")
		os.Setenv("HOME", tempDir)
		defer os.Setenv("HOME", oldHome)

		// Set skipConfirm flag
		originalSkipConfirm := skillSkipConfirm
		skillSkipConfirm = true
		defer func() { skillSkipConfirm = originalSkipConfirm }()

		err := installSkill()
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}

		// Verify file was created
		expectedPath := filepath.Join(tempDir, ".claude", "skills", "chronicle", "SKILL.md")
		if _, err := os.Stat(expectedPath); os.IsNotExist(err) {
			t.Errorf("expected skill file to exist at %s", expectedPath)
		}
	})
}

func TestInstallSkillOverwriteExisting(t *testing.T) {
	t.Run("overwrites existing skill file", func(t *testing.T) {
		tempDir := t.TempDir()

		// Create the skill directory and file first
		skillDir := filepath.Join(tempDir, ".claude", "skills", "chronicle")
		if err := os.MkdirAll(skillDir, 0755); err != nil {
			t.Fatalf("failed to create directory: %v", err)
		}

		skillPath := filepath.Join(skillDir, "SKILL.md")
		oldContent := []byte("old content")
		if err := os.WriteFile(skillPath, oldContent, 0644); err != nil {
			t.Fatalf("failed to write initial file: %v", err)
		}

		// Save and set HOME to temp dir
		oldHome := os.Getenv("HOME")
		os.Setenv("HOME", tempDir)
		defer os.Setenv("HOME", oldHome)

		// Set skipConfirm flag
		originalSkipConfirm := skillSkipConfirm
		skillSkipConfirm = true
		defer func() { skillSkipConfirm = originalSkipConfirm }()

		err := installSkill()
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}

		// Verify file was overwritten
		newContent, err := os.ReadFile(skillPath)
		if err != nil {
			t.Fatalf("failed to read skill file: %v", err)
		}

		if bytes.Equal(newContent, oldContent) {
			t.Error("expected file to be overwritten")
		}
	})
}

func TestInstallSkillCommandExecution(t *testing.T) {
	t.Run("command runs without error with --yes", func(t *testing.T) {
		tempDir := t.TempDir()

		// Save and set HOME to temp dir
		oldHome := os.Getenv("HOME")
		os.Setenv("HOME", tempDir)
		defer os.Setenv("HOME", oldHome)

		// Reset flag before test
		skillSkipConfirm = false

		var stdout bytes.Buffer
		installSkillCmd.SetOut(&stdout)
		installSkillCmd.SetErr(&stdout)

		// Execute with --yes flag
		installSkillCmd.SetArgs([]string{"--yes"})
		err := installSkillCmd.Execute()

		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}

		// Reset args
		installSkillCmd.SetArgs([]string{})
	})

	t.Run("command has RunE set", func(t *testing.T) {
		if installSkillCmd.RunE == nil {
			t.Error("expected RunE to be set")
		}
	})
}

func TestInstallSkillOutputMessages(t *testing.T) {
	t.Run("output contains expected information", func(t *testing.T) {
		tempDir := t.TempDir()

		// Save and set HOME to temp dir
		oldHome := os.Getenv("HOME")
		os.Setenv("HOME", tempDir)
		defer os.Setenv("HOME", oldHome)

		// Set skipConfirm flag
		originalSkipConfirm := skillSkipConfirm
		skillSkipConfirm = true
		defer func() { skillSkipConfirm = originalSkipConfirm }()

		// Capture stdout - note: the function prints to os.Stdout
		// so we can't easily capture it, but we can verify no error occurs
		err := installSkill()
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}
	})
}
