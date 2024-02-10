package config

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/ioz/lockfile"
	"github.com/neilotoole/sq/libsq/core/options"
)

// Store saves and loads config.
type Store interface {
	// Exists returns true if the config exists in the store.
	Exists() bool

	// Save writes config to the store.
	Save(ctx context.Context, cfg *Config) error

	// Load reads config from the store.
	Load(ctx context.Context) (*Config, error)

	// Location returns the location of the store, typically
	// a file path.
	Location() string

	// Lockfile returns the lockfile used by the store, but does not acquire
	// the lock, which is the caller's responsibility. The lock should always
	// be acquired before mutating config. It is also the caller's responsibility
	// to release the acquired lock when done.
	Lockfile() (lockfile.Lockfile, error)
}

// DiscardStore implements Store but its Save method is no-op
// and Load always returns a new empty Config. Useful for testing.
type DiscardStore struct{}

// Exists implements Store.Exists. It returns true.
func (s DiscardStore) Exists() bool {
	return true
}

// Lockfile implements Store.Lockfile.
func (DiscardStore) Lockfile() (lockfile.Lockfile, error) {
	f, err := os.CreateTemp("", fmt.Sprintf("sq-%d.lock", os.Getpid()))
	if err != nil {
		return "", errz.Err(err)
	}
	fname := f.Name()
	if err = f.Close(); err != nil {
		return "", errz.Err(err)
	}
	return lockfile.Lockfile(fname), nil
}

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

// OptConfigLockTimeout is the time allowed to acquire the config lock.
var OptConfigLockTimeout = options.NewDuration(
	"config.lock.timeout",
	nil,
	time.Second*5,
	"Wait timeout to acquire config lock",
	`Wait timeout to acquire the config lock (which prevents multiple sq instances
stepping on each other's config changes). During this period, retry will occur
if the lock is already held by another process. If zero, no retry occurs.`,
)
