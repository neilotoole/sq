package sqlh

import (
	"testing"

	"os"

	"fmt"

	"time"

	"strings"

	"github.com/neilotoole/sq-driver/hackery/database/sql"
	_ "github.com/neilotoole/sq-driver/hackery/drivers/mysql"
	"github.com/stretchr/testify/require"
)

func TestNullBytes(t *testing.T) {

	var dest sql.Scanner
	var data []byte

	nb := NullBytes{}
	dest = &nb
	data = []byte("hello")

	err := dest.Scan(data)
	require.Nil(t, err)
	require.True(t, nb.Valid)
	require.Equal(t, data, nb.Bytes)
	v, err := nb.Value()
	require.Nil(t, err)
	require.Equal(t, data, v)

	// try with nil
	nb = NullBytes{}
	dest = &nb

	err = dest.Scan(nil)
	require.Nil(t, err)
	require.False(t, nb.Valid)
	v, err = nb.Value()
	require.Nil(t, err)
	require.Nil(t, v)
	//b, ok := v.([]byte)
	//require.True(t, ok)
	//require.Zero(t, b)

}

const mysqlDSEnv = `SQ_TEST_DS_MYSQL`

func init() {
	os.Setenv(mysqlDSEnv, `root:root@tcp(localhost:33067)/sq_mydb1`)
}

func TestIntegrationMySQL(t *testing.T) {

	t.Skip("need to fix ds env issue")

	ds, ok := os.LookupEnv(mysqlDSEnv)
	if !ok {
		t.Skipf("%s not present", mysqlDSEnv)
	}

	t.Logf("using data source: %s", ds)

	db, err := sql.Open("mysql", ds)
	require.Nil(t, err)
	require.NotNil(t, db)
	defer func() {
		require.Nil(t, db.Close())
	}()

	ts := time.Now()
	tblName := fmt.Sprintf("tbl_sqldatatype_%d", ts.Unix())
	createSQL := mysqlCreateSQL(tblName)
	t.Logf("creating table: %s", createSQL)
	_, err = db.Exec(createSQL)
	require.Nil(t, err)

	dropSQL := fmt.Sprintf(mysqlDropStmtTpl, tblName)
	defer func() {
		t.Logf("dropping test table: %s", dropSQL)
		_, err := db.Exec(dropSQL)
		require.Nil(t, err)
	}()

	numCols := len(mysqlColNames)

	insertSQL := fmt.Sprintf("INSERT INTO %s VALUES (%s)", tblName, placeholders(numCols))
	t.Logf("insert stmt: %s", insertSQL)
	insertStmt, err := db.Prepare(insertSQL)
	require.Nil(t, err)

	testVals := mysqlTestVals()

	for _, vals := range testVals {
		res, err := insertStmt.Exec(vals...)
		require.Nil(t, err)
		affected, err := res.RowsAffected()
		require.Nil(t, err)
		require.Equal(t, int64(1), affected)
	}

	selectSQL := fmt.Sprintf("SELECT * FROM %s", tblName)
	t.Logf("SELECT stmt: %s", selectSQL)
	rows, err := db.Query(selectSQL)
	require.Nil(t, err)
	require.NotNil(t, rows)

	defer func() {
		require.Nil(t, rows.Close())
	}()

	rCols, err := rows.Columns()
	require.Nil(t, err)
	require.Len(t, rCols, numCols)

	colTypes, err := rows.ColumnTypes()
	require.Nil(t, err)
	require.Len(t, colTypes, numCols)

	dataTypes, err := DataTypeFromCols(colTypes)
	require.Nil(t, err)
	require.Len(t, dataTypes, numCols)

	for ri := 0; rows.Next(); ri++ {
		t.Logf("*** scanning row %d ***", ri)
		dests := ScanDests(dataTypes)
		err := rows.Scan(dests...)
		require.Nil(t, err)
		for ci, val := range dests {
			v := ExtractValue(val)
			t.Logf("[%d:%d:%s] %T:  %v", ri, ci, mysqlColNames[ci], v, v)
		}
	}
}

func mysqlCreateSQL(tblName string) string {
	vars := make([]interface{}, 1+len(mysqlColNames))
	vars[0] = tblName
	for i, col := range mysqlColNames {
		vars[i+1] = col
	}

	createSQL := fmt.Sprintf(mysqlCreateStmtTpl, vars...)
	return createSQL

}

func mysqlTestVals() [][]interface{} {
	ts := time.Now()
	mysqlTestVals := [][]interface{}{
		{
			int64(1),
			int64(1),
			nil,
			true,
			nil,
			11,
			nil,
			float64(11.11),
			nil,
			ts,
			nil,
			ts,
			nil,
			ts,
			nil,
			ts,
			nil,
			"2016",
			nil,
			"hello world 1",
			nil,
			[]byte("i am binary data 1"),
			nil,
			"i am very long text 1",
			nil,
		},
		{
			int64(2),
			int64(2),
			nil,
			true,
			nil,
			22,
			nil,
			float64(22.22),
			nil,
			ts,
			nil,
			ts,
			nil,
			ts,
			nil,
			ts,
			nil,
			"2026",
			nil,
			"hello world 2",
			nil,
			[]byte("i am binary data 2"),
			nil,
			"i am very long text 2",
			nil,
		},
	}

	return mysqlTestVals
}

// placeholders returns a placeholder string with n elements, e.g. "?, ?, ?"
func placeholders(n int) string {

	p := make([]string, n)

	for i := range p {
		p[i] = "?"
	}

	return strings.Join(p, ", ")
}

var mysqlColNames = []string{
	"col_id",
	"col_int",
	"col_int_n",
	"col_bool",
	"col_bool_n",
	"col_decimal",
	"col_decimal_n",
	"col_float",
	"col_float_n",
	"col_timestamp",
	"col_timestamp_n",
	"col_date",
	"col_date_n",
	"col_time",
	"col_time_n",
	"col_datetime",
	"col_datetime_n",
	"col_year",
	"col_year_n",
	"col_varchar",
	"col_varchar_n",
	"col_blob",
	"col_blob_n",
	"col_longtext",
	"col_longtext_n",
}

const mysqlDropStmtTpl = `DROP TABLE IF EXISTS %s`

const mysqlCreateStmtTpl = `CREATE TABLE %s
(
    %s INT(11) PRIMARY KEY NOT NULL AUTO_INCREMENT,
    %s INT(11) NOT NULL,
    %s INT(11),
    %s TINYINT(1) NOT NULL,
    %s TINYINT(1),
    %s DECIMAL(10) NOT NULL,
    %s DECIMAL(10),
    %s FLOAT NOT NULL,
    %s FLOAT,
    %s TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL,
    %s TIMESTAMP NULL,
    %s DATE NOT NULL,
    %s DATE,
    %s TIME NOT NULL,
    %s TIME,
    %s DATETIME NOT NULL,
    %s DATETIME,
    %s YEAR(4) NOT NULL,
    %s YEAR(4),
    %s VARCHAR(255) NOT NULL,
    %s VARCHAR(255),
    %s BLOB NOT NULL,
    %s BLOB,
    %s LONGTEXT NOT NULL,
    %s LONGTEXT
)`

/*

	testVals := [][]interface{}{
		{
			1,               // col_id
			1,               // col_int
			nil,             // col_int_n
			true,            // col_bool
			nil,             // col_bool_n
			11,              // col_decimal
			nil,             // col_decimal_n
			11.11,           // col_float
			nil,             // col_float_n
			ts,              // col_timestamp
			nil,             // col_timestamp_n
			ts,              // col_date
			nil,             // col_date_n
			ts,              // col_time
			nil,             // col_time_n
			ts,              // col_datetime
			nil,             // col_datetime_n,
			"2016",          // col_year
			nil,             // col_year_n
			"hello world 1", // col_varchar
			nil,             // col_varchar_n
			[]byte("i am binary data 1"), // col_blob
			nil, // col_blob_n
			"i am very long text 1", // col_longtext
			nil, // col_longtext_n
		},
		{
			2,               // col_id
			2,               // col_int
			nil,             // col_int_n
			true,            // col_bool
			nil,             // col_bool_n
			22,              // col_decimal
			nil,             // col_decimal_n
			22.22,           // col_float
			nil,             // col_float_n
			ts,              // col_timestamp
			nil,             // col_timestamp_n
			ts,              // col_date
			nil,             // col_date_n
			ts,              // col_time
			nil,             // col_time_n
			ts,              // col_datetime
			nil,             // col_datetime_n,
			"2026",          // col_year
			nil,             // col_year_n
			"hello world 2", // col_varchar
			nil,             // col_varchar_n
			[]byte("i am binary data 2"), // col_blob
			nil, // col_blob_n
			"i am very long text 2", // col_longtext
			nil, // col_longtext_n
		},
	}
*/
