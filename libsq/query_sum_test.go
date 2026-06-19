package libsq_test

import (
	"testing"

	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/libsq/core/kind"
	"github.com/neilotoole/sq/libsq/core/schema"
	"github.com/neilotoole/sq/libsq/core/stringz"
	"github.com/neilotoole/sq/libsq/core/tablefq"
	"github.com/neilotoole/sq/libsq/source/drivertype"
	"github.com/neilotoole/sq/testh"
	"github.com/neilotoole/sq/testh/sakila"
	"github.com/neilotoole/sq/testh/tu"
)

// TestQuery_sum_floatColumn verifies that sum() over a FLOAT/DOUBLE column
// surfaces as kind.Decimal (issues #839, #853). Sakila has no float column, so a
// one-column float table is created per driver. This is the regression guard
// for MySQL, whose native sum() over a float column returns DOUBLE (kind.Float)
// and which needed an explicit cast override; the other cast-based drivers
// coerce via their result cast. DuckDB takes a different path: a DECIMAL cast
// would regress its native HUGEINT integer range, so it kind-pins sum() to
// decimal and coerces the float value in its record munge (#853). ClickHouse is
// skipped (see below) for an insert-pipeline limitation.
func TestQuery_sum_floatColumn(t *testing.T) {
	for _, handle := range sakila.SQLLatest() {
		t.Run(handle, func(t *testing.T) {
			tu.SkipShort(t, true)

			th, src, drvr, _, db := testh.NewWith(t, handle)

			// ClickHouse batch INSERT requires the transaction-wrapped insert
			// pipeline (NewBatchInsert), not the bare PrepareInsertStmt used here,
			// so the rows wouldn't land. ClickHouse's sum(float) harmonization is
			// already covered structurally: it uses the same result cast as the
			// int/decimal cases in TestQuery_func, which force every sum() to
			// decimal regardless of the operand type.
			tu.SkipIf(t, src.Type == drivertype.ClickHouse,
				"ClickHouse needs the batch-insert pipeline; sum(float) is covered via its result cast")

			tblName := stringz.UniqTableName("sum_float")
			tblDef := schema.NewTable(tblName, []string{"col_float"}, []kind.Kind{kind.Float})
			require.NoError(t, drvr.CreateTable(th.Context, db, tblDef))
			t.Cleanup(func() { th.DropTable(src, tablefq.From(tblName)) })

			// PrepareInsertStmt needs a single connection.
			conn, err := db.Conn(th.Context)
			require.NoError(t, err)
			defer func() { require.NoError(t, conn.Close()) }()

			execer, err := drvr.PrepareInsertStmt(th.Context, conn, tblName, []string{"col_float"}, 1)
			require.NoError(t, err)

			// 1.25 + 2.5 = 3.75 (all exactly representable as float64).
			for _, v := range []float64{1.25, 2.5} {
				rec := []any{v}
				require.NoError(t, execer.Munge(rec))
				_, err = execer.Exec(th.Context, rec...)
				require.NoError(t, err)
			}
			// Close the execer before querying: some drivers (ClickHouse) only
			// flush a prepared-insert batch to the table on close.
			require.NoError(t, execer.Close())

			sink, err := th.QuerySLQ(src.Handle+" | ."+tblName+" | sum(.col_float)", nil)
			require.NoError(t, err)
			require.Len(t, sink.Recs, 1)
			// sum(float) must surface as decimal on every driver reached here
			// (ClickHouse is skipped above), value 3.75.
			assertSinkColDecimal(0, "3.75", nil)(t, sink)
		})
	}
}
