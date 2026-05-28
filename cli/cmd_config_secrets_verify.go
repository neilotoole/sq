package cli

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/neilotoole/sq/cli/run"
	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/secret"
)

func newConfigSecretsTestCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "test [@HANDLE]",
		Args:  cobra.RangeArgs(0, 1),
		Short: "Resolve every secret ref, report pass/fail",
		Long: `Diagnostic: for each source (or just one if a handle is given),
resolve every ${scheme:path} placeholder in its Location and report
whether the resolve succeeds.

Useful when keyring permissions change or to smoke-check that all
secrets are reachable before running a query.`,
		RunE: execConfigSecretsTest,
		Example: `  # Test all sources
  $ sq config secrets test --all

  # Test a single source
  $ sq config secrets test @sakila`,
	}
	cmd.Flags().Bool("all", false, "Test every source (mutually exclusive with @HANDLE)")
	return cmd
}

func execConfigSecretsTest(cmd *cobra.Command, args []string) error {
	ru := run.FromContext(cmd.Context())
	ctx := cmd.Context()
	reg := ru.SecretRegistry

	allFlag := cmdFlagIsSetTrue(cmd, "all")
	switch {
	case len(args) == 0 && !allFlag:
		return errz.New("specify @HANDLE or --all")
	case len(args) == 1 && allFlag:
		return errz.New("--all and @HANDLE are mutually exclusive")
	}

	var handles []string
	if len(args) == 1 {
		handles = []string{args[0]}
	} else {
		for _, src := range ru.Config.Collection.Sources() {
			handles = append(handles, src.Handle)
		}
	}

	var anyFailed bool
	for _, h := range handles {
		src, err := ru.Config.Collection.Get(h)
		if err != nil {
			fmt.Fprintf(ru.Out, "%s  ERROR  %v\n", h, err)
			anyFailed = true
			continue
		}
		if err := testSourceSecrets(ctx, reg, src.Location); err != nil {
			fmt.Fprintf(ru.Out, "%s  FAIL   %v\n", h, err)
			anyFailed = true
			continue
		}
		fmt.Fprintf(ru.Out, "%s  OK\n", h)
	}

	if anyFailed {
		return errz.New("one or more sources failed secret resolution")
	}
	return nil
}

func testSourceSecrets(ctx context.Context, reg *secret.Registry, loc string) error {
	refs, err := secret.ExtractRefs(loc)
	if err != nil {
		return err
	}
	for _, ref := range refs {
		if _, err := reg.ResolveScheme(ctx, ref.Scheme, ref.Path); err != nil {
			return fmt.Errorf("${%s:%s}: %w", ref.Scheme, ref.Path, err)
		}
	}
	return nil
}
