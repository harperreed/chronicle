// ABOUTME: Sync subcommand for local storage management
// ABOUTME: Provides status, repair (vacuum), reset, and wipe commands for any backend
package cli

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/fatih/color"
	"github.com/harper/chronicle/internal/config"
	"github.com/spf13/cobra"
)

var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Manage local chronicle database",
	Long: `Manage your local chronicle database.

Commands:
  status  - Show database status and entry count
  repair  - Optimize database (vacuum)
  reset   - Clear all entries
  wipe    - Delete database file completely

Examples:
  chronicle sync status
  chronicle sync repair`,
}

var syncStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show database status",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		fmt.Printf("Backend:   %s\n", cfg.GetBackend())
		fmt.Printf("Data dir:  %s\n", cfg.GetDataDir())

		// Open store and get stats
		store, err := cfg.OpenStorage()
		if err != nil {
			fmt.Printf("Status:    Error (%v)\n", err)
			return nil
		}
		defer func() { _ = store.Close() }()

		// Count entries
		entries, err := store.ListEntries(0) // 0 = no limit
		if err != nil {
			fmt.Printf("Status:    Error reading entries (%v)\n", err)
			return nil
		}

		color.Green("Status:    OK")
		fmt.Printf("Entries:   %d\n", len(entries))
		fmt.Printf("Modified:  %s\n", store.LastModified().Format("2006-01-02 15:04:05"))

		return nil
	},
}

var syncRepairCmd = &cobra.Command{
	Use:   "repair",
	Short: "Optimize database",
	Long: `Optimize the chronicle database.

This runs SQLite VACUUM to:
- Reclaim unused space
- Defragment the database
- Rebuild indices`,
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("Optimizing chronicle storage...")

		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		store, err := cfg.OpenStorage()
		if err != nil {
			return fmt.Errorf("failed to open storage: %w", err)
		}
		defer func() { _ = store.Close() }()

		if err := store.Vacuum(); err != nil {
			return fmt.Errorf("vacuum failed: %w", err)
		}

		color.Green("Storage optimized successfully.")
		return nil
	},
}

var syncResetCmd = &cobra.Command{
	Use:   "reset",
	Short: "Clear all entries",
	Long: `Clear all entries from the database.

This will delete all chronicle entries but keep the database file.
This cannot be undone!`,
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("This will delete all chronicle entries.")
		fmt.Print("Continue? [y/N]: ")

		reader := bufio.NewReader(os.Stdin)
		confirmation, _ := reader.ReadString('\n')
		confirmation = strings.TrimSpace(strings.ToLower(confirmation))

		if confirmation != "y" && confirmation != "yes" {
			fmt.Println("Aborted.")
			return nil
		}

		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		store, err := cfg.OpenStorage()
		if err != nil {
			return fmt.Errorf("failed to open storage: %w", err)
		}
		defer func() { _ = store.Close() }()

		if err := store.Reset(); err != nil {
			return fmt.Errorf("reset failed: %w", err)
		}

		color.Green("All entries deleted.")
		return nil
	},
}

var syncWipeCmd = &cobra.Command{
	Use:   "wipe",
	Short: "Delete database file completely",
	Long: `Completely delete the chronicle database file.

This will:
- Delete the database file
- Delete any WAL/SHM files

THIS CANNOT BE UNDONE!`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		dataDir := cfg.GetDataDir()
		backend := cfg.GetBackend()

		fmt.Println("This will DELETE all chronicle data completely.")
		fmt.Printf("Backend:  %s\n", backend)
		fmt.Printf("Data dir: %s\n", dataDir)
		fmt.Println("\nTHIS CANNOT BE UNDONE!")
		fmt.Print("\nType 'wipe' to confirm: ")

		reader := bufio.NewReader(os.Stdin)
		confirmation, _ := reader.ReadString('\n')
		confirmation = strings.TrimSpace(confirmation)

		if confirmation != "wipe" {
			fmt.Println("Aborted.")
			return nil
		}

		deleted := 0
		if backend == "sqlite" {
			// Remove database and related files
			dbPath := config.DefaultDBPath(dataDir)
			files := []string{
				dbPath,
				dbPath + "-wal",
				dbPath + "-shm",
			}
			for _, f := range files {
				if err := os.Remove(f); err == nil {
					deleted++
				}
			}
		} else {
			// Remove all data directory contents for markdown backend
			entries, readErr := os.ReadDir(dataDir)
			if readErr == nil {
				for _, entry := range entries {
					path := filepath.Join(dataDir, entry.Name())
					if err := os.RemoveAll(path); err == nil {
						deleted++
					}
				}
			}
		}

		color.Green("Data wiped!")
		fmt.Printf("Deleted %d item(s).\n", deleted)
		return nil
	},
}

func init() {
	syncCmd.AddCommand(syncStatusCmd)
	syncCmd.AddCommand(syncRepairCmd)
	syncCmd.AddCommand(syncResetCmd)
	syncCmd.AddCommand(syncWipeCmd)

	rootCmd.AddCommand(syncCmd)
}
