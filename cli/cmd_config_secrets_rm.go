package cli

import (
	"github.com/spf13/cobra"

	"github.com/neilotoole/sq/libsq/core/secret/keyring"
)

func newConfigSecretsRmCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "rm PATH",
		Aliases: []string{"remove", "delete"},
		Args:    cobra.ExactArgs(1),
		Short:   "Delete a keyring secret",
		Long: `Delete the keyring secret at PATH. Deleting a non-existent entry
is not an error (idempotent).

This removes the keyring entry but does NOT touch any YAML source that
references it; a remaining ${keyring:PATH} reference will fail to
resolve at connect time. Use 'sq config secrets ls' to find references
before removing.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return keyring.New().Delete(cmd.Context(), args[0])
		},
		Example: `  $ sq config secrets rm j2k7m3pxtz`,
	}
	return cmd
}
