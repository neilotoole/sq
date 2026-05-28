package secret

import (
	"fmt"

	"github.com/neilotoole/sq/libsq/core/options"
)

// OptSecretsDefault sets the default storage backend used by sq add when
// a password is present. Valid values are "inline" (store the password
// verbatim in the YAML config — historical default) or "keyring" (write
// the password to the OS keyring and replace it with a ${keyring:...}
// placeholder).
var OptSecretsDefault = options.NewString(
	"secrets.default",
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
	`Default secret storage backend used by "sq add" when a password is present.

  inline   Store the password verbatim in the YAML config. Historical default.
  keyring  Write the password to the OS keyring and store a ${keyring:...}
           placeholder in the config.

Per-command flags --keyring and --inline-password override this setting.`,
)
