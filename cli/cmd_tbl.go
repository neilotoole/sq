package cli

import (
	"database/sql"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/neilotoole/sq/cli/flag"
	"github.com/neilotoole/sq/cli/run"
	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/stringz"
	"github.com/neilotoole/sq/libsq/core/tablefq"
	"github.com/neilotoole/sq/libsq/driver"
	"github.com/neilotoole/sq/libsq/source"
)

func newTblCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tbl",
		Short: "Useful table actions (copy, truncate, drop)",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
		Example: `  # Copy table actor to new table actor2
  $ sq tbl copy @sakila_sl3.actor actor2

  # Truncate table actor2
  $ sq tbl truncate @sakila_sl3.actor2

  # Drop table actor2
  $ sq tbl drop @sakila_sl3.actor2`,
	}

	return cmd
}

func newTblCopyCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:               "copy @HANDLE.TABLE NEWTABLE",
		Short:             "Make a copy of a table",
		Long:              `Make a copy of a table in the same database. The table data is also copied by default.`,
		ValidArgsFunction: completeTblCopy,
		RunE:              execTblCopy,
		Example: `  # Copy table "actor" in @sakila_sl3 to new table "actor2"
  $ sq tbl copy @sakila_sl3.actor .actor2

  # Copy table "actor" in active src to table "actor2"
  $ sq tbl copy .actor .actor2

  # Copy table "actor" in active src to generated table name (e.g. "@sakila_sl3.actor_copy__1ae03e9b")
  $ sq tbl copy .actor

  # Copy table structure, but don't copy table data
  $ sq tbl copy --data=false .actor`,
	}

	addTextFormatFlags(cmd)
	cmd.Flags().BoolP(flag.JSON, flag.JSONShort, false, flag.JSONUsage)
	cmd.Flags().BoolP(flag.Compact, flag.CompactShort, false, flag.CompactUsage)
	cmd.Flags().Bool(flag.TblData, true, flag.TblDataUsage)

	return cmd
}

func execTblCopy(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	ru := run.FromContext(ctx)
	if len(args) == 0 || len(args) > 2 {
		return errz.New("one or two table args required")
	}

	tblHandles, err := parseTableHandleArgs(ru.DriverRegistry, ru.Config.Collection, args)
	if err != nil {
		return err
	}

	if tblHandles[0].tbl == "" {
		return errz.Errorf("arg {%s} does not specify a table name", args[0])
	}

	switch len(tblHandles) {
	case 1:
		// Make a copy of the first tbl handle
		tblHandles = append(tblHandles, tblHandles[0])
		// But we can't copy the table to itself, so we create a new name
		tblHandles[1].tbl = stringz.UniqTableName(tblHandles[0].tbl + "_copy")
	case 2:
		if tblHandles[1].tbl == "" {
			tblHandles[1].tbl = stringz.UniqTableName(tblHandles[0].tbl + "_copy")
		}
	default:
		return errz.New("one or two table args required")
	}

	if tblHandles[0].src.Handle != tblHandles[1].src.Handle {
		return errz.Errorf("tbl copy only works on the same source, but got %s.%s --> %s.%s",
			tblHandles[0].handle, tblHandles[0].tbl,
			tblHandles[1].handle, tblHandles[1].tbl)
	}

	if tblHandles[0].tbl == tblHandles[1].tbl {
		return errz.Errorf("cannot copy table %s.%s to itself", tblHandles[0].handle, tblHandles[0].tbl)
	}

	sqlDrvr, ok := tblHandles[0].drvr.(driver.SQLDriver)
	if !ok {
		return errz.Errorf("driver type {%s} (%s) doesn't support dropping tables", tblHandles[0].src.Type,
			tblHandles[0].src.Handle)
	}

	copyData := true // copy data by default
	if cmdFlagChanged(cmd, flag.TblData) {
		copyData, err = cmd.Flags().GetBool(flag.TblData)
		if err != nil {
			return errz.Err(err)
		}
	}

	if err = applyCollectionOptions(cmd, ru.Config.Collection); err != nil {
		return err
	}

	var pool driver.Pool
	pool, err = ru.Sources.Open(ctx, tblHandles[0].src)
	if err != nil {
		return err
	}

	db, err := pool.DB(ctx)
	if err != nil {
		return err
	}

	fromTbl := tablefq.New(tblHandles[0].tbl)
	toTbl := tablefq.New(tblHandles[1].tbl)

	copied, err := sqlDrvr.CopyTable(ctx, db, fromTbl, toTbl, copyData)
	if err != nil {
		return errz.Wrapf(err, "failed tbl copy %s.%s --> %s.%s",
			tblHandles[0].handle, tblHandles[0].tbl,
			tblHandles[1].handle, tblHandles[1].tbl)
	}

	msg := fmt.Sprintf("Copied table: %s.%s --> %s.%s",
		tblHandles[0].handle, tblHandles[0].tbl,
		tblHandles[1].handle, tblHandles[1].tbl)

	if copyData {
		switch copied {
		case 1:
			msg += " (1 row copied)"
		default:
			msg += fmt.Sprintf(" (%d rows copied)", copied)
		}
	}

	fmt.Fprintln(ru.Out, msg)
	return nil
}

func newTblTruncateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "truncate @HANDLE.TABLE|.TABLE",
		Short: "Truncate one or more tables",
		Long: `Truncate one or more tables. Note that this command
only applies to SQL sources.`,
		RunE: execTblTruncate,
		ValidArgsFunction: (&handleTableCompleter{
			onlySQL: true,
		}).complete,
		Example: `  # truncate table "actor"" in source @sakila_sl3
  $ sq tbl truncate @sakila_sl3.actor

  # truncate table "payment"" in the active src
  $ sq tbl truncate .payment

  # truncate multiple tables
  $ sq tbl truncate .payment @sakila_sl3.actor`,
	}

	addTextFormatFlags(cmd)
	cmd.Flags().BoolP(flag.JSON, flag.JSONShort, false, flag.JSONUsage)

	return cmd
}

func execTblTruncate(cmd *cobra.Command, args []string) (err error) {
	ru := run.FromContext(cmd.Context())
	var tblHandles []tblHandle
	tblHandles, err = parseTableHandleArgs(ru.DriverRegistry, ru.Config.Collection, args)
	if err != nil {
		return err
	}

	if err = applyCollectionOptions(cmd, ru.Config.Collection); err != nil {
		return err
	}

	for _, tblH := range tblHandles {
		var affected int64
		affected, err = tblH.drvr.Truncate(cmd.Context(), tblH.src, tblH.tbl, true)
		if err != nil {
			return err
		}

		msg := fmt.Sprintf("Truncated %d row(s) from %s.%s", affected, tblH.src.Handle, tblH.tbl)
		msg = stringz.Plu(msg, int(affected))
		fmt.Fprintln(ru.Out, msg)
	}

	return nil
}

func newTblDropCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "drop @HANDLE.TABLE",
		Short: "Drop one or more tables",
		Long: `Drop one or more tables. Note that this command
only applies to SQL sources.`,
		RunE: execTblDrop,
		ValidArgsFunction: (&handleTableCompleter{
			onlySQL: true,
		}).complete,
		Example: `# drop table "actor" in src @sakila_sl3
  $ sq tbl drop @sakila_sl3.actor

  # drop table "payment"" in the active src
  $ sq tbl drop .payment

  # drop multiple tables
  $ sq drop .payment @sakila_sl3.actor`,
	}

	return cmd
}

func execTblDrop(cmd *cobra.Command, args []string) (err error) {
	ctx := cmd.Context()
	ru := run.FromContext(ctx)
	var tblHandles []tblHandle
	tblHandles, err = parseTableHandleArgs(ru.DriverRegistry, ru.Config.Collection, args)
	if err != nil {
		return err
	}

	if err = applyCollectionOptions(cmd, ru.Config.Collection); err != nil {
		return err
	}

	for _, tblH := range tblHandles {
		sqlDrvr, ok := tblH.drvr.(driver.SQLDriver)
		if !ok {
			return errz.Errorf("driver type {%s} (%s) doesn't support dropping tables", tblH.src.Type, tblH.src.Handle)
		}

		var pool driver.Pool
		if pool, err = ru.Sources.Open(ctx, tblH.src); err != nil {
			return err
		}

		var db *sql.DB
		if db, err = pool.DB(ctx); err != nil {
			return err
		}

		targetTbl := tablefq.New(tblH.tbl)
		if err = sqlDrvr.DropTable(cmd.Context(), db, targetTbl, false); err != nil {
			return err
		}

		fmt.Fprintf(ru.Out, "Dropped table %s.%s\n", tblH.src.Handle, tblH.tbl)
	}

	return nil
}

// parseTableHandleArgs parses args of the form:
//
//	@HANDLE1.TABLE1 .TABLE2 .TABLE3 @HANDLE2.TABLE4 .TABLEN
//
// It returns a slice of tblHandle, one for each arg. If an arg
// does not have a HANDLE, the active src is assumed: it's an error
// if no active src. It is also an error if len(args) is zero.
func parseTableHandleArgs(dp driver.Provider, coll *source.Collection, args []string) ([]tblHandle, error) {
	if len(args) == 0 {
		return nil, errz.New(msgInvalidArgs)
	}

	var tblHandles []tblHandle
	activeSrc := coll.Active()

	// We iterate over the args several times, because we want
	// to present error checks consistently.
	for _, arg := range args {
		handle, tbl, err := source.ParseTableHandle(arg)
		if err != nil {
			return nil, err
		}

		tblHandles = append(tblHandles, tblHandle{
			handle: handle,
			tbl:    tbl,
		})
	}

	for i := range tblHandles {
		if tblHandles[i].tbl == "" {
			return nil, errz.Errorf("arg[%d] {%s} doesn't specify a table", i, args[i])
		}

		if tblHandles[i].handle == "" {
			// It's a table name without a handle, so we use the active src
			if activeSrc == nil {
				return nil, errz.Errorf("arg[%d] {%s} doesn't specify a handle and there's no active source",
					i, args[i])
			}

			tblHandles[i].handle = activeSrc.Handle
		}

		src, err := coll.Get(tblHandles[i].handle)
		if err != nil {
			return nil, err
		}

		drvr, err := dp.DriverFor(src.Type)
		if err != nil {
			return nil, err
		}

		tblHandles[i].src = src
		tblHandles[i].drvr = drvr
	}

	return tblHandles, nil
}

// tblHandle represents a @HANDLE.TABLE, with the handle's associated
// src and driver.
type tblHandle struct {
	handle string
	tbl    string
	src    *source.Source
	drvr   driver.Driver
}
