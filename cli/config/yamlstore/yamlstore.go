package yamlstore

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/neilotoole/sq/cli/buildinfo"
	"github.com/neilotoole/sq/cli/config"
	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/ioz"
	"github.com/neilotoole/sq/libsq/core/lg"
	"github.com/neilotoole/sq/libsq/core/lg/lga"
	"github.com/neilotoole/sq/libsq/core/options"
	"github.com/neilotoole/sq/libsq/source"
)

// Origin of the config path.
// See Store.PathOrigin.
const (
	originFlag    = "flag"
	originEnv     = "env"
	originDefault = "default"
)

// Store provides persistence of config via YAML file.
// It implements config.Store.
type Store struct {
	// Path is the location of the config file
	Path string

	// PathOrigin is one of "flag", "env", or "default".
	PathOrigin string

	// If HookLoad is non-nil, it is invoked by Load
	// on Path's bytes before the YAML is unmarshalled.
	// This allows expansion of variables etc.
	HookLoad func(data []byte) ([]byte, error)

	// ExtPaths holds locations of potential ext config, both dirs and files (with suffix ".sq.yml")
	ExtPaths []string

	// UpgradeRegistry holds upgrade funcs for upgrading the config file.
	UpgradeRegistry UpgradeRegistry
}

// String returns a log/debug-friendly representation.
func (fs *Store) String() string {
	return fmt.Sprintf("config via %s: %v", fs.PathOrigin, fs.Path)
}

// Location implements Store. It returns the location of the config dir.
func (fs *Store) Location() string {
	return filepath.Dir(fs.Path)
}

// Load reads config from disk. It implements Store.
func (fs *Store) Load(ctx context.Context) (*config.Config, error) {
	log := lg.FromContext(ctx)
	log.Debug("Loading config from file", lga.Path, fs.Path)

	if fs.UpgradeRegistry != nil {
		mightNeedUpgrade, foundVers, err := checkNeedsUpgrade(fs.Path)
		if err != nil {
			return nil, errz.Wrapf(err, "config: %s", fs.Path)
		}

		if mightNeedUpgrade {
			log.Info("Upgrade config?", lga.From, foundVers, lga.To, buildinfo.Version)
			if _, err = fs.doUpgrade(ctx, foundVers, buildinfo.Version); err != nil {
				return nil, err
			}

			// We do a cycle of loading and saving the config after the upgrade,
			// because the upgrade may have written YAML via a map, which
			// doesn't preserve order. Loading and saving should fix that.
			cfg, err := fs.doLoad(ctx)
			if err != nil {
				return nil, errz.Wrapf(err, "config: %s: load failed after config upgrade", fs.Path)
			}

			if err = fs.Save(ctx, cfg); err != nil {
				return nil, errz.Wrapf(err, "config: %s: save failed after config upgrade", fs.Path)
			}
		}
	}

	return fs.doLoad(ctx)
}

func (fs *Store) doLoad(ctx context.Context) (*config.Config, error) {
	bytes, err := os.ReadFile(fs.Path)
	if err != nil {
		return nil, errz.Wrapf(err, "config: failed to load file: %s", fs.Path)
	}

	loadHookFn := fs.HookLoad
	if loadHookFn != nil {
		bytes, err = loadHookFn(bytes)
		if err != nil {
			return nil, err
		}
	}

	cfg := config.New()
	err = ioz.UnmarshallYAML(bytes, cfg)
	if err != nil {
		return nil, errz.Wrapf(err, "config: %s: failed to unmarshal config YAML", fs.Path)
	}

	if cfg.Version == "" {
		cfg.Version = buildinfo.Version
	}

	cfg.Options, err = options.DefaultRegistry.Process(cfg.Options)
	if err != nil {
		return nil, err
	}

	repaired, err := source.VerifyIntegrity(cfg.Collection)
	if err != nil {
		if repaired {
			// The config was repaired. Save the changes.
			err = errz.Combine(err, fs.Save(ctx, cfg))
		}
		return nil, errz.Wrapf(err, "config: %s", fs.Path)
	}

	err = fs.loadExt(cfg)
	if err != nil {
		return nil, err
	}

	return cfg, nil
}

// Save writes config to disk. It implements Store.
func (fs *Store) Save(_ context.Context, cfg *config.Config) error {
	if fs == nil {
		return errz.New("config file store is nil")
	}

	if err := config.Valid(cfg); err != nil {
		return err
	}

	data, err := ioz.MarshalYAML(cfg)
	if err != nil {
		return err
	}

	return fs.Write(data)
}

// Write writes the config bytes to disk.
func (fs *Store) Write(data []byte) error {
	// It's possible that the parent dir of fs.Path doesn't exist.
	dir := filepath.Dir(fs.Path)
	err := os.MkdirAll(dir, 0o750)
	if err != nil {
		return errz.Wrapf(err, "failed to make parent dir of sq config file: %s", dir)
	}

	err = os.WriteFile(fs.Path, data, 0o600)
	if err != nil {
		return errz.Wrap(err, "failed to save config file")
	}

	return nil
}

// fileExists returns true if the backing file can be accessed, false if it doesn't
// exist or on any error.
func (fs *Store) fileExists() bool {
	_, err := os.Stat(fs.Path)
	return err == nil
}
