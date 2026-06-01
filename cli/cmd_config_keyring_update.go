package cli

import (
	"errors"

	"github.com/spf13/cobra"

	"github.com/neilotoole/sq/cli/flag"
	"github.com/neilotoole/sq/cli/run"
	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/secret"
	"github.com/neilotoole/sq/libsq/core/secret/keyring"
)

func newConfigKeyringUpdateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "update PATH [VALUE]",
		Args:  cobra.RangeArgs(1, 2),
		Short: "Update an existing keyring entry",
		Long: `Write a new value to an existing keyring entry at PATH.
Errors if no entry exists at PATH; use 'sq config keyring create' to
add a new entry.

PATH is the body of a ${keyring:PATH} placeholder. Typically used to
rotate a credential: pass the same PATH that already appears in a
source's Location, with a new VALUE. The Location does not need to
change.

If VALUE is omitted, --password (-p) is required: sq then reads the
value from stdin (piped data or, if stdin is a TTY, an interactive
prompt). Stdin is NOT consulted unless -p is set.`,
		RunE: execConfigKeyringUpdate,
		Example: `  # Rotate the value at an sq-generated id
  $ sq config keyring update j2k7m3pxtz 'postgres://alice:newpw@db/sakila'

  # Pipe a value from stdin to keep it out of shell history
  $ sq config keyring update j2k7m3pxtz -p < secret.txt

  # Prompt interactively
  $ sq config keyring update j2k7m3pxtz -p
  Password: ****`,
		ValidArgsFunction: completeKeyringPath,
	}
	cmd.Flags().BoolP(flag.PasswordPrompt, flag.PasswordPromptShort, false, flag.PasswordPromptUsage)
	addKeyringFormatFlags(cmd)
	return cmd
}

func execConfigKeyringUpdate(cmd *cobra.Command, args []string) error {
	ru := run.FromContext(cmd.Context())
	path := args[0]

	kr := keyring.NewStore()
	if _, err := kr.Resolve(cmd.Context(), path); err != nil {
		if errors.Is(err, secret.ErrNotFound) {
			return errz.Errorf(
				"no keyring entry at %q: use 'sq config keyring create' to add it",
				path)
		}
		return err
	}

	value, err := readKeyringValueArg(cmd, args)
	if err != nil {
		return err
	}
	if err := kr.Set(cmd.Context(), path, string(value)); err != nil {
		return err
	}
	return ru.Writers.Keyring.Updated(path)
}
