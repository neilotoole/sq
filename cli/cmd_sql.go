package cli

import (
	"context"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/neilotoole/sq/cli/flag"
	"github.com/neilotoole/sq/cli/output"
	"github.com/neilotoole/sq/cli/run"
	"github.com/neilotoole/sq/drivers/duckdb"
	"github.com/neilotoole/sq/libsq"
	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/lg"
	"github.com/neilotoole/sq/libsq/core/lg/lga"
	"github.com/neilotoole/sq/libsq/core/tuning"
	"github.com/neilotoole/sq/libsq/driver"
	"github.com/neilotoole/sq/libsq/driver/dialect"
	"github.com/neilotoole/sq/libsq/source"
	"github.com/neilotoole/sq/libsq/source/drivertype"
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

	cmd.Flags().Bool(flag.SQLReadOnly, false, flag.SQLReadOnlyUsage)
	cmd.Flags().Bool(flag.SQLReadOnlyAlias, false, flag.SQLReadOnlyAliasUsage)

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

	// --readonly / --ro: opt the raw-SQL command into read-only mode for
	// the source. Two things happen here, BEFORE determineSources:
	//   1. Peek at the would-be active source and surface the URL-conflict
	//      error preemptively. Doing this after determineSources would let
	//      verifySourceCatalogSchema briefly open the file READ_WRITE (the
	//      URL wins over the RO ctx) before the error fires, defeating the
	//      whole point of the conflict surfacing.
	//   2. Flip the ctx so any pre-open inside determineSources sees the
	//      RO hint. Skip the flip for --insert (execSQLInsert opens destGrip
	//      first on the RW ctx before flipping to RO for the source side).
	readOnlySrc := cmdFlagIsSetTrue(cmd, flag.SQLReadOnly) ||
		cmdFlagIsSetTrue(cmd, flag.SQLReadOnlyAlias)
	if readOnlySrc {
		if peek := peekActiveSrc(cmd, ru.Config.Collection); peek != nil &&
			peek.Type == drivertype.DuckDB {
			// Only READ_WRITE is a hard conflict. access_mode=AUTOMATIC is
			// overridden to READ_ONLY by the driver (see
			// duckdb.ApplyReadOnlyToLocation), and READ_ONLY already agrees.
			if mode, ok := duckdb.ExplicitAccessMode(peek.Location); ok &&
				strings.EqualFold(mode, "READ_WRITE") {
				return errz.Errorf(
					"sql: --%s conflicts with access_mode=READ_WRITE in %s",
					flag.SQLReadOnly, peek.Handle)
			}
		}
		if !cmdFlagChanged(cmd, flag.Insert) {
			ctx = driver.WithReadOnlyExplicit(ctx)
			cmd.SetContext(ctx)
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
		// so we just want to print the records. RO ctx (if requested)
		// was established above, before determineSources.
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

	return execSQLInsert(ctx, ru, activeSrc, destSrc, destTbl, readOnlySrc)
}

// execSQLPrint executes the SQL input, and either prints the resulting records
// (if the SQL input is a query), or executes the SQL input statement and prints
// the count of affected rows from the statement execution.
func execSQLPrint(ctx context.Context, ru *run.Run, fromSrc *source.Source) error {
	args := ru.Args
	grip, err := ru.Grips.Open(ctx, fromSrc)
	if err != nil {
		return err
	}

	sql := args[0]

	// Detect if this is a query (SELECT) or statement (CREATE, INSERT, etc.)
	execMode, err := grip.SQLDriver().Dialect().ExecModeFor(sql)
	if err != nil {
		return err
	}
	lg.FromContext(ctx).Debug("Determined SQL exec mode for SQL input", lga.Mode, execMode)

	if execMode != dialect.ExecModeQuery {
		// ExecModeExec: use DB.Exec (returns affected count, not rows)
		start := time.Now()
		affected, execErr := libsq.ExecSQL(ctx, grip, nil, sql)
		elapsed := time.Since(start)
		if execErr != nil {
			return execErr
		}

		// Some databases (e.g. ClickHouse) don't reliably report rows
		// affected for DML. The raw protocol returns 0, which is misleading
		// because it implies "no rows were affected" when the truth is
		// "we don't know." Convert 0 to -1 so the output layer can display
		// an appropriate message.
		if grip.SQLDriver().Dialect().IsRowsAffectedUnsupported && affected == 0 {
			affected = dialect.RowsAffectedUnsupported
		}

		return ru.Writers.StmtExec.StmtExecuted(ctx, fromSrc, affected, elapsed)
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
// into destTbl in destSrc. readOnlySrc controls whether the source
// (fromSrc) is opened READ_ONLY; the destination is always opened
// READ_WRITE so the INSERT can succeed.
func execSQLInsert(ctx context.Context, ru *run.Run,
	fromSrc, destSrc *source.Source, destTbl string, readOnlySrc bool,
) error {
	args := ru.Args
	grips := ru.Grips
	ctx, cancelFn := context.WithCancel(ctx)
	defer cancelFn()

	// Open destGrip FIRST on the RW ctx so the destination opens
	// READ_WRITE. The Grips cache keys by src.Handle, so if fromSrc
	// shares a handle with destSrc (self-insert), the later
	// grips.Open(ctx, fromSrc) returns this cached RW grip.
	destGrip, err := grips.Open(ctx, destSrc)
	if err != nil {
		return err
	}

	// Now mark the ctx read-only for the source-side open. Skips the
	// rewrite if the user didn't pass --readonly. Explicit, because
	// readOnlySrc is only true when the user passed --readonly.
	if readOnlySrc {
		ctx = driver.WithReadOnlyExplicit(ctx)
	}

	fromGrip, err := grips.Open(ctx, fromSrc)
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

	start := time.Now()
	err = libsq.QuerySQL(ctx, fromGrip, nil, inserter, args[0])
	if err != nil {
		return errz.Wrapf(err, "insert to {%s} failed", source.Target(destSrc, destTbl))
	}

	affected, err := inserter.Wait() // Stop for the writer to finish processing
	elapsed := time.Since(start)
	if err != nil {
		return errz.Wrapf(err, "insert %s.%s failed", destSrc.Handle, destTbl)
	}

	lg.FromContext(ctx).Debug("Rows inserted", lga.Target, source.Target(destSrc, destTbl),
		lga.Count, affected, lga.Elapsed, elapsed)

	return ru.Writers.RecordInsert.RecordsInserted(ctx, destSrc, destTbl, affected, elapsed)
}
