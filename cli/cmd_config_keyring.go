package cli

import (
	"github.com/spf13/cobra"
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

  # Set a keyring secret interactively
  $ sq config keyring set @sakila/password -p

  # Migrate inline passwords into the keyring
  $ sq config keyring migrate --all`,
	}
	return cmd
}
