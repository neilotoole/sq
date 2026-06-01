package secret

import (
	"fmt"

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
		return fmt.Errorf("must be 'inline' or 'keyring', got %q", s)
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
