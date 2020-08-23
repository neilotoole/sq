package mysql_test

import (
	"testing"

	"github.com/neilotoole/lg/testlg"
	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/drivers/mysql"
	"github.com/neilotoole/sq/libsq/core/kind"
	"github.com/neilotoole/sq/testh"
	"github.com/neilotoole/sq/testh/sakila"
)

func TestKindFromDBTypeName(t *testing.T) {
	t.Parallel()

	testCases := map[string]kind.Kind{
		"":                 kind.Unknown,
		"INTEGER":          kind.KindInt,
		"INT":              kind.KindInt,
		"SMALLINT":         kind.KindInt,
		"TINYINT":          kind.KindInt,
		"MEDIUMINT":        kind.KindInt,
		"BIGINT":           kind.KindInt,
		"BIT":              kind.KindInt,
		"DECIMAL":          kind.KindDecimal,
		"DECIMAL(5,2)":     kind.KindDecimal,
		"NUMERIC":          kind.KindDecimal,
		"FLOAT":            kind.KindFloat,
		"FLOAT(8)":         kind.KindFloat,
		"FLOAT(7,4)":       kind.KindFloat,
		"REAL":             kind.KindFloat,
		"DOUBLE":           kind.KindFloat,
		"DOUBLE PRECISION": kind.KindFloat,
		"DATE":             kind.KindDate,
		"DATETIME":         kind.KindDatetime,
		"TIMESTAMP":        kind.KindDatetime,
		"TIME":             kind.KindTime,
		"YEAR":             kind.KindInt,
		"CHAR":             kind.Text,
		"VARCHAR":          kind.Text,
		"VARCHAR(64)":      kind.Text,
		"TINYTEXT":         kind.Text,
		"TEXT":             kind.Text,
		"MEDIUMTEXT":       kind.Text,
		"LONGTEXT":         kind.Text,
		"BINARY":           kind.KindBytes,
		"BINARY(4)":        kind.KindBytes,
		"VARBINARY":        kind.KindBytes,
		"BLOB":             kind.KindBytes,
		"MEDIUMBLOB":       kind.KindBytes,
		"LONGBLOB":         kind.KindBytes,
		"ENUM":             kind.Text,
		"SET":              kind.Text,
		"BOOL":             kind.KindBool,
		"BOOLEAN":          kind.KindBool,
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
