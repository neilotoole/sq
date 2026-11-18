package sqlite3_test

import (
	"fmt"
	"os"
	"strings"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/libsq/core/stringz"
	"github.com/neilotoole/sq/libsq/source"
	"github.com/neilotoole/sq/testh"
	"github.com/neilotoole/sq/testh/fixt"
)

// This file holds the "type_test" fixture: a table with a column per common
// SQLite type, and rows of values for it. The metadata benchmarks use it to
// create many populated tables. The driver's column types are tested by
// TestColumnTypes (coltype_test.go).

// typeTestTableDDLPath is the location of the SQL CREATE statement
// for the "type_test" table.
const typeTestTableDDLPath = "testdata/type_test.ddl"

var typeTestColNames = []string{
	"col_id",
	"col_int",
	"col_int_n",
	"col_double",
	"col_double_n",
	"col_boolean",
	"col_boolean_n",
	"col_text",
	"col_text_n",
	"col_blob",
	"col_blob_n",
	"col_datetime",
	"col_datetime_n",
	"col_date",
	"col_date_n",
	"col_time",
	"col_time_n",
	"col_decimal",
	"col_decimal_n",
}

// typeTestVals is the set of vals inserted to the type_test table (and
// is expected when querying that table).
//
//   - Row 0 contains zero values (0, empty string, etc).
//   - Row 1 contains non-zero values.
//   - Row 2 contains non-zero values and nil values for cols that permit
//     nil values (those cols ending with _n such as col_int_n).
var typeTestVals = [][]any{
	{
		int64(1),        // col_id
		fixt.IntZ,       // col_int
		fixt.IntZ,       // col_int_n
		fixt.FloatZ,     // col_double
		fixt.FloatZ,     // col_double_n
		fixt.BoolZ,      // col_boolean
		fixt.BoolZ,      // col_boolean_n
		fixt.TextZ,      // col_text
		fixt.TextZ,      // col_text_n
		fixt.BytesZ,     // col_blob
		fixt.BytesZ,     // col_blob_n
		fixt.DatetimeZ,  // col_datetime
		fixt.DatetimeZ,  // col_datetime_n
		fixt.DateZ,      // col_date
		fixt.DateZ,      // col_date_n
		fixt.TimeOfDayZ, // col_time
		fixt.TimeOfDayZ, // col_time_n
		fixt.DecimalZ,   // col_decimal
		fixt.DecimalZ,   // col_decimal_n
	},
	{
		int64(2),       // col_id
		fixt.Int,       // col_int
		fixt.Int,       // col_int_n
		fixt.Float,     // col_double
		fixt.Float,     // col_double_n
		fixt.Bool,      // col_boolean
		fixt.Bool,      // col_boolean_n
		fixt.Text,      // col_text
		fixt.Text,      // col_text_n
		fixt.Bytes,     // col_blob
		fixt.Bytes,     // col_blob_n
		fixt.Datetime,  // col_datetime
		fixt.Datetime,  // col_datetime_n
		fixt.Date,      // col_date
		fixt.Date,      // col_date_n
		fixt.TimeOfDay, // col_time
		fixt.TimeOfDay, // col_time_n
		fixt.Decimal,   // col_decimal
		fixt.Decimal,   // col_decimal_n
	},
	{
		int64(3),       // col_id
		fixt.Int,       // col_int
		nil,            // col_int_n
		fixt.Float,     // col_double
		nil,            // col_double_n
		fixt.Bool,      // col_boolean
		nil,            // col_boolean_n
		fixt.Text,      // col_text
		nil,            // col_text_n
		fixt.Bytes,     // col_blob
		nil,            // col_blob_n
		fixt.Datetime,  // col_datetime
		nil,            // col_datetime_n
		fixt.Date,      // col_date
		nil,            // col_date_n
		fixt.TimeOfDay, // col_time
		nil,            // col_time_n
		fixt.Decimal,   // col_decimal
		nil,            // col_decimal_n
	},
}

// createTypeTestTbls creates nTimes instances of the type_test table,
// returning the actual table names used. If withData is true, the
// test data is also loaded. It is the caller's responsibility to drop
// the created tables.
func createTypeTestTbls(th *testh.Helper, src *source.Source, nTimes int, withData bool) (tblNames []string) {
	const canonicalTblName = "type_test"
	const insertTpl = "INSERT INTO %s (%s) VALUES %s"
	t := th.T
	db := th.OpenDB(src)

	tblDDL, err := os.ReadFile(typeTestTableDDLPath)
	require.NoError(t, err)

	baseTblName := stringz.UniqTableName(canonicalTblName)
	placeholders := th.SQLDriverFor(src).Dialect().Placeholders(len(typeTestColNames), 1)

	for i := range nTimes {
		actualTblName := fmt.Sprintf("%s_%d", baseTblName, i)
		createStmt := strings.Replace(string(tblDDL), canonicalTblName, actualTblName, 1)

		_, err = db.ExecContext(th.Context, createStmt)
		require.NoError(t, err)

		if !withData {
			continue
		}

		insertStmt := fmt.Sprintf(insertTpl, actualTblName, strings.Join(typeTestColNames, ", "), placeholders)
		for _, insertVals := range typeTestVals {
			_, err = db.Exec(insertStmt, insertVals...)
			require.NoError(t, err)
		}

		tblNames = append(tblNames, actualTblName)
	}

	return tblNames
}
