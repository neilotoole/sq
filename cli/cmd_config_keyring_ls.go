package cli

import (
	"sort"

	"github.com/spf13/cobra"

	"github.com/neilotoole/sq/cli/flag"
	"github.com/neilotoole/sq/cli/output"
	"github.com/neilotoole/sq/cli/run"
	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/secret"
	"github.com/neilotoole/sq/libsq/core/secret/keyring"
	"github.com/neilotoole/sq/libsq/source"
)

func newConfigKeyringLsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "ls",
		Aliases: []string{"list"},
		Args:    cobra.NoArgs,
		Short:   "List keyring entries and their status",
		Long: `List every keyring entry under the sq service, reconciled against
the ${keyring:<path>} placeholders referenced by sources.

Output columns:
  STATUS    referenced | orphan | missing (see below).
  PATH      The keyring entry path, e.g. "j2k7m3pxtz".
  HANDLE    The @handle that references the entry (blank for orphans).
  DRIVER    The source's driver type (blank for orphans).

Status values:
  referenced  Present in the keyring and referenced by a source.
  orphan      Present in the keyring, referenced by no source. Remove
              with 'sq config keyring prune'.
  missing     Referenced by a source but absent from the keyring; that
              source will fail to resolve its secret at connect time.

A path referenced by multiple sources yields one row per source. Only
${keyring:...} refs are reconciled; ${env:...} and ${file:...} are
external and not listed.`,
		RunE: execConfigKeyringLs,
	}
	addKeyringFormatFlags(cmd)
	return cmd
}

func execConfigKeyringLs(cmd *cobra.Command, _ []string) error {
	ru := run.FromContext(cmd.Context())
	refs, err := collectKeyringRefs(ru.Config.Collection.Sources())
	if err != nil {
		return err
	}
	stored, err := keyring.NewStore().List(cmd.Context())
	if err != nil {
		return err
	}
	rows := reconcileKeyringEntries(refs, stored)
	return ru.Writers.Keyring.List(rows)
}

// reconcileKeyringEntries builds the unified ls rows by classifying each
// keyring path. A referenced path present in the keyring is "referenced";
// a referenced path absent from the keyring is "missing"; a stored path
// that no source references is an "orphan". Referenced/missing rows keep
// their handle and driver (one row per referencing source); orphan rows
// have neither. Rows are sorted referenced, then orphan, then missing,
// breaking ties by path then handle.
func reconcileKeyringEntries(refs []output.KeyringRef, stored []string) []output.KeyringRef {
	storedSet := make(map[string]struct{}, len(stored))
	for _, s := range stored {
		storedSet[s] = struct{}{}
	}
	referencedSet := make(map[string]struct{}, len(refs))

	rows := make([]output.KeyringRef, 0, len(refs)+len(stored))
	for _, r := range refs {
		referencedSet[r.Path] = struct{}{}
		r.Status = output.KeyringStatusReferenced
		if _, ok := storedSet[r.Path]; !ok {
			r.Status = output.KeyringStatusMissing
		}
		rows = append(rows, r)
	}
	for _, s := range stored {
		if _, ok := referencedSet[s]; ok {
			continue
		}
		rows = append(rows, output.KeyringRef{Status: output.KeyringStatusOrphan, Path: s})
	}

	sort.Slice(rows, func(i, j int) bool {
		if ri, rj := keyringStatusRank(rows[i].Status), keyringStatusRank(rows[j].Status); ri != rj {
			return ri < rj
		}
		if rows[i].Path != rows[j].Path {
			return rows[i].Path < rows[j].Path
		}
		return rows[i].Handle < rows[j].Handle
	})
	return rows
}

// keyringStatusRank orders statuses for display: referenced first (the
// healthy common case), then orphan, then missing.
func keyringStatusRank(status string) int {
	switch status {
	case output.KeyringStatusReferenced:
		return 0
	case output.KeyringStatusOrphan:
		return 1
	case output.KeyringStatusMissing:
		return 2
	default:
		return 3
	}
}

// collectKeyringRefs extracts (path, handle, driver) rows from srcs,
// sorted by path then handle so shared paths cluster on adjacent rows.
// Only ${keyring:...} refs are included; env and file refs are ignored.
// No deduplication: a shared path yields one row per referencing source,
// which is how sharing becomes visible in the output.
//
// Returns an error if any source has a malformed placeholder in its
// Location. A malformed Location causes ExtractRefs to discard all refs
// from that Location, so continuing silently would produce an incomplete
// referenced set. For destructive operations such as prune, an incomplete
// set means live, referenced entries could be misclassified as orphans
// and deleted. Hard-failing here prevents that data loss.
func collectKeyringRefs(srcs []*source.Source) ([]output.KeyringRef, error) {
	var rows []output.KeyringRef
	for _, src := range srcs {
		refs, err := secret.ExtractRefs(src.Location)
		if err != nil {
			return nil, errz.Wrapf(err, "source %s has a malformed placeholder in its location", src.Handle)
		}
		for _, ref := range refs {
			if ref.Scheme != "keyring" {
				continue
			}
			rows = append(rows, output.KeyringRef{
				Path:   ref.Path,
				Handle: src.Handle,
				Driver: string(src.Type),
			})
		}
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Path != rows[j].Path {
			return rows[i].Path < rows[j].Path
		}
		return rows[i].Handle < rows[j].Handle
	})
	return rows, nil
}

// addKeyringFormatFlags registers the output-format + header flags
// supported by the keyring subcommands: --text/--json (-j) for format
// selection, plus --header (-h) / --no-header (-H) for header control
// (mirrors the slq/sql ergonomic of accepting both forms; mutually
// exclusive).
func addKeyringFormatFlags(cmd *cobra.Command) {
	cmd.Flags().BoolP(flag.Text, flag.TextShort, false, flag.TextUsage)
	cmd.Flags().BoolP(flag.JSON, flag.JSONShort, false, flag.JSONUsage)
	cmd.Flags().BoolP(flag.Header, flag.HeaderShort, true, flag.HeaderUsage)
	cmd.Flags().BoolP(flag.NoHeader, flag.NoHeaderShort, false, flag.NoHeaderUsage)
	cmd.MarkFlagsMutuallyExclusive(flag.Header, flag.NoHeader)
}
