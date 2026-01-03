package cli

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/neilotoole/sq/cli/flag"
	"github.com/neilotoole/sq/cli/output"
	"github.com/neilotoole/sq/cli/run"
	"github.com/neilotoole/sq/libsq"
	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/lg"
	"github.com/neilotoole/sq/libsq/core/lg/lga"
	"github.com/neilotoole/sq/libsq/core/lg/lgm"
	"github.com/neilotoole/sq/libsq/core/stringz"
	"github.com/neilotoole/sq/libsq/core/tuning"
	"github.com/neilotoole/sq/libsq/source"
)

func newSQLCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sql QUERY|STMT",
		Short: "Execute DB-native SQL query or statement",
		Long: `Execute a SQL query or statement against the active source using the
source's SQL dialect. Use flag --src=@HANDLE to specify an alternative
source.`,
		RunE: execSQL,
		Example: `  # Select from active source
  $ sq sql 'SELECT * FROM actor'

  # Select from a specified source
  $ sq sql --src=@sakila_pg12 'SELECT * FROM actor'

  # Drop table @sakila_pg12.actor
  $ sq sql --src=@sakila_pg12 'DROP TABLE actor'

  # Select from active source and write results to @sakila_ms17.actor
  $ sq sql 'SELECT * FROM actor' --insert=@sakila_ms17.actor`,
	}

	addQueryCmdFlags(cmd)

	// TODO: These flags aren't actually implemented yet.
	// And... this entire --exec/--query mechanism needs to be revisited.
	// It's probably the case that sq can figure out whether to use
	// Query or Exec based on the SQL statement. Probably using
	// an antlr parser for each driver's SQL language.
	// Anyway, because the flags were already present in previous
	// releases, I'm reverting the (very recent) deletion of these
	// flags and instead making them hidden, so that their use
	// doesn't result in an error. The flags still don't actually do anything.

	// User explicitly wants to execute the SQL using sql.DB.Query
	cmd.Flags().Bool(flag.SQLQuery, false, flag.SQLQueryUsage)
	panicOn(cmd.Flags().MarkHidden(flag.SQLQuery))
	// User explicitly wants to execute the SQL using sql.DB.Exec
	cmd.Flags().Bool(flag.SQLExec, false, flag.SQLExecUsage)
	panicOn(cmd.Flags().MarkHidden(flag.SQLExec))
	return cmd
}

func execSQL(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	ru := run.FromContext(ctx)
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

	err := determineSources(ctx, ru, true)
	if err != nil {
		return err
	}

	coll := ru.Config.Collection
	// activeSrc is guaranteed to be non-nil after
	// determineSources successfully returns.
	activeSrc := coll.Active()

	if err = applySourceOptions(cmd, activeSrc); err != nil {
		return err
	}

	if !cmdFlagChanged(cmd, flag.Insert) {
		// The user didn't specify the --insert=@src.tbl flag,
		// so we just want to print the records.
		return execSQLPrint(ctx, ru, activeSrc)
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

	return execSQLInsert(ctx, ru, activeSrc, destSrc, destTbl)
}

// isQueryStatement returns true if the SQL appears to be a query (SELECT)
// that returns rows, false if it's a statement (CREATE, INSERT, UPDATE, etc.)
// that should use ExecContext.
func isQueryStatement(sql string) bool {
	// Trim whitespace and convert to uppercase for comparison
	sql = strings.TrimSpace(sql)
	if sql == "" {
		return true // default to query
	}

	// Remove leading comments
	for strings.HasPrefix(sql, "--") || strings.HasPrefix(sql, "/*") {
		if strings.HasPrefix(sql, "--") {
			idx := strings.Index(sql, "\n")
			if idx < 0 {
				return true
			}
			sql = strings.TrimSpace(sql[idx+1:])
		} else if strings.HasPrefix(sql, "/*") {
			idx := strings.Index(sql, "*/")
			if idx < 0 {
				return true
			}
			sql = strings.TrimSpace(sql[idx+2:])
		}
	}

	sqlUpper := strings.ToUpper(sql)

	// Check for query statements (return rows)
	queryPrefixes := []string{"SELECT", "WITH", "SHOW", "DESCRIBE", "DESC", "EXPLAIN"}
	for _, prefix := range queryPrefixes {
		if strings.HasPrefix(sqlUpper, prefix+" ") || strings.HasPrefix(sqlUpper, prefix+"\t") ||
			strings.HasPrefix(sqlUpper, prefix+"\n") || strings.HasPrefix(sqlUpper, prefix+"\r") ||
			sqlUpper == prefix {
			return true
		}
	}

	// Everything else (CREATE, INSERT, UPDATE, DELETE, DROP, ALTER, etc.) is a statement
	return false
}

// execSQLPrint executes the SQL and prints resulting records
// to the configured writer.
func execSQLPrint(ctx context.Context, ru *run.Run, fromSrc *source.Source) error {
	args := ru.Args
	grip, err := ru.Grips.Open(ctx, fromSrc)
	if err != nil {
		return err
	}

	sql := args[0]

	// Detect if this is a query (SELECT) or statement (CREATE, INSERT, etc.)
	if !isQueryStatement(sql) {
		// This is a DDL/DML statement, use ExecSQL
		affected, execErr := libsq.ExecSQL(ctx, grip, nil, sql)
		if execErr != nil {
			return execErr
		}
		_, _ = fmt.Fprintf(ru.Out, stringz.Plu("Affected %d row(s)\n", int(affected)), affected)
		return nil
	}

	// This is a query, use QuerySQL
	recw := output.NewRecordWriterAdapter(ctx, ru.Writers.Record)
	err = libsq.QuerySQL(ctx, grip, nil, recw, sql)
	if err != nil {
		return err
	}
	_, err = recw.Wait() // Stop for the writer to finish processing
	return err
}

// execSQLInsert executes the SQL and inserts resulting records
// into destTbl in destSrc.
func execSQLInsert(ctx context.Context, ru *run.Run,
	fromSrc, destSrc *source.Source, destTbl string,
) error {
	args := ru.Args
	grips := ru.Grips
	ctx, cancelFn := context.WithCancel(ctx)
	defer cancelFn()

	fromGrip, err := grips.Open(ctx, fromSrc)
	if err != nil {
		return err
	}

	destGrip, err := grips.Open(ctx, destSrc)
	if err != nil {
		return err
	}

	// Note: We don't need to worry about closing fromGrip and
	// destGrip because they are closed by grips.Close, which
	// is invoked by ru.Close, and ru is closed further up the
	// stack.
	inserter := libsq.NewDBWriter(
		"Insert records",
		destGrip,
		destTbl,
		tuning.OptRecBufSize.Get(destSrc.Options),
		libsq.DBWriterCreateTableIfNotExistsHook(destTbl),
	)
	err = libsq.QuerySQL(ctx, fromGrip, nil, inserter, args[0])
	if err != nil {
		return errz.Wrapf(err, "insert to {%s} failed", source.Target(destSrc, destTbl))
	}

	affected, err := inserter.Wait() // Stop for the writer to finish processing
	if err != nil {
		return errz.Wrapf(err, "insert %s.%s failed", destSrc.Handle, destTbl)
	}

	lg.FromContext(ctx).Debug(lgm.RowsAffected, lga.Count, affected)

	// TODO: Should really use a Printer here
	_, _ = fmt.Fprintf(ru.Out, stringz.Plu("Inserted %d row(s) into %s\n",
		int(affected)), affected, source.Target(destSrc, destTbl))
	return nil
}
