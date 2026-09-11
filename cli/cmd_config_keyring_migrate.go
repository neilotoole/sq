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
		// migrate's optional arg is a source handle (not a keyring path),
		// so it completes handles, like the other handle-taking commands.
		ValidArgsFunction: completeHandle(1, true),
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
	// migrate rewrites sq.yml (unlike the other keyring subcommands, which
	// touch only the keyring), so it takes the config lock and runs against
	// a freshly-reloaded config, like add/rm/mv and the other config writers.
	cmdMarkRequiresConfigLock(cmd)
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

	// Nothing actionable (e.g. a collection of file sources with no inline
	// credentials): report and stop without prompting or applying.
	actionable := 0
	for _, p := range plans {
		if p.reason == "" {
			actionable++
		}
	}
	if actionable == 0 {
		return ru.Writers.Keyring.Migrate(planRowsForReport(plans), true)
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
			// Declining is a deliberate non-success: nothing is migrated, so
			// exit non-zero (like apt/dnf "Abort.") rather than report success.
			return errz.New("migration cancelled")
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

// applyMigratePlans performs the migration atomically. It writes every
// eligible source's keyring entry and rewrites its Location in memory,
// then saves the config exactly once. If any step fails (minting an ID,
// writing the keyring, or the single config save), the whole batch is
// rolled back: every keyring entry written this run is deleted, every
// Location is restored, and the config is left untouched. So a run is
// all-or-nothing; a failure migrates no sources. Skipped plans don't
// appear in the result because they were already reported during the
// plan phase.
func applyMigratePlans(ctx context.Context, ru *run.Run, plans []migratePlan) (
	[]output.KeyringMigrateRow, error,
) {
	kr := keyring.NewStore()

	// done tracks each source whose keyring entry was written and Location
	// rewritten in memory, so a later failure can roll the whole batch back.
	type applied struct {
		src *source.Source
		id  string
		old string
	}
	var done []applied

	rollbackAll := func() {
		for _, a := range done {
			a.src.Location = a.old
			if delErr := kr.Delete(ctx, a.id); delErr != nil {
				// Rollback delete failed: the keyring entry written this
				// run may orphan. Log it so the failure is recoverable
				// from debug output and via 'sq config keyring prune'.
				lg.FromContext(ctx).Warn("Failed to roll back keyring entry during migrate rollback",
					lga.Path, a.id, lga.Handle, a.src.Handle, lga.Err, delErr)
			}
		}
	}

	// failAll rolls the batch back and reports every eligible source as
	// failed (rolled back): migration is all-or-nothing, so on any failure
	// nothing is persisted.
	failAll := func(cause string) ([]output.KeyringMigrateRow, error) {
		rollbackAll()
		rows := make([]output.KeyringMigrateRow, 0, len(done))
		for _, p := range plans {
			if p.reason != "" {
				continue
			}
			rows = append(rows, output.KeyringMigrateRow{
				Handle: p.src.Handle,
				Status: output.KeyringMigrateStatusFailed,
				Error:  "rolled back: " + cause,
			})
		}
		return rows, errz.Errorf(
			"migration failed and was rolled back; no sources were changed: %s", cause,
		)
	}

	for _, p := range plans {
		if p.reason != "" {
			continue
		}
		id, err := kr.NewID(ctx)
		if err != nil {
			return failAll("mint keyring id for " + p.src.Handle + ": " + err.Error())
		}
		// The stored Location is a placeholder template in which '$$'
		// escapes a literal '$' (e.g. written by the v0.54.0 config
		// upgrade). The keyring slot holds a literal value that
		// Registry.Expand splices raw at connect time, so unescape
		// here; storing the template bytes verbatim would hand the
		// driver a wrong (still-escaped) credential. Safe because
		// migrateSkipReason guarantees zero placeholder refs.
		if err = kr.Set(ctx, id, secret.Unescape(p.src.Location)); err != nil {
			return failAll("write keyring for " + p.src.Handle + ": " + err.Error())
		}
		done = append(done, applied{src: p.src, id: id, old: p.src.Location})
		p.src.Location = "${keyring:" + id + "}"
	}

	if len(done) == 0 {
		// Nothing was eligible (e.g. JSON mode on an all-skipped collection,
		// where the text-mode short-circuit doesn't apply): don't rewrite
		// sq.yml for a no-op.
		return nil, nil
	}

	if err := ru.ConfigStore.Save(ctx, ru.Config); err != nil {
		return failAll("save config: " + err.Error())
	}

	rows := make([]output.KeyringMigrateRow, 0, len(done))
	for _, a := range done {
		rows = append(rows, output.KeyringMigrateRow{
			Handle:      a.src.Handle,
			Status:      output.KeyringMigrateStatusMigrated,
			NewLocation: a.src.Location,
		})
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
		// A genuinely malformed DSN should not be silently treated as a
		// credential-less source. Surface the parse error's reason so the
		// user sees why the source was skipped. Use only url.Error's Err
		// field, not the loc string, which could contain inline credentials.
		msg := "parse failed"
		var ue *url.Error
		if errors.As(err, &ue) && ue.Err != nil {
			msg = ue.Err.Error()
		}
		return "malformed location: " + msg
	}
	if u.Scheme == "" {
		// Not a connection URL (a file path, document source, and so on):
		// there are no inline credentials to relocate. A scheme-bearing URL
		// with an empty host still falls through to the password checks below,
		// so one that carries a password is migrated rather than skipped.
		return "no credentials to migrate"
	}
	if u.User == nil {
		return "no password to migrate"
	}
	if _, has := u.User.Password(); !has {
		return "no password to migrate"
	}
	return ""
}

// promptYesNo writes prompt to out and reads a single y/n response from in.
// "y"/"yes" returns true; "n"/"no", or an empty line accepting the [y/N]
// default, returns false. Matching is case-insensitive and trims surrounding
// whitespace. Any other input is an error (it does not retry), as is EOF with
// no answer given, so the caller exits non-zero rather than treating it as a
// silent "no".
func promptYesNo(in io.Reader, out io.Writer, prompt string) (bool, error) {
	fmt.Fprintf(out, "%s [y/N] ", prompt)
	line, err := bufio.NewReader(in).ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return false, errz.Err(err)
	}

	resp := strings.TrimSpace(line)
	switch strings.ToLower(resp) {
	case "y", "yes":
		return true, nil
	case "n", "no":
		return false, nil
	case "":
		// An empty line (the user pressed Enter) accepts the [y/N] default
		// of No. An empty read at EOF means no answer was given at all.
		if errors.Is(err, io.EOF) {
			return false, errz.New("no response to prompt")
		}
		return false, nil
	default:
		return false, errz.Errorf("unrecognized response to prompt: %q", resp)
	}
}
