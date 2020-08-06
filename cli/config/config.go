// Package config holds CLI configuration.
package config

import (
	"time"

	"github.com/neilotoole/sq/drivers/userdriver"
	"github.com/neilotoole/sq/libsq/errz"
	"github.com/neilotoole/sq/libsq/notify"
	"github.com/neilotoole/sq/libsq/source"
	"github.com/neilotoole/sq/libsq/stringz"
)

// Config holds application config/session data.
type Config struct {
	Options      Options       `yaml:"options" json:"options"`
	Sources      *source.Set   `yaml:"sources" json:"sources"`
	Notification *Notification `yaml:"notification" json:"notification"`

	// Ext holds sq config extensions, such as user driver config.
	Ext Ext `yaml:"-" json:"-"`
}

func (c *Config) String() string {
	return stringz.SprintJSON(c)
}

// Ext holds additional config (extensions) loaded from other
// config files, e.g. ~/.config/sq/ext/*.sq.yml
type Ext struct {
	UserDrivers []*userdriver.DriverDef `yaml:"user_drivers" json:"user_drivers"`
}

// Options contains sq default values.
type Options struct {
	Timeout time.Duration `yaml:"timeout" json:"timeout"`
	Format  Format        `yaml:"output_format" json:"output_format"`
	Header  bool          `yaml:"output_header" json:"output_header"`
}

// Notification holds notification configuration.
type Notification struct {
	Enabled      []string             `yaml:"enabled" json:"enabled"`
	Destinations []notify.Destination `yaml:"destinations" json:"destinations"`
}

// New returns a config instance with default options set.
func New() *Config {
	cfg := &Config{}

	// By default, we want header to be true; this is
	// ugly wrt applyDefaults, as the zero value of a bool
	// is false, but we actually want it to be true for Header.
	cfg.Options.Header = true

	applyDefaults(cfg)
	return cfg
}

// applyDefaults checks if required values are present, and if not, sets them.
func applyDefaults(cfg *Config) {
	if cfg.Sources == nil {
		cfg.Sources = &source.Set{}
	}

	if cfg.Notification == nil {
		cfg.Notification = &Notification{}
	}

	if cfg.Options.Format == "" {
		cfg.Options.Format = FormatTable
	}

	if cfg.Options.Timeout == 0 {
		cfg.Options.Timeout = 10 * time.Second
	}
}

// Format is a sq output format such as json or xml.
type Format string

// UnmarshalText implements encoding.TextUnmarshaler.
func (f *Format) UnmarshalText(text []byte) error {
	switch Format(text) {
	default:
		return errz.Errorf("unknown output format %q", string(text))
	case FormatJSON, FormatJSONA, FormatJSONL, FormatTable, FormatGrid, FormatRaw,
		FormatHTML, FormatMarkdown, FormatXLSX, FormatXML, FormatCSV, FormatTSV:
	}

	*f = Format(text)
	return nil
}

// Constants
const (
	FormatJSON     Format = "json"
	FormatJSONL    Format = "jsonl"
	FormatJSONA    Format = "jsona"
	FormatTable    Format = "table" // FIXME: rename to FormatText
	FormatGrid     Format = "grid"
	FormatRaw      Format = "raw"
	FormatHTML     Format = "html"
	FormatMarkdown Format = "markdown"
	FormatXLSX     Format = "xlsx"
	FormatXML      Format = "xml"
	FormatCSV      Format = "csv"
	FormatTSV      Format = "tsv"
)
