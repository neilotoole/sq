package postgres_test

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
		1,                // col_id
		fixt.IntZ,        // col_bigint
		fixt.IntZ,        // col_bigint_n
		1,                // col_bigserial
		fixtBitZ,         // col_bit
		fixtBitZ,         // col_bit_n
		fixt.BitStringZ,  // col_bitvarying
		fixt.BitStringZ,  // col_bitvarying_n
		fixt.BoolZ,       // col_boolean
		fixt.BoolZ,       // col_boolean_n
		fixtBoxZ,         // col_box
		fixtBoxZ,         // col_box_n
		fixt.BytesZ,      // col_bytea
		fixt.BytesZ,      // col_bytea_n
		fixt.TextZ,       // col_character
		fixt.TextZ,       // col_character_n
		fixt.TextZ,       // col_character_varying
		fixt.TextZ,       // col_character_varying_n
		fixtCIDRZ,        // col_cidr
		fixtCIDRZ,        // col_cidr_n
		fixtCircleZ,      // col_circle
		fixtCircleZ,      // col_circle_n
		fixt.DateZ,       // col_date
		fixt.DateZ,       // col_date_n
		fixt.FloatZ,      // col_double_precision
		fixt.FloatZ,      // col_double_precision_n
		fixtInetZ,        // col_inet
		fixtInetZ,        // col_inet_n
		fixt.IntZ,        // col_integer
		fixt.IntZ,        // col_integer_n
		fixtIntervalZ,    // col_interval
		fixtIntervalZ,    // col_interval_n
		fixt.JSONZ,       // col_json
		fixt.JSONZ,       // col_json_n
		fixt.JSONZ,       // col_jsonb
		fixt.JSONZ,       // col_jsonb_n
		fixtLineZ,        // col_line
		fixtLineZ,        // col_line_n
		fixtLineSegmentZ, // col_lseg
		fixtLineSegmentZ, // col_lseg_n
		fixtMACAddrZ,     // col_macaddr
		fixtMACAddrZ,     // col_macaddr_n
		fixt.MoneyZ,      // col_money
		fixt.MoneyZ,      // col_money_n
		fixt.DecimalZ,    // col_numeric
		fixt.DecimalZ,    // col_numeric_n
		fixtPathZ,        // col_path
		fixtPathZ,        // col_path_n
		fixtPgLSNZ,       // col_pg_lsn
		fixtPgLSNZ,       // col_pg_lsn_n
		fixtPointZ,       // col_point
		fixtPointZ,       // col_point_n
		fixtPolygonZ,     // col_polygon
		fixtPolygonZ,     // col_polygon_n
		fixt.FloatZ,      // col_real
		fixt.FloatZ,      // col_real_n
		fixt.IntZ,        // col_smallint
		fixt.IntZ,        // col_smallint_n
		1,                // col_smallserial
		1,                // col_serial
		fixt.TextZ,       // col_text
		fixt.TextZ,       // col_text_n
		fixt.TimeOfDayZ,  // col_time
		fixt.TimeOfDayZ,  // col_time_n
		fixt.TimeOfDayZ,  // col_timetz
		fixt.TimeOfDayZ,  // col_timetz_n
		fixt.DatetimeZ,   // col_timestamp
		fixt.DatetimeZ,   // col_timestamp_n
		fixt.DatetimeZ,   // col_timestamptz
		fixt.DatetimeZ,   // col_timestamptz_n
		fixtTSQueryZ,     // col_tsquery
		fixtTSQueryZ,     // col_tsquery_n
		fixtTSVectorZ,    // col_tsvector
		fixtTSVectorZ,    // col_tsvector_n
		fixtUUIDZ,        // col_uuid
		fixtUUIDZ,        // col_uuid_n
		fixtXMLZ,         // col_xml
		fixtXMLZ,         // col_xml_n
	},
	{
		2,               // col_id
		fixt.Int,        // col_bigint
		fixt.Int,        // col_bigint_n
		2,               // col_bigserial
		fixtBit,         // col_bit
		fixtBit,         // col_bit_n
		fixt.BitString,  // col_bitvarying
		fixt.BitString,  // col_bitvarying_n
		fixt.Bool,       // col_boolean
		fixt.Bool,       // col_boolean_n
		fixtBox,         // col_box
		fixtBox,         // col_box_n
		fixt.Bytes,      // col_bytea
		fixt.Bytes,      // col_bytea_n
		fixt.Text,       // col_character
		fixt.Text,       // col_character_n
		fixt.Text,       // col_character_varying
		fixt.Text,       // col_character_varying_n
		fixtCIDR,        // col_cidr
		fixtCIDR,        // col_cidr_n
		fixtCircle,      // col_circle
		fixtCircle,      // col_circle_n
		fixt.Date,       // col_date
		fixt.Date,       // col_date_n
		fixt.Float,      // col_double_precision
		fixt.Float,      // col_double_precision_n
		fixtInet,        // col_inet
		fixtInet,        // col_inet_n
		fixt.Int,        // col_integer
		fixt.Int,        // col_integer_n
		fixtInterval,    // col_interval
		fixtInterval,    // col_interval_n
		fixt.JSON,       // col_json
		fixt.JSON,       // col_json_n
		fixt.JSON,       // col_jsonb
		fixt.JSON,       // col_jsonb_n
		fixtLine,        // col_line
		fixtLine,        // col_line_n
		fixtLineSegment, // col_lseg
		fixtLineSegment, // col_lseg_n
		fixtMACAddr,     // col_macaddr
		fixtMACAddr,     // col_macaddr_n
		fixt.Money,      // col_money
		fixt.Money,      // col_money_n
		fixt.Decimal,    // col_numeric
		fixt.Decimal,    // col_numeric_n
		fixtPath,        // col_path
		fixtPath,        // col_path_n
		fixtPgLSN,       // col_pg_lsn
		fixtPgLSN,       // col_pg_lsn_n
		fixtPoint,       // col_point
		fixtPoint,       // col_point_n
		fixtPolygon,     // col_polygon
		fixtPolygon,     // col_polygon_n
		fixt.Float,      // col_real
		fixt.Float,      // col_real_n
		fixt.Int,        // col_smallint
		fixt.Int,        // col_smallint_n
		2,               // col_smallserial
		2,               // col_serial
		fixt.Text,       // col_text
		fixt.Text,       // col_text_n
		fixt.TimeOfDay,  // col_time
		fixt.TimeOfDay,  // col_time_n
		fixt.TimeOfDay,  // col_timet
		fixt.TimeOfDay,  // col_timet_n
		fixt.Datetime,   // col_timestamp
		fixt.Datetime,   // col_timestamp_n
		fixt.Datetime,   // col_timestampt
		fixt.Datetime,   // col_timestampt_n
		fixtTSQuery,     // col_tsquery
		fixtTSQuery,     // col_tsquery_n
		fixtTSVector,    // col_tsvector
		fixtTSVector,    // col_tsvector_n
		fixtUUID,        // col_uuid
		fixtUUID,        // col_uuid_n
		fixtXML,         // col_xml
		fixtXML,         // col_xml_n
	},
	{
		3,               // col_id
		fixt.Int,        // col_bigint
		nil,             // col_bigint_n
		3,               // col_bigserial
		fixtBit,         // col_bit
		nil,             // col_bit_n
		fixt.BitString,  // col_bitvarying
		nil,             // col_bitvarying_n
		fixt.Bool,       // col_boolean
		nil,             // col_boolean_n
		fixtBox,         // col_box
		nil,             // col_box_n
		fixt.Bytes,      // col_bytea
		nil,             // col_bytea_n
		fixt.Text,       // col_character
		nil,             // col_character_n
		fixt.Text,       // col_character_varying
		nil,             // col_character_varying_n
		fixtCIDR,        // col_cidr
		nil,             // col_cidr_n
		fixtCircle,      // col_circle
		nil,             // col_circle_n
		fixt.Date,       // col_date
		nil,             // col_date_n
		fixt.Float,      // col_double_precision
		nil,             // col_double_precision_n
		fixtInet,        // col_inet
		nil,             // col_inet_n
		fixt.Int,        // col_integer
		nil,             // col_integer_n
		fixtInterval,    // col_interval
		nil,             // col_interval_n
		fixt.JSON,       // col_json
		nil,             // col_json_n
		fixt.JSON,       // col_jsonb
		nil,             // col_jsonb_n
		fixtLine,        // col_line
		nil,             // col_line_n
		fixtLineSegment, // col_lseg
		nil,             // col_lseg_n
		fixtMACAddr,     // col_macaddr
		nil,             // col_macaddr_n
		fixt.Money,      // col_money
		nil,             // col_money_n
		fixt.Decimal,    // col_numeric
		nil,             // col_numeric_n
		fixtPath,        // col_path
		nil,             // col_path_n
		fixtPgLSN,       // col_pg_lsn
		nil,             // col_pg_lsn_n
		fixtPoint,       // col_point
		nil,             // col_point_n
		fixtPolygon,     // col_polygon
		nil,             // col_polygon_n
		fixt.Float,      // col_real
		nil,             // col_real_n
		fixt.Int,        // col_smallint
		nil,             // col_smallint_n
		3,               // col_smallserial
		3,               // col_serial
		fixt.Text,       // col_text
		nil,             // col_text_n
		fixt.TimeOfDay,  // col_time
		nil,             // col_time_n
		fixt.TimeOfDay,  // col_timet
		nil,             // col_timet_n
		fixt.Datetime,   // col_timestamp
		nil,             // col_timestamp_n
		fixt.Datetime,   // col_timestampt
		nil,             // col_timestampt_n
		fixtTSQuery,     // col_tsquery
		nil,             // col_tsquery_n
		fixtTSVector,    // col_tsvector
		nil,             // col_tsvector_n
		fixtUUID,        // col_uuid
		nil,             // col_uuid_n
		fixtXML,         // col_xml
		nil,             // col_xml_n
	},
}

// typeTestColNames holds type_test table column names.
var typeTestColNames = []string{
	"col_id",
	"col_bigint",
	"col_bigint_n",
	"col_bigserial", // no col_bigserial_n, because serial cannot be NULL
	"col_bit",
	"col_bit_n",
	"col_bitvarying",
	"col_bitvarying_n",
	"col_boolean",
	"col_boolean_n",
	"col_box",
	"col_box_n",
	"col_bytea",
	"col_bytea_n",
	"col_character",
	"col_character_n",
	"col_character_varying",
	"col_character_varying_n",
	"col_cidr",
	"col_cidr_n",
	"col_circle",
	"col_circle_n",
	"col_date",
	"col_date_n",
	"col_double_precision",
	"col_double_precision_n",
	"col_inet",
	"col_inet_n",
	"col_integer",
	"col_integer_n",
	"col_interval",
	"col_interval_n",
	"col_json",
	"col_json_n",
	"col_jsonb",
	"col_jsonb_n",
	"col_line",
	"col_line_n",
	"col_lseg",
	"col_lseg_n",
	"col_macaddr",
	"col_macaddr_n",
	"col_money",
	"col_money_n",
	"col_numeric",
	"col_numeric_n",
	"col_path",
	"col_path_n",
	"col_pg_lsn",
	"col_pg_lsn_n",
	"col_point",
	"col_point_n",
	"col_polygon",
	"col_polygon_n",
	"col_real",
	"col_real_n",
	"col_smallint",
	"col_smallint_n",
	"col_smallserial", // no col_smallserial_n, because serial cannot be NULL
	"col_serial",      // no col_serial_n, because serial cannot be NULL
	"col_text",
	"col_text_n",
	"col_time",
	"col_time_n",
	"col_timetz",
	"col_timetz_n",
	"col_timestamp",
	"col_timestamp_n",
	"col_timestamptz",
	"col_timestamptz_n",
	"col_tsquery",
	"col_tsquery_n",
	"col_tsvector",
	"col_tsvector_n",
	"col_uuid",
	"col_uuid_n",
	"col_xml",
	"col_xml_n",
}

const (
	fixtBit          = "1"
	fixtBitZ         = "0"
	fixtBitVar       = "1001"
	fixtBitVarZ      = "0"
	fixtBox          = "(0,0), (7,7)"
	fixtBoxZ         = "(0,0), (0,0)"
	fixtCIDR         = "192.168.0.1"
	fixtCIDRZ        = "0.0.0.0"
	fixtInet         = "192.168.0.1"
	fixtInetZ        = "0.0.0.0"
	fixtCircle       = "((0,0), 7)"
	fixtCircleZ      = "((0,0), 0)"
	fixtInterval     = "7 seconds"
	fixtIntervalZ    = "0 seconds"
	fixtLine         = `[(0,0),(7,0)]`
	fixtLineZ        = `[(0,0),(1,0)]` // line must have two distinct points
	fixtLineSegment  = `[(0,0),(7,0)]`
	fixtLineSegmentZ = `[(0,0),(0,0)]`
	fixtMACAddr      = "07:07:07:07:07:07"
	fixtMACAddrZ     = "00:00:00:00:00:00"
	fixtPath         = "[(0,0),(7,0)]"
	fixtPathZ        = "[(0,0),(0,0)]"
	fixtPgLSN        = "7/7"
	fixtPgLSNZ       = "0/0"
	fixtPoint        = "(7,7)"
	fixtPointZ       = "(0,0)"
	fixtPolygon      = "((0,0),(0,7),(7,0))"
	fixtPolygonZ     = "((0,0))"
	fixtTSQuery      = "alfa | bravo"
	fixtTSQueryZ     = ""
	fixtTSVector     = "alfa bravo charlie delta"
	fixtTSVectorZ    = ""
	fixtUUID         = "77777777-7777-7777-7777-777777777777"
	fixtUUIDZ        = "00000000-0000-0000-0000-000000000000"
	fixtXML          = "<val>7</val>"
	fixtXMLZ         = ""
)

// createTypeTestTbl creates the type_test table, returning the actual table
// named used. If withData is true, the test data is also loaded.
// It is the caller's responsibility to drop the created table.
func createTypeTestTable(th *testh.Helper, src *source.Source, withData bool) (rowCount int64, actualTblName string) {
	const canonicalTblName = "type_test"
	t := th.T
	db := th.OpenDB(src)
	tblDDL, err := os.ReadFile(typeTestTableDDLPath)
	require.NoError(t, err)

	// replace the canonical table name
	actualTblName = stringz.UniqTableName(canonicalTblName)
	createStmt := strings.Replace(string(tblDDL), canonicalTblName, actualTblName, 1)

	_, err = db.ExecContext(th.Context, createStmt)
	require.NoError(t, err)

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

	testCases := sakila.PgAll()
	for _, handle := range testCases {
		t.Run(handle, func(t *testing.T) {
			t.Parallel()

			th := testh.New(t)
			src := th.Source(handle)
			insertCount, actualTblName := createTypeTestTable(th, src, true)
			t.Cleanup(func() { th.DropTable(src, tablefq.From(actualTblName)) })
			sink := &testh.RecordSink{}
			recw := output.NewRecordWriterAdapter(th.Context, sink)
			err := libsq.QuerySQL(th.Context, th.Open(src), nil, recw, "SELECT * FROM "+actualTblName)
			require.NoError(t, err)
			written, err := recw.Wait()
			require.NoError(t, err)
			require.Equal(t, insertCount, written)
		})
	}
}
