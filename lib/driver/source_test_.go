package driver

import (
	"github.com/neilotoole/gotils/testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const typMySQL = Type("mysql")
const typPostgres = Type("postgres")
const typSQLite3 = Type("sqlite3")
const typXSLX = Type("xlsx")

func TestSource_Driver(t *testing.T) {

	src, err := NewSource("a1", "mysql://user:pass@localhost:3306/mydb1")
	assert.Nil(t, err)
	assert.Equal(t, Type("mysql"), src.Type)
	assert.Equal(t, "[a1] mysql://user:pass@localhost:3306/mydb1", src.String())
	assert.Equal(t, "user:pass@localhost:3306/mydb1", src.ConnURI())

	src, err = NewSource("a1", "postgres://pqgotest:password@localhost/pqgotest")
	assert.Nil(t, err)
	assert.Equal(t, Type("postgres"), src.Type)
	assert.Equal(t, "[a1] postgres://pqgotest:password@localhost/pqgotest", src.String())
	assert.Equal(t, "postgres://pqgotest:password@localhost/pqgotest", src.ConnURI())

}

func TestSourceGetTypeFromRef(t *testing.T) {

	items := []struct {
		loc string
		typ Type
	}{
		{`mysql://root:root@tcp(localhost:33067)/sq_mydb1`, typMySQL},
		{`postgres://sq:sq@localhost/sq_pg1?sslmode=disable`, typPostgres},
		{`sqlite3:///Users/neilotoole/nd/go/src/github.com/neilotoole/sq/test/sqlite/sqlite_db1`, typSQLite3},
		//{`xlsx:///Users/neilotoole/nd/go/src/github.com/neilotoole/sq/test/xlsx/test.xlsx`, typXSLX},
		{`https://s3.amazonaws.com/sq.neilotoole.io/testdata/1.0/xslx/test.xlsx`, typXSLX},
		{`/Users/neilotoole/nd/go/src/github.com/neilotoole/sq/test/xlsx/test.xlsx`, typXSLX},
	}

	for _, item := range items {

		typ, err := GetTypeFromSourceLocation(item.loc)
		assert.Nil(t, err)
		assert.Equal(t, item.typ, typ)
	}

	//typ, err := getTypeFromRef()

	// A ref can look like:
	//NAME              DRIVER    REF
	//my1               mysql     mysql://root:root@tcp(localhost:33067)/sq_mydb1
	//pg1               postgres  postgres://sq:sq@localhost/sq_pg1?sslmode=disable
	//sl1               sqlite3   sqlite3:///Users/neilotoole/nd/go/src/github.com/neilotoole/sq/test/sqlite/sqlite_db1
	//excel1            xlsx      xlsx:///Users/neilotoole/nd/go/src/github.com/neilotoole/sq/test/xlsx/test.xlsx
	//
	//excel2            xlsx      /Users/neilotoole/nd/go/src/github.com/neilotoole/sq/test/xlsx/test.xlsx
	//excel3            xlsx      test.xlsx
	//excel4            xlsx      https://s3.amazonaws.com/sq.neilotoole.io/testdata/1.0/xslx/test.xlsx

}

func TestDataSources(t *testing.T) {

	driverMySQL := Type("mysql")
	driverPostgres := Type("postgres")

	srcs := NewSourceSet()

	mydb1, err := NewSource("mydb1", "mysql://user:pass@localhost:3306/mydb1")
	require.Nil(t, err)
	require.NotNil(t, mydb1)
	require.Equal(t, driverMySQL, mydb1.Type)

	pg1, err := NewSource("pg1", "postgres://pqgotest:password@localhost/pqgotest")
	require.Nil(t, err)
	require.NotNil(t, pg1)
	require.Equal(t, driverPostgres, pg1.Type)

	err = srcs.Add(mydb1)
	require.Nil(t, err)
	srcs.Add(pg1)
	require.Nil(t, err)

	require.Equal(t, 2, len(srcs.Items))

	src, ok := srcs.Active()
	assert.Nil(t, src)
	assert.False(t, ok)

	src, err = srcs.SetActive(mydb1.Handle)
	assert.Nil(t, err)
	assert.NotNil(t, src)
	src, ok = srcs.Active()
	assert.NotNil(t, src)
	assert.True(t, ok)
	assert.Equal(t, mydb1.Handle, src.Handle)

	src, err = srcs.SetActive(pg1.Handle)
	assert.Nil(t, err)
	assert.NotNil(t, src)
	src, ok = srcs.Active()
	assert.NotNil(t, src)
	assert.True(t, ok)
	assert.Equal(t, pg1.Handle, src.Handle)

	// Remove an item
	err = srcs.Remove(pg1.Handle)
	assert.Nil(t, err)

	src, err = srcs.Get(pg1.Handle)
	assert.Nil(t, src)
	assert.NotNil(t, err)
	i, src := srcs.IndexOf(pg1.Handle)
	assert.Equal(t, -1, i)
	assert.Equal(t, 1, len(srcs.Items))
	src, ok = srcs.Active()
	assert.Nil(t, src)
	assert.False(t, ok)

	// Remove the other item
	src, err = srcs.SetActive(mydb1.Handle)
	assert.Nil(t, err)
	assert.NotNil(t, src)
	err = srcs.Remove(mydb1.Handle)
	assert.Nil(t, err)

	src, err = srcs.Get(mydb1.Handle)
	assert.Nil(t, src)
	assert.Error(t, err)
	i, src = srcs.IndexOf(mydb1.Handle)
	assert.Equal(t, -1, i)
	assert.Equal(t, 0, len(srcs.Items))
	src, ok = srcs.Active()
	assert.Nil(t, src)
	assert.False(t, ok)

}
