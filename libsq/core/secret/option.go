package secret

import (
	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/options"
)

// OptSecretsStore selects the default secret storage backend used by
// sq add when the source's URL carries a password. Valid values are
// "inline" (store the URL verbatim in the YAML config — historical
// default) or "keyring" (write the full DSN to the OS keyring and
// store a bare "${keyring:<id>}" placeholder as the YAML Location).
//
// The config key is "secrets.store"; the option can be overridden per
// invocation with the --store flag on sq add.
var OptSecretsStore = options.NewString(
	"secrets.store",
	nil,
	"inline",
	func(s string) error {
		switch s {
		case "inline", "keyring":
			return nil
		}
		return errz.Errorf("must be 'inline' or 'keyring', got %q", s)
	},
	"Default secret storage for sq add",
	`Default secret storage backend used by "sq add" when the source URL
carries a password.

  inline   Store the URL verbatim in the YAML config (password and all).
           Historical default.
  keyring  Write the full DSN to the OS keyring at a fresh opaque ID
           and store a bare "${keyring:<id>}" placeholder as the YAML
           Location. The keyring entry holds the entire DSN; only the
           opaque ID lands in YAML.

The --store flag on sq add overrides this setting per invocation.`,
)

// OptSecretsReveal controls whether secrets (URL-style passwords in
// source locations, stored keyring values) are printed verbatim in
// output. When false (the default), they are redacted to "xxxxx". When
// true, they are shown.
//
// The config key is "secrets.reveal"; the option can be flipped per
// invocation with the global --reveal flag, or via the deprecated
// --no-redact alias.
//
// Replaces the legacy "redact" option, with inverted polarity:
// secrets.reveal = !redact. A YAML upgrade step migrates existing
// configs automatically.
var OptSecretsReveal = options.NewBool(
	"secrets.reveal",
	nil,
	false,
	"Show secret values in output",
	`Show secret values in output. When true, passwords in source
locations and stored keyring values are printed verbatim; when false
(the default), they are redacted in output that prints a location or
keyring value.

This is the persistent form of the global --reveal flag.`,
	options.TagOutput,
)
