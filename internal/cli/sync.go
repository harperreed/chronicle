// ABOUTME: Sync subcommand for vault integration
// ABOUTME: Provides init, login, status, now, pending, logout, and wipe commands
package cli

import (
	"bufio"
	"context"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/fatih/color"
	"github.com/harper/chronicle/internal/config"
	"github.com/harper/chronicle/internal/db"
	"github.com/harper/chronicle/internal/sync"
	"github.com/harperreed/sweet/vault"
	"github.com/oklog/ulid/v2"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Manage cloud sync for chronicle data",
	Long: `Sync your chronicle data securely to the cloud using E2E encryption.

Commands:
  init    - Initialize sync configuration
  login   - Login to sync server
  status  - Show sync status
  now     - Manually trigger sync
  pending - Show changes waiting to sync
  logout  - Clear authentication
  wipe    - Clear all sync data

Examples:
  chronicle sync init
  chronicle sync login --server https://api.storeusa.org
  chronicle sync status
  chronicle sync now`,
}

var syncInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize sync configuration",
	RunE: func(cmd *cobra.Command, args []string) error {
		if sync.ConfigExists() {
			return fmt.Errorf("config already exists at %s\nUse 'chronicle sync status' to view or delete the file to reinitialize", sync.ConfigPath())
		}

		cfg, err := sync.InitConfig()
		if err != nil {
			return fmt.Errorf("failed to initialize config: %w", err)
		}

		fmt.Println("Sync initialized")
		fmt.Printf("  Config: %s\n", sync.ConfigPath())
		fmt.Printf("  Device: %s\n", cfg.DeviceID)
		fmt.Println("\nNext: Run 'chronicle sync login' to authenticate")

		return nil
	},
}

var syncLoginCmd = &cobra.Command{
	Use:   "login",
	Short: "Login to sync server",
	Long: `Login to sync service with your credentials and recovery phrase.

Your recovery phrase is used to derive encryption keys - the server
never sees your data in plaintext.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		server, _ := cmd.Flags().GetString("server")

		cfg, _ := sync.LoadConfig()
		if cfg == nil {
			cfg = &sync.Config{}
		}

		serverURL := server
		if serverURL == "" {
			serverURL = cfg.Server
		}
		if serverURL == "" {
			serverURL = "https://api.storeusa.org"
		}

		reader := bufio.NewReader(os.Stdin)

		fmt.Print("Email: ")
		email, _ := reader.ReadString('\n')
		email = strings.TrimSpace(email)
		if email == "" {
			return fmt.Errorf("email required")
		}

		fmt.Print("Password: ")
		passwordBytes, err := term.ReadPassword(syscall.Stdin)
		fmt.Println()
		if err != nil {
			return fmt.Errorf("read password: %w", err)
		}
		password := string(passwordBytes)
		if password == "" {
			return fmt.Errorf("password cannot be empty")
		}

		fmt.Print("Recovery phrase (12 or 24 words): ")
		mnemonic, _ := reader.ReadString('\n')
		mnemonic = strings.TrimSpace(mnemonic)

		parsed, err := vault.ParseMnemonic(mnemonic)
		if err != nil {
			return fmt.Errorf("invalid recovery phrase: must be 12 or 24 words")
		}
		// Verify it's actually 12 or 24 words
		wordCount := len(strings.Fields(mnemonic))
		if wordCount != 12 && wordCount != 24 {
			return fmt.Errorf("invalid recovery phrase: must be 12 or 24 words")
		}
		_ = parsed

		// Ensure device ID exists BEFORE login (v0.3.0 requirement)
		if cfg.DeviceID == "" {
			cfg.DeviceID = ulid.Make().String()
		}

		fmt.Printf("\nLogging in to %s...\n", serverURL)
		client := vault.NewPBAuthClient(serverURL)

		// v0.3.0: Login now registers device at auth time
		result, err := client.Login(context.Background(), email, password, cfg.DeviceID)
		if err != nil {
			return fmt.Errorf("login failed: %w", err)
		}

		seed, err := vault.ParseSeedPhrase(mnemonic)
		if err != nil {
			return fmt.Errorf("parse mnemonic: %w", err)
		}
		derivedKeyHex := hex.EncodeToString(seed.Raw)

		cfg.Server = serverURL
		cfg.UserID = result.UserID
		cfg.Token = result.Token.Token
		cfg.RefreshToken = result.RefreshToken
		cfg.TokenExpires = result.Token.Expires.Format(time.RFC3339)
		cfg.DerivedKey = derivedKeyHex
		if cfg.VaultDB == "" {
			cfg.VaultDB = filepath.Join(sync.ConfigDir(), "vault.db")
		}

		if err := sync.SaveConfig(cfg); err != nil {
			return fmt.Errorf("save config: %w", err)
		}

		color.Green("\n✓ Logged in successfully")
		fmt.Printf("  User ID: %s\n", cfg.UserID)
		fmt.Printf("  Device: %s\n", cfg.DeviceID[:8]+"...")
		fmt.Printf("  Token expires: %s\n", result.Token.Expires.Format(time.RFC3339))

		// Sync immediately after login to pull existing data
		fmt.Println("\nSyncing existing data...")
		dataHome := config.GetDataHome()
		dbPath := filepath.Join(dataHome, "chronicle", "chronicle.db")
		appDB, err := db.InitDB(dbPath)
		if err != nil {
			fmt.Printf("Warning: Could not sync: %v\n", err)
			return nil
		}
		defer func() { _ = appDB.Close() }()

		syncer, err := sync.NewSyncer(cfg, appDB)
		if err != nil {
			fmt.Printf("Warning: Could not sync: %v\n", err)
			return nil
		}
		defer func() { _ = syncer.Close() }()

		ctx := context.Background()
		if err := syncer.Sync(ctx); err != nil {
			fmt.Printf("Warning: Sync failed: %v\n", err)
			return nil
		}

		fmt.Println("Sync complete")
		return nil
	},
}

var syncStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show sync status",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := sync.LoadConfig()
		if err != nil {
			return fmt.Errorf("load config: %w", err)
		}

		fmt.Printf("Config:    %s\n", sync.ConfigPath())
		fmt.Printf("Server:    %s\n", valueOrNone(cfg.Server))
		fmt.Printf("User ID:   %s\n", valueOrNone(cfg.UserID))
		fmt.Printf("Device ID: %s\n", valueOrNone(cfg.DeviceID))
		fmt.Printf("Vault DB:  %s\n", valueOrNone(cfg.VaultDB))

		if cfg.DerivedKey != "" {
			fmt.Println("Keys:      configured")
		} else {
			fmt.Println("Keys:      (not set)")
		}

		printTokenStatus(cfg)

		if cfg.IsConfigured() {
			dataHome := config.GetDataHome()
			dbPath := filepath.Join(dataHome, "chronicle", "chronicle.db")
			appDB, err := db.InitDB(dbPath)
			if err == nil {
				defer func() { _ = appDB.Close() }()

				syncer, err := sync.NewSyncer(cfg, appDB)
				if err == nil {
					defer func() { _ = syncer.Close() }()
					ctx := context.Background()

					pending, err := syncer.PendingCount(ctx)
					if err == nil {
						fmt.Printf("\nPending:   %d changes\n", pending)
					}

					lastSeq, err := syncer.LastSyncedSeq(ctx)
					if err == nil && lastSeq != "0" {
						fmt.Printf("Last sync: seq %s\n", lastSeq)
					}
				}
			}
		}

		return nil
	},
}

var syncNowCmd = &cobra.Command{
	Use:   "now",
	Short: "Manually trigger sync",
	RunE: func(cmd *cobra.Command, args []string) error {
		verbose, _ := cmd.Flags().GetBool("verbose")

		cfg, err := sync.LoadConfig()
		if err != nil {
			return fmt.Errorf("load config: %w", err)
		}

		if !cfg.IsConfigured() {
			return fmt.Errorf("sync not configured - run 'chronicle sync login' first")
		}

		dataHome := config.GetDataHome()
		dbPath := filepath.Join(dataHome, "chronicle", "chronicle.db")
		appDB, err := db.InitDB(dbPath)
		if err != nil {
			return fmt.Errorf("open database: %w", err)
		}
		defer func() { _ = appDB.Close() }()

		syncer, err := sync.NewSyncer(cfg, appDB)
		if err != nil {
			return fmt.Errorf("create syncer: %w", err)
		}
		defer func() { _ = syncer.Close() }()

		ctx := context.Background()

		var events *vault.SyncEvents
		if verbose {
			events = &vault.SyncEvents{
				OnStart: func() {
					fmt.Println("Syncing...")
				},
				OnPush: func(pushed, remaining int) {
					fmt.Printf("  Pushed %d changes (%d remaining)\n", pushed, remaining)
				},
				OnPull: func(pulled int) {
					if pulled > 0 {
						fmt.Printf("  Pulled %d changes\n", pulled)
					}
				},
				OnComplete: func(pushed, pulled int) {
					fmt.Printf("  Total: %d pushed, %d pulled\n", pushed, pulled)
				},
			}
		} else {
			fmt.Println("Syncing...")
		}

		if err := syncer.SyncWithEvents(ctx, events); err != nil {
			return fmt.Errorf("sync failed: %w", err)
		}

		fmt.Println("Sync complete")
		return nil
	},
}

var syncPendingCmd = &cobra.Command{
	Use:   "pending",
	Short: "Show changes waiting to sync",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := sync.LoadConfig()
		if err != nil {
			return fmt.Errorf("load config: %w", err)
		}

		if !cfg.IsConfigured() {
			fmt.Println("Sync not configured. Run 'chronicle sync login' first.")
			return nil
		}

		dataHome := config.GetDataHome()
		dbPath := filepath.Join(dataHome, "chronicle", "chronicle.db")
		appDB, err := db.InitDB(dbPath)
		if err != nil {
			return fmt.Errorf("open database: %w", err)
		}
		defer func() { _ = appDB.Close() }()

		syncer, err := sync.NewSyncer(cfg, appDB)
		if err != nil {
			return fmt.Errorf("create syncer: %w", err)
		}
		defer func() { _ = syncer.Close() }()

		items, err := syncer.PendingChanges(context.Background())
		if err != nil {
			return fmt.Errorf("get pending: %w", err)
		}

		if len(items) == 0 {
			fmt.Println("No pending changes - everything is synced!")
			return nil
		}

		fmt.Printf("Pending changes (%d):\n\n", len(items))
		for _, item := range items {
			fmt.Printf("  %s  %-10s  %s\n",
				item.ChangeID[:8],
				item.Entity,
				item.TS.Format("2006-01-02 15:04:05"))
		}
		fmt.Printf("\nRun 'chronicle sync now' to push these changes.\n")

		return nil
	},
}

var syncLogoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "Clear authentication",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := sync.LoadConfig()
		if err != nil {
			return fmt.Errorf("load config: %w", err)
		}

		if cfg.Token == "" {
			fmt.Println("Not logged in")
			return nil
		}

		cfg.Token = ""
		cfg.RefreshToken = ""
		cfg.TokenExpires = ""

		if err := sync.SaveConfig(cfg); err != nil {
			return fmt.Errorf("save config: %w", err)
		}

		fmt.Println("Logged out successfully")
		return nil
	},
}

var syncWipeCmd = &cobra.Command{
	Use:   "wipe",
	Short: "Wipe all sync data and start fresh",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := sync.LoadConfig()
		if err != nil {
			return fmt.Errorf("load config: %w", err)
		}

		if !cfg.IsConfigured() {
			return fmt.Errorf("sync not configured - run 'chronicle sync login' first")
		}

		fmt.Println("This will DELETE all sync data on the server and locally.")
		fmt.Println("Your local chronicle data will NOT be affected.")
		fmt.Print("\nType 'wipe' to confirm: ")

		reader := bufio.NewReader(os.Stdin)
		confirmation, _ := reader.ReadString('\n')
		confirmation = strings.TrimSpace(confirmation)

		if confirmation != "wipe" {
			fmt.Println("Aborted.")
			return nil
		}

		fmt.Println("\nWiping server data...")
		client := vault.NewClient(vault.SyncConfig{
			BaseURL:   cfg.Server,
			DeviceID:  cfg.DeviceID,
			AuthToken: cfg.Token,
		})

		ctx := context.Background()
		deleted, err := client.Wipe(ctx)
		if err != nil {
			return fmt.Errorf("wipe server data: %w", err)
		}
		fmt.Printf("Server data wiped (%d records deleted)\n", deleted)

		fmt.Println("Removing local vault database...")
		if err := os.Remove(cfg.VaultDB); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove vault.db: %w", err)
		}
		fmt.Println("Local vault.db removed")

		fmt.Println("\nSync data cleared. Run 'chronicle sync now' to re-push local data.")
		return nil
	},
}

func init() {
	syncLoginCmd.Flags().String("server", "", "sync server URL (default: https://api.storeusa.org)")
	syncNowCmd.Flags().BoolP("verbose", "v", false, "show detailed sync information")

	syncCmd.AddCommand(syncInitCmd)
	syncCmd.AddCommand(syncLoginCmd)
	syncCmd.AddCommand(syncStatusCmd)
	syncCmd.AddCommand(syncNowCmd)
	syncCmd.AddCommand(syncPendingCmd)
	syncCmd.AddCommand(syncLogoutCmd)
	syncCmd.AddCommand(syncWipeCmd)

	rootCmd.AddCommand(syncCmd)
}

func valueOrNone(s string) string {
	if s == "" {
		return "(not set)"
	}
	return s
}

func printTokenStatus(cfg *sync.Config) {
	if cfg.Token == "" {
		fmt.Println("\nStatus:    Not logged in")
		return
	}

	fmt.Println()
	if cfg.TokenExpires == "" {
		fmt.Println("Token:     valid (no expiry info)")
		return
	}

	expires, err := time.Parse(time.RFC3339, cfg.TokenExpires)
	if err != nil {
		fmt.Printf("Token:     valid (invalid expiry: %v)\n", err)
		return
	}

	now := time.Now()
	if expires.Before(now) {
		fmt.Printf("Token:     EXPIRED (%s ago)\n", now.Sub(expires).Round(time.Second))
		if cfg.RefreshToken != "" {
			fmt.Println("           (has refresh token - run 'chronicle sync now' to refresh)")
		}
	} else {
		fmt.Printf("Token:     valid (expires in %s)\n", formatDuration(expires.Sub(now)))
	}
}

func formatDuration(d time.Duration) string {
	d = d.Round(time.Second)
	h := d / time.Hour
	d -= h * time.Hour
	m := d / time.Minute
	d -= m * time.Minute
	s := d / time.Second

	if h > 0 {
		return fmt.Sprintf("%dh %dm", h, m)
	}
	if m > 0 {
		return fmt.Sprintf("%dm %ds", m, s)
	}
	return fmt.Sprintf("%ds", s)
}
