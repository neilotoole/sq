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
		"INTEGER":          kind.Int,
		"INT":              kind.Int,
		"SMALLINT":         kind.Int,
		"TINYINT":          kind.Int,
		"MEDIUMINT":        kind.Int,
		"BIGINT":           kind.Int,
		"BIT":              kind.Int,
		"DECIMAL":          kind.Decimal,
		"DECIMAL(5,2)":     kind.Decimal,
		"NUMERIC":          kind.Decimal,
		"FLOAT":            kind.Float,
		"FLOAT(8)":         kind.Float,
		"FLOAT(7,4)":       kind.Float,
		"REAL":             kind.Float,
		"DOUBLE":           kind.Float,
		"DOUBLE PRECISION": kind.Float,
		"DATE":             kind.Date,
		"DATETIME":         kind.Datetime,
		"TIMESTAMP":        kind.Datetime,
		"TIME":             kind.Time,
		"YEAR":             kind.Int,
		"CHAR":             kind.Text,
		"VARCHAR":          kind.Text,
		"VARCHAR(64)":      kind.Text,
		"TINYTEXT":         kind.Text,
		"TEXT":             kind.Text,
		"MEDIUMTEXT":       kind.Text,
		"LONGTEXT":         kind.Text,
		"BINARY":           kind.Bytes,
		"BINARY(4)":        kind.Bytes,
		"VARBINARY":        kind.Bytes,
		"BLOB":             kind.Bytes,
		"MEDIUMBLOB":       kind.Bytes,
		"LONGBLOB":         kind.Bytes,
		"ENUM":             kind.Text,
		"SET":              kind.Text,
		"BOOL":             kind.Bool,
		"BOOLEAN":          kind.Bool,
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
