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
  wipe    - Clear all sync data and start fresh

Examples:
  chronicle sync status
  chronicle sync link`,
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

var syncWipeCmd = &cobra.Command{
	Use:   "wipe",
	Short: "Wipe all sync data and start fresh",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := charm.GetClient()
		if err != nil {
			return fmt.Errorf("not connected to Charm: %w", err)
		}

		fmt.Println("This will DELETE all chronicle data from Charm Cloud.")
		fmt.Println("Your entries will be wiped from all linked devices.")
		fmt.Print("\nType 'wipe' to confirm: ")

		reader := bufio.NewReader(os.Stdin)
		confirmation, _ := reader.ReadString('\n')
		confirmation = strings.TrimSpace(confirmation)

		if confirmation != "wipe" {
			fmt.Println("Aborted.")
			return nil
		}

		fmt.Println("\nWiping all chronicle data...")
		if err := c.Reset(); err != nil {
			return fmt.Errorf("wipe failed: %w", err)
		}

		color.Green("Wipe complete!")
		fmt.Println("All chronicle entries have been deleted.")

		return nil
	},
}

func init() {
	syncCmd.AddCommand(syncStatusCmd)
	syncCmd.AddCommand(syncLinkCmd)
	syncCmd.AddCommand(syncUnlinkCmd)
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
