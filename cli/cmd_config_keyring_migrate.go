package cli

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"strings"

	"github.com/spf13/cobra"

	"github.com/neilotoole/sq/cli/output"
	"github.com/neilotoole/sq/cli/output/format"
	"github.com/neilotoole/sq/cli/run"
	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/lg"
	"github.com/neilotoole/sq/libsq/core/lg/lga"
	"github.com/neilotoole/sq/libsq/core/secret"
	"github.com/neilotoole/sq/libsq/core/secret/keyring"
	"github.com/neilotoole/sq/libsq/source"
)

const (
	flagMigrateAll    = "all"
	flagMigrateDryRun = "dry-run"
	flagMigrateYes    = "yes"
)

func newConfigKeyringMigrateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "migrate [@HANDLE]",
		Args:  cobra.RangeArgs(0, 1),
		Short: "Migrate inline-credential sources to the keyring",
		Long: `For each source (or one specified by handle), write its
Location URL to the OS keyring at a fresh opaque ID and replace the
Location with a bare ${keyring:<id>} placeholder. The driver type stays
in the driver: field; the keyring entry holds the entire DSN.

Sources skipped automatically:
  - Non-URL locations (file paths, sqlite, Excel, etc.)
  - URLs with no password component
  - Locations that already contain a ${...} placeholder

Use --dry-run to preview without making any changes. Use --yes to skip
the confirmation prompt.`,
		RunE: execConfigKeyringMigrate,
		Example: `  # Preview the migration
  $ sq config keyring migrate --all --dry-run

  # Migrate every source without prompting
  $ sq config keyring migrate --all --yes

  # Migrate a single source
  $ sq config keyring migrate @sakila`,
	}
	cmd.Flags().Bool(flagMigrateAll, false, "Migrate every source")
	cmd.Flags().Bool(flagMigrateDryRun, false, "Show planned changes, make no writes")
	cmd.Flags().Bool(flagMigrateYes, false, "Skip the confirmation prompt")
	addKeyringFormatFlags(cmd)
	return cmd
}

type migratePlan struct {
	src    *source.Source
	reason string // populated when skipped
}

func execConfigKeyringMigrate(cmd *cobra.Command, args []string) error {
	ru := run.FromContext(cmd.Context())
	ctx := cmd.Context()

	if len(args) == 0 && !cmdFlagIsSetTrue(cmd, flagMigrateAll) {
		return errz.New("specify @HANDLE or --all")
	}

	srcs, err := selectMigrateSources(ru, args)
	if err != nil {
		return err
	}

	plans := buildMigratePlans(srcs)

	dryRun := cmdFlagIsSetTrue(cmd, flagMigrateDryRun)
	if dryRun {
		return ru.Writers.Keyring.Migrate(planRowsForReport(plans), true)
	}

	// Non-dry-run: print the plan in text mode so the user can see what
	// will happen before the prompt. In JSON mode, skip the pre-prompt
	// preview — the consumer reads a single envelope after apply.
	if outputFormatIsJSON(ru) {
		// JSON: skip preview, skip confirmation prompt, apply directly.
		// JSON callers are non-interactive; --yes is implied.
		rows, err := applyMigratePlans(ctx, ru, plans)
		writerErr := ru.Writers.Keyring.Migrate(rows, false)
		if err != nil {
			return err
		}
		return writerErr
	}

	if err := ru.Writers.Keyring.Migrate(planRowsForReport(plans), true); err != nil {
		return err
	}

	if !cmdFlagIsSetTrue(cmd, flagMigrateYes) {
		ok, err := promptYesNo(ru.Stdin, ru.Out, "Proceed with migration?")
		if err != nil {
			return err
		}
		if !ok {
			return nil
		}
	}

	rows, applyErr := applyMigratePlans(ctx, ru, plans)
	if writerErr := ru.Writers.Keyring.Migrate(rows, false); writerErr != nil && applyErr == nil {
		return writerErr
	}
	return applyErr
}

func selectMigrateSources(ru *run.Run, args []string) ([]*source.Source, error) {
	if len(args) == 1 {
		s, err := ru.Config.Collection.Get(args[0])
		if err != nil {
			return nil, err
		}
		return []*source.Source{s}, nil
	}
	return ru.Config.Collection.Sources(), nil
}

func buildMigratePlans(srcs []*source.Source) []migratePlan {
	out := make([]migratePlan, 0, len(srcs))
	for _, s := range srcs {
		out = append(out, migratePlan{src: s, reason: migrateSkipReason(s.Location)})
	}
	return out
}

// planRowsForReport converts plans into writer rows for a dry-run /
// pre-apply preview: each plan is either a "skip" with a reason or a
// "planned" migration.
func planRowsForReport(plans []migratePlan) []output.KeyringMigrateRow {
	rows := make([]output.KeyringMigrateRow, 0, len(plans))
	for _, p := range plans {
		if p.reason != "" {
			rows = append(rows, output.KeyringMigrateRow{
				Handle: p.src.Handle,
				Status: output.KeyringMigrateStatusSkip,
				Reason: p.reason,
			})
			continue
		}
		rows = append(rows, output.KeyringMigrateRow{
			Handle: p.src.Handle,
			Status: output.KeyringMigrateStatusPlanned,
		})
	}
	return rows
}

// applyMigratePlans performs the migration and returns one row per
// non-skipped plan describing the outcome. Skipped plans don't appear
// in the result because they were already reported during the plan
// phase. The returned error is non-nil if at least one source failed
// to migrate; rows for both successes and failures are still returned.
func applyMigratePlans(ctx context.Context, ru *run.Run, plans []migratePlan) (
	[]output.KeyringMigrateRow, error,
) {
	kr := keyring.NewStore()
	rows := make([]output.KeyringMigrateRow, 0, len(plans))
	var anyFailed bool
	for _, p := range plans {
		if p.reason != "" {
			continue
		}
		id, err := kr.NewID(ctx)
		if err != nil {
			rows = append(rows, output.KeyringMigrateRow{
				Handle: p.src.Handle,
				Status: output.KeyringMigrateStatusFailed,
				Error:  "mint keyring id: " + err.Error(),
			})
			anyFailed = true
			continue
		}
		// The stored Location is a placeholder template in which '$$'
		// escapes a literal '$' (e.g. written by the v0.54.0 config
		// upgrade). The keyring slot holds a literal value that
		// Registry.Expand splices raw at connect time, so unescape
		// here; storing the template bytes verbatim would hand the
		// driver a wrong (still-escaped) credential. Safe because
		// migrateSkipReason guarantees zero placeholder refs.
		if err = kr.Set(ctx, id, secret.Unescape(p.src.Location)); err != nil {
			rows = append(rows, output.KeyringMigrateRow{
				Handle: p.src.Handle,
				Status: output.KeyringMigrateStatusFailed,
				Error:  "write keyring: " + err.Error(),
			})
			anyFailed = true
			continue
		}
		oldLoc := p.src.Location
		p.src.Location = "${keyring:" + id + "}"
		if err = ru.ConfigStore.Save(ctx, ru.Config); err != nil {
			p.src.Location = oldLoc
			if delErr := kr.Delete(ctx, id); delErr != nil {
				// Rollback failed: the keyring entry just written may
				// orphan, with no easy way for the user to find it
				// (orphan-listing is pending — see #715). Log so the
				// failure is at least recoverable from debug output.
				lg.FromContext(ctx).Warn("Failed to roll back keyring entry on migrate save error",
					lga.Path, id, lga.Handle, p.src.Handle, lga.Err, delErr)
			}
			rows = append(rows, output.KeyringMigrateRow{
				Handle: p.src.Handle,
				Status: output.KeyringMigrateStatusFailed,
				Error:  "save config (rolled back): " + err.Error(),
			})
			anyFailed = true
			continue
		}
		rows = append(rows, output.KeyringMigrateRow{
			Handle:      p.src.Handle,
			Status:      output.KeyringMigrateStatusMigrated,
			NewLocation: p.src.Location,
		})
	}
	if anyFailed {
		return rows, errz.New("one or more sources failed to migrate")
	}
	return rows, nil
}

// outputFormatIsJSON reports whether the resolved output format is JSON,
// whether selected via the --json flag or the config "format" option. It
// must agree with the writer selection in newWriters, which keys off the
// same resolved format (see getFormat): whenever the JSON keyring writer
// is in play, the command must behave non-interactively.
func outputFormatIsJSON(ru *run.Run) bool {
	if ru == nil || ru.Config == nil {
		return false
	}
	return getFormat(ru.Cmd, ru.Config.Options) == format.JSON
}

// migrateSkipReason inspects loc and returns a non-empty reason string
// when the source should be skipped by migrate, or "" when it is a
// valid candidate. Skip rules, in order:
//   - Malformed placeholder syntax (don't compound the broken state by
//     stamping the Location into the keyring).
//   - Already contains a ${...} placeholder (idempotent re-runs).
//   - Not a URL (file paths, sqlite/Excel, etc. — nothing to migrate).
//   - URL has no password component (no secret to relocate).
func migrateSkipReason(loc string) string {
	refs, refsErr := secret.ExtractRefs(loc)
	switch {
	case refsErr != nil:
		// Surface the parse error explicitly rather than silently
		// treating a malformed placeholder as "no placeholder" and
		// proceeding to write the broken Location into the keyring.
		return "malformed placeholder syntax: " + refsErr.Error()
	case len(refs) > 0:
		return "already has a placeholder"
	}
	u, err := url.Parse(loc)
	if err != nil {
		// A genuinely malformed DSN should not be silently classified
		// as "not a URL" — the user needs to see WHY their source was
		// skipped. Extract just the parse error's reason (url.Error's
		// Err field) to avoid echoing the loc string itself, which
		// could contain inline credentials.
		msg := "parse failed"
		var ue *url.Error
		if errors.As(err, &ue) && ue.Err != nil {
			msg = ue.Err.Error()
		}
		return "not a URL: " + msg
	}
	if u.Scheme == "" || u.Host == "" {
		return "not a URL"
	}
	if u.User == nil {
		return "no password component"
	}
	if _, has := u.User.Password(); !has {
		return "no password component"
	}
	return ""
}

// promptYesNo writes prompt to out and reads a y/n response from in.
// Returns true on "y"/"yes" (case-insensitive); false on anything else
// or EOF (so just pressing Enter answers "no", matching the [y/N]
// default).
func promptYesNo(in io.Reader, out io.Writer, prompt string) (bool, error) {
	fmt.Fprintf(out, "%s [y/N] ", prompt)
	line, err := bufio.NewReader(in).ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return false, err
	}
	resp := strings.ToLower(strings.TrimSpace(line))
	return resp == "y" || resp == "yes", nil
}
