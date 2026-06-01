package cli

import (
	"github.com/spf13/cobra"

	"github.com/neilotoole/sq/cli/run"
	"github.com/neilotoole/sq/libsq/core/secret/keyring"
)

const flagSecretReveal = "reveal"

func newConfigKeyringGetCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get PATH",
		Args:  cobra.ExactArgs(1),
		Short: "Get a keyring secret",
		Long: `Print metadata for the keyring secret at PATH. With --reveal,
prints the secret value itself to stdout.

PATH is the body of a ${keyring:PATH} placeholder. Use 'sq config
keyring ls' to find the ids referenced by your sources.

By default (no --reveal), the value is NOT printed — only that the
entry exists. With --reveal, the value (which under Form B is the
entire DSN, including credentials) is written to stdout.`,
		RunE: execConfigKeyringGet,
		Example: `  # Confirm the entry exists at an sq-generated id
  $ sq config keyring get j2k7m3pxtz

  # Print the stored value
  $ sq config keyring get j2k7m3pxtz --reveal`,
		ValidArgsFunction: completeKeyringPath,
	}
	cmd.Flags().Bool(flagSecretReveal, false,
		"Print the secret value (default: only confirm existence)")
	addKeyringFormatFlags(cmd)
	return cmd
}

func execConfigKeyringGet(cmd *cobra.Command, args []string) error {
	ru := run.FromContext(cmd.Context())
	path := args[0]

	value, err := keyring.NewStore().Resolve(cmd.Context(), path)
	if err != nil {
		return err
	}

	revealed := cmdFlagIsSetTrue(cmd, flagSecretReveal)
	return ru.Writers.Keyring.Get(path, value, revealed)
}
