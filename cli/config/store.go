package config

import (
	"context"
)

// Store saves and loads config.
type Store interface {
	// Save writes config to the store.
	Save(ctx context.Context, cfg *Config) error

	// Load reads config from the store.
	Load(ctx context.Context) (*Config, error)

	// Location returns the location of the store, typically
	// a file path.
	Location() string
}

// DiscardStore implements Store but its Save method is no-op
// and Load always returns a new empty Config. Useful for testing.
type DiscardStore struct{}

var _ Store = (*DiscardStore)(nil)

// Load returns a new empty Config.
func (DiscardStore) Load(context.Context) (*Config, error) {
	return New(), nil
}

// Save is no-op.
func (DiscardStore) Save(context.Context, *Config) error {
	return nil
}

// Location returns /dev/null.
func (DiscardStore) Location() string {
	return "/dev/null"
}
