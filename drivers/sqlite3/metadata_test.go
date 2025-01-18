package sqlite3_test

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/drivers/sqlite3"
	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/kind"
	"github.com/neilotoole/sq/libsq/core/lg"
	"github.com/neilotoole/sq/libsq/core/lg/lgt"
	"github.com/neilotoole/sq/libsq/core/sqlz"
	"github.com/neilotoole/sq/libsq/core/tablefq"
	"github.com/neilotoole/sq/libsq/source/metadata"
	"github.com/neilotoole/sq/testh"
	"github.com/neilotoole/sq/testh/sakila"
	"github.com/neilotoole/sq/testh/testsrc"
)

func TestSimple(t *testing.T) {
	t.Parallel()

	const query = `SELECT * from actor limit 1`
	wantKinds := []kind.Kind{kind.Int, kind.Text, kind.Text, kind.Datetime}

	th := testh.New(t)
	src := th.Source(sakila.SL3)
	sink, err := th.QuerySQL(src, nil, query)
	require.NoError(t, err)
	require.Equal(t, 1, len(sink.Recs))
	require.Equal(t, wantKinds, sink.RecMeta.Kinds())
	row := sink.Recs[0]
	for i := range row {
		require.NotNil(t, row[i])
	}
}

// TestScalarFuncsQuery performs a smoke test of executing
// a query with some scalar funcs to verify that
// column type info is being correctly determined.
func TestScalarFuncsQuery(t *testing.T) {
	t.Parallel()

	const query = `SELECT 'huzzah', NULL, ABS(film_id), LOWER(rating),
    	LAST_INSERT_ROWID(), MAX(rental_rate, replacement_cost)
		FROM film LIMIT 1`

	wantKinds := []kind.Kind{
		kind.Text,
		kind.Unknown,
		kind.Int,
		kind.Text,
		kind.Int,
		kind.Float,
	}

	th := testh.New(t)
	src := th.Source(sakila.SL3)
	sink, err := th.QuerySQL(src, nil, query)
	require.NoError(t, err)
	require.Equal(t, 1, len(sink.Recs))
	require.Equal(t, wantKinds, sink.RecMeta.Kinds())
}

// TestTypeTime tests the behavior of CURRENT_TIME.
// Apparently it's coming back to us as a string, thus
// it will be interpreted as kind.Text, not kind.Time.
// This is probably the best we can do, without attempting
// to scan each value to check for time-ness.
func TestCurrentTime(t *testing.T) {
	t.Parallel()

	const query = `SELECT CURRENT_TIME AS time_now`

	wantKinds := []kind.Kind{
		kind.Text, // We wish this could be kind.Time
	}

	th := testh.New(t)
	src := th.Source(sakila.SL3)
	sink, err := th.QuerySQL(src, nil, query)
	require.NoError(t, err)
	require.Equal(t, 1, len(sink.Recs))
	require.Equal(t, wantKinds, sink.RecMeta.Kinds())
}

func TestKindFromDBTypeName(t *testing.T) {
	t.Parallel()

	ctx := lg.NewContext(context.Background(), lgt.New(t))

	testCases := map[string]kind.Kind{
		"":                       kind.Bytes,
		"NUMERIC":                kind.Decimal,
		"INT":                    kind.Int,
		"INTEGER":                kind.Int,
		"TINYINT":                kind.Int,
		"SMALLINT":               kind.Int,
		"MEDIUMINT":              kind.Int,
		"BIGINT":                 kind.Int,
		"UNSIGNED BIG INT":       kind.Int,
		"INT2":                   kind.Int,
		"INT8":                   kind.Int,
		"CHARACTER(20)":          kind.Text,
		"VARCHAR(255)":           kind.Text,
		"VARYING CHARACTER(255)": kind.Text,
		"NCHAR(55)":              kind.Text,
		"NATIVE CHARACTER(70)":   kind.Text,
		"NVARCHAR(100)":          kind.Text,
		"TEXT":                   kind.Text,
		"CLOB":                   kind.Text,
		"REAL":                   kind.Float,
		"DOUBLE":                 kind.Float,
		"DOUBLE PRECISION":       kind.Float,
		"FLOAT":                  kind.Float,
		"DECIMAL(10,5)":          kind.Decimal,
		"BOOLEAN":                kind.Bool,
		"DATETIME":               kind.Datetime,
		"TIMESTAMP":              kind.Datetime,
		"DATE":                   kind.Date,
		"TIME":                   kind.Time,
	}

	for dbTypeName, wantKind := range testCases {
		gotKind := sqlite3.KindFromDBTypeName(ctx, "col", dbTypeName, nil)
		require.Equal(t, wantKind, gotKind, "%s should produce %s but got %s", dbTypeName)
	}
}

//nolint:lll
func TestRecordMetadata(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		tbl       string
		rowCount  int64
		colNames  []string
		colKinds  []kind.Kind
		scanTypes []reflect.Type
		colsMeta  []*metadata.Column
	}{
		{
			tbl:      sakila.TblActor,
			rowCount: sakila.TblActorCount,
			colNames: sakila.TblActorCols(),
			colKinds: []kind.Kind{kind.Int, kind.Text, kind.Text, kind.Datetime},
			scanTypes: []reflect.Type{
				sqlz.RTypeNullInt64, sqlz.RTypeNullString, sqlz.RTypeNullString,
				sqlz.RTypeNullTime,
			},
			colsMeta: []*metadata.Column{
				{Name: "actor_id", Position: 0, PrimaryKey: true, BaseType: "INTEGER", ColumnType: "INTEGER", Kind: kind.Int, Nullable: false},
				{Name: "first_name", Position: 1, BaseType: "VARCHAR(45)", ColumnType: "VARCHAR(45)", Kind: kind.Text, Nullable: false},
				{Name: "last_name", Position: 2, BaseType: "VARCHAR(45)", ColumnType: "VARCHAR(45)", Kind: kind.Text, Nullable: false},
				{Name: "last_update", Position: 3, BaseType: "TIMESTAMP", ColumnType: "TIMESTAMP", Kind: kind.Datetime, Nullable: false, DefaultValue: "CURRENT_TIMESTAMP"},
			},
		},
		{
			tbl:       sakila.TblFilmActor,
			rowCount:  sakila.TblFilmActorCount,
			colNames:  sakila.TblFilmActorCols(),
			colKinds:  []kind.Kind{kind.Int, kind.Int, kind.Datetime},
			scanTypes: []reflect.Type{sqlz.RTypeNullInt64, sqlz.RTypeNullInt64, sqlz.RTypeNullTime},
			colsMeta: []*metadata.Column{
				{Name: "actor_id", Position: 0, PrimaryKey: true, BaseType: "INT", ColumnType: "INT", Kind: kind.Int, Nullable: false},
				{Name: "film_id", Position: 1, PrimaryKey: true, BaseType: "INT", ColumnType: "INT", Kind: kind.Int, Nullable: false},
				{Name: "last_update", Position: 2, BaseType: "TIMESTAMP", ColumnType: "TIMESTAMP", Kind: kind.Datetime, Nullable: false},
			},
		},
		{
			tbl:      sakila.TblPayment,
			rowCount: sakila.TblPaymentCount,
			colNames: sakila.TblPaymentCols(),
			colKinds: []kind.Kind{kind.Int, kind.Int, kind.Int, kind.Int, kind.Decimal, kind.Datetime, kind.Datetime},
			scanTypes: []reflect.Type{
				sqlz.RTypeNullInt64, sqlz.RTypeNullInt64, sqlz.RTypeNullInt64,
				sqlz.RTypeNullInt64, sqlz.RTypeNullDecimal, sqlz.RTypeNullTime, sqlz.RTypeNullTime,
			},
			colsMeta: []*metadata.Column{
				{Name: "payment_id", Position: 0, PrimaryKey: true, BaseType: "INT", ColumnType: "INT", Kind: kind.Int, Nullable: false},
				{Name: "customer_id", Position: 1, BaseType: "INT", ColumnType: "INT", Kind: kind.Int, Nullable: false},
				{Name: "staff_id", Position: 2, BaseType: "SMALLINT", ColumnType: "SMALLINT", Kind: kind.Int, Nullable: false},
				{Name: "rental_id", Position: 3, BaseType: "INT", ColumnType: "INT", Kind: kind.Int, Nullable: true, DefaultValue: "NULL"},
				{Name: "amount", Position: 4, BaseType: "DECIMAL(5,2)", ColumnType: "DECIMAL(5,2)", Kind: kind.Decimal, Nullable: false},
				{Name: "payment_date", Position: 5, BaseType: "TIMESTAMP", ColumnType: "TIMESTAMP", Kind: kind.Datetime, Nullable: false},
				{Name: "last_update", Position: 6, BaseType: "TIMESTAMP", ColumnType: "TIMESTAMP", Kind: kind.Datetime, Nullable: false},
			},
		},
	}

	for _, tc := range testCases {
		tc := tc

		t.Run(tc.tbl, func(t *testing.T) {
			t.Parallel()

			th, _, drvr, grip, db := testh.NewWith(t, sakila.SL3)

			query := fmt.Sprintf("SELECT %s FROM %s", strings.Join(tc.colNames, ", "), tc.tbl)
			rows, err := db.QueryContext(th.Context, query) //nolint:rowserrcheck
			require.NoError(t, err)
			t.Cleanup(func() { assert.NoError(t, rows.Close()) })

			hasNext := rows.Next() // invoke rows.HasMore before invoking RecordMeta
			colTypes, err := rows.ColumnTypes()
			require.NoError(t, err)

			recMeta, _, err := drvr.RecordMeta(th.Context, colTypes)
			require.NoError(t, err)
			require.Equal(t, len(tc.colNames), len(recMeta))

			scanDests := recMeta.NewScanRow()
			for hasNext {
				// Scan rows to verify scan dests are ok
				require.NoError(t, rows.Scan(scanDests...))
				hasNext = rows.Next()
			}

			require.NoError(t, rows.Err())

			gotScanTypes := recMeta.ScanTypes()
			require.Equal(t, len(tc.scanTypes), len(gotScanTypes))
			for i := range tc.scanTypes {
				require.Equal(t, tc.scanTypes[i], gotScanTypes[i])
			}

			// Now check our table metadata
			gotTblMeta, err := grip.TableMetadata(th.Context, tc.tbl)
			require.NoError(t, err)
			require.Equal(t, tc.tbl, gotTblMeta.Name)
			require.Equal(t, tc.rowCount, gotTblMeta.RowCount)
			require.Equal(t, len(tc.colsMeta), len(gotTblMeta.Columns))

			for i := range tc.colsMeta {
				require.Equal(t, *tc.colsMeta[i], *gotTblMeta.Columns[i])
			}
		})
	}
}

func TestPayments(t *testing.T) {
	t.Parallel()
	th := testh.New(t)
	src := th.Source(sakila.SL3)

	sink, err := th.QuerySQL(src, nil, "SELECT * FROM payment")
	require.NoError(t, err)
	require.Equal(t, sakila.TblPaymentCount, len(sink.Recs))
}

// TestAggregateFuncsQuery performs a smoke test of executing
// a query with aggregate funcs to verify that
// column type info is being correctly determined.
func TestAggregateFuncsQuery(t *testing.T) {
	t.Parallel()

	const query = `SELECT COUNT(*),
		SUM(rental_rate),
		TOTAL(rental_rate),
		AVG(rental_rate),
		MAX(rental_rate),
		MIN(rental_rate),
		MAX(title),
		MAX(last_update),
		GROUP_CONCAT(rating,',')
	FROM film`

	th := testh.New(t)
	src := th.Source(sakila.SL3)
	sink, err := th.QuerySQL(src, nil, query)
	require.NoError(t, err)
	require.Equal(t, 1, len(sink.Recs))
}

func BenchmarkDatabase_SourceMetadata(b *testing.B) {
	const numTables = 1000

	th, src, drvr, grip, db := testh.NewWith(b, testsrc.MiscDB)
	tblNames := createTypeTestTbls(th, src, numTables, true)

	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		srcMeta, err := grip.SourceMetadata(th.Context, false)
		require.NoError(b, err)
		require.True(b, len(srcMeta.Tables) > len(tblNames))
	}
	b.StopTimer()

	for _, tblName := range tblNames {
		require.NoError(b, drvr.DropTable(th.Context, db, tablefq.From(tblName), true))
	}
}

func TestGetTblRowCounts(t *testing.T) {
	const numTables = 10

	th, src, _, _, db := testh.NewWith(t, testsrc.MiscDB)

	tblNames := createTypeTestTbls(th, src, numTables, true)

	counts, err := sqlite3.GetTblRowCounts(th.Context, db, tblNames)
	require.NoError(t, err)
	require.Equal(t, len(tblNames), len(counts))
}

func BenchmarkGetTblRowCounts(b *testing.B) {
	const numTables = 1300

	th, src, drvr, _, db := testh.NewWith(b, testsrc.MiscDB)

	tblNames := createTypeTestTbls(th, src, numTables, true)

	testCases := []struct {
		name string
		fn   func(ctx context.Context, db sqlz.DB, tblNames []string) ([]int64, error)
	}{
		{name: "benchGetTblRowCountsBaseline", fn: benchGetTblRowCountsBaseline},
		{name: "getTblRowCounts", fn: sqlite3.GetTblRowCounts},
	}

	for _, tc := range testCases {
		tc := tc

		b.Run(tc.name, func(b *testing.B) {
			for n := 0; n < b.N; n++ {
				counts, err := tc.fn(th.Context, db, tblNames)
				require.NoError(b, err)
				require.Len(b, counts, len(tblNames))
			}
		})
	}

	for _, tblName := range tblNames {
		require.NoError(b, drvr.DropTable(th.Context, db, tablefq.From(tblName), true))
	}
}

// benchGetTblRowCountsBaseline is a baseline impl of getTblRowCounts
// for benchmark comparison.
func benchGetTblRowCountsBaseline(ctx context.Context, db sqlz.DB, tblNames []string,
) ([]int64, error) {
	tblCounts := make([]int64, len(tblNames))

	for i := range tblNames {
		row := db.QueryRowContext(ctx, fmt.Sprintf("SELECT COUNT(*) FROM %q", tblNames[i]))
		err := row.Scan(&tblCounts[i])
		if err != nil {
			return nil, errz.Err(err)
		}
	}

	return tblCounts, nil
}
