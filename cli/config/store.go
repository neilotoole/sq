package config

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/neilotoole/sq/libsq/core/lg"
	"github.com/neilotoole/sq/libsq/core/lg/lga"

	"github.com/neilotoole/sq/libsq/core/ioz"

	"github.com/neilotoole/sq/cli/flag"

	"github.com/spf13/pflag"

	"github.com/neilotoole/sq/cli/buildinfo"
	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/source"

	"gopkg.in/yaml.v3"
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

// YAMLFileStore provides persistence of config via YAML file.
type YAMLFileStore struct {
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

	// upgradeReg holds upgrade funcs for upgrading the config file.
	upgradeReg upgradeRegistry
}

// Origin of the config path.
const (
	originFlag    = "flag"
	originEnv     = "env"
	originDefault = "default"
)

// String returns a log/debug-friendly representation.
func (fs *YAMLFileStore) String() string {
	return fmt.Sprintf("config via %s: %v", fs.PathOrigin, fs.Path)
}

// Location implements Store. It returns the location of the config dir.
func (fs *YAMLFileStore) Location() string {
	return filepath.Dir(fs.Path)
}

// Load reads config from disk. It implements Store.
func (fs *YAMLFileStore) Load(ctx context.Context) (*Config, error) {
	log := lg.FromContext(ctx)
	log.Debug("Loading config from file", lga.Path, fs.Path)

	if fs.upgradeReg == nil {
		// Use the package-level registry by default.
		// This is not ideal, but test code can change this
		// if needed.
		fs.upgradeReg = defaultUpgradeReg
	}

	mightNeedUpgrade, foundVers, err := checkNeedsUpgrade(fs.Path)
	if err != nil {
		return nil, errz.Wrapf(err, "config: %s", fs.Path)
	}

	if mightNeedUpgrade {
		log.Info("Upgrade config?", lga.From, foundVers, lga.To, buildinfo.Version)
		if _, err = fs.UpgradeConfig(ctx, foundVers, buildinfo.Version); err != nil {
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

	return fs.doLoad(ctx)
}

func (fs *YAMLFileStore) doLoad(ctx context.Context) (*Config, error) {
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

	cfg := &Config{}
	err = yaml.Unmarshal(bytes, cfg)
	if err != nil {
		return nil, errz.Wrapf(err, "config: %s: failed to unmarshal config YAML", fs.Path)
	}

	initCfg(cfg)

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

// loadExt loads extension config files into cfg.
func (fs *YAMLFileStore) loadExt(cfg *Config) error {
	const extSuffix = ".sq.yml"
	var extCfgCandidates []string

	for _, extPath := range fs.ExtPaths {
		// TODO: This seems overly complicated: could just use glob
		//  for any files in the same or child dir?
		if fiExtPath, err := os.Stat(extPath); err == nil {
			// path exists

			if fiExtPath.IsDir() {
				files, err := os.ReadDir(extPath)
				if err != nil {
					// just continue; no means of logging this yet (logging may
					// not have bootstrapped), and we shouldn't stop bootstrap
					// because of bad sqext files.
					continue
				}

				for _, file := range files {
					if file.IsDir() {
						// We don't currently descend through sub dirs
						continue
					}

					if !strings.HasSuffix(file.Name(), extSuffix) {
						continue
					}

					extCfgCandidates = append(extCfgCandidates, filepath.Join(extPath, file.Name()))
				}

				continue
			}

			// it's a file
			if !strings.HasSuffix(fiExtPath.Name(), extSuffix) {
				continue
			}
			extCfgCandidates = append(extCfgCandidates, filepath.Join(extPath, fiExtPath.Name()))
		}
	}

	for _, fp := range extCfgCandidates {
		bytes, err := os.ReadFile(fp)
		if err != nil {
			return errz.Wrapf(err, "error reading config ext file: %s", fp)
		}
		ext := &Ext{}

		err = yaml.Unmarshal(bytes, ext)
		if err != nil {
			return errz.Wrapf(err, "error parsing config ext file: %s", fp)
		}

		cfg.Ext.UserDrivers = append(cfg.Ext.UserDrivers, ext.UserDrivers...)
	}

	return nil
}

// Save writes config to disk. It implements Store.
func (fs *YAMLFileStore) Save(_ context.Context, cfg *Config) error {
	if fs == nil {
		return errz.New("config file store is nil")
	}

	if err := Valid(cfg); err != nil {
		return err
	}

	data, err := ioz.MarshalYAML(cfg)
	if err != nil {
		return err
	}

	return fs.doSave(data)
}

func (fs *YAMLFileStore) doSave(data []byte) error {
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

// FileExists returns true if the backing file can be accessed, false if it doesn't
// exist or on any error.
func (fs *YAMLFileStore) FileExists() bool {
	_, err := os.Stat(fs.Path)
	return err == nil
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
