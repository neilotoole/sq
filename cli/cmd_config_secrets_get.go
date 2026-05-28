package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/neilotoole/sq/cli/run"
	"github.com/neilotoole/sq/libsq/core/secret/keyring"
)

const flagSecretReveal = "reveal"

func newConfigSecretsGetCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get PATH",
		Args:  cobra.ExactArgs(1),
		Short: "Get a keyring secret",
		Long: `Print metadata for the keyring secret at PATH. With --reveal,
prints the secret value itself to stdout.

By default (no --reveal), the secret value is NOT printed — only that the
entry exists. Use --reveal explicitly to acknowledge that you want the
secret on screen.`,
		RunE: execConfigSecretsGet,
		Example: `  # Confirm the secret exists
  $ sq config secrets get @sakila/password

  # Print the secret value
  $ sq config secrets get @sakila/password --reveal`,
	}
	cmd.Flags().Bool(flagSecretReveal, false,
		"Print the secret value (default: only confirm existence)")
	return cmd
}

func execConfigSecretsGet(cmd *cobra.Command, args []string) error {
	ru := run.FromContext(cmd.Context())
	path := args[0]

	value, err := keyring.New().Resolve(cmd.Context(), path)
	if err != nil {
		return err
	}

	if cmdFlagIsSetTrue(cmd, flagSecretReveal) {
		fmt.Fprintln(ru.Out, value)
		return nil
	}
	fmt.Fprintf(ru.Out, "secret exists: %s\n", path)
	return nil
}
