package config

import (
	"time"

	"github.com/neilotoole/go-lg/lg"
	"github.com/neilotoole/sq/lib/drvr"
)

var buildVersion string
var buildTimestamp string

type QueryMode string

const ModeSQ QueryMode = "sq"
const ModeNativeSQL QueryMode = "native"

type Format string

const FormatJSON Format = "json"
const FormatTable Format = "table"
const FormatGrid Format = "grid"
const FormatRaw Format = "raw"
const FormatXLSX Format = "xlsx"
const FormatCSV Format = "csv"
const FormatTSV Format = "tsv"

var conf *Config
var str Store

// Config holds application config/session data.
type Config struct {
	cfgDir    string
	Options   Options         `yaml:"options"`
	Log       Log             `yaml:"log"`
	SourceSet *drvr.SourceSet `yaml:"sources"`
}

type Options struct {
	Timeout   time.Duration `yaml:"timeout"`
	QueryMode QueryMode     `yaml:"query_mode"`
	Format    Format        `yaml:"output_format"`
	Header    bool          `yaml:"output_header"`
}

type Log struct {
	Enabled     bool     `yaml:"enabled"`
	Filepath    string   `yaml:"filepath"`
	Levels      []string `yaml:"levels"`
	ExcludePkgs []string `yaml:"exclude_pkgs"`
}

// TODO: need to add a file write lock

//// Default returns the default config singleton.
//func Default() *Config {
//
//	if conf == nil {
//
//		if str == nil {
//			panic("config.Default() invoked before config.SetStore() ")
//		}
//
//		cfg, err := str.Load()
//		if err != nil {
//			// TODO: should try to load before this
//			panic(err)
//		}
//		conf = cfg
//	}
//
//	return conf
//}

// SetStore specifies the store for config persistence.
func SetStore(store Store) {
	str = store
}

//// defaultPath returns ~/.sq/sq.yml
//func defaultPath() (string, error) {
//
//	return filepath.Join(util.ConfigDir(), "sq.yml"), nil
//}

// NewConfig returns a config instance with default options set.
func NewConfig() *Config {
	cfg := &Config{}
	applyDefaults(cfg)
	return cfg

}

// applyDefaults checks if required values are present, and if not, sets them.
func applyDefaults(cfg *Config) {
	lg.Debugf("checking that cfg has default values set")

	if cfg.SourceSet == nil {
		cfg.SourceSet = &drvr.SourceSet{}
	}

	if cfg.Options.QueryMode == "" {
		cfg.Options.QueryMode = defaults.QueryMode
	}

	if cfg.Options.Format == "" {
		cfg.Options.Format = defaults.Format
	}

	if cfg.Options.Timeout == 0 {
		cfg.Options.Timeout = defaults.Timeout
	}
}

// Defaults contains the (factory-supplied) config defaults.
var defaults = struct {
	Timeout   time.Duration
	QueryMode QueryMode
	Format    Format
}{
	10 * time.Second,
	ModeSQ,
	FormatJSON,
}

//func init() {
//	//lg.Debugf("configuring factory settings")
//	//defaults.Timeout = 10 * time.Second
//	//defaults.QueryMode = ModeSQ
//	//defaults.Format = FormatJSON
//
//	// if the envar is set, then we use that as the default filestore.
//	envar := "SQ_CONFIG_FILEPATH"
//	configPath, ok := os.LookupEnv(envar)
//	if ok {
//		lg.Debugf("attempting to create filestore from %q with value %q", envar, configPath)
//		store, err := NewFileStore(configPath)
//		if err != nil {
//			lg.Fatalf("Fatal error: unable to initialize config: %v", err)
//		}
//
//		SetStore(store)
//		lg.Debugf("successfully set config filestore to %q", configPath)
//	}
//
//}
