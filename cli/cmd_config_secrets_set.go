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

PATH is the body of a ${keyring:PATH} placeholder. sq itself generates
opaque 10-char IDs (e.g. "j2k7m3pxtz") via 'sq add --store keyring',
but PATH accepts any string — you can use a hand-crafted path for
shared or hand-managed entries.

If VALUE is omitted, --password (-p) is required: sq then reads the
value from stdin (piped data or, if stdin is a TTY, an interactive
prompt). Stdin is NOT consulted unless -p is set.

Typically used to rotate a credential: pass the same PATH that already
appears in a source's Location, with a new VALUE. The Location does
not need to change.`,
		RunE: execConfigSecretsSet,
		Example: `  # Rotate the value at an sq-generated id
  $ sq config secrets set j2k7m3pxtz 'postgres://alice:newpw@db/sakila'

  # Pipe a value from stdin to keep it out of shell history
  $ sq config secrets set j2k7m3pxtz -p < secret.txt

  # Prompt interactively
  $ sq config secrets set j2k7m3pxtz -p
  Password: ****`,
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
