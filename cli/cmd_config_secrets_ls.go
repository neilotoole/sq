package cli

import (
	"fmt"
	"sort"

	"github.com/spf13/cobra"

	"github.com/neilotoole/sq/cli/run"
	"github.com/neilotoole/sq/libsq/core/secret"
)

func newConfigSecretsLsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "ls",
		Aliases: []string{"list"},
		Args:    cobra.NoArgs,
		Short:   "List keyring refs known to sq",
		Long: `List every ${keyring:...} placeholder found across the configured
source locations. The output is derived from the YAML config — no
persistent index is maintained.

Refs in the keyring with no corresponding YAML placeholder are NOT
listed (they're effectively orphans).`,
		RunE: execConfigSecretsLs,
	}
	return cmd
}

func execConfigSecretsLs(cmd *cobra.Command, _ []string) error {
	ru := run.FromContext(cmd.Context())

	seen := make(map[string]struct{})
	var paths []string
	for _, src := range ru.Config.Collection.Sources() {
		refs, err := secret.ExtractRefs(src.Location)
		if err != nil {
			return err
		}
		for _, ref := range refs {
			if ref.Scheme != "keyring" {
				continue
			}
			if _, ok := seen[ref.Path]; ok {
				continue
			}
			seen[ref.Path] = struct{}{}
			paths = append(paths, ref.Path)
		}
	}
	sort.Strings(paths)
	for _, p := range paths {
		fmt.Fprintln(ru.Out, p)
	}
	return nil
}
