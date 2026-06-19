package cli

import (
	"sort"

	"github.com/spf13/cobra"

	"github.com/neilotoole/sq/cli/output"
	"github.com/neilotoole/sq/cli/run"
	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/secret/keyring"
)

const flagPruneDryRun = "dry-run"

func newConfigKeyringPruneCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "prune",
		Args:  cobra.NoArgs,
		Short: "Delete orphaned keyring entries",
		Long: `Delete every keyring entry under the sq service that no source
references. An entry is an orphan when no source Location contains a
${keyring:PATH} placeholder naming it; hand-crafted references count, so
an entry wired into any source is never pruned.

Both sq-generated opaque IDs and user-named entries are pruned. Use
'sq config keyring ls' to review entries first, and --dry-run to preview
what prune would remove without deleting anything.`,
		RunE: execConfigKeyringPrune,
		Example: `  # Preview orphans without deleting
  $ sq config keyring prune --dry-run

  # Delete all orphaned entries
  $ sq config keyring prune`,
	}
	cmd.Flags().Bool(flagPruneDryRun, false, "Show orphans that would be deleted, make no changes")
	addKeyringFormatFlags(cmd)
	return cmd
}

func execConfigKeyringPrune(cmd *cobra.Command, _ []string) error {
	ru := run.FromContext(cmd.Context())
	kr := keyring.NewStore()

	stored, err := kr.List(cmd.Context())
	if err != nil {
		return err
	}

	referenced := make(map[string]struct{})
	for _, ref := range collectKeyringRefs(ru.Config.Collection.Sources()) {
		referenced[ref.Path] = struct{}{}
	}

	orphans := make([]string, 0, len(stored))
	for _, path := range stored {
		if _, ok := referenced[path]; !ok {
			orphans = append(orphans, path)
		}
	}
	sort.Strings(orphans)

	dryRun := cmdFlagIsSetTrue(cmd, flagPruneDryRun)
	rows := make([]output.KeyringPruneRow, 0, len(orphans))
	var failed int
	for _, path := range orphans {
		row := output.KeyringPruneRow{Path: path, Kind: output.KeyringKindNamed}
		if keyring.IsID(path) {
			row.Kind = output.KeyringKindID
		}
		switch {
		case dryRun:
			row.Status = output.KeyringPruneStatusPlanned
		default:
			if delErr := kr.Delete(cmd.Context(), path); delErr != nil {
				row.Status = output.KeyringPruneStatusFailed
				row.Error = delErr.Error()
				failed++
			} else {
				row.Status = output.KeyringPruneStatusDeleted
			}
		}
		rows = append(rows, row)
	}

	writeErr := ru.Writers.Keyring.Prune(rows, dryRun)
	var summaryErr error
	if failed > 0 {
		summaryErr = errz.Errorf("failed to delete %d of %d orphaned keyring entries", failed, len(orphans))
	}
	return errz.Append(writeErr, summaryErr)
}
