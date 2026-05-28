package cli

import (
	"github.com/spf13/cobra"
)

func newConfigSecretsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "secrets",
		Args:  cobra.NoArgs,
		Short: "Manage source secrets",
		Long: `View and manage secrets referenced from source locations.

Source location fields may contain ${scheme:path} placeholders that are
resolved at connect time. sq ships with three resolver schemes:

  keyring   OS keyring (macOS Keychain, Windows Credential Manager,
            Secret Service on Linux). Managed by 'sq config secrets'.
  env       Environment variable. Read-only at connect time.
  file      File contents (single trailing newline trimmed). Read-only.
            Path must be absolute or start with ~/ (current user's home);
            relative paths are rejected.

Examples:

  location: postgres://alice:${keyring:@sakila/password}@db/sakila
  location: postgres://alice:${env:DB_PROD_PASSWORD}@db/sakila
  location: postgres://alice:${file:/run/secrets/db_prod_pw}@db/sakila
  location: postgres://alice:${file:~/.sq/db_prod_pw}@db/sakila

The subcommands below manage keyring entries. 'env' and 'file' references
do not need management here — sq reads them at connect time directly.`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
		Example: `  # List secret refs known to sq
  $ sq config secrets ls

  # Set a secret interactively
  $ sq config secrets set @sakila/password -p

  # Migrate inline passwords to the keyring
  $ sq config secrets migrate --all`,
	}
	return cmd
}
