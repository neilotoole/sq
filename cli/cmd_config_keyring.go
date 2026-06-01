package cli

import (
	"github.com/spf13/cobra"

	"github.com/neilotoole/sq/cli/flag"
	"github.com/neilotoole/sq/cli/run"
	"github.com/neilotoole/sq/libsq/core/errz"
)

func newConfigKeyringCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "keyring",
		Args:  cobra.NoArgs,
		Short: "Manage OS-keyring entries used by source secrets",
		Long: `View and manage entries in the OS keyring that source locations
reference via ${keyring:<id>} placeholders.

Source location fields may contain ${scheme:path} placeholders that are
resolved at connect time. sq ships with three resolver schemes:

  keyring   OS keyring (macOS Keychain, Windows Credential Manager,
            Secret Service on Linux). Read and write.
  env       Environment variable. Read-only at connect time.
  file      File contents (single trailing newline trimmed). Read-only.
            Path must be absolute, start with ~/ (current user's home),
            or use the empty-authority file URI form (file:///path).
            Relative and remote (file://host/path) forms are rejected.

This command group manages the keyring scheme only. 'env' and 'file'
references are external — sq reads them at connect time but does not
write to them. Use 'sq ping' to verify end-to-end that env/file refs
resolve correctly.

Examples of placeholder forms in a source's Location:

  location: postgres://alice:${keyring:j2k7m3pxtz}@db/sakila
  location: postgres://alice:${env:DB_PROD_PASSWORD}@db/sakila
  location: postgres://alice:${file:/run/secrets/db_prod_pw}@db/sakila
  location: postgres://alice:${file:~/.sq/db_prod_pw}@db/sakila
  location: postgres://alice:${file:///run/secrets/db_prod_pw}@db/sakila`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
		Example: `  # List keyring paths referenced by sources
  $ sq config keyring ls

  # Create a new keyring entry, prompting for the value
  $ sq config keyring create @sakila/password -p

  # Rotate an existing entry
  $ sq config keyring update @sakila/password -p

  # Migrate inline passwords into the keyring
  $ sq config keyring migrate --all`,
	}
	return cmd
}

// readKeyringValueArg returns the secret value to write to the keyring,
// sourced from either args[1] (the explicit VALUE argument) or stdin
// (when --password / -p was set). Returns an error if neither source
// is available — the caller advertises both options in its --help.
func readKeyringValueArg(cmd *cobra.Command, args []string) ([]byte, error) {
	if len(args) == 2 {
		return []byte(args[1]), nil
	}
	if !cmdFlagIsSetTrue(cmd, flag.PasswordPrompt) {
		return nil, errz.New("must provide VALUE argument or --password flag")
	}
	ru := run.FromContext(cmd.Context())
	return readPassword(cmd.Context(), ru.Stdin, ru.Out, ru.Writers.PrOut)
}
