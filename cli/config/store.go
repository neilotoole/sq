package config

import (
	"fmt"
	"os"
	"path/filepath"

	"strings"

	"github.com/neilotoole/sq/cli/buildinfo"
	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/source"

	"gopkg.in/yaml.v2"
)

// Store saves and loads config.
type Store interface {
	// Save writes config to the store.
	Save(cfg *Config) error

	// Load reads config from the store.
	Load() (*Config, error)

	// Location returns the location of the store, typically
	// a file path.
	Location() string
}

// YAMLFileStore provides persistence of config via YAML file.
type YAMLFileStore struct {
	// Path is the location of the config file
	Path string

	// If HookLoad is non-nil, it is invoked by Load
	// on Path's bytes before the YAML is unmarshaled.
	// This allows expansion of variables etc.
	HookLoad func(data []byte) ([]byte, error)

	// ExtPaths holds locations of potential ext config, both dirs and files (with suffix ".sq.yml")
	ExtPaths []string
}

func (fs *YAMLFileStore) String() string {
	return fmt.Sprintf("config filestore: %v", fs.Path)
}

// Location implements Store.
func (fs *YAMLFileStore) Location() string {
	return fs.Path
}

// Load reads config from disk.
func (fs *YAMLFileStore) Load() (*Config, error) {
	bytes, err := os.ReadFile(fs.Path)
	if err != nil {
		return nil, errz.Wrapf(err, "config: failed to load file %q", fs.Path)
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

	err = source.VerifySetIntegrity(cfg.Sources)
	if err != nil {
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

	for _, f := range extCfgCandidates {
		bytes, err := os.ReadFile(f)
		if err != nil {
			return errz.Wrapf(err, "error reading config ext file %q", f)
		}
		ext := &Ext{}

		err = yaml.Unmarshal(bytes, ext)
		if err != nil {
			return errz.Wrapf(err, "error parsing config ext file %q", f)
		}

		cfg.Ext.UserDrivers = append(cfg.Ext.UserDrivers, ext.UserDrivers...)
	}

	return nil
}

// Save writes config to disk.
func (fs *YAMLFileStore) Save(cfg *Config) error {
	if fs == nil {
		return errz.New("config file store is nil")
	}

	if buildinfo.Version != "" {
		cfg.Version = buildinfo.Version
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return errz.Wrap(err, "failed to marshal config to YAML")
	}

	// It's possible that the parent dir of fs.Path doesn't exist.
	dir := filepath.Dir(fs.Path)
	err = os.MkdirAll(dir, 0750)
	if err != nil {
		return errz.Wrapf(err, "failed to make parent dir of sq config file: %s", dir)
	}

	err = os.WriteFile(fs.Path, data, 0600)
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
type DiscardStore struct {
}

var _ Store = (*DiscardStore)(nil)

// Load returns a new empty Config.
func (DiscardStore) Load() (*Config, error) {
	return New(), nil
}

// Save is no-op.
func (DiscardStore) Save(*Config) error {
	return nil
}

// Location returns /dev/null.
func (DiscardStore) Location() string {
	return "/dev/null"
}
