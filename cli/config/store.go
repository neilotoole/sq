package config

import (
	"context"
	"io"
	"os"
	"path/filepath"

	"github.com/neilotoole/sq/cli/flag"

	"github.com/spf13/pflag"

	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/source"
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

// DefaultLoad loads sq config from the default location
// (~/.config/sq/sq.yml) or the location specified in envars.
func DefaultLoad(ctx context.Context, osArgs []string) (*Config, Store, error) {
	var (
		cfgDir string
		origin string
		ok     bool
		err    error
	)

	if cfgDir, ok, _ = getConfigDirFromFlag(osArgs); ok {
		origin = originFlag
	} else if cfgDir, ok = getConfigDirFromEnv(); ok {
		origin = originEnv
	} else {
		origin = originDefault
		if cfgDir, err = getDefaultConfigDir(); err != nil {
			return nil, nil, err
		}
	}

	cfgPath := filepath.Join(cfgDir, "sq.yml")
	extDir := filepath.Join(cfgDir, "ext")
	cfgStore := &YAMLFileStore{
		Path:       cfgPath,
		PathOrigin: origin,
		ExtPaths:   []string{extDir},
	}

	if !cfgStore.FileExists() {
		cfg := New()
		return cfg, cfgStore, nil
	}

	// file does exist, let's try to load it
	cfg, err := cfgStore.Load(ctx)
	if err != nil {
		return nil, nil, err
	}

	if _, err = source.VerifyIntegrity(cfg.Collection); err != nil {
		return nil, nil, err
	}

	return cfg, cfgStore, nil
}

// getConfigDirFromFlag parses osArgs looking for flag.ConfigUsage.
// We need to do manual flag parsing because config is loaded before
// cobra is initialized.
func getConfigDirFromFlag(osArgs []string) (dir string, ok bool, err error) {
	fs := pflag.NewFlagSet("bootstrap", pflag.ContinueOnError)
	fs.ParseErrorsWhitelist.UnknownFlags = true
	fs.SetOutput(io.Discard)

	_ = fs.String(flag.Config, "", flag.ConfigUsage)
	if err = fs.Parse(osArgs); err != nil {
		return "", false, errz.Err(err)
	}

	if !fs.Changed(flag.Config) {
		return "", false, nil
	}

	if dir, err = fs.GetString(flag.Config); err != nil {
		return "", false, errz.Err(err)
	}

	if dir == "" {
		return "", false, nil
	}

	return dir, true, nil
}

// getDefaultConfigDir returns "~/.config/sq".
func getDefaultConfigDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		// TODO: we should be able to run without the homedir... revisit this
		return "", errz.Wrap(err, "unable to get user home dir")
	}

	cfgDir := filepath.Join(home, ".config", "sq")
	return cfgDir, nil
}

func getConfigDirFromEnv() (string, bool) {
	if cfgDir, ok := os.LookupEnv(EnvarConfig); ok && cfgDir != "" {
		return cfgDir, ok
	}

	if cfgDir, ok := os.LookupEnv(EnvarConfigDir); ok && cfgDir != "" {
		return cfgDir, ok
	}

	return "", false
}

// Origin of the config path.
const (
	originFlag    = "flag"
	originEnv     = "env"
	originDefault = "default"
)
