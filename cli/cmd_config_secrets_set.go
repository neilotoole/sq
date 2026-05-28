package cli

import (
	"github.com/spf13/cobra"

	"github.com/neilotoole/sq/cli/flag"
	"github.com/neilotoole/sq/cli/run"
	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/secret/keyring"
)

func newConfigSecretsSetCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "set PATH [VALUE]",
		Args:  cobra.RangeArgs(1, 2),
		Short: "Set a keyring secret",
		Long: `Write a secret value to the OS keyring at the given PATH.

PATH is the value used in a ${keyring:PATH} placeholder. For example,
a path of "@sakila/password" is referenced from a source location as
${keyring:@sakila/password}.

If VALUE is omitted, read from stdin (when piped) or prompt the user
via the -p flag.`,
		RunE: execConfigSecretsSet,
		Example: `  # Set with explicit value
  $ sq config secrets set @sakila/password hunter2

  # Set interactively
  $ sq config secrets set @sakila/password -p
  Password: ****

  # Set from a file
  $ sq config secrets set @sakila/password -p < password.txt`,
	}
	cmd.Flags().BoolP(flag.PasswordPrompt, flag.PasswordPromptShort, false, flag.PasswordPromptUsage)
	return cmd
}

func execConfigSecretsSet(cmd *cobra.Command, args []string) error {
	ru := run.FromContext(cmd.Context())
	path := args[0]

	var value []byte
	switch {
	case len(args) == 2:
		value = []byte(args[1])
	case cmdFlagIsSetTrue(cmd, flag.PasswordPrompt):
		var err error
		value, err = readPassword(cmd.Context(), ru.Stdin, ru.Out, ru.Writers.PrOut)
		if err != nil {
			return err
		}
	default:
		return errz.New("must provide VALUE argument or --password flag")
	}

	return keyring.New().Set(cmd.Context(), path, string(value))
}
