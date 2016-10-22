package config

import (
	"time"

	"github.com/neilotoole/go-lg/lg"
	"github.com/neilotoole/sq/libsq/drvr"
)

type QueryMode string

const ModeSLQ QueryMode = "slq"
const ModeNativeSQL QueryMode = "native"

type Format string

const FormatJSON Format = "json"
const FormatTable Format = "table"
const FormatGrid Format = "grid"
const FormatRaw Format = "raw"
const FormatXLSX Format = "xlsx"
const FormatXML Format = "xml"
const FormatCSV Format = "csv"
const FormatTSV Format = "tsv"

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

// New returns a config instance with default options set.
func New() *Config {
	lg.Debugf("new config instance")
	cfg := &Config{}
	applyDefaults(cfg)
	return cfg

}

// applyDefaults checks if required values are present, and if not, sets them.
func applyDefaults(cfg *Config) {

	if cfg.SourceSet == nil {
		cfg.SourceSet = drvr.NewSourceSet()
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
	ModeSLQ,
	FormatJSON,
}
