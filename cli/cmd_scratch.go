package cli

import (
	"github.com/neilotoole/sq/drivers/sqlite3"
	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/source"

	"github.com/spf13/cobra"
)

// TODO: dump all this "internal" stuff: make the options as follows: @HANDLE, file, memory

func newScratchCmd() (*cobra.Command, runFunc) {
	cmd := &cobra.Command{
		Use: "scratch [@HANDLE|internal|internal:file|internal:mem|@scratch]",
		// This command is likely to be ditched in favor of a generalized "config" cmd
		// such as "sq config scratchdb=@my1"
		Hidden: true,
		Example: `   # get scratch data source
   $ sq scratch
   # set @my1 as scratch data source
   $ sq scratch @my1
   # use the default embedded db
   $ sq scratch internal
   # explicitly specify use of embedded file db
   $ sq scratch internal:file
   # explicitly specify use of embedded memory db
   $ sq scratch internal:mem
   # restore default scratch db (equivalent to "internal")
   $ sq scratch @scratch`,
		Short: "Get or set scratch data source",
		Long: `Get or set scratch data source. The scratch db is used internally by sq for multiple purposes such as
importing non-SQL data, or cross-database joins. If no argument provided, get the current scratch data
source. Otherwise, set @HANDLE or an internal db as the scratch data source. The reserved handle "@scratch" resets the
`,
	}

	return cmd, execScratch
}

func execScratch(rc *RunContext, cmd *cobra.Command, args []string) error {
	if len(args) > 1 {
		return errz.Errorf(msgInvalidArgs)
	}

	cfg := rc.Config

	var src *source.Source
	var err error
	defaultScratch := &source.Source{
		Handle:   source.ScratchHandle,
		Location: "internal:file",
		Type:     sqlite3.Type,
	}

	if len(args) == 0 {
		// Print the scratch src
		src = cfg.Sources.Scratch()
		if src == nil {
			src = defaultScratch
		}

		return rc.writers.srcw.Source(src)
	}

	// Set the scratch src

	switch args[0] {
	case "internal", "internal:file", "internal:mem":
		// TODO: currently only supports file sqlite3 db, fairly trivial to do mem as well
		_, _ = cfg.Sources.SetScratch("")
		src = defaultScratch
	default:
		src, err = cfg.Sources.SetScratch(args[0])
		if err != nil {
			return err
		}
	}

	err = rc.ConfigStore.Save(rc.Config)
	if err != nil {
		return err
	}

	return rc.writers.srcw.Source(src)
}
