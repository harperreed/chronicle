// ABOUTME: Sync subcommand for Charm cloud integration
// ABOUTME: Provides status, link, unlink, and wipe commands (SSH key auth)
package cli

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/charm/client"
	"github.com/charmbracelet/charm/proto"
	"github.com/fatih/color"
	"github.com/harper/chronicle/internal/charm"
	"github.com/spf13/cobra"
)

var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Manage cloud sync for chronicle data",
	Long: `Sync your chronicle data securely to the cloud using Charm.

Authentication is automatic via SSH keys - no login required!

Commands:
  status  - Show sync status and Charm user ID
  link    - Link this device to another Charm account
  unlink  - Disconnect this device from Charm
  repair  - Repair database corruption
  reset   - Reset database to clean state
  wipe    - Completely wipe all data including cloud backups

Examples:
  chronicle sync status
  chronicle sync link
  chronicle sync repair --force`,
}

var syncStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show sync status",
	RunE: func(cmd *cobra.Command, args []string) error {
		// Get Charm client
		c, err := charm.GetClient()
		if err != nil {
			fmt.Printf("Charm:     not connected (%v)\n", err)
			fmt.Println("\nRun 'chronicle sync link' to connect to a Charm account.")
			return nil
		}

		// Get user ID
		id, err := c.ID()
		if err != nil {
			fmt.Printf("Charm:     error getting ID (%v)\n", err)
			return nil
		}

		fmt.Printf("Charm ID:  %s\n", id)
		fmt.Printf("Server:    %s\n", charm.GetCharmHost())

		if c.IsLinked() {
			color.Green("Status:    Connected and syncing")
		} else {
			color.Yellow("Status:    Not linked")
			fmt.Println("\nRun 'chronicle sync link' to link to a Charm account.")
		}

		return nil
	},
}

var syncLinkCmd = &cobra.Command{
	Use:   "link",
	Short: "Link this device to a Charm account",
	Long: `Link this device to an existing Charm account.

This will generate a link code that you can enter on another device
that's already linked to your Charm account.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cc, err := client.NewClientWithDefaults()
		if err != nil {
			return fmt.Errorf("failed to create Charm client: %w", err)
		}

		// Check if already linked
		if _, err := cc.ID(); err == nil {
			color.Green("Already linked to a Charm account!")
			fmt.Println("Run 'chronicle sync status' to see your account info.")
			return nil
		}

		fmt.Println("Generating link request...")
		fmt.Println("Enter this code on a device that's already linked to your Charm account.")

		lh := &linkHandler{}
		if err := cc.LinkGen(lh); err != nil {
			return fmt.Errorf("link failed: %w", err)
		}

		return nil
	},
}

var syncUnlinkCmd = &cobra.Command{
	Use:   "unlink",
	Short: "Disconnect this device from Charm",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("This will disconnect this device from your Charm account.")
		fmt.Println("Your local data will remain, but it won't sync anymore.")
		fmt.Print("\nType 'unlink' to confirm: ")

		reader := bufio.NewReader(os.Stdin)
		confirmation, _ := reader.ReadString('\n')
		confirmation = strings.TrimSpace(confirmation)

		if confirmation != "unlink" {
			fmt.Println("Aborted.")
			return nil
		}

		cc, err := client.NewClientWithDefaults()
		if err != nil {
			return fmt.Errorf("failed to create Charm client: %w", err)
		}

		// Get current auth keys and remove them
		keys, err := cc.AuthorizedKeysWithMetadata()
		if err != nil {
			return fmt.Errorf("failed to get authorized keys: %w", err)
		}

		// Find and remove the current device's key
		for _, key := range keys.Keys {
			if key.Key != "" {
				if err := cc.UnlinkAuthorizedKey(key.Key); err != nil {
					fmt.Printf("Warning: failed to unlink key: %v\n", err)
				}
			}
		}

		color.Green("\nDevice unlinked successfully")
		fmt.Println("Run 'chronicle sync link' to link to a different account.")

		return nil
	},
}

var (
	repairForce bool
)

var syncRepairCmd = &cobra.Command{
	Use:   "repair",
	Short: "Repair database corruption",
	Long: `Attempt to repair database corruption issues.

This runs SQLite integrity checks and repairs:
- WAL checkpoint
- Remove shared memory file
- Integrity check
- VACUUM

Use --force to attempt recovery and cloud reset if corruption persists.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("Repairing chronicle database...")

		// Call repair directly without opening the client
		// This works even when the database is too corrupted to open normally
		result, err := charm.RepairDB(repairForce)
		if err != nil {
			return fmt.Errorf("repair failed: %w", err)
		}

		// Display repair results with checkmarks
		if result.WalCheckpointed {
			color.Green("  ✓ WAL checkpointed")
		}
		if result.ShmRemoved {
			color.Green("  ✓ SHM file removed")
		}
		if result.IntegrityOK {
			color.Green("  ✓ Integrity check passed")
		} else {
			color.Red("  ✗ Integrity check failed")
		}
		if result.Vacuumed {
			color.Green("  ✓ Database vacuumed")
		}
		if result.RecoveryAttempted {
			color.Yellow("  ! Recovery attempted")
		}
		if result.ResetFromCloud {
			color.Green("  ✓ Reset from cloud")
		}

		fmt.Println()
		if result.IntegrityOK {
			color.Green("Repair complete.")
		} else if !repairForce {
			color.Yellow("Repair incomplete. Run with --force to attempt recovery and cloud reset.")
		} else {
			color.Red("Repair failed. Database may be unrecoverable.")
		}

		return nil
	},
}

var syncResetCmd = &cobra.Command{
	Use:   "reset",
	Short: "Reset database to clean state",
	Long: `Reset the local database to a clean state.

This will:
- Delete all local chronicle data
- Re-sync from Charm Cloud

Your cloud data will NOT be affected.
This works even when the database is corrupted.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("This will delete all local chronicle data and re-sync from cloud.")
		fmt.Print("Continue? [y/N]: ")

		reader := bufio.NewReader(os.Stdin)
		confirmation, _ := reader.ReadString('\n')
		confirmation = strings.TrimSpace(strings.ToLower(confirmation))

		if confirmation != "y" && confirmation != "yes" {
			fmt.Println("Aborted.")
			return nil
		}

		fmt.Println("\nResetting database...")
		// Call reset directly without opening the client
		// This works even when the database is too corrupted to open normally
		if err := charm.ResetDBFromCloud(); err != nil {
			return fmt.Errorf("reset failed: %w", err)
		}

		color.Green("Reset complete!")
		fmt.Println("Database has been reset and re-synced from cloud.")

		return nil
	},
}

var syncWipeCmd = &cobra.Command{
	Use:   "wipe",
	Short: "Completely wipe all data including cloud backups",
	Long: `Completely wipe all chronicle data from everywhere.

This will:
- Delete all local chronicle data
- Delete all cloud backups
- Remove data from all linked devices

THIS CANNOT BE UNDONE!`,
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := charm.GetClient()
		if err != nil {
			return fmt.Errorf("not connected to Charm: %w", err)
		}

		fmt.Println("This will DELETE all chronicle data from EVERYWHERE.")
		fmt.Println("This includes:")
		fmt.Println("  - All local data")
		fmt.Println("  - All cloud backups")
		fmt.Println("  - Data on all linked devices")
		fmt.Println("\nTHIS CANNOT BE UNDONE!")
		fmt.Print("\nType 'wipe' to confirm: ")

		reader := bufio.NewReader(os.Stdin)
		confirmation, _ := reader.ReadString('\n')
		confirmation = strings.TrimSpace(confirmation)

		if confirmation != "wipe" {
			fmt.Println("Aborted.")
			return nil
		}

		fmt.Println("\nWiping all chronicle data...")
		result, err := c.Wipe()
		if err != nil {
			return fmt.Errorf("wipe failed: %w", err)
		}

		fmt.Printf("Cloud backups deleted: %d\n", result.CloudBackupsDeleted)
		fmt.Printf("Local files deleted:   %d\n", result.LocalFilesDeleted)
		color.Green("Wipe complete!")
		fmt.Println("All chronicle data has been permanently deleted.")

		return nil
	},
}

func init() {
	// Add --force flag to repair command
	syncRepairCmd.Flags().BoolVarP(&repairForce, "force", "f", false, "Force repair even if database appears healthy")

	syncCmd.AddCommand(syncStatusCmd)
	syncCmd.AddCommand(syncLinkCmd)
	syncCmd.AddCommand(syncUnlinkCmd)
	syncCmd.AddCommand(syncRepairCmd)
	syncCmd.AddCommand(syncResetCmd)
	syncCmd.AddCommand(syncWipeCmd)

	rootCmd.AddCommand(syncCmd)
}

// linkHandler implements proto.LinkHandler for the link flow.
type linkHandler struct{}

func (lh *linkHandler) TokenCreated(l *proto.Link) {
	fmt.Printf("\nLink code: %s\n\n", l.Token)
	fmt.Println("Waiting for approval...")
}

func (lh *linkHandler) TokenSent(l *proto.Link) {
	// Token has been validated
}

func (lh *linkHandler) ValidToken(l *proto.Link) {
	// Linking complete
}

func (lh *linkHandler) InvalidToken(l *proto.Link) {
	fmt.Println("Invalid or expired token. Please try again.")
}

func (lh *linkHandler) Request(l *proto.Link) bool {
	fmt.Printf("\nLink request from: %s\n", l.RequestAddr)
	fmt.Print("Approve? [y/N]: ")

	reader := bufio.NewReader(os.Stdin)
	response, _ := reader.ReadString('\n')
	response = strings.TrimSpace(strings.ToLower(response))

	return response == "y" || response == "yes"
}

func (lh *linkHandler) RequestDenied(l *proto.Link) {
	fmt.Println("Link request denied.")
}

func (lh *linkHandler) SameUser(l *proto.Link) {
	color.Green("\nSuccessfully linked!")
}

func (lh *linkHandler) Success(l *proto.Link) {
	color.Green("\nSuccessfully linked!")
}

func (lh *linkHandler) Timeout(l *proto.Link) {
	fmt.Println("\nLink request timed out. Please try again.")
}

func (lh *linkHandler) Error(l *proto.Link) {
	fmt.Println("\nError during linking")
}
