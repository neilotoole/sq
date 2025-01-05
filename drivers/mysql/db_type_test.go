package mysql_test

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/cli/output"
	"github.com/neilotoole/sq/libsq"
	"github.com/neilotoole/sq/libsq/core/stringz"
	"github.com/neilotoole/sq/libsq/core/tablefq"
	"github.com/neilotoole/sq/libsq/source"
	"github.com/neilotoole/sq/testh"
	"github.com/neilotoole/sq/testh/fixt"
	"github.com/neilotoole/sq/testh/sakila"
)

// typeTestTableDDLPath is the location of the SQL CREATE statement
// for the "type_test" table that is used to verify handling of the
// driver's various data types.
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
		1,               //	col_id
		fixt.IntZ,       //	col_bigint
		fixt.IntZ,       //	col_bigint_n
		fixt.BitStringZ, //	col_binary
		fixt.BitStringZ, //	col_binary_n
		fixt.IntZ,       //	col_bit
		fixt.IntZ,       //	col_bit_n
		fixt.BytesZ,     //	col_blob
		fixt.BytesZ,     //	col_blob_n
		fixt.BoolZ,      //	col_bool
		fixt.BoolZ,      //	col_bool_n
		fixt.TextZ,      //	col_char
		fixt.TextZ,      //	col_char_n
		fixt.DateZ,      //	col_date
		fixt.DateZ,      //	col_date_n
		fixt.DatetimeZ,  //	col_datetime
		fixt.DatetimeZ,  //	col_datetime_n
		fixt.DecimalZ,   //	col_decimal
		fixt.DecimalZ,   //	col_decimal_n
		fixt.FloatZ,     //	col_double
		fixt.FloatZ,     //	col_double_n
		fixt.EnumAlfa,   //	col_enum
		fixt.EnumAlfa,   //	col_enum_n
		fixt.FloatZ,     //	col_float
		fixt.FloatZ,     //	col_float_n
		fixt.IntZ,       //	col_int
		fixt.IntZ,       //	col_int_n
		fixt.IntZ,       //	col_int24
		fixt.IntZ,       //	col_int24_n
		fixt.BytesZ,     //	col_longblob
		fixt.BytesZ,     //	col_longblob_n
		fixt.TextZ,      //	col_longtext
		fixt.TextZ,      //	col_longtext_n
		fixt.BytesZ,     //	col_mediumblob
		fixt.BytesZ,     //	col_mediumblob_n
		fixt.IntZ,       //	col_mediumint
		fixt.IntZ,       //	col_mediumint_n
		fixt.TextZ,      //	col_mediumtext
		fixt.TextZ,      //	col_mediumtext_n
		fixt.EnumAlfa,   //	col_set
		fixt.EnumAlfa,   //	col_set_n
		fixt.IntZ,       //	col_smallint
		fixt.IntZ,       //	col_smallint_n
		fixt.TextZ,      //	col_text
		fixt.TextZ,      //	col_text_n
		fixt.TimeOfDayZ, //	col_time
		fixt.TimeOfDayZ, //	col_time_n
		fixt.DatetimeZ,  //	col_timestamp
		fixt.DatetimeZ,  //	col_timestamp_n
		fixt.IntZ,       //	col_tinyint
		fixt.IntZ,       //	col_tinyint_n
		fixt.BytesZ,     //	col_tinyblob
		fixt.BytesZ,     //	col_tinyblob_n
		fixt.TextZ,      //	col_tinytext
		fixt.TextZ,      //	col_tinytext_n
		fixt.BitStringZ, //	col_varbinary
		fixt.BitStringZ, //	col_varbinary_n
		fixt.TextZ,      //	col_varchar
		fixt.TextZ,      //	col_varchar_n
		fixt.IntZ,       //	col_year
		fixt.IntZ,       //	col_year_n
	},
	{ // non-zero values
		2,              //	col_id
		fixt.Int,       //	col_bigint
		fixt.Int,       //	col_bigint_n
		fixt.BitString, //	col_binary
		fixt.BitString, //	col_binary_n
		fixt.Int,       //	col_bit
		fixt.Int,       //	col_bit_n
		fixt.Bytes,     //	col_blob
		fixt.Bytes,     //	col_blob_n
		fixt.Bool,      //	col_bool
		fixt.Bool,      //	col_bool_n
		fixt.Text,      //	col_char
		fixt.Text,      //	col_char_n
		fixt.Date,      //	col_date
		fixt.Date,      //	col_date_n
		fixt.Datetime,  //	col_datetime
		fixt.Datetime,  //	col_datetime_n
		fixt.Decimal,   //	col_decimal
		fixt.Decimal,   //	col_decimal_n
		fixt.Float,     //	col_double
		fixt.Float,     //	col_double_n
		fixt.EnumBravo, //	col_enum
		fixt.EnumBravo, //	col_enum_n
		fixt.Float,     //	col_float
		fixt.Float,     //	col_float_n
		fixt.Int,       //	col_int
		fixt.Int,       //	col_int_n
		fixt.Int,       //	col_int24
		fixt.Int,       //	col_int24_n
		fixt.Bytes,     //	col_longblob
		fixt.Bytes,     //	col_longblob_n
		fixt.Text,      //	col_longtext
		fixt.Text,      //	col_longtext_n
		fixt.Bytes,     //	col_mediumblob
		fixt.Bytes,     //	col_mediumblob_n
		fixt.Int,       //	col_mediumint
		fixt.Int,       //	col_mediumint_n
		fixt.Text,      //	col_mediumtext
		fixt.Text,      //	col_mediumtext_n
		fixt.EnumBravo, //	col_set
		fixt.EnumBravo, //	col_set_n
		fixt.Int,       //	col_smallint
		fixt.Int,       //	col_smallint_n
		fixt.Text,      //	col_text
		fixt.Text,      //	col_text_n
		fixt.TimeOfDay, //	col_time
		fixt.TimeOfDay, //	col_time_n
		fixt.Datetime,  //	col_timestamp
		fixt.Datetime,  //	col_timestamp_n
		fixt.Int,       //	col_tinyint
		fixt.Int,       //	col_tinyint_n
		fixt.Bytes,     //	col_tinyblob
		fixt.Bytes,     //	col_tinyblob_n
		fixt.Text,      //	col_tinytext
		fixt.Text,      //	col_tinytext_n
		fixt.BitString, //	col_varbinary
		fixt.BitString, //	col_varbinary_n
		fixt.Text,      //	col_varchar
		fixt.Text,      //	col_varchar_n
		fixt.Int,       //	col_year
		fixt.Int,       //	col_year_n
	},
	{
		3,              //	col_id
		fixt.Int,       //	col_bigint
		nil,            //	col_bigint_n
		fixt.BitString, //	col_binary
		nil,            //	col_binary_n
		fixt.Int,       //	col_bit
		nil,            //	col_bit_n
		fixt.Bytes,     //	col_blob
		nil,            //	col_blob_n
		fixt.Bool,      //	col_bool
		nil,            //	col_bool_n
		fixt.Text,      //	col_char
		nil,            //	col_char_n
		fixt.Date,      //	col_date
		nil,            //	col_date_n
		fixt.Datetime,  //	col_datetime
		nil,            //	col_datetime_n
		fixt.Decimal,   //	col_decimal
		nil,            //	col_decimal_n
		fixt.Float,     //	col_double
		nil,            //	col_double_n
		fixt.EnumBravo, //	col_enum
		nil,            //	col_enum_n
		fixt.Float,     //	col_float
		nil,            //	col_float_n
		fixt.Int,       //	col_int
		nil,            //	col_int_n
		fixt.Int,       //	col_int24
		nil,            //	col_int24_n
		fixt.Bytes,     //	col_longblob
		nil,            //	col_longblob_n
		fixt.Text,      //	col_longtext
		nil,            //	col_longtext_n
		fixt.Bytes,     //	col_mediumblob
		nil,            //	col_mediumblob_n
		fixt.Int,       //	col_mediumint
		nil,            //	col_mediumint_n
		fixt.Text,      //	col_mediumtext
		nil,            //	col_mediumtext_n
		fixt.EnumBravo, //	col_set
		nil,            //	col_set_n
		fixt.Int,       //	col_smallint
		nil,            //	col_smallint_n
		fixt.Text,      //	col_text
		nil,            //	col_text_n
		fixt.TimeOfDay, //	col_time
		nil,            //	col_time_n
		fixt.Datetime,  //	col_timestamp
		nil,            //	col_timestamp_n
		fixt.Int,       //	col_tinyint
		nil,            //	col_tinyint_n
		fixt.Bytes,     //	col_tinyblob
		nil,            //	col_tinyblob_n
		fixt.Text,      //	col_tinytext
		nil,            //	col_tinytext_n
		fixt.BitString, //	col_varbinary
		nil,            //	col_varbinary_n
		fixt.Text,      //	col_varchar
		nil,            //	col_varchar_n
		fixt.Int,       //	col_year
		nil,            //	col_year_n
	},
}

// typeTestColNames holds type_test table column names.
// TODO: add spatial types.
var typeTestColNames = []string{
	"col_id",
	"col_bigint",
	"col_bigint_n",
	"col_binary",
	"col_binary_n",
	"col_bit",
	"col_bit_n",
	"col_blob",
	"col_blob_n",
	"col_bool",
	"col_bool_n",
	"col_char",
	"col_char_n",
	"col_date",
	"col_date_n",
	"col_datetime",
	"col_datetime_n",
	"col_decimal",
	"col_decimal_n",
	"col_double",
	"col_double_n",
	"col_enum",
	"col_enum_n",
	"col_float",
	"col_float_n",
	"col_int",
	"col_int_n",
	"col_int24",
	"col_int24_n",
	"col_longblob",
	"col_longblob_n",
	"col_longtext",
	"col_longtext_n",
	"col_mediumblob",
	"col_mediumblob_n",
	"col_mediumint",
	"col_mediumint_n",
	"col_mediumtext",
	"col_mediumtext_n",
	"col_set",
	"col_set_n",
	"col_smallint",
	"col_smallint_n",
	"col_text",
	"col_text_n",
	"col_time",
	"col_time_n",
	"col_timestamp",
	"col_timestamp_n",
	"col_tinyint",
	"col_tinyint_n",
	"col_tinyblob",
	"col_tinyblob_n",
	"col_tinytext",
	"col_tinytext_n",
	"col_varbinary",
	"col_varbinary_n",
	"col_varchar",
	"col_varchar_n",
	"col_year",
	"col_year_n",
}

// createTypeTestTbl creates the type_test table, returning the actual table
// named used. If withData is true, the test data is also loaded.
// It is the caller's responsibility to drop the created table.
func createTypeTestTable(th *testh.Helper, src *source.Source, withData bool) (name string) {
	const (
		canonicalTblName = "type_test"
		insertTpl        = "INSERT INTO %s (%s) VALUES %s"
	)

	t := th.T
	db := th.OpenDB(src)

	tblDDL, err := os.ReadFile(typeTestTableDDLPath)
	require.NoError(t, err)

	// Replace the canonical table name
	actualTblName := stringz.UniqTableName(canonicalTblName)
	createStmt := strings.Replace(string(tblDDL), canonicalTblName, actualTblName, 1)

	// Create the table
	_, err = db.ExecContext(th.Context, createStmt)
	require.NoError(t, err)

	if !withData {
		return actualTblName
	}

	// Insert values
	placeholders := th.SQLDriverFor(src).Dialect().Placeholders(len(typeTestColNames), 1)
	insertStmt := fmt.Sprintf(insertTpl, actualTblName, strings.Join(typeTestColNames, ", "), placeholders)
	for i, insertRowVals := range typeTestVals {
		th.T.Logf("row[%d]: vals: %v", i, insertRowVals)
		_, err := db.Exec(insertStmt, insertRowVals...)
		require.NoError(t, err)
	}

	return actualTblName
}

// TestDatabaseTypes checks that our driver is dealing with database
// types correctly. The test constructs a "type_test" table with cols
// for various database types, inserts known data, and checks that
// the returned data matches the inserted data, including verifying
// that NULL is handled correctly.
func TestDatabaseTypes(t *testing.T) { //nolint:tparallel
	const wantRowCount = 3

	testCases := sakila.MyAll()
	for _, handle := range testCases {
		handle := handle

		t.Run(handle, func(t *testing.T) {
			t.Parallel()

			th := testh.New(t)
			src := th.Source(handle)
			t.Logf("using source %s: %s", src.Handle, src.Location)

			actualTblName := createTypeTestTable(th, src, true)
			t.Cleanup(func() { th.DropTable(src, tablefq.From(actualTblName)) })

			sink := &testh.RecordSink{}
			recw := output.NewRecordWriterAdapter(th.Context, sink)
			err := libsq.QuerySQL(th.Context, th.Open(src), nil, recw, "SELECT * FROM "+actualTblName)
			require.NoError(t, err)
			written, err := recw.Wait()
			require.NoError(t, err)

			require.Equal(t, int64(wantRowCount), written)
			require.Equal(t, wantRowCount, len(sink.Recs))
		})
	}
}

// TestDatabaseTypeJSON explicitly tests the JSON type
// introduced in MySQL v5.7.8.
func TestDatabaseTypeJSON(t *testing.T) {
	t.Parallel()

	const (
		canonicalTblName = "type_test_json"
		createStmtTpl    = `create table type_test_json
(
	col_id INT auto_increment primary key,
	col_json json not null,
	col_json_n json null
)`
	)

	testVals := [][]any{
		{
			int64(1),  // col_id
			fixt.JSON, // col_json
			fixt.JSON, // col_json_n
		},
		{
			int64(2),   // col_id
			fixt.JSONZ, // col_json
			fixt.JSONZ, // col_json_n
		},
		{
			int64(3),  // col_id
			fixt.JSON, // col_json
			nil,       // col_json_n
		},
	}

	// MySQL 5.6 doesn't support JSON type
	testCases := []string{sakila.My57, sakila.My8}
	for _, handle := range testCases {
		handle := handle

		t.Run(handle, func(t *testing.T) {
			t.Parallel()

			th, src, _, _, db := testh.NewWith(t, handle)

			// replace the canonical table name
			actualTblName := stringz.UniqTableName(canonicalTblName)
			createStmt := strings.Replace(createStmtTpl, canonicalTblName, actualTblName, 1)
			// Create the table

			_, err := db.ExecContext(th.Context, createStmt)
			require.NoError(t, err)
			t.Cleanup(func() { th.DropTable(src, tablefq.From(actualTblName)) })

			// Insert data
			insertStmt := fmt.Sprintf("INSERT INTO %s (col_id, col_json, col_json_n) VALUES (?,?,?)", actualTblName)
			for _, insertRowVals := range testVals {
				_, err = db.Exec(insertStmt, insertRowVals...)
				require.NoError(t, err)
			}

			// Query the inserted data
			sink := &testh.RecordSink{}
			recw := output.NewRecordWriterAdapter(th.Context, sink)
			err = libsq.QuerySQL(th.Context, th.Open(src), nil, recw, "SELECT * FROM "+actualTblName)
			require.NoError(t, err)
			written, err := recw.Wait()
			require.NoError(t, err)

			require.Equal(t, int64(len(testVals)), written)
			require.Equal(t, len(testVals), len(sink.Recs))
			for i := range testVals {
				for j := range testVals[i] {
					require.Equal(t, testVals[i][j], stringz.Val(sink.Recs[i][j]))
				}
			}
		})
	}
}
