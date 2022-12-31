package cli

import (
	"github.com/neilotoole/sq/libsq/source"
	"github.com/samber/lo"
	"github.com/spf13/cobra"
)

func newSrcRemoveCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use: "rm @HANDLE1 [@HANDLE2...]",
		Example: `  # Remove @my1 data source
  $ sq rm @my1

  # Remove multiple data sources
  $ sq rm @my1 @pg1 @sqlserver1`,
		Short:             "Remove data source",
		Long:              "Remove data source.",
		Args:              cobra.MinimumNArgs(1),
		RunE:              execSrcRemove,
		ValidArgsFunction: completeHandle(0),
	}

	cmd.Flags().BoolP(flagJSON, flagJSONShort, false, flagJSONUsage)

	return cmd
}

func execSrcRemove(cmd *cobra.Command, args []string) error {
	rc := RunContextFrom(cmd.Context())
	cfg := rc.Config

	args = lo.Uniq(args)
	srcs := make([]*source.Source, len(args))
	for i := range args {
		src, err := cfg.Sources.Get(args[i])
		if err != nil {
			return err
		}

		err = cfg.Sources.Remove(src.Handle)
		if err != nil {
			return err
		}

		srcs[i] = src
	}

	if err := rc.ConfigStore.Save(cfg); err != nil {
		return err
	}

	return rc.writers.srcw.Removed(srcs...)
}
