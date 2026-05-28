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
	"github.com/neilotoole/sq/libsq/source/location"
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
		Short: "Migrate inline passwords to the keyring",
		Long: `For each source (or one specified by handle), extract any
inline URL password from its Location, write it to the OS keyring at
sq/<handle>/password, and replace the inline password with
${keyring:<handle>/password}.

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
	src         *source.Source
	newLocation string
	password    string // URL-decoded
	reason      string // populated when skipped
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
		p := migratePlan{src: s}
		refs, refsErr := secret.ExtractRefs(s.Location)
		switch {
		case refsErr == nil && len(refs) > 0:
			p.reason = "already has a placeholder"
		default:
			pw, withoutPW, ok := decomposeURLForMigrate(s.Location)
			switch {
			case !ok && !looksLikeURL(s.Location):
				p.reason = "not a URL"
			case !ok:
				p.reason = "no password component"
			default:
				placeholder := fmt.Sprintf("${keyring:%s/password}", s.Handle)
				newLoc, err := location.WithPasswordPlaceholder(withoutPW, placeholder)
				if err != nil {
					p.reason = fmt.Sprintf("could not rewrite location: %v", err)
				} else {
					p.newLocation = newLoc
					p.password = pw
				}
			}
		}
		out = append(out, p)
	}
	return out
}

func printMigratePlans(out io.Writer, plans []migratePlan) {
	for _, p := range plans {
		if p.reason != "" {
			fmt.Fprintf(out, "%s  skip   (%s)\n", p.src.Handle, p.reason)
			continue
		}
		fmt.Fprintf(out, "%s  ->     %s\n", p.src.Handle, p.newLocation)
	}
}

func applyMigratePlans(ctx context.Context, ru *run.Run, plans []migratePlan) error {
	kr := keyring.New()
	var anyFailed bool
	for _, p := range plans {
		if p.reason != "" {
			continue
		}
		krPath := p.src.Handle + "/password"
		if err := kr.Set(ctx, krPath, p.password); err != nil {
			fmt.Fprintf(ru.Out, "%s  FAIL   write keyring: %v\n", p.src.Handle, err)
			anyFailed = true
			continue
		}
		oldLoc := p.src.Location
		p.src.Location = p.newLocation
		if err := ru.ConfigStore.Save(ctx, ru.Config); err != nil {
			p.src.Location = oldLoc
			_ = kr.Delete(ctx, krPath)
			fmt.Fprintf(ru.Out, "%s  FAIL   save config (rolled back): %v\n", p.src.Handle, err)
			anyFailed = true
			continue
		}
		fmt.Fprintf(ru.Out, "%s  done\n", p.src.Handle)
	}
	if anyFailed {
		return errz.New("one or more sources failed to migrate")
	}
	return nil
}

// decomposeURLForMigrate splits loc into its URL-decoded password and a
// version of loc with the password removed. ok is true only when loc
// looks like a URL with userinfo containing a password.
func decomposeURLForMigrate(loc string) (password, locWithoutPW string, ok bool) {
	u, err := url.Parse(loc)
	if err != nil || u.Scheme == "" {
		return "", loc, false
	}
	if u.User == nil {
		return "", loc, false
	}
	pw, has := u.User.Password()
	if !has {
		return "", loc, false
	}
	u.User = url.User(u.User.Username())
	return pw, u.String(), true
}

// looksLikeURL returns true when loc has both a scheme and a host, i.e.
// it is a network URL rather than a local file path.
func looksLikeURL(loc string) bool {
	u, err := url.Parse(loc)
	return err == nil && u.Scheme != "" && u.Host != ""
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
