// ABOUTME: Charm KV client wrapper for chronicle data storage
// ABOUTME: Provides thread-safe initialization and automatic cloud sync via SSH keys
package charm

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"

	"github.com/charmbracelet/charm/client"
	"github.com/charmbracelet/charm/kv"
)

const (
	// EntryPrefix is the key prefix for chronicle entries.
	EntryPrefix = "entry:"

	// DBName is the KV database name for chronicle.
	DBName = "chronicle"
)

var (
	globalClient *Client
	clientOnce   sync.Once
	clientErr    error
)

// Config holds client configuration.
type Config struct {
	AutoSync bool `json:"auto_sync"`
}

// DefaultConfig returns the default configuration.
func DefaultConfig() *Config {
	return &Config{
		AutoSync: true,
	}
}

// Client wraps Charm KV for chronicle operations.
type Client struct {
	kv     *kv.KV
	cc     *client.Client
	config *Config
	mu     sync.RWMutex
}

// InitClient initializes the global client (thread-safe, idempotent).
func InitClient() error {
	clientOnce.Do(func() {
		globalClient, clientErr = NewClient(DefaultConfig())
	})
	return clientErr
}

// GetClient returns the global client, initializing if needed.
func GetClient() (*Client, error) {
	if err := InitClient(); err != nil {
		return nil, err
	}
	return globalClient, nil
}

// NewClient creates a new Charm client.
func NewClient(cfg *Config) (*Client, error) {
	if cfg == nil {
		cfg = DefaultConfig()
	}

	// Open KV with defaults (uses CHARM_HOST env or charm.2389.dev)
	// Falls back to read-only mode if another process holds the lock
	db, err := kv.OpenWithDefaultsFallback(DBName)
	if err != nil {
		return nil, fmt.Errorf("open charm kv: %w", err)
	}

	c := &Client{
		kv:     db,
		cc:     db.Client(),
		config: cfg,
	}

	// Pull remote data on startup (skip in read-only mode)
	if cfg.AutoSync && !db.IsReadOnly() {
		_ = db.Sync()
	}

	return c, nil
}

// Close releases client resources.
func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.kv != nil {
		return c.kv.Close()
	}
	return nil
}

// ID returns the Charm user ID.
func (c *Client) ID() (string, error) {
	return c.cc.ID()
}

// IsReadOnly returns true if the database is open in read-only mode.
// This happens when another process (like an MCP server) holds the lock.
func (c *Client) IsReadOnly() bool {
	return c.kv.IsReadOnly()
}

// Sync syncs with the Charm Cloud.
// Skips sync if in read-only mode.
func (c *Client) Sync() error {
	if c.IsReadOnly() {
		return nil
	}
	return c.kv.Sync()
}

// syncIfEnabled syncs if auto-sync is enabled and not in read-only mode.
func (c *Client) syncIfEnabled() {
	if c.config.AutoSync && !c.IsReadOnly() {
		_ = c.kv.Sync()
	}
}

// Reset wipes all local data and re-syncs from cloud.
func (c *Client) Reset() error {
	return c.kv.Reset()
}

// Repair attempts to repair database corruption.
func (c *Client) Repair(force bool) (*kv.RepairResult, error) {
	return kv.Repair(DBName, force)
}

// ResetDB resets the database to a clean state.
func (c *Client) ResetDB() error {
	return kv.Reset(DBName)
}

// Wipe completely wipes all data including cloud backups.
func (c *Client) Wipe() (*kv.WipeResult, error) {
	return kv.Wipe(DBName)
}

// Set stores a key-value pair.
func (c *Client) Set(key, value []byte) error {
	if c.IsReadOnly() {
		return fmt.Errorf("cannot write: database is locked by another process (MCP server?)")
	}
	if err := c.kv.Set(key, value); err != nil {
		return err
	}
	c.syncIfEnabled()
	return nil
}

// Get retrieves a value by key.
func (c *Client) Get(key []byte) ([]byte, error) {
	return c.kv.Get(key)
}

// Delete removes a key.
func (c *Client) Delete(key []byte) error {
	if c.IsReadOnly() {
		return fmt.Errorf("cannot write: database is locked by another process (MCP server?)")
	}
	if err := c.kv.Delete(key); err != nil {
		return err
	}
	c.syncIfEnabled()
	return nil
}

// SetJSON stores a JSON-serialized value.
func (c *Client) SetJSON(key []byte, value any) error {
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}
	return c.Set(key, data)
}

// GetJSON retrieves and unmarshals a JSON value.
func (c *Client) GetJSON(key []byte, dest any) error {
	data, err := c.Get(key)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, dest)
}

// IsLinked returns true if this device is linked to a Charm account.
func (c *Client) IsLinked() bool {
	_, err := c.cc.ID()
	return err == nil
}

// GetCharmHost returns the configured Charm host.
func GetCharmHost() string {
	if host := os.Getenv("CHARM_HOST"); host != "" {
		return host
	}
	return "charm.2389.dev"
}

// RepairDB attempts to repair a corrupted database without opening it.
// This can be called even when the database is too corrupted to open normally.
func RepairDB(force bool) (*kv.RepairResult, error) {
	return kv.Repair(DBName, force)
}

// ResetDBFromCloud resets the database without requiring an open client.
// This deletes local data and re-syncs from cloud.
func ResetDBFromCloud() error {
	return kv.Reset(DBName)
}
