// ABOUTME: Install Claude Code skill for chronicle
// ABOUTME: Embeds and installs the skill definition to ~/.claude/skills/

package cli

import (
	"bufio"
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

//go:embed skill/SKILL.md
var skillFS embed.FS

var skillSkipConfirm bool

var installSkillCmd = &cobra.Command{
	Use:   "install-skill",
	Short: "Install Claude Code skill",
	Long: `Install the chronicle skill for Claude Code.

This copies the skill definition to ~/.claude/skills/chronicle/
so Claude Code can use chronicle commands contextually.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return installSkill()
	},
}

func init() {
	installSkillCmd.Flags().BoolVarP(&skillSkipConfirm, "yes", "y", false, "Skip confirmation prompt")
	rootCmd.AddCommand(installSkillCmd)
}

func installSkill() error {
	// Determine destination
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}

	skillDir := filepath.Join(home, ".claude", "skills", "chronicle")
	skillPath := filepath.Join(skillDir, "SKILL.md")

	// Show explanation
	fmt.Println("┌─────────────────────────────────────────────────────────────┐")
	fmt.Println("│            Chronicle Skill for Claude Code                  │")
	fmt.Println("└─────────────────────────────────────────────────────────────┘")
	fmt.Println()
	fmt.Println("This will install the chronicle skill, enabling Claude Code to:")
	fmt.Println()
	fmt.Println("  • Log activities and accomplishments")
	fmt.Println("  • Track what you did and when")
	fmt.Println("  • Search your activity history")
	fmt.Println("  • Use the /chronicle slash command")
	fmt.Println()
	fmt.Println("Destination:")
	fmt.Printf("  %s\n", skillPath)
	fmt.Println()

	// Check if already installed
	if _, err := os.Stat(skillPath); err == nil {
		fmt.Println("Note: A skill file already exists and will be overwritten.")
		fmt.Println()
	}

	// Ask for confirmation unless --yes flag is set
	if !skillSkipConfirm {
		fmt.Print("Install the chronicle skill? [y/N] ")
		reader := bufio.NewReader(os.Stdin)
		response, err := reader.ReadString('\n')
		if err != nil {
			return fmt.Errorf("failed to read response: %w", err)
		}
		response = strings.TrimSpace(strings.ToLower(response))
		if response != "y" && response != "yes" {
			fmt.Println("Installation canceled.")
			return nil
		}
		fmt.Println()
	}

	// Read embedded skill file
	content, err := skillFS.ReadFile("skill/SKILL.md")
	if err != nil {
		return fmt.Errorf("failed to read embedded skill: %w", err)
	}

	// Create directory
	if err := os.MkdirAll(skillDir, 0750); err != nil {
		return fmt.Errorf("failed to create skill directory: %w", err)
	}

	// Write skill file
	if err := os.WriteFile(skillPath, content, 0600); err != nil {
		return fmt.Errorf("failed to write skill file: %w", err)
	}

	fmt.Println("✓ Installed chronicle skill successfully!")
	fmt.Println()
	fmt.Println("Claude Code will now recognize /chronicle commands.")
	fmt.Println("Try asking Claude: \"Log that I deployed the new API\" or \"What did I do yesterday?\"")
	return nil
}
