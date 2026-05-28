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
		Short:   "List secret refs known to sq",
		Long: `List every ${scheme:path} placeholder found across the configured
source locations, one per line as "scheme:path". Output is derived from
the YAML config — no persistent index is maintained.

Entries in the OS keyring (or environment variables, or files) that no
source references are NOT listed; they're effectively orphans.`,
		RunE: execConfigSecretsLs,
	}
	return cmd
}

func execConfigSecretsLs(cmd *cobra.Command, _ []string) error {
	ru := run.FromContext(cmd.Context())

	seen := make(map[string]struct{})
	var refs []string
	for _, src := range ru.Config.Collection.Sources() {
		extracted, err := secret.ExtractRefs(src.Location)
		if err != nil {
			return err
		}
		for _, ref := range extracted {
			key := ref.Scheme + ":" + ref.Path
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			refs = append(refs, key)
		}
	}
	sort.Strings(refs)
	for _, r := range refs {
		fmt.Fprintln(ru.Out, r)
	}
	return nil
}
