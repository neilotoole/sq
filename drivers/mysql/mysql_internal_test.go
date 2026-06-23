package mysql

import (
	"database/sql"
	"reflect"
	"testing"
	"time"

	"github.com/go-sql-driver/mysql"
	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/libsq/ast/render"
	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/jointype"
	"github.com/neilotoole/sq/libsq/core/kind"
	"github.com/neilotoole/sq/libsq/core/record"
	"github.com/neilotoole/sq/libsq/core/schema"
	"github.com/neilotoole/sq/libsq/core/sqlz"
	"github.com/neilotoole/sq/libsq/driver"
	"github.com/neilotoole/sq/libsq/source"
	"github.com/neilotoole/sq/libsq/source/drivertype"
)

func TestDriverFor(t *testing.T) {
	p := &Provider{}

	drvr, err := p.DriverFor(drivertype.MySQL)
	require.NoError(t, err)
	require.NotNil(t, drvr)

	_, err = p.DriverFor(drivertype.Pg)
	require.Error(t, err, "non-MySQL type should be rejected")
}

func TestValidateSource(t *testing.T) {
	d := &driveri{}

	src := &source.Source{Handle: "@my", Type: drivertype.MySQL, Location: "mysql://x"}
	got, err := d.ValidateSource(src)
	require.NoError(t, err)
	require.Equal(t, src, got)

	_, err = d.ValidateSource(&source.Source{Handle: "@bad", Type: drivertype.Pg})
	require.Error(t, err, "non-MySQL source type should be rejected")
}

func TestConnParams(t *testing.T) {
	d := &driveri{}
	params := d.ConnParams()

	// Regression: the go-sql-driver DSN param is "maxAllowedPacket",
	// not "maxAllowedPackage". A typo here silently feeds the user a
	// param name the driver ignores.
	require.Contains(t, params, "maxAllowedPacket")
	require.NotContains(t, params, "maxAllowedPackage")

	// Spot-check a few other documented params are present.
	require.Contains(t, params, "parseTime")
	require.Contains(t, params, "tls")
	require.Equal(t, collations, params["collation"])
}

func TestLocationShape(t *testing.T) {
	d := &driveri{}
	shape := d.LocationShape()
	require.Equal(t, drivertype.MySQL, shape.Type)
	require.Equal(t, []string{"mysql"}, shape.Schemes)
	require.NotEmpty(t, shape.Segments)
}

func TestDriverMetadata(t *testing.T) {
	d := &driveri{}
	md := d.DriverMetadata()
	require.Equal(t, drivertype.MySQL, md.Type)
	require.True(t, md.IsSQL)
	require.Equal(t, 3306, md.DefaultPort)
}

func TestDialect(t *testing.T) {
	d := &driveri{}
	dl := d.Dialect()
	require.Equal(t, drivertype.MySQL, dl.Type)
	require.True(t, dl.IntBool)
	require.False(t, dl.Catalog)
	// MySQL does not support FULL OUTER JOIN.
	require.NotContains(t, dl.Joins, jointype.FullOuter)
}

func TestRenderer(t *testing.T) {
	d := &driveri{}
	r := d.Renderer()
	require.NotNil(t, r)
	// The MySQL renderer installs a number of function overrides; a few
	// that are load-bearing for portability (issues #594, #839).
	require.NotNil(t, r.FunctionOverrides)
	require.NotEmpty(t, r.FunctionNames)
}

func TestDBTypeNameFromKind(t *testing.T) {
	testCases := map[kind.Kind]string{ //nolint:exhaustive // Unknown/Null tested via panic below
		kind.Text:     "TEXT",
		kind.Int:      "INT",
		kind.Float:    "DOUBLE",
		kind.Decimal:  "DECIMAL",
		kind.Bool:     "TINYINT(1)",
		kind.Datetime: "DATETIME",
		kind.Time:     "TIME",
		kind.Date:     "DATE",
		kind.Bytes:    "BLOB",
	}

	for knd, want := range testCases {
		require.Equal(t, want, dbTypeNameFromKind(knd), "kind %s", knd)
	}

	// kind.Unknown / kind.Null are not mappable to a column type, and
	// the function panics rather than emit invalid DDL.
	require.Panics(t, func() { _ = dbTypeNameFromKind(kind.Unknown) })
	require.Panics(t, func() { _ = dbTypeNameFromKind(kind.Null) })
}

func TestCanonicalTableType(t *testing.T) {
	require.Equal(t, sqlz.TableTypeTable, canonicalTableType("BASE TABLE"))
	require.Equal(t, sqlz.TableTypeView, canonicalTableType("VIEW"))
	require.Equal(t, "", canonicalTableType("SYSTEM VIEW"))
	require.Equal(t, "", canonicalTableType(""))
}

func TestBuildUpdateStmt(t *testing.T) {
	got, err := buildUpdateStmt("person", []string{"name", "age"}, "age > 18")
	require.NoError(t, err)
	require.Equal(t, "UPDATE `person` SET `name` = ?, `age` = ? WHERE age > 18", got)

	got, err = buildUpdateStmt("person", []string{"name"}, "")
	require.NoError(t, err)
	require.Equal(t, "UPDATE `person` SET `name` = ?", got)

	_, err = buildUpdateStmt("person", nil, "")
	require.Error(t, err, "no columns should be an error")
}

func TestBuildCreateTableStmt_Minimal(t *testing.T) {
	tbl := schema.NewTable("t", []string{"a"}, []kind.Kind{kind.Int})
	got := buildCreateTableStmt(tbl)
	require.Equal(t, "CREATE TABLE `t` (\n`a` INT\n)", got)
}

func TestBuildCreateTableStmt_Full(t *testing.T) {
	tbl := schema.NewTable(
		"person",
		[]string{"id", "name", "email", "org_id", "mgr_id"},
		[]kind.Kind{kind.Int, kind.Text, kind.Text, kind.Int, kind.Int},
	)
	tbl.PKColName = "id"
	tbl.AutoIncrement = true

	// name: NOT NULL + default.
	tbl.Cols[1].NotNull = true
	tbl.Cols[1].HasDefault = true
	// email: unique.
	tbl.Cols[2].Unique = true
	// org_id: FK with default (empty) ON DELETE / ON UPDATE -> CASCADE.
	tbl.Cols[3].ForeignKey = &schema.FKConstraint{RefTable: "org", RefCol: "id"}
	// mgr_id: FK with explicit actions.
	tbl.Cols[4].ForeignKey = &schema.FKConstraint{
		RefTable: "person", RefCol: "id", OnDelete: "SET NULL", OnUpdate: "RESTRICT",
	}

	got := buildCreateTableStmt(tbl)

	require.Contains(t, got, "CREATE TABLE `person` (")
	require.Contains(t, got, "`id` INT AUTO_INCREMENT")
	require.Contains(t, got, "PRIMARY KEY (`id`)")
	require.Contains(t, got, "UNIQUE KEY `person_id_uindex` (`id`)")
	require.Contains(t, got, "`name` TEXT  NOT NULL") // text default is empty string
	require.Contains(t, got, "UNIQUE KEY `person_email_uindex` (`email`)")
	// FK with defaulted actions.
	require.Contains(t, got, "REFERENCES `org` (`id`) ON DELETE CASCADE ON UPDATE CASCADE")
	// FK with explicit actions.
	require.Contains(t, got, "REFERENCES `person` (`id`) ON DELETE SET NULL ON UPDATE RESTRICT")
}

func TestExtractTblNameFromNotExistErr(t *testing.T) {
	t.Run("nil", func(t *testing.T) {
		_, _, ok := extractTblNameFromNotExistErr(nil)
		require.False(t, ok)
	})

	t.Run("non_mysql_err", func(t *testing.T) {
		_, _, ok := extractTblNameFromNotExistErr(errz.New("boom"))
		require.False(t, ok)
	})

	t.Run("wrong_code", func(t *testing.T) {
		err := &mysql.MySQLError{Number: errNumConCount, Message: "too many"}
		_, _, ok := extractTblNameFromNotExistErr(err)
		require.False(t, ok)
	})

	t.Run("schema_dot_table", func(t *testing.T) {
		err := &mysql.MySQLError{
			Number:  errNumTableNotExist,
			Message: "Table 'sakila.actor' doesn't exist",
		}
		schma, tbl, ok := extractTblNameFromNotExistErr(err)
		require.True(t, ok)
		require.Equal(t, "sakila", schma)
		require.Equal(t, "actor", tbl)
	})

	t.Run("no_separator", func(t *testing.T) {
		err := &mysql.MySQLError{
			Number:  errNumTableNotExist,
			Message: "Table 'actor' doesn't exist",
		}
		_, _, ok := extractTblNameFromNotExistErr(err)
		require.False(t, ok, "a message without a schema.table separator yields no name")
	})

	t.Run("empty_after_trim", func(t *testing.T) {
		err := &mysql.MySQLError{
			Number:  errNumTableNotExist,
			Message: "Table '' doesn't exist",
		}
		_, _, ok := extractTblNameFromNotExistErr(err)
		require.False(t, ok)
	})
}

func TestErrw(t *testing.T) {
	require.NoError(t, errw(nil))

	generic := errz.New("plain")
	require.Error(t, errw(generic))

	notExist := &mysql.MySQLError{Number: errNumTableNotExist, Message: "Table 'x.y' doesn't exist"}
	wrapped := errw(notExist)
	require.Error(t, wrapped)
	require.True(t, errz.Has[*driver.NotExistError](wrapped),
		"table-not-exist must be wrapped as a NotExistError")
}

func TestIsErrTooManyConnections(t *testing.T) {
	require.False(t, isErrTooManyConnections(nil))
	require.False(t, isErrTooManyConnections(errz.New("nope")))

	err := &mysql.MySQLError{Number: errNumConCount, Message: "Too many connections"}
	require.True(t, isErrTooManyConnections(err))
	require.True(t, isErrTooManyConnections(errw(err)), "wrapped error should still match")
}

func TestMungeSetDatetimeFromString(t *testing.T) {
	rec := []any{nil}

	mungeSetDatetimeFromString("2021-08-30T11:50:00Z", 0, rec)
	got, ok := rec[0].(time.Time)
	require.True(t, ok, "a valid RFC3339 string should be parsed to time.Time")
	require.Equal(t, 2021, got.Year())

	// An unparseable string leaves rec unchanged.
	rec[0] = "not-a-time"
	mungeSetDatetimeFromString("not-a-time", 0, rec)
	require.Equal(t, "not-a-time", rec[0])
}

func TestMungeSetZeroValue(t *testing.T) {
	meta := record.Meta{
		record.NewFieldMeta(
			&record.ColumnTypeData{Name: "n", Kind: kind.Int, ScanType: reflect.TypeFor[int64]()},
			"n",
		),
	}
	rec := []any{nil}
	mungeSetZeroValue(0, rec, meta)
	require.Equal(t, int64(0), rec[0])
}

// mkMeta builds a single-column record.Meta for munge tests.
func mkMeta(name string, knd kind.Kind, nullable bool, scanType reflect.Type) record.Meta {
	return record.Meta{
		record.NewFieldMeta(&record.ColumnTypeData{
			Name:        name,
			Kind:        knd,
			ScanType:    scanType,
			HasNullable: true,
			Nullable:    nullable,
		}, name),
	}
}

func TestNewInsertMungeFunc(t *testing.T) {
	t.Run("len_mismatch", func(t *testing.T) {
		meta := mkMeta("n", kind.Int, false, reflect.TypeFor[int64]())
		fn := newInsertMungeFunc("t", meta)
		err := fn(record.Record{1, 2})
		require.Error(t, err, "record/dest column count mismatch must error")
	})

	t.Run("nil_non_nullable_gets_zero", func(t *testing.T) {
		meta := mkMeta("n", kind.Int, false, reflect.TypeFor[int64]())
		fn := newInsertMungeFunc("t", meta)
		rec := record.Record{nil}
		require.NoError(t, fn(rec))
		require.Equal(t, int64(0), rec[0])
	})

	t.Run("text_left_untouched", func(t *testing.T) {
		meta := mkMeta("s", kind.Text, true, reflect.TypeFor[string]())
		fn := newInsertMungeFunc("t", meta)
		rec := record.Record{"hello"}
		require.NoError(t, fn(rec))
		require.Equal(t, "hello", rec[0])
	})

	t.Run("empty_string_nullable_becomes_nil", func(t *testing.T) {
		meta := mkMeta("d", kind.Datetime, true, reflect.TypeFor[sql.NullTime]())
		fn := newInsertMungeFunc("t", meta)
		s := ""
		rec := record.Record{&s}
		require.NoError(t, fn(rec))
		require.Nil(t, rec[0])
	})

	t.Run("datetime_string_parsed", func(t *testing.T) {
		meta := mkMeta("d", kind.Datetime, true, reflect.TypeFor[sql.NullTime]())
		fn := newInsertMungeFunc("t", meta)
		s := "2021-08-30T11:50:00Z"
		rec := record.Record{&s}
		require.NoError(t, fn(rec))
		got, ok := rec[0].(time.Time)
		require.True(t, ok)
		require.Equal(t, 2021, got.Year())
	})

	t.Run("empty_value_string_nullable_becomes_nil", func(t *testing.T) {
		// A non-pointer empty string into a nullable non-text column.
		meta := mkMeta("n", kind.Int, true, reflect.TypeFor[int64]())
		fn := newInsertMungeFunc("t", meta)
		rec := record.Record{""}
		require.NoError(t, fn(rec))
		require.Nil(t, rec[0])
	})

	t.Run("empty_value_string_non_nullable_gets_zero", func(t *testing.T) {
		// A non-pointer empty string into a non-nullable non-text column.
		meta := mkMeta("n", kind.Int, false, reflect.TypeFor[int64]())
		fn := newInsertMungeFunc("t", meta)
		rec := record.Record{""}
		require.NoError(t, fn(rec))
		require.Equal(t, int64(0), rec[0])
	})

	t.Run("empty_ptr_string_non_nullable_gets_zero", func(t *testing.T) {
		meta := mkMeta("n", kind.Int, false, reflect.TypeFor[int64]())
		fn := newInsertMungeFunc("t", meta)
		s := ""
		rec := record.Record{&s}
		require.NoError(t, fn(rec))
		require.Equal(t, int64(0), rec[0])
	})
}

func TestGetNewRecordFunc(t *testing.T) {
	t.Run("happy_path", func(t *testing.T) {
		meta := mkMeta("n", kind.Int, false, reflect.TypeFor[int64]())
		fn := getNewRecordFunc(meta)
		v := int64(42)
		rec, err := fn([]any{&v})
		require.NoError(t, err)
		require.Equal(t, int64(42), rec[0])
	})

	t.Run("unknown_type_errors", func(t *testing.T) {
		// A value whose type NewRecordFromScanRow does not recognize is
		// added to the skipped set; getNewRecordFunc cannot munge it and
		// must return an error rather than emit a bad record.
		meta := mkMeta("x", kind.Unknown, true, reflect.TypeFor[struct{}]())
		fn := getNewRecordFunc(meta)
		type weird struct{ v int }
		_, err := fn([]any{&weird{v: 1}})
		require.Error(t, err)
	})
}

func TestRenderFuncRowNum(t *testing.T) {
	rc := &render.Context{Fragments: &render.Fragments{}}

	frag, err := renderFuncRowNum(rc, nil)
	require.NoError(t, err)
	require.Contains(t, frag, ":=")
	require.Contains(t, frag, "@row_number_")
	require.Len(t, rc.Fragments.PreExecStmts, 1)
	require.Len(t, rc.Fragments.PostExecStmts, 1)
	require.Contains(t, rc.Fragments.PreExecStmts[0], "SET @row_number_")
	require.Contains(t, rc.Fragments.PreExecStmts[0], "= 0;")
}
