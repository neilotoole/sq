package cli

import (
	"slices"
	"strings"

	"github.com/spf13/cobra"

	"github.com/neilotoole/sq/cli/run"
	"github.com/neilotoole/sq/libsq/core/secret"
	"github.com/neilotoole/sq/libsq/core/secret/keyring"
)

func newConfigKeyringRmCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "rm PATH",
		Args:  cobra.ExactArgs(1),
		Short: "Delete a keyring secret",
		Long: `Delete the keyring secret at PATH. Deleting a non-existent entry
is not an error (idempotent).

This removes the keyring entry but does NOT touch any YAML source that
references it; a remaining ${keyring:PATH} reference will fail to
resolve at connect time. Use 'sq config keyring ls' to find references
before removing.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ru := run.FromContext(cmd.Context())
			path := args[0]
			if err := keyring.NewStore().Delete(cmd.Context(), path); err != nil {
				return err
			}
			return ru.Writers.Keyring.Rm(path)
		},
		ValidArgsFunction: completeKeyringPath,
		Example:           `  $ sq config keyring rm j2k7m3pxtz`,
	}
	addKeyringFormatFlags(cmd)
	return cmd
}

// completeKeyringPath suggests keyring paths referenced by sources in
// the active config. There's no portable way to enumerate keyring
// entries via the OS APIs we use, so the candidate set is derived
// from ${keyring:<path>} occurrences across the source collection.
// Orphan entries (referenced by nothing) won't appear; that's the
// same limitation that "sq config keyring ls" has.
func completeKeyringPath(cmd *cobra.Command, args []string, toComplete string) (
	[]string, cobra.ShellCompDirective,
) {
	if len(args) >= 1 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	ru := getRun(cmd)
	if ru == nil || ru.Config == nil || ru.Config.Collection == nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	seen := make(map[string]struct{})
	for _, src := range ru.Config.Collection.Sources() {
		refs, err := secret.ExtractRefs(src.Location)
		if err != nil {
			continue
		}
		for _, ref := range refs {
			if ref.Scheme != "keyring" {
				continue
			}
			if !strings.HasPrefix(ref.Path, toComplete) {
				continue
			}
			seen[ref.Path] = struct{}{}
		}
	}
	out := make([]string, 0, len(seen))
	for p := range seen {
		out = append(out, p)
	}
	slices.Sort(out)
	return out, cobra.ShellCompDirectiveNoFileComp
}
