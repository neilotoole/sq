package sqlite3_test

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/neilotoole/lg/testlg"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/drivers/sqlite3"
	"github.com/neilotoole/sq/libsq/source"
	"github.com/neilotoole/sq/libsq/sqlz"
	"github.com/neilotoole/sq/testh"
	"github.com/neilotoole/sq/testh/sakila"
)

func TestKindFromDBTypeName(t *testing.T) {
	t.Parallel()

	testCases := map[string]sqlz.Kind{
		"":                       sqlz.KindBytes,
		"NUMERIC":                sqlz.KindDecimal,
		"INT":                    sqlz.KindInt,
		"INTEGER":                sqlz.KindInt,
		"TINYINT":                sqlz.KindInt,
		"SMALLINT":               sqlz.KindInt,
		"MEDIUMINT":              sqlz.KindInt,
		"BIGINT":                 sqlz.KindInt,
		"UNSIGNED BIG INT":       sqlz.KindInt,
		"INT2":                   sqlz.KindInt,
		"INT8":                   sqlz.KindInt,
		"CHARACTER(20)":          sqlz.KindText,
		"VARCHAR(255)":           sqlz.KindText,
		"VARYING CHARACTER(255)": sqlz.KindText,
		"NCHAR(55)":              sqlz.KindText,
		"NATIVE CHARACTER(70)":   sqlz.KindText,
		"NVARCHAR(100)":          sqlz.KindText,
		"TEXT":                   sqlz.KindText,
		"CLOB":                   sqlz.KindText,
		"REAL":                   sqlz.KindFloat,
		"DOUBLE":                 sqlz.KindFloat,
		"DOUBLE PRECISION":       sqlz.KindFloat,
		"FLOAT":                  sqlz.KindFloat,
		"DECIMAL(10,5)":          sqlz.KindDecimal,
		"BOOLEAN":                sqlz.KindBool,
		"DATETIME":               sqlz.KindDatetime,
		"TIMESTAMP":              sqlz.KindDatetime,
		"DATE":                   sqlz.KindDate,
		"TIME":                   sqlz.KindTime,
	}

	log := testlg.New(t)
	for dbTypeName, wantKind := range testCases {
		gotKind := sqlite3.KindFromDBTypeName(log, "col", dbTypeName, nil)
		require.Equal(t, wantKind, gotKind, "%s should produce %s but got %s", dbTypeName)
	}
}

func TestRecordMetadata(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		tbl       string
		rowCount  int64
		colNames  []string
		colKinds  []sqlz.Kind
		scanTypes []reflect.Type
		colsMeta  []*source.ColMetadata
	}{
		{
			tbl:       sakila.TblActor,
			rowCount:  sakila.TblActorCount,
			colNames:  sakila.TblActorCols,
			colKinds:  []sqlz.Kind{sqlz.KindInt, sqlz.KindText, sqlz.KindText, sqlz.KindDatetime},
			scanTypes: []reflect.Type{sqlz.RTypeNullInt64, sqlz.RTypeNullString, sqlz.RTypeNullString, sqlz.RTypeNullTime},
			colsMeta: []*source.ColMetadata{
				{Name: "actor_id", Position: 0, PrimaryKey: true, BaseType: "INTEGER", ColumnType: "INTEGER", Kind: sqlz.KindInt, Nullable: false},
				{Name: "first_name", Position: 1, BaseType: "VARCHAR(45)", ColumnType: "VARCHAR(45)", Kind: sqlz.KindText, Nullable: false},
				{Name: "last_name", Position: 2, BaseType: "VARCHAR(45)", ColumnType: "VARCHAR(45)", Kind: sqlz.KindText, Nullable: false},
				{Name: "last_update", Position: 3, BaseType: "TIMESTAMP", ColumnType: "TIMESTAMP", Kind: sqlz.KindDatetime, Nullable: false, DefaultValue: "CURRENT_TIMESTAMP"},
			},
		},
		{
			tbl:       sakila.TblFilmActor,
			rowCount:  sakila.TblFilmActorCount,
			colNames:  sakila.TblFilmActorCols,
			colKinds:  []sqlz.Kind{sqlz.KindInt, sqlz.KindInt, sqlz.KindDatetime},
			scanTypes: []reflect.Type{sqlz.RTypeNullInt64, sqlz.RTypeNullInt64, sqlz.RTypeNullTime},
			colsMeta: []*source.ColMetadata{
				{Name: "actor_id", Position: 0, PrimaryKey: true, BaseType: "INT", ColumnType: "INT", Kind: sqlz.KindInt, Nullable: false},
				{Name: "film_id", Position: 1, PrimaryKey: true, BaseType: "INT", ColumnType: "INT", Kind: sqlz.KindInt, Nullable: false},
				{Name: "last_update", Position: 2, BaseType: "TIMESTAMP", ColumnType: "TIMESTAMP", Kind: sqlz.KindDatetime, Nullable: false},
			},
		},
		{
			tbl:       sakila.TblPayment,
			rowCount:  sakila.TblPaymentCount,
			colNames:  sakila.TblPaymentCols,
			colKinds:  []sqlz.Kind{sqlz.KindInt, sqlz.KindInt, sqlz.KindInt, sqlz.KindInt, sqlz.KindDecimal, sqlz.KindDatetime, sqlz.KindDatetime},
			scanTypes: []reflect.Type{sqlz.RTypeNullInt64, sqlz.RTypeNullInt64, sqlz.RTypeNullInt64, sqlz.RTypeNullInt64, sqlz.RTypeNullString, sqlz.RTypeNullTime, sqlz.RTypeNullTime},
			colsMeta: []*source.ColMetadata{
				{Name: "payment_id", Position: 0, PrimaryKey: true, BaseType: "INT", ColumnType: "INT", Kind: sqlz.KindInt, Nullable: false},
				{Name: "customer_id", Position: 1, BaseType: "INT", ColumnType: "INT", Kind: sqlz.KindInt, Nullable: false},
				{Name: "staff_id", Position: 2, BaseType: "SMALLINT", ColumnType: "SMALLINT", Kind: sqlz.KindInt, Nullable: false},
				{Name: "rental_id", Position: 3, BaseType: "INT", ColumnType: "INT", Kind: sqlz.KindInt, Nullable: true, DefaultValue: "NULL"},
				{Name: "amount", Position: 4, BaseType: "DECIMAL(5,2)", ColumnType: "DECIMAL(5,2)", Kind: sqlz.KindDecimal, Nullable: false},
				{Name: "payment_date", Position: 5, BaseType: "TIMESTAMP", ColumnType: "TIMESTAMP", Kind: sqlz.KindDatetime, Nullable: false},
				{Name: "last_update", Position: 6, BaseType: "TIMESTAMP", ColumnType: "TIMESTAMP", Kind: sqlz.KindDatetime, Nullable: false},
			},
		},
	}

	for _, tc := range testCases {
		tc := tc

		t.Run(tc.tbl, func(t *testing.T) {
			t.Parallel()

			th := testh.New(t)
			src := th.Source(sakila.SL3)
			dbase := th.Open(src)

			query := fmt.Sprintf("SELECT %s FROM %s", strings.Join(tc.colNames, ", "), tc.tbl)
			rows, err := dbase.DB().QueryContext(th.Context, query) // nolint:rowserrcheck
			require.NoError(t, err)
			t.Cleanup(func() { assert.NoError(t, rows.Close()) })

			hasNext := rows.Next() // invoke rows.Next before invoking RecordMeta
			colTypes, err := rows.ColumnTypes()
			require.NoError(t, err)

			recMeta, _, err := th.SQLDriverFor(src).RecordMeta(colTypes)
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
			gotTblMeta, err := dbase.TableMetadata(th.Context, tc.tbl)
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

	sink, err := th.QuerySQL(src, "SELECT * FROM payment")
	require.NoError(t, err)
	require.Equal(t, sakila.TblPaymentCount, len(sink.Recs))
}

// TestAggregateFuncsQuery performs a smoke test of executing
// a query with aggregate funcs to verify that
// column type info is being correctly determined.
func TestAggregateFuncsQuery(t *testing.T) {
	t.Parallel()

	const query = `SELECT COUNT(*), SUM(rental_rate), TOTAL(rental_rate),
	AVG(rental_rate), MAX(rental_rate), MIN(rental_rate),
	MAX(title), MAX(last_update), GROUP_CONCAT(rating,',')
	FROM film`

	th := testh.New(t)
	src := th.Source(sakila.SL3)
	sink, err := th.QuerySQL(src, query)
	require.NoError(t, err)
	require.Equal(t, 1, len(sink.Recs))
}

// TestScalarFuncsQuery performs a smoke test of executing
// a query with some scalar funcs to verify that
// column type info is being correctly determined.
func TestScalarFuncsQuery(t *testing.T) {
	t.Parallel()

	const query = `SELECT NULL, ABS(film_id), LOWER(rating), LAST_INSERT_ROWID(),
	MAX(rental_rate, replacement_cost)
	FROM film`
	wantKinds := []sqlz.Kind{sqlz.KindBytes, sqlz.KindInt, sqlz.KindText, sqlz.KindInt, sqlz.KindFloat}

	th := testh.New(t)
	src := th.Source(sakila.SL3)
	sink, err := th.QuerySQL(src, query)
	require.NoError(t, err)
	require.Equal(t, sakila.TblFilmCount, len(sink.Recs))
	require.Equal(t, wantKinds, sink.RecMeta.Kinds())
}
