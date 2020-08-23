package cli

import (
	"context"
	"fmt"
	"strings"

	"github.com/neilotoole/sq/libsq"
	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/source"

	"github.com/spf13/cobra"

	"github.com/neilotoole/sq/cli/output"
	"github.com/neilotoole/sq/libsq/core/stringz"
)

func newSQLCmd() (*cobra.Command, runFunc) {
	cmd := &cobra.Command{
		Use:   "sql QUERY|STMT",
		Short: "Execute DB-native SQL query or statement",
		Long: `Execute a SQL query or statement using the source's SQL dialect
against the active source. Use the --src=@HANDLE to specify
an alternative source.

If flag --query is set, sq will run the input as a query
(SELECT) and return the query rows. If the --exec flag is set,
sq will execute the input and return the result. If neither
flag is set, sq determines the appropriate mode.`,
		Example: `  # Select from active source
  sq sql 'SELECT * FROM actor'

  # Select from a specified source
  sq sql --src=@sakila_pg12 'SELECT * FROM actor'

  # Drop table @sakila_pg12.actor
  sq sql --exec --src=@sakila_pg12 'DROP TABLE actor'

  # Select from active source and write results to @pg1.actor
  sq sql 'SELECT * FROM actor' --insert=@sakila_pg12.actor`,
	}

	addQueryCmdFlags(cmd)

	// User explicitly wants to execute the SQL using sql.DB.Query
	cmd.Flags().Bool(flagSQLQuery, false, flagSQLQueryUsage)
	// User explicitly wants to execute the SQL using sql.DB.Exec
	cmd.Flags().Bool(flagSQLExec, false, flagSQLExecUsage)

	return cmd, execSQL
}

func execSQL(rc *RunContext, cmd *cobra.Command, args []string) error {
	switch len(args) {
	default:
		// FIXME: we should allow multiple args and concat them
		return errz.New("a single query string is required")
	case 0:
		return errz.New("empty SQL query string")
	case 1:
		if strings.TrimSpace(args[0]) == "" {
			return errz.New("empty SQL query string")
		}
	}

	err := determineSources(rc)
	if err != nil {
		return err
	}

	srcs := rc.Config.Sources
	// activeSrc is guaranteed to be non-nil after
	// determineSources successfully returns.
	activeSrc := srcs.Active()

	if !cmdFlagChanged(cmd, flagInsert) {
		// The user didn't specify the --insert=@src.tbl flag,
		// so we just want to print the records.
		return execSQLPrint(rc, activeSrc)
	}

	// Instead of printing the records, they will be
	// written to another database
	insertTo, _ := cmd.Flags().GetString(flagInsert)
	if insertTo == "" {
		return errz.Errorf("invalid --%s value: empty", flagInsert)
	}

	destHandle, destTbl, err := source.ParseTableHandle(insertTo)
	if err != nil {
		return errz.Wrapf(err, "invalid --%s value", flagInsert)
	}

	destSrc, err := srcs.Get(destHandle)
	if err != nil {
		return err
	}

	return execSQLInsert(rc, activeSrc, destSrc, destTbl)
}

// execSQLPrint executes the SQL and prints resulting records
// to the configured writer.
func execSQLPrint(rc *RunContext, fromSrc *source.Source) error {
	args := rc.Args
	dbase, err := rc.databases.Open(rc.Context, fromSrc)
	if err != nil {
		return err
	}

	recw := output.NewRecordWriterAdapter(rc.writers.recordw)
	err = libsq.QuerySQL(rc.Context, rc.Log, dbase, recw, args[0])
	if err != nil {
		return err
	}
	_, err = recw.Wait() // Wait for the writer to finish processing
	return err
}

// execSQLInsert executes the SQL and inserts resulting records
// into destTbl in destSrc.
func execSQLInsert(rc *RunContext, fromSrc, destSrc *source.Source, destTbl string) error {
	args := rc.Args
	dbases := rc.databases
	ctx, cancelFn := context.WithCancel(rc.Context)
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

	inserter := libsq.NewDBWriter(rc.Log, destDB, destTbl, libsq.DefaultRecordChSize)
	err = libsq.QuerySQL(ctx, rc.Log, fromDB, inserter, args[0])
	if err != nil {
		return errz.Wrapf(err, "insert %s.%s failed", destSrc.Handle, destTbl)
	}

	affected, err := inserter.Wait() // Wait for the writer to finish processing
	if err != nil {
		return errz.Wrapf(err, "insert %s.%s failed", destSrc.Handle, destTbl)
	}

	rc.Log.Debugf("Rows affected: %d", affected)

	fmt.Fprintf(rc.Out, stringz.Plu("Inserted %d row(s) into %s.%s\n", int(affected)), affected, destSrc.Handle, destTbl)
	return nil
}
