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

func newConfigKeyringCreateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create PATH [VALUE]",
		Args:  cobra.RangeArgs(1, 2),
		Short: "Create a new keyring entry",
		Long: `Create a new entry in the OS keyring at the given PATH.
Errors if an entry already exists at PATH; use 'sq config keyring
update' to change the value of an existing entry.

PATH is the body of a ${keyring:PATH} placeholder. sq itself generates
opaque 10-char IDs (e.g. "j2k7m3pxtz") via 'sq add --store keyring',
so most users never run 'create' directly. PATH accepts any string —
you can use a hand-crafted path for shared or composition entries.

If VALUE is omitted, --password (-p) is required: sq then reads the
value from stdin (piped data or, if stdin is a TTY, an interactive
prompt). Stdin is NOT consulted unless -p is set.`,
		RunE: execConfigKeyringCreate,
		Example: `  # Create with an inline value
  $ sq config keyring create my_db_pw 'postgres://alice:hunter2@db/sakila'

  # Create with a piped value (keeps secrets out of shell history)
  $ sq config keyring create my_db_pw -p < secret.txt

  # Prompt interactively
  $ sq config keyring create my_db_pw -p
  Password: ****`,
	}
	cmd.Flags().BoolP(flag.PasswordPrompt, flag.PasswordPromptShort, false, flag.PasswordPromptUsage)
	addKeyringFormatFlags(cmd)
	return cmd
}

func execConfigKeyringCreate(cmd *cobra.Command, args []string) error {
	ru := run.FromContext(cmd.Context())
	path := args[0]

	kr := keyring.NewStore()
	if _, err := kr.Resolve(cmd.Context(), path); err == nil {
		return errz.Errorf(
			"keyring entry already exists at %q: use 'sq config keyring update' to change its value",
			path,
		)
	} else if !errors.Is(err, secret.ErrNotFound) {
		return err
	}

	value, err := readKeyringValueArg(cmd, args)
	if err != nil {
		return err
	}
	if err := kr.Set(cmd.Context(), path, string(value)); err != nil {
		return err
	}
	return ru.Writers.Keyring.Created(path)
}
