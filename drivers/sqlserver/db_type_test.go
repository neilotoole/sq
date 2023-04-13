package sqlserver_test

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/neilotoole/sq/testh/fixt"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/cli/output"
	"github.com/neilotoole/sq/libsq"
	"github.com/neilotoole/sq/libsq/core/stringz"
	"github.com/neilotoole/sq/libsq/source"
	"github.com/neilotoole/sq/testh"
	"github.com/neilotoole/sq/testh/sakila"
)

// typeTestTableDDLPath is the location of the SQL CREATE statement
// for the "type_test" table that is used to verify handling of the
// the driver's various data types.
const typeTestTableDDLPath = "testdata/type_test.ddl"

// typeTestVals is the set of vals inserted to the type_test table (and
// is expected when querying that table).
//
//   - Row 0 contains zero values (0, empty string, etc).
//   - Row 1 contains non-zero values.
//   - Row 2 contains non-zero values and nil values for cols that permit
//     nil values (those cols ending with _n such as col_text_n).
var typeTestVals = [][]any{
	{
		1,               // col_id
		fixt.IntZ,       // col_bigint
		fixt.IntZ,       // col_bigint_n
		fixt.BytesZ,     // col_binary
		fixt.BytesZ,     // col_binary_n
		fixt.BoolZ,      // col_bit
		fixt.BoolZ,      // col_bit_n
		fixt.BoolZ,      // col_bool
		fixt.BoolZ,      // col_bool_n
		fixt.TextZ,      // col_char
		fixt.TextZ,      // col_char_n
		fixt.DateZ,      // col_date
		fixt.DateZ,      // col_date_n
		fixt.DatetimeZ,  // col_datetime
		fixt.DatetimeZ,  // col_datetime_n
		fixt.DatetimeZ,  // col_datetime2
		fixt.DatetimeZ,  // col_datetime2_n
		fixt.DecimalZ,   // col_decimal
		fixt.DecimalZ,   // col_decimal_n
		fixt.FloatZ,     // col_float
		fixt.FloatZ,     // col_float_n
		fixt.IntZ,       // col_int
		fixt.IntZ,       // col_int_n
		fixt.DecimalZ,   // col_money
		fixt.DecimalZ,   // col_money_n
		fixt.TextZ,      // col_nchar
		fixt.TextZ,      // col_nchar_n
		fixt.TextZ,      // col_nvarchar
		fixt.TextZ,      // col_nvarchar_n
		fixt.DecimalZ,   // col_numeric
		fixt.DecimalZ,   // col_numeric_n
		fixt.FloatZ,     // col_real
		fixt.FloatZ,     // col_real_n
		fixt.DatetimeZ,  // col_smalldatetime
		fixt.DatetimeZ,  // col_smalldatetime_n
		fixt.IntZ,       // col_smallint
		fixt.IntZ,       // col_smallint_n
		fixt.DecimalZ,   // col_smallmoney
		fixt.DecimalZ,   // col_smallmoney_n
		fixt.TimeOfDayZ, // col_time
		fixt.TimeOfDayZ, // col_time_n
		fixt.IntZ,       // col_tinyint
		fixt.IntZ,       // col_tinyint_n
		fixt.UUIDZ,      // col_uuid
		fixt.UUIDZ,      // col_uuid_n
		fixt.BytesZ,     // col_varbinary
		fixt.BytesZ,     // col_varbinary_n
		fixt.TextZ,      // col_varchar
		fixt.TextZ,      // col_varchar_n
	},
	{
		2,              // col_id
		fixt.Int,       // col_bigint
		fixt.Int,       // col_bigint_n
		fixt.Bytes,     // col_binary
		fixt.Bytes,     // col_binary_n
		fixt.Bool,      // col_bit
		fixt.Bool,      // col_bit_n
		fixt.Bool,      // col_bool
		fixt.Bool,      // col_bool_n
		fixt.Text,      // col_char
		fixt.Text,      // col_char_n
		fixt.Date,      // col_date
		fixt.Date,      // col_date_n
		fixt.Datetime,  // col_datetime
		fixt.Datetime,  // col_datetime_n
		fixt.Datetime,  // col_datetime2
		fixt.Datetime,  // col_datetime2_n
		fixt.Decimal,   // col_decimal
		fixt.Decimal,   // col_decimal_n
		fixt.Float,     // col_float
		fixt.Float,     // col_float_n
		fixt.Int,       // col_int
		fixt.Int,       // col_int_n
		fixt.Decimal,   // col_money
		fixt.Decimal,   // col_money_n
		fixt.Text,      // col_nchar
		fixt.Text,      // col_nchar_n
		fixt.Text,      // col_nvarchar
		fixt.Text,      // col_nvarchar_n
		fixt.Decimal,   // col_numeric
		fixt.Decimal,   // col_numeric_n
		fixt.Float,     // col_real
		fixt.Float,     // col_real_n
		fixt.Datetime,  // col_smalldatetime
		fixt.Datetime,  // col_smalldatetime_n
		fixt.Int,       // col_smallint
		fixt.Int,       // col_smallint_n
		fixt.Decimal,   // col_smallmoney
		fixt.Decimal,   // col_smallmoney_n
		fixt.TimeOfDay, // col_time
		fixt.TimeOfDay, // col_time_n
		fixt.Int,       // col_tinyint
		fixt.Int,       // col_tinyint_n
		fixt.UUID,      // col_uuid
		fixt.UUID,      // col_uuid_n
		fixt.Bytes,     // col_varbinary
		fixt.Bytes,     // col_varbinary_n
		fixt.Text,      // col_varchar
		fixt.Text,      // col_varchar_n
	},
	{
		3,              // col_id
		fixt.Int,       // col_bigint
		nil,            // col_bigint_n
		fixt.Bytes,     // col_binary
		fixtBytesNil,   // col_binary_n  - NOTE use of fixtBytesNil
		fixt.Bool,      // col_bit
		nil,            // col_bit_n
		fixt.Bool,      // col_bool
		nil,            // col_bool_n
		fixt.Text,      // col_char
		nil,            // col_char_n
		fixt.Date,      // col_date
		nil,            // col_date_n
		fixt.Datetime,  // col_datetime
		nil,            // col_datetime_n
		fixt.Datetime,  // col_datetime2
		nil,            // col_datetime2_n
		fixt.Decimal,   // col_decimal
		nil,            // col_decimal_n
		fixt.Float,     // col_float
		nil,            // col_float_n
		fixt.Int,       // col_int
		nil,            // col_int_n
		fixt.Decimal,   // col_money
		nil,            // col_money_n
		fixt.Text,      // col_nchar
		nil,            // col_nchar_n
		fixt.Text,      // col_nvarchar
		nil,            // col_nvarchar_n
		fixt.Decimal,   // col_numeric
		nil,            // col_numeric_n
		fixt.Float,     // col_real
		nil,            // col_real_n
		fixt.Datetime,  // col_smalldatetime
		nil,            // col_smalldatetime_n
		fixt.Int,       // col_smallint
		nil,            // col_smallint_n
		fixt.Decimal,   // col_smallmoney
		nil,            // col_smallmoney_n
		fixt.TimeOfDay, // col_time
		nil,            // col_time_n
		fixt.Int,       // col_tinyint
		nil,            // col_tinyint_n
		fixt.UUID,      // col_uuid
		nil,            // col_uuid_n
		fixt.Bytes,     // col_varbinary
		fixtBytesNil,   // col_varbinary_n - NOTE use of fixtBytesNil
		fixt.Text,      // col_varchar
		nil,            // col_varchar_n
	},
}

// typeTestColNames holds type_test table column names.
var typeTestColNames = []string{
	"col_id",
	"col_bigint",
	"col_bigint_n",
	"col_binary",
	"col_binary_n",
	"col_bit",
	"col_bit_n",
	"col_bool",
	"col_bool_n",
	"col_char",
	"col_char_n",
	"col_date",
	"col_date_n",
	"col_datetime",
	"col_datetime_n",
	"col_datetime2",
	"col_datetime2_n",
	"col_decimal",
	"col_decimal_n",
	"col_float",
	"col_float_n",
	"col_int",
	"col_int_n",
	"col_money",
	"col_money_n",
	"col_nchar",
	"col_nchar_n",
	"col_nvarchar",
	"col_nvarchar_n",
	"col_numeric",
	"col_numeric_n",
	"col_real",
	"col_real_n",
	"col_smalldatetime",
	"col_smalldatetime_n",
	"col_smallint",
	"col_smallint_n",
	"col_smallmoney",
	"col_smallmoney_n",
	"col_time",
	"col_time_n",
	"col_tinyint",
	"col_tinyint_n",
	"col_uuid",
	"col_uuid_n",
	"col_varbinary",
	"col_varbinary_n",
	"col_varchar",
	"col_varchar_n",
}

// fixtBytesNil is used as the NULL value for insert to BINARY columns
// because of a bug with the mssql driver.
// See: https://github.com/denisenkom/go-mssqldb/issues/196
var fixtBytesNil []byte

// createTypeTestTbl creates the type_test table, returning the actual table
// named used. If withData is true, the test data is also loaded.
// It is the caller's responsibility to drop the created table.
func createTypeTestTable(th *testh.Helper, src *source.Source, withData bool) (rowCount int64, actualTblName string) {
	const canonicalTblName = "type_test"
	t, db := th.T, th.Open(src).DB()
	tblDDL, err := os.ReadFile(typeTestTableDDLPath)
	require.NoError(t, err)

	// replace the canonical table name
	actualTblName = stringz.UniqTableName(canonicalTblName)
	createStmt := strings.Replace(string(tblDDL), canonicalTblName, actualTblName, 1)

	_, err = db.ExecContext(th.Context, createStmt)
	require.NoError(t, err)
	t.Logf("Created table %s.%s", src.Handle, actualTblName)

	if !withData {
		return 0, actualTblName
	}

	placeholders := th.SQLDriverFor(src).Dialect().Placeholders(len(typeTestColNames), 1)
	const insertTpl = "INSERT INTO %s (%s) VALUES %s"
	insertStmt := fmt.Sprintf(insertTpl, actualTblName, strings.Join(typeTestColNames, ", "), placeholders)

	for _, insertRowVals := range typeTestVals {
		res, err := db.Exec(insertStmt, insertRowVals...)
		require.NoError(t, err)
		affected, err := res.RowsAffected()
		require.NoError(t, err)
		require.Equal(t, int64(1), affected)
		rowCount += affected
	}

	return rowCount, actualTblName
}

// TestDatabaseTypes checks that our driver is dealing with database
// types correctly. The test constructs a "type_test" table with cols
// for various database types, inserts known data, and checks that
// the returned data matches the inserted data, including verifying
// that NULL is handled correctly.
func TestDatabaseTypes(t *testing.T) {
	t.Parallel()

	testCases := []string{sakila.MS}
	for _, handle := range testCases {
		handle := handle

		t.Run(handle, func(t *testing.T) {
			t.Parallel()

			th := testh.New(t)
			src := th.Source(handle)
			insertCount, actualTblName := createTypeTestTable(th, src, true)
			t.Cleanup(func() { th.DropTable(src, actualTblName) })

			sink := &testh.RecordSink{}
			recw := output.NewRecordWriterAdapter(sink)
			err := libsq.QuerySQL(th.Context, th.Open(src), recw, fmt.Sprintf("SELECT * FROM %s", actualTblName))
			require.NoError(t, err)
			written, err := recw.Wait()
			require.NoError(t, err)
			require.Equal(t, insertCount, written)
			require.Equal(t, insertCount, int64(len(sink.Recs)))
		})
	}
}

// Test_MSSQLDB_DriverIssue196 illustrates a bug in the go-mssqldb driver.
// When attempting to insert a NULL value into BINARY or VARBINARY columns,
// the go-mssqldb driver treats a Go nil value incorrectly.
// A workaround is to explicitly insert "([]byte)(nil)" instead
// of "nil" which the driver then handles correctly.
// See https://github.com/denisenkom/go-mssqldb/issues/196
func Test_MSSQLDB_DriverIssue196(t *testing.T) {
	t.Parallel()

	// Note that although this test only checks BINARY, the same
	// behaviour is expected of VARBINARY.
	const (
		canonicalTblName = "type_test_issue_196"
		insertTpl        = "INSERT INTO %s (col_binary_n) VALUES %s"
		createStmtTpl    = `CREATE TABLE type_test_issue_196 (col_binary_n BINARY(255))`
	)

	actualTblName := stringz.UniqTableName(canonicalTblName)
	createStmt := strings.Replace(createStmtTpl, canonicalTblName, actualTblName, 1)

	// Create the demonstration table
	th := testh.New(t)
	src := th.Source(sakila.MS)
	db := th.Open(src).DB()
	_, err := db.ExecContext(th.Context, createStmt)
	require.NoError(t, err)

	t.Logf("Created table %s.%s", src.Handle, actualTblName)

	// Drop the newly-created table on cleanup
	t.Cleanup(func() {
		th.DropTable(src, actualTblName)
	})

	// Build the INSERT statement
	placeholders := th.SQLDriverFor(src).Dialect().Placeholders(1, 1)
	insertStmt := fmt.Sprintf(insertTpl, actualTblName, placeholders)

	// Insert empty byte slice, should work
	_, err = db.ExecContext(th.Context, insertStmt, []byte{})
	require.NoError(t, err, "empty byte slice insert is expected to work with this go-mssqldb driver version")

	// Insert non-empty byte slice, should work
	_, err = db.ExecContext(th.Context, insertStmt, []byte("hello"))
	require.NoError(t, err, "non-empty byte slice insert is expected to work with this go-mssqldb driver version")

	// Insert ([]byte)(nil), should work
	_, err = db.ExecContext(th.Context, insertStmt, []byte(nil))
	require.NoError(t, err, "([]byte)(nil) insert is expected to work with this go-mssqldb driver version")

	// Insert nil, expected to fail with this driver version
	_, err = db.ExecContext(th.Context, insertStmt, nil)
	require.Error(t, err, "nil insert is, alas, expected to fail with this go-mssqldb driver version")
}
