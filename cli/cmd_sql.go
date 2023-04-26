package cli

import (
	"context"
	"fmt"
	"strings"

	"github.com/neilotoole/sq/cli/flag"

	"github.com/neilotoole/sq/libsq/core/lg/lgm"

	"github.com/neilotoole/sq/libsq/core/lg/lga"

	"github.com/neilotoole/sq/libsq"
	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/driver"
	"github.com/neilotoole/sq/libsq/source"

	"github.com/spf13/cobra"

	"github.com/neilotoole/sq/cli/output"
	"github.com/neilotoole/sq/libsq/core/stringz"
)

func newSQLCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sql QUERY|STMT",
		Short: "Execute DB-native SQL query or statement",
		Long: `Execute a SQL query or statement against the active source using the
source's SQL dialect. Use flag --src=@HANDLE to specify an alternative
source.

If flag --query is set, sq will run the input as a query
(SELECT) and return the query rows. If flag --exec is set,
sq will execute the input and return the result. If neither
flag is set, sq attempts to determine the appropriate mode.`,
		RunE: execSQL,
		Example: `  # Select from active source
  $ sq sql 'SELECT * FROM actor'

  # Select from a specified source
  $ sq sql --src=@sakila_pg12 'SELECT * FROM actor'

  # Drop table @sakila_pg12.actor
  $ sq sql --exec --src=@sakila_pg12 'DROP TABLE actor'

  # Select from active source and write results to @sakila_ms17.actor
  $ sq sql 'SELECT * FROM actor' --insert=@sakila_ms17.actor`,
	}

	addQueryCmdFlags(cmd)

	// User explicitly wants to execute the SQL using sql.DB.Query
	cmd.Flags().Bool(flag.SQLQuery, false, flag.SQLQueryUsage)
	// User explicitly wants to execute the SQL using sql.DB.Exec
	cmd.Flags().Bool(flag.SQLExec, false, flag.SQLExecUsage)

	return cmd
}

func execSQL(cmd *cobra.Command, args []string) error {
	rc := RunContextFrom(cmd.Context())
	switch len(args) {
	default:
		return errz.New("a single query string is required")
	case 0:
		return errz.New("no SQL query string")
	case 1:
		if strings.TrimSpace(args[0]) == "" {
			return errz.New("empty SQL query string")
		}
	}

	err := determineSources(cmd.Context(), rc)
	if err != nil {
		return err
	}

	coll := rc.Config.Collection
	// activeSrc is guaranteed to be non-nil after
	// determineSources successfully returns.
	activeSrc := coll.Active()

	if err = applySourceOptions(cmd, activeSrc); err != nil {
		return err
	}

	if !cmdFlagChanged(cmd, flag.Insert) {
		// The user didn't specify the --insert=@src.tbl flag,
		// so we just want to print the records.
		return execSQLPrint(cmd.Context(), rc, activeSrc)
	}

	// Instead of printing the records, they will be
	// written to another database
	insertTo, _ := cmd.Flags().GetString(flag.Insert)
	if insertTo == "" {
		return errz.Errorf("invalid --%s value: empty", flag.Insert)
	}

	destHandle, destTbl, err := source.ParseTableHandle(insertTo)
	if err != nil {
		return errz.Wrapf(err, "invalid --%s value", flag.Insert)
	}

	destSrc, err := coll.Get(destHandle)
	if err != nil {
		return err
	}

	return execSQLInsert(cmd.Context(), rc, activeSrc, destSrc, destTbl)
}

// execSQLPrint executes the SQL and prints resulting records
// to the configured writer.
func execSQLPrint(ctx context.Context, rc *RunContext, fromSrc *source.Source) error {
	args := rc.Args
	dbase, err := rc.databases.Open(ctx, fromSrc)
	if err != nil {
		return err
	}

	recw := output.NewRecordWriterAdapter(rc.writers.recordw)
	err = libsq.QuerySQL(ctx, dbase, recw, args[0])
	if err != nil {
		return err
	}
	_, err = recw.Wait() // Wait for the writer to finish processing
	return err
}

// execSQLInsert executes the SQL and inserts resulting records
// into destTbl in destSrc.
func execSQLInsert(ctx context.Context, rc *RunContext, fromSrc, destSrc *source.Source, destTbl string) error {
	args := rc.Args
	dbases := rc.databases
	ctx, cancelFn := context.WithCancel(ctx)
	defer cancelFn()

	fromDB, err := dbases.Open(ctx, fromSrc)
	if err != nil {
		return err
	}

	destDB, err := dbases.Open(ctx, destSrc)
	if err != nil {
		return err
	}

	// Note: We don't need to worry about closing fromDB and
	// destDB because they are closed by dbases.Close, which
	// is invoked by rc.Close, and rc is closed further up the
	// stack.

	inserter := libsq.NewDBWriter(
		rc.Log,
		destDB,
		destTbl,
		driver.Tuning.RecordChSize,
		libsq.DBWriterCreateTableIfNotExistsHook(destTbl),
	)
	err = libsq.QuerySQL(ctx, fromDB, inserter, args[0])
	if err != nil {
		return errz.Wrapf(err, "insert to {%s} failed", source.Target(destSrc, destTbl))
	}

	affected, err := inserter.Wait() // Wait for the writer to finish processing
	if err != nil {
		return errz.Wrapf(err, "insert %s.%s failed", destSrc.Handle, destTbl)
	}

	rc.Log.Debug(lgm.RowsAffected, lga.Count, affected)

	// TODO: Should really use a Printer here
	fmt.Fprintf(rc.Out, stringz.Plu("Inserted %d row(s) into %s\n",
		int(affected)), affected, source.Target(destSrc, destTbl))
	return nil
}
