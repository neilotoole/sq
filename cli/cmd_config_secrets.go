package cli

import (
	"github.com/spf13/cobra"
)

func newConfigSecretsCmd() *cobra.Command { //nolint:unused // wired in Task 18
	cmd := &cobra.Command{
		Use:   "secrets",
		Args:  cobra.NoArgs,
		Short: "Manage source secrets in the OS keyring",
		Long: `View and manage secret values stored in the OS keyring.

Secrets are referenced from source locations via ${keyring:<path>}
placeholders. For example:

  location: postgres://alice:${keyring:@sakila/password}@db/sakila

At connect time, sq reads the secret from the keyring and substitutes it
into the location. Use the subcommands below to inspect, set, delete, and
migrate secrets.`,
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
