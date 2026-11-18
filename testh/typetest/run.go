package typetest

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/cli/output"
	"github.com/neilotoole/sq/libsq"
	"github.com/neilotoole/sq/libsq/core/sqlz"
	"github.com/neilotoole/sq/libsq/core/stringz"
	"github.com/neilotoole/sq/libsq/core/tablefq"
	"github.com/neilotoole/sq/libsq/source"
	"github.com/neilotoole/sq/libsq/source/metadata"
	"github.com/neilotoole/sq/testh"
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

	// RowIDDecl declares the row_id column, e.g. "INT NOT NULL PRIMARY KEY".
	RowIDDecl string

	// Cases is the driver's case table.
	Cases []Case
}

// Run creates a table with a column for each case and variant, inserts
// three rows, and checks what sq reports: the inspect metadata from both
// TableMetadata and SourceMetadata, and, for each column, the query result
// kind and values, read with one query per column and then with one query
// over all columns together.
//
// Run skips under -short and when cfg.Handle isn't configured. It doesn't
// call t.Parallel; the caller decides.
func Run(t *testing.T, cfg Config) { //nolint:thelper // Run is the test body; failures should point into it.
	tu.SkipShort(t, true)

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

	tbl := setupTable(th, src, cfg, serverVer, cols)

	t.Run("TableMetadata", func(t *testing.T) {
		tblMeta, err := th.TableMetadata(src, tbl)
		require.NoError(t, err)
		checkMeta(t, "TableMetadata", serverVer, tblMeta, cols)
	})

	t.Run("SourceMetadata", func(t *testing.T) {
		srcMeta, err := th.SourceMetadata(src)
		require.NoError(t, err)
		tblMeta := srcMeta.Table(tbl)
		require.NotNil(t, tblMeta, "SourceMetadata: table %s not found", tbl)
		checkMeta(t, "SourceMetadata", serverVer, tblMeta, cols)
	})

	for _, col := range cols {
		t.Run(col.Name+"/query", func(t *testing.T) {
			checkColumnQuery(t, th, src, serverVer, tbl, col)
		})
	}

	t.Run("select_all", func(t *testing.T) {
		checkSelectAll(t, th, src, serverVer, tbl, cols)
	})
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
	_, err = conn.ExecContext(ctx, CreateTable(tbl, cfg.RowIDDecl, cols, colDecl))
	require.NoError(t, err)
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
func checkMeta(t *testing.T, path, serverVer string, tblMeta *metadata.Table, cols []Column) {
	t.Helper()

	wantNames := []string{rowIDCol}
	for _, col := range cols {
		wantNames = append(wantNames, col.Name)
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
		require.NoError(t, MatchQueryErr(queryErr, col), "server %s", serverVer)
		return
	}

	// th.QuerySQL waits for the record writer before returning the sink.
	sink, err := th.QuerySQL(src, nil, query)
	require.NoError(t, err, "column %s (%s) on server %s: %s", col.Name, col.Case.Decl, serverVer, query)
	checkRecords(t, serverVer, sink, 1, col)
}

// checkSelectAll reads every column without an expected query error in one
// query, and checks each against its expectations.
func checkSelectAll(t *testing.T, th *testh.Helper, src *source.Source, serverVer, tbl string, cols []Column) {
	t.Helper()
	query := SelectAll(tbl, cols)
	sink, err := th.QuerySQL(src, nil, query)
	require.NoError(t, err, "server %s: %s", serverVer, query)

	pos := 1 // position 0 is row_id
	for _, col := range cols {
		if col.QueryErr.Set {
			continue
		}
		checkRecords(t, serverVer, sink, pos, col)
		pos++
	}
}

// checkRecords checks the field at position pos of sink's three records
// against col's expected kind and values.
func checkRecords(t *testing.T, serverVer string, sink *testh.RecordSink, pos int, col Column) {
	t.Helper()
	require.Len(t, sink.Recs, 3, "column %s (%s) on server %s: record count", col.Name, col.Case.Decl, serverVer)
	require.Equal(t, col.QueryKind, sink.RecMeta[pos].Kind(),
		"column %s (%s) on server %s: query result kind", col.Name, col.Case.Decl, serverVer)
	for row := range 3 {
		require.Equal(t, col.Want[row], sink.Recs[row][pos],
			"column %s (%s) on server %s: row %d value", col.Name, col.Case.Decl, serverVer, row+1)
	}
}
