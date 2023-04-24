// Package config holds CLI configuration.
package config

import (
	"github.com/neilotoole/sq/cli/buildinfo"
	"github.com/neilotoole/sq/libsq/core/options"

	"github.com/neilotoole/sq/libsq/core/errz"
	"golang.org/x/mod/semver"

	"github.com/neilotoole/sq/drivers/userdriver"
	"github.com/neilotoole/sq/libsq/core/stringz"
	"github.com/neilotoole/sq/libsq/source"
)

const (
	EnvarLogPath     = "SQ_LOGFILE"
	EnvarLogTruncate = "SQ_LOGFILE_TRUNCATE"

	// EnvarConfigDir is the legacy envar for config location.
	// Instead use EnvarConfig.
	EnvarConfigDir = "SQ_CONFIGDIR"

	// EnvarConfig is the envar for config location.
	EnvarConfig = "SQ_CONFIG"
)

// Config holds application config/session data.
type Config struct {
	// Version is the config version. This will allow sq to
	// upgrade config files if needed. It must be a valid semver.
	Version string `yaml:"config.version" json:"config_version"`

	// Options contains default settings, such as output format.
	Options options.Options `yaml:"options" json:"options"`

	// Collection is the set of data sources.
	Collection *source.Collection `yaml:"collection" json:"collection"`

	// Ext holds sq config extensions, such as user driver config.
	Ext Ext `yaml:"-" json:"-"`
}

// String returns a log/debug-friendly representation.
func (c *Config) String() string {
	return stringz.SprintJSON(c)
}

// Ext holds additional config (extensions) loaded from other
// config files, e.g. ~/.config/sq/ext/*.sq.yml.
type Ext struct {
	UserDrivers []*userdriver.DriverDef `yaml:"user.drivers" json:"user.drivers"`
}

// New returns a config instance with default options set.
func New() *Config {
	cfg := &Config{
		Collection: &source.Collection{},
		Options:    options.Options{},
		Version:    buildinfo.Version,
	}

	// By default, we want header to be true; this is
	// ugly wrt initCfg, as the zero value of a bool
	// is false, but we actually want it to be true for Header.
	// cfg.Options["format.header"] = true
	// FIXME: use options.DefaultRegistry.Process(cfg.Options)
	// Also, do this for each Source in cfg.Collection.

	// if cfg.Version == "" {
	//	cfg.Version = buildinfo.Version
	// }

	return cfg
}

// Valid returns an error if cfg is not valid.
func Valid(cfg *Config) error {
	if cfg == nil {
		return errz.New("config is nil")
	}

	if !semver.IsValid(cfg.Version) {
		return errz.Errorf("config: invalid '.config_version': %s", cfg.Version)
	}

	if cfg.Collection != nil {
		if _, err := source.VerifyIntegrity(cfg.Collection); err != nil {
			return errz.Wrap(err, "config: invalid '.sources'")
		}
	}

	return nil
}
