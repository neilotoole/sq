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

	"github.com/neilotoole/sq/cli/run"
	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/secret"
	"github.com/neilotoole/sq/libsq/core/secret/keyring"
	"github.com/neilotoole/sq/libsq/source"
)

const (
	flagMigrateAll    = "all"
	flagMigrateDryRun = "dry-run"
	flagMigrateYes    = "yes"
)

func newConfigSecretsMigrateCmd() *cobra.Command {
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
		RunE: execConfigSecretsMigrate,
		Example: `  # Preview the migration
  $ sq config secrets migrate --all --dry-run

  # Migrate every source without prompting
  $ sq config secrets migrate --all --yes

  # Migrate a single source
  $ sq config secrets migrate @sakila`,
	}
	cmd.Flags().Bool(flagMigrateAll, false, "Migrate every source")
	cmd.Flags().Bool(flagMigrateDryRun, false, "Show planned changes, make no writes")
	cmd.Flags().Bool(flagMigrateYes, false, "Skip the confirmation prompt")
	return cmd
}

type migratePlan struct {
	src    *source.Source
	reason string // populated when skipped
}

func execConfigSecretsMigrate(cmd *cobra.Command, args []string) error {
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
	printMigratePlans(ru.Out, plans)

	if cmdFlagIsSetTrue(cmd, flagMigrateDryRun) {
		return nil
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

	return applyMigratePlans(ctx, ru, plans)
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

func printMigratePlans(out io.Writer, plans []migratePlan) {
	for _, p := range plans {
		if p.reason != "" {
			fmt.Fprintf(out, "%s  skip   (%s)\n", p.src.Handle, p.reason)
			continue
		}
		fmt.Fprintf(out, "%s  ->     ${keyring:<new-id>}\n", p.src.Handle)
	}
}

func applyMigratePlans(ctx context.Context, ru *run.Run, plans []migratePlan) error {
	kr := keyring.New()
	var anyFailed bool
	for _, p := range plans {
		if p.reason != "" {
			continue
		}
		id, err := kr.NewID(ctx)
		if err != nil {
			fmt.Fprintf(ru.Out, "%s  FAIL   mint keyring id: %v\n", p.src.Handle, err)
			anyFailed = true
			continue
		}
		if err = kr.Set(ctx, id, p.src.Location); err != nil {
			fmt.Fprintf(ru.Out, "%s  FAIL   write keyring: %v\n", p.src.Handle, err)
			anyFailed = true
			continue
		}
		oldLoc := p.src.Location
		p.src.Location = "${keyring:" + id + "}"
		if err = ru.ConfigStore.Save(ctx, ru.Config); err != nil {
			p.src.Location = oldLoc
			_ = kr.Delete(ctx, id)
			fmt.Fprintf(ru.Out, "%s  FAIL   save config (rolled back): %v\n", p.src.Handle, err)
			anyFailed = true
			continue
		}
		fmt.Fprintf(ru.Out, "%s  done   ->  %s\n", p.src.Handle, p.src.Location)
	}
	if anyFailed {
		return errz.New("one or more sources failed to migrate")
	}
	return nil
}

// migrateSkipReason inspects loc and returns a non-empty reason string
// when the source should be skipped by migrate, or "" when it is a
// valid candidate. Skip rules, in order:
//   - Already contains a ${...} placeholder (idempotent re-runs).
//   - Not a URL (file paths, sqlite/Excel, etc. — nothing to migrate).
//   - URL has no password component (no secret to relocate).
func migrateSkipReason(loc string) string {
	if refs, _ := secret.ExtractRefs(loc); len(refs) > 0 {
		return "already has a placeholder"
	}
	u, err := url.Parse(loc)
	if err != nil || u.Scheme == "" || u.Host == "" {
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
