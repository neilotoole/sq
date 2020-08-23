package mysql_test

import (
	"testing"

	"github.com/neilotoole/lg/testlg"
	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/drivers/mysql"
	"github.com/neilotoole/sq/libsq/core/sqlz"
	"github.com/neilotoole/sq/testh"
	"github.com/neilotoole/sq/testh/sakila"
)

func TestKindFromDBTypeName(t *testing.T) {
	t.Parallel()

	testCases := map[string]sqlz.Kind{
		"":                 sqlz.KindUnknown,
		"INTEGER":          sqlz.KindInt,
		"INT":              sqlz.KindInt,
		"SMALLINT":         sqlz.KindInt,
		"TINYINT":          sqlz.KindInt,
		"MEDIUMINT":        sqlz.KindInt,
		"BIGINT":           sqlz.KindInt,
		"BIT":              sqlz.KindInt,
		"DECIMAL":          sqlz.KindDecimal,
		"DECIMAL(5,2)":     sqlz.KindDecimal,
		"NUMERIC":          sqlz.KindDecimal,
		"FLOAT":            sqlz.KindFloat,
		"FLOAT(8)":         sqlz.KindFloat,
		"FLOAT(7,4)":       sqlz.KindFloat,
		"REAL":             sqlz.KindFloat,
		"DOUBLE":           sqlz.KindFloat,
		"DOUBLE PRECISION": sqlz.KindFloat,
		"DATE":             sqlz.KindDate,
		"DATETIME":         sqlz.KindDatetime,
		"TIMESTAMP":        sqlz.KindDatetime,
		"TIME":             sqlz.KindTime,
		"YEAR":             sqlz.KindInt,
		"CHAR":             sqlz.KindText,
		"VARCHAR":          sqlz.KindText,
		"VARCHAR(64)":      sqlz.KindText,
		"TINYTEXT":         sqlz.KindText,
		"TEXT":             sqlz.KindText,
		"MEDIUMTEXT":       sqlz.KindText,
		"LONGTEXT":         sqlz.KindText,
		"BINARY":           sqlz.KindBytes,
		"BINARY(4)":        sqlz.KindBytes,
		"VARBINARY":        sqlz.KindBytes,
		"BLOB":             sqlz.KindBytes,
		"MEDIUMBLOB":       sqlz.KindBytes,
		"LONGBLOB":         sqlz.KindBytes,
		"ENUM":             sqlz.KindText,
		"SET":              sqlz.KindText,
		"BOOL":             sqlz.KindBool,
		"BOOLEAN":          sqlz.KindBool,
	}

	log := testlg.New(t)
	for dbTypeName, wantKind := range testCases {
		gotKind := mysql.KindFromDBTypeName(log, "col", dbTypeName)
		require.Equal(t, wantKind, gotKind, "%q should produce %s but got %s", dbTypeName, wantKind, gotKind)
	}
}

func TestDatabase_SourceMetadata(t *testing.T) {
	for _, handle := range sakila.MyAll() {
		handle := handle

		t.Run(handle, func(t *testing.T) {
			t.Parallel()

			th, _, dbase, _ := testh.NewWith(t, handle)
			md, err := dbase.SourceMetadata(th.Context)
			require.NoError(t, err)
			require.Equal(t, "sakila", md.Name)
			require.Equal(t, handle, md.Handle)

			tblActor := md.Tables[0]
			require.Equal(t, sakila.TblActor, tblActor.Name)
			require.Equal(t, int64(sakila.TblActorCount), tblActor.RowCount)
			require.Equal(t, len(sakila.TblActorCols()), len(tblActor.Columns))
		})
	}
}

func TestDatabase_TableMetadata(t *testing.T) {
	for _, handle := range sakila.MyAll() {
		handle := handle

		t.Run(handle, func(t *testing.T) {
			t.Parallel()

			th, _, dbase, _ := testh.NewWith(t, handle)
			md, err := dbase.TableMetadata(th.Context, sakila.TblActor)
			require.NoError(t, err)
			require.Equal(t, sakila.TblActor, md.Name)
		})
	}
}
