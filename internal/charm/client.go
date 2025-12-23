// ABOUTME: Charm KV client wrapper using transactional Do API
// ABOUTME: Short-lived connections to avoid lock contention with other MCP servers

package charm

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/charmbracelet/charm/client"
	"github.com/charmbracelet/charm/kv"
	charmproto "github.com/charmbracelet/charm/proto"
)

const (
	// EntryPrefix is the key prefix for chronicle entries.
	EntryPrefix = "entry:"

	// DBName is the KV database name for chronicle.
	DBName = "chronicle"
)

// Client holds configuration for KV operations.
// Unlike the previous implementation, it does NOT hold a persistent connection.
// Each operation opens the database, performs the operation, and closes it.
type Client struct {
	dbName   string
	autoSync bool
}

// Option configures a Client.
type Option func(*Client)

// WithDBName sets the database name.
func WithDBName(name string) Option {
	return func(c *Client) {
		c.dbName = name
	}
}

// WithAutoSync enables or disables auto-sync after writes.
func WithAutoSync(enabled bool) Option {
	return func(c *Client) {
		c.autoSync = enabled
	}
}

// NewClient creates a new client with the given options.
func NewClient(cfg *Config) (*Client, error) {
	if cfg == nil {
		var err error
		cfg, err = LoadConfig()
		if err != nil {
			return nil, err
		}
	}

	// Set charm host if configured
	if cfg.CharmHost != "" {
		if err := os.Setenv("CHARM_HOST", cfg.CharmHost); err != nil {
			return nil, err
		}
	}

	c := &Client{
		dbName:   DBName,
		autoSync: cfg.AutoSync,
	}
	return c, nil
}

// Get retrieves a value by key (read-only, no lock contention).
func (c *Client) Get(key []byte) ([]byte, error) {
	var val []byte
	err := kv.DoReadOnly(c.dbName, func(k *kv.KV) error {
		var err error
		val, err = k.Get(key)
		return err
	})
	return val, err
}

// Set stores a value with the given key.
func (c *Client) Set(key, value []byte) error {
	return kv.Do(c.dbName, func(k *kv.KV) error {
		if err := k.Set(key, value); err != nil {
			return err
		}
		if c.autoSync {
			return k.Sync()
		}
		return nil
	})
}

// Delete removes a key.
func (c *Client) Delete(key []byte) error {
	return kv.Do(c.dbName, func(k *kv.KV) error {
		if err := k.Delete(key); err != nil {
			return err
		}
		if c.autoSync {
			return k.Sync()
		}
		return nil
	})
}

// Keys returns all keys in the database.
func (c *Client) Keys() ([][]byte, error) {
	var keys [][]byte
	err := kv.DoReadOnly(c.dbName, func(k *kv.KV) error {
		var err error
		keys, err = k.Keys()
		return err
	})
	return keys, err
}

// DoReadOnly executes a function with read-only database access.
// Use this for batch read operations that need multiple Gets.
func (c *Client) DoReadOnly(fn func(k *kv.KV) error) error {
	return kv.DoReadOnly(c.dbName, fn)
}

// Do executes a function with write access to the database.
// Use this for batch write operations.
func (c *Client) Do(fn func(k *kv.KV) error) error {
	return kv.Do(c.dbName, func(k *kv.KV) error {
		if err := fn(k); err != nil {
			return err
		}
		if c.autoSync {
			return k.Sync()
		}
		return nil
	})
}

// Sync triggers a manual sync with the charm server.
func (c *Client) Sync() error {
	return kv.Do(c.dbName, func(k *kv.KV) error {
		return k.Sync()
	})
}

// Reset clears all data (nuclear option).
func (c *Client) Reset() error {
	return kv.Do(c.dbName, func(k *kv.KV) error {
		return k.Reset()
	})
}

// ID returns the charm user ID for this device.
func (c *Client) ID() (string, error) {
	cc, err := client.NewClientWithDefaults()
	if err != nil {
		return "", err
	}
	return cc.ID()
}

// User returns the current charm user information.
func (c *Client) User() (*charmproto.User, error) {
	cc, err := client.NewClientWithDefaults()
	if err != nil {
		return nil, err
	}
	return cc.Bio()
}

// Link initiates the charm linking process for this device.
func (c *Client) Link() error {
	cc, err := client.NewClientWithDefaults()
	if err != nil {
		return err
	}
	_, err = cc.Bio()
	return err
}

// Unlink removes the charm account association from this device.
func (c *Client) Unlink() error {
	return c.Reset()
}

// Config returns the current configuration.
func (c *Client) Config() *Config {
	cfg, _ := LoadConfig()
	return cfg
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

// --- Legacy compatibility layer ---
// These functions maintain backwards compatibility with existing code.

var globalClient *Client

// InitClient initializes the global charm client.
// With the new architecture, this just creates a Client instance.
func InitClient() error {
	if globalClient != nil {
		return nil
	}
	var err error
	globalClient, err = NewClient(nil)
	return err
}

// GetClient returns the global client, initializing if needed.
func GetClient() (*Client, error) {
	if err := InitClient(); err != nil {
		return nil, err
	}
	return globalClient, nil
}

// ResetClient resets the global client singleton.
func ResetClient() error {
	globalClient = nil
	return nil
}

// Close is a no-op for backwards compatibility.
// With Do API, connections are automatically closed after each operation.
func (c *Client) Close() error {
	return nil
}

// IsLinked returns true if this device is linked to a Charm account.
func (c *Client) IsLinked() bool {
	_, err := c.ID()
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
