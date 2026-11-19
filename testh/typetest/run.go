package typetest

import (
	"context"
	"database/sql/driver"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/cli/output"
	"github.com/neilotoole/sq/libsq"
	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/record"
	"github.com/neilotoole/sq/libsq/core/sqlz"
	"github.com/neilotoole/sq/libsq/core/stringz"
	"github.com/neilotoole/sq/libsq/core/tablefq"
	"github.com/neilotoole/sq/libsq/source"
	"github.com/neilotoole/sq/libsq/source/metadata"
	"github.com/neilotoole/sq/testh"
	"github.com/neilotoole/sq/testh/sakila"
	"github.com/neilotoole/sq/testh/tu"
)

// Config configures Run for one driver.
type Config struct {
	// ColumnDecl declares one column. Nil means DefaultColumnDecl.
	ColumnDecl func(decl string, v Variant) string

	// CheckSettings verifies the server settings that the expectations
	// assume, on the connection that creates the table. It returns an error
	// naming the setting if one doesn't match. Nil means no check.
	CheckSettings func(ctx context.Context, db sqlz.DB, serverVer string) error

	// Handle is the source handle, e.g. sakila.My.
	Handle string

	// FoldName maps a table or column name as the harness writes it (lower
	// case, unquoted) to the name the database reports, e.g. strings.ToUpper
	// for Oracle, which folds unquoted names to upper case. Nil means names are
	// reported as written.
	FoldName func(string) string

	// Literal renders an input as a SQL literal in the driver's dialect, for
	// the generated DDL file (see CheckDDL). Nil means DefaultLiteral only.
	Literal LiteralFunc

	// DDLPrelude holds statements the generated DDL file runs before it
	// creates the table, to set up the session its literals assume: MySQL's
	// SET time_zone = '+00:00', for example, since MySQL reads a TIMESTAMP
	// literal in the session's time zone. The ddl subtest runs them too.
	DDLPrelude []string

	// DDLCheckSession holds statements the ddl subtest runs on the connection
	// that loads the generated DDL, before the DDL itself, and that aren't
	// written to the file. Set that session's time zone to CheckTimeZone, so
	// that a time literal that depends on the loader's time zone reads back
	// wrong and fails the check. Leave it empty for a database with no session
	// time zone.
	DDLCheckSession []string

	// TableSuffix, if set, is appended to the CREATE TABLE statement, e.g.
	// "ENGINE = MergeTree() ORDER BY row_id" for ClickHouse.
	TableSuffix string

	// RowIDDecl declares the row_id column, e.g. "INT NOT NULL PRIMARY KEY".
	RowIDDecl string

	// Cases is the driver's case table.
	Cases []Case
}

// Run creates a table with a column for each case and variant, inserts
// three rows, and checks what sq reports: the inspect metadata from both
// TableMetadata and SourceMetadata, and, for each column, the query result
// kind and values, read with one query per column and then with one query
// over all columns together. Its ddl subtest then creates a second table from
// the generated DDL's literals and checks its values too: see the package
// doc's Generated DDL section.
//
// Run skips when cfg.Handle isn't configured, and under -short when it's an
// external database. Embedded sources (SQLite, DuckDB) always run. It
// doesn't call t.Parallel; the caller decides.
func Run(t *testing.T, cfg Config) { //nolint:thelper // Run is the test body; failures should point into it.
	if slices.Contains(sakila.SQLAllExternal(), cfg.Handle) {
		tu.SkipShort(t, true)
	}

	th := testh.New(t)
	if !th.SourceConfigured(cfg.Handle) {
		t.Skipf("source %s not configured", cfg.Handle)
	}
	src := th.Source(cfg.Handle)

	serverVer, err := th.Open(src).DBSemver(th.Context)
	require.NoError(t, err)
	t.Logf("%s: server version %s", cfg.Handle, serverVer)

	cols, err := Resolve(cfg.Cases, serverVer)
	require.NoError(t, err)
	require.NotEmpty(t, cols)

	fold := cfg.FoldName
	if fold == nil {
		fold = func(s string) string { return s }
	}

	tbl := setupTable(th, src, cfg, serverVer, cols)
	cols = expandTableToken(cols, fold(tbl))

	if dumpEnabled(t) {
		dumpActual(t, th, src, fold, serverVer, tbl, cols)
	}

	t.Run("TableMetadata", func(t *testing.T) {
		tblMeta, err := th.TableMetadata(src, fold(tbl))
		require.NoError(t, err)
		checkMeta(t, "TableMetadata", fold, serverVer, tblMeta, cols)
	})

	t.Run("SourceMetadata", func(t *testing.T) {
		srcMeta, err := th.SourceMetadata(src)
		require.NoError(t, err)
		tblMeta := srcMeta.Table(fold(tbl))
		require.NotNil(t, tblMeta, "SourceMetadata: table %s not found", fold(tbl))
		checkMeta(t, "SourceMetadata", fold, serverVer, tblMeta, cols)
	})

	for _, col := range cols {
		t.Run(col.Name+"/query", func(t *testing.T) {
			checkColumnQuery(t, th, src, serverVer, tbl, col)
		})
	}

	t.Run("select_all", func(t *testing.T) {
		checkSelectAll(t, th, src, fold, serverVer, tbl, cols)
	})

	t.Run("ddl", func(t *testing.T) {
		checkDDLLoads(t, th, src, cfg, fold, serverVer, cols)
	})
}

// checkDDLLoads executes the statements of the generated DDL file (see
// CheckDDL), rendered for this server's version under a table name of its
// own, and runs the whole-table value checks against that table, then the
// per-column check of each column with an expected query error. It proves
// the file loads on this server, and that each input's SQL literal reads back
// as expected (see Want.LiteralVal), or fails to as the bound input does.
func checkDDLLoads(t *testing.T, th *testh.Helper, src *source.Source, cfg Config, fold func(string) string,
	serverVer string, cols []Column,
) {
	t.Helper()

	tbl := stringz.UniqTableName("coltypes")
	stmts, err := RenderStatements(cfg, tbl, serverVer)
	require.NoError(t, err)

	loadDDL(t, th, src, cfg, serverVer, tbl, stmts)

	litCols := slices.Clone(cols)
	for i := range litCols {
		litCols[i].Want = litCols[i].LiteralWant
	}
	// Subtests, so that a failure in one doesn't hide the others.
	t.Run("select_all", func(t *testing.T) {
		checkSelectAll(t, th, src, fold, serverVer, tbl, litCols)
	})

	// checkSelectAll leaves out the columns with an expected query error.
	// Query each of those on its own, as Run does for the bound table, so a
	// literal that clears or changes the error fails too.
	for _, col := range litCols {
		if col.QueryErr.Set {
			t.Run(col.Name+"/query", func(t *testing.T) {
				checkColumnQuery(t, th, src, serverVer, tbl, col)
			})
		}
	}
}

// loadDDL executes stmts, which create table tbl, on a connection of its
// own, set up as cfg.DDLCheckSession says. It discards the connection before
// returning, so that its session settings don't leak into the pool the
// checks read through, and so the checks can use it, as they must with a
// pool of one connection (conn.max-open=1).
func loadDDL(t *testing.T, th *testh.Helper, src *source.Source, cfg Config, serverVer, tbl string,
	stmts []string,
) {
	t.Helper()

	conn, err := th.OpenDB(src).Conn(th.Context)
	require.NoError(t, err)
	defer func() {
		_ = conn.Raw(func(any) error { return driver.ErrBadConn })
		_ = conn.Close()
	}()

	for _, stmt := range cfg.DDLCheckSession {
		_, err = conn.ExecContext(th.Context, stmt)
		require.NoError(t, err, "server %s: DDLCheckSession statement: %s", serverVer, stmt)
	}

	created := false
	for i, stmt := range stmts {
		_, err = conn.ExecContext(th.Context, stmt)
		require.NoError(t, err, "server %s: generated DDL statement %d: %s", serverVer, i+1, stmt)
		if !created && strings.HasPrefix(stmt, "CREATE TABLE ") {
			created = true
			t.Cleanup(func() { th.DropTable(src, tablefq.From(tbl)) })
		}
	}
}

// TableToken stands for the test table's name in an expected default_value,
// whose text can include it: the table name is generated per run. For
// example, a Postgres serial column's default is
// "nextval('" + TableToken + "_col_serial_seq'::regclass)".
const TableToken = "{table}"

// expandTableToken returns a copy of cols with TableToken in each expected
// default_value replaced by tbl.
func expandTableToken(cols []Column, tbl string) []Column {
	out := slices.Clone(cols)
	for i := range out {
		out[i].Default = strings.ReplaceAll(out[i].Default, TableToken, tbl)
	}
	return out
}

// setupTable checks the server settings, creates the table and inserts the
// three rows, all on one connection, and returns the table name. It
// registers a cleanup that drops the table.
func setupTable(th *testh.Helper, src *source.Source, cfg Config, serverVer string, cols []Column) string {
	t := th.T
	ctx := th.Context

	// Closed when setup returns, before any inspect or query: those borrow
	// from the same grip's pool, and with conn.max-open=1 a conn held here
	// would block them indefinitely.
	conn, err := th.OpenDB(src).Conn(ctx)
	require.NoError(t, err)
	defer func() { require.NoError(t, conn.Close()) }()

	if cfg.CheckSettings != nil {
		require.NoError(t, cfg.CheckSettings(ctx, conn, serverVer))
	}

	colDecl := cfg.ColumnDecl
	if colDecl == nil {
		colDecl = DefaultColumnDecl
	}

	tbl := stringz.UniqTableName("type_test")
	createStmt := CreateTable(tbl, cfg.RowIDDecl, cols, colDecl, cfg.TableSuffix)
	_, err = conn.ExecContext(ctx, createStmt)
	require.NoError(t, err, "server %s: create table: %s", serverVer, createStmt)
	t.Cleanup(func() { th.DropTable(src, tablefq.From(tbl)) })

	placeholders := th.SQLDriverFor(src).Dialect().Placeholders
	for row := range 3 {
		query, args := Insert(tbl, cols, row, placeholders)
		_, err = conn.ExecContext(ctx, query, args...)
		require.NoError(t, err, "server %s: insert row %d: %s", serverVer, row+1, query)
	}

	return tbl
}

// checkMeta checks tblMeta, from the inspect path named by path, against
// cols: the column names in order, then each column's fields.
func checkMeta(t *testing.T, path string, fold func(string) string, serverVer string, tblMeta *metadata.Table,
	cols []Column,
) {
	t.Helper()

	wantNames := []string{fold(rowIDCol)}
	for _, col := range cols {
		wantNames = append(wantNames, fold(col.Name))
	}
	gotNames := make([]string, len(tblMeta.Columns))
	for i, c := range tblMeta.Columns {
		gotNames[i] = c.Name
	}
	require.Equal(t, wantNames, gotNames, "%s: server %s: column names", path, serverVer)

	for i, col := range cols {
		t.Run(col.Name, func(t *testing.T) {
			if msgs := DiffMeta(path, serverVer, tblMeta.Columns[i+1], col); len(msgs) > 0 {
				t.Error(strings.Join(msgs, "\n"))
			}
		})
	}
}

// checkColumnQuery reads col alone and checks its result kind and values,
// or its expected query error.
func checkColumnQuery(t *testing.T, th *testh.Helper, src *source.Source, serverVer, tbl string, col Column) {
	t.Helper()
	query := SelectColumn(tbl, col)

	if col.QueryErr.Set {
		// The record writer fills the sink on its own goroutine. Wait for it
		// to finish before leaving, even though the sink isn't read.
		recw := output.NewRecordWriterAdapter(th.Context, &testh.RecordSink{})
		queryErr := libsq.QuerySQL(th.Context, th.Open(src), nil, recw, nil, query)
		_, _ = recw.Wait()
		require.NoError(t, MatchQueryErr(queryErr, col, query), "server %s", serverVer)
		return
	}

	// th.QuerySQL waits for the record writer before returning the sink.
	sink, err := th.QuerySQL(src, nil, query)
	require.NoError(t, err, "column %s (%s) on server %s: %s", col.Name, col.Case.Decl, serverVer, query)
	checkRecords(t, serverVer, sink, 1, col)
}

// checkSelectAll reads every column without an expected query error in one
// query, and checks each against its expectations.
func checkSelectAll(t *testing.T, th *testh.Helper, src *source.Source, fold func(string) string, serverVer,
	tbl string, cols []Column,
) {
	t.Helper()
	query := SelectAll(tbl, cols)
	sink, err := th.QuerySQL(src, nil, query)
	require.NoError(t, err, "server %s: %s", serverVer, query)

	// Results are mapped to columns by position, so check the positions
	// first: a mismatch fails here with the names, not as an index panic.
	gotNames := make([]string, len(sink.RecMeta))
	for i, fm := range sink.RecMeta {
		gotNames[i] = fm.Name()
	}
	wantNames := SelectAllNames(cols)
	for i := range wantNames {
		wantNames[i] = fold(wantNames[i])
	}
	require.Equal(t, wantNames, gotNames, "server %s: result column names: %s", serverVer, query)

	pos := 1 // position 0 is row_id
	for _, col := range cols {
		if col.QueryErr.Set {
			continue
		}
		checkRecords(t, serverVer, sink, pos, col)
		pos++
	}
}

// diffRowIDs returns an error unless recs are the three rows in order: each
// record's first field, row_id, is int64 1, 2 and 3. Every query orders by
// row_id, and the value checks rely on it, so this makes that assumption
// explicit: a failure here points at row order or row_id, not at a value.
func diffRowIDs(recs []record.Record) error {
	for i, rec := range recs {
		if len(rec) == 0 || rec[0] != any(int64(i+1)) {
			var got any
			if len(rec) > 0 {
				got = rec[0]
			}
			return errz.Errorf("record %d: want row_id int64(%d), got %T(%v)", i, i+1, got, got)
		}
	}
	return nil
}

// checkRecords checks the field at position pos of sink's three records
// against col's expected kind and values.
func checkRecords(t *testing.T, serverVer string, sink *testh.RecordSink, pos int, col Column) {
	t.Helper()
	require.Len(t, sink.Recs, 3, "column %s (%s) on server %s: record count", col.Name, col.Case.Decl, serverVer)
	require.NoError(t, diffRowIDs(sink.Recs), "column %s (%s) on server %s: row_id", col.Name, col.Case.Decl,
		serverVer)
	// Compared by name, so a failure reads "int" rather than 3.
	require.Equal(t, col.QueryKind.String(), sink.RecMeta[pos].Kind().String(),
		"column %s (%s) on server %s: query result kind", col.Name, col.Case.Decl, serverVer)
	for row := range 3 {
		require.Equal(t, col.Want[row], sink.Recs[row][pos],
			"column %s (%s) on server %s: row %d value", col.Name, col.Case.Decl, serverVer, row+1)
	}
}
