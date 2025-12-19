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
	"github.com/dgraph-io/badger/v3"
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
	db, err := kv.OpenWithDefaults(DBName)
	if err != nil {
		return nil, fmt.Errorf("open charm kv: %w", err)
	}

	c := &Client{
		kv:     db,
		cc:     db.Client(),
		config: cfg,
	}

	// Pull remote data on startup
	if cfg.AutoSync {
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

// Sync syncs with the Charm Cloud.
func (c *Client) Sync() error {
	return c.kv.Sync()
}

// syncIfEnabled syncs if auto-sync is enabled.
func (c *Client) syncIfEnabled() {
	if c.config.AutoSync {
		_ = c.kv.Sync()
	}
}

// Reset wipes all local data and re-syncs from cloud.
func (c *Client) Reset() error {
	return c.kv.Reset()
}

// Set stores a key-value pair.
func (c *Client) Set(key, value []byte) error {
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
	if err := c.kv.Delete(key); err != nil {
		return err
	}
	c.syncIfEnabled()
	return nil
}

// View executes a read-only transaction.
func (c *Client) View(fn func(txn *badger.Txn) error) error {
	return c.kv.View(fn)
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
