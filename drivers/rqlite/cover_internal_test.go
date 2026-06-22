package rqlite

import (
	"context"
	"database/sql"
	"errors"
	"net/http/httptest"
	"net/url"
	"reflect"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/libsq/ast"
	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/kind"
	"github.com/neilotoole/sq/libsq/core/record"
	"github.com/neilotoole/sq/libsq/core/schema"
	"github.com/neilotoole/sq/libsq/core/sqlz"
	"github.com/neilotoole/sq/libsq/core/tablefq"
	"github.com/neilotoole/sq/libsq/driver"
	"github.com/neilotoole/sq/libsq/source"
	"github.com/neilotoole/sq/libsq/source/drivertype"
)

// anyPtr returns a *any pointing at v, to exercise the *any unwrap in
// newRecordFromScanRow.
func anyPtr(v any) *any { return &v }

// TestProvider_DriverFor verifies the Provider returns a driver for the
// rqlite type and rejects everything else.
func TestProvider_DriverFor(t *testing.T) {
	p := &Provider{}

	drvr, err := p.DriverFor(drivertype.Rqlite)
	require.NoError(t, err)
	require.NotNil(t, drvr)
	_, ok := drvr.(*driveri)
	require.True(t, ok)

	_, err = p.DriverFor(drivertype.Type("not-a-real-driver"))
	require.Error(t, err)
}

// TestDriverMetadata verifies the static driver metadata.
func TestDriverMetadata(t *testing.T) {
	d := &driveri{}
	md := d.DriverMetadata()
	require.Equal(t, drivertype.Rqlite, md.Type)
	require.Equal(t, "rqlite", md.Description)
	require.Equal(t, "https://rqlite.io", md.Doc)
	require.True(t, md.IsSQL)
	require.Equal(t, defaultPort, md.DefaultPort)
}

// TestLocationShape verifies the location grammar declaration.
func TestLocationShape(t *testing.T) {
	d := &driveri{}
	shape := d.LocationShape()
	require.Equal(t, drivertype.Rqlite, shape.Type)
	require.Equal(t, []string{"rqlite"}, shape.Schemes)
	require.Len(t, shape.Segments, 3)
	require.Equal(t, driver.SegCredentials, shape.Segments[0].Kind)
	require.True(t, shape.Segments[0].Optional)
	require.Equal(t, driver.SegAuthority, shape.Segments[1].Kind)
	require.Equal(t, driver.SegConnParams, shape.Segments[2].Kind)
}

// TestDialect verifies the SQLite-flavored dialect.
func TestDialect(t *testing.T) {
	d := &driveri{}
	dl := d.Dialect()
	require.Equal(t, drivertype.Rqlite, dl.Type)
	require.Equal(t, 500, dl.MaxBatchValues)
	require.False(t, dl.Catalog)
	require.NotNil(t, dl.Enquote)
	require.Equal(t, `"a""b"`, dl.Enquote(`a"b`))
}

// TestErrWrapFunc verifies ErrWrapFunc returns the package errw wrapper.
func TestErrWrapFunc(t *testing.T) {
	d := &driveri{}
	fn := d.ErrWrapFunc()
	require.NotNil(t, fn)
	require.Nil(t, fn(nil))

	notExist := fn(errors.New("no such table: actor"))
	require.Error(t, notExist)
	require.True(t, errz.Has[*driver.NotExistError](notExist))

	require.Error(t, fn(errors.New("boom")))
}

// TestRenderer verifies the SLQ function overrides are registered.
func TestRenderer(t *testing.T) {
	d := &driveri{}
	r := d.Renderer()
	require.NotNil(t, r)

	for _, fn := range []string{
		ast.FuncNameSchema, ast.FuncNameCatalog,
		ast.FuncNameContains, ast.FuncNameStartsWith, ast.FuncNameEndsWith,
		ast.FuncNameIContains, ast.FuncNameIStartsWith, ast.FuncNameIEndsWith,
		ast.FuncNameLike, ast.FuncNameILike,
	} {
		require.NotNil(t, r.FunctionOverrides[fn], "override missing for %s", fn)
	}

	require.Equal(t, kind.Decimal, r.FunctionResultKinds[ast.FuncNameSum])
}

// TestSchemaCatalogUnsupported verifies the schema/catalog stubs that do
// not need a database connection.
func TestSchemaCatalogUnsupported(t *testing.T) {
	d := &driveri{}
	ctx := context.Background()

	require.Error(t, d.CreateSchema(ctx, nil, "x"))
	require.Error(t, d.DropSchema(ctx, nil, "x"))

	// Catalogs are unsupported; the trio all report it as an error so a
	// source configured with a catalog gets an accurate diagnostic.
	exists, err := d.CatalogExists(ctx, nil, "x")
	require.Error(t, err)
	require.False(t, exists)

	_, err = d.CurrentCatalog(ctx, nil)
	require.Error(t, err)

	_, err = d.ListCatalogs(ctx, nil)
	require.Error(t, err)
}

// TestDBTypeForKind_Panic verifies the unknown-kind panic path.
func TestDBTypeForKind_Panic(t *testing.T) {
	require.Panics(t, func() {
		_ = DBTypeForKind(kind.Kind(999))
	})
}

// TestKindFromDBTypeName_EmptyAndAffinity covers the empty-dbtype and
// affinity-fallback branches not exercised by the existing test.
func TestKindFromDBTypeName_EmptyAndAffinity(t *testing.T) {
	ctx := context.Background()

	// Empty dbTypeName, nil scanType -> Bytes (SQLite "no type" affinity).
	require.Equal(t, kind.Bytes, kindFromDBTypeName(ctx, "c", "", nil))

	// Empty dbTypeName, scanType breaks the tie.
	require.Equal(t, kind.Int, kindFromDBTypeName(ctx, "c", "", sqlz.RTypeInt64))
	require.Equal(t, kind.Float, kindFromDBTypeName(ctx, "c", "", sqlz.RTypeFloat64))
	require.Equal(t, kind.Text, kindFromDBTypeName(ctx, "c", "", sqlz.RTypeString))
	require.Equal(t, kind.Bytes, kindFromDBTypeName(ctx, "c", "", sqlz.RTypeBytes))
	require.Equal(t, kind.Unknown, kindFromDBTypeName(ctx, "c", "", sqlz.RTypeBool))

	// Affinity fallback via substring on a non-direct-match name.
	require.Equal(t, kind.Float, kindFromDBTypeName(ctx, "c", "someREAL", nil))
	require.Equal(t, kind.Float, kindFromDBTypeName(ctx, "c", "myFLOAty", nil))
	require.Equal(t, kind.Float, kindFromDBTypeName(ctx, "c", "DOUBlish", nil))

	// Wholly unrecognized -> Unknown (warn path).
	require.Equal(t, kind.Unknown, kindFromDBTypeName(ctx, "c", "GIBBERISH", nil))
}

// TestEpochToTime verifies the seconds-vs-milliseconds heuristic.
func TestEpochToTime(t *testing.T) {
	// Below threshold: treated as seconds.
	require.Equal(t, time.Unix(1000, 0).UTC(), epochToTime(1000))
	// Above threshold: treated as milliseconds.
	got := epochToTime(2_000_000_000_000)
	require.Equal(t, time.Unix(0, 2_000_000_000_000*int64(time.Millisecond)).UTC(), got)
	// Negative above threshold magnitude: also milliseconds.
	got = epochToTime(-2_000_000_000_000)
	require.Equal(t, time.Unix(0, -2_000_000_000_000*int64(time.Millisecond)).UTC(), got)
}

// TestNullTime_Scan covers every branch of nullTime.Scan and scanText.
func TestNullTime_Scan(t *testing.T) {
	t.Run("nil", func(t *testing.T) {
		var n nullTime
		require.NoError(t, n.Scan(nil))
		require.False(t, n.Valid)
	})

	t.Run("time.Time", func(t *testing.T) {
		now := time.Now().UTC()
		var n nullTime
		require.NoError(t, n.Scan(now))
		require.True(t, n.Valid)
		require.True(t, n.IsTime)
		require.Equal(t, now, n.Time)
	})

	t.Run("string parseable with Z", func(t *testing.T) {
		var n nullTime
		require.NoError(t, n.Scan("2024-01-02T03:04:05Z"))
		require.True(t, n.Valid)
		require.True(t, n.IsTime)
		require.Equal(t, 2024, n.Time.Year())
	})

	t.Run("string date only", func(t *testing.T) {
		var n nullTime
		require.NoError(t, n.Scan("2024-01-02"))
		require.True(t, n.Valid)
		require.True(t, n.IsTime)
	})

	t.Run("string unparseable preserved", func(t *testing.T) {
		var n nullTime
		require.NoError(t, n.Scan("not a date"))
		require.True(t, n.Valid)
		require.False(t, n.IsTime)
		require.Equal(t, "not a date", n.String)
	})

	t.Run("bytes", func(t *testing.T) {
		var n nullTime
		require.NoError(t, n.Scan([]byte("2024-01-02 03:04:05")))
		require.True(t, n.Valid)
		require.True(t, n.IsTime)
	})

	t.Run("int64 epoch", func(t *testing.T) {
		var n nullTime
		require.NoError(t, n.Scan(int64(1000)))
		require.True(t, n.Valid)
		require.True(t, n.IsTime)
		require.Equal(t, time.Unix(1000, 0).UTC(), n.Time)
	})

	t.Run("float64 preserved as text", func(t *testing.T) {
		var n nullTime
		require.NoError(t, n.Scan(2451545.0))
		require.True(t, n.Valid)
		require.False(t, n.IsTime)
		require.Equal(t, "2451545", n.String)
	})

	t.Run("unsupported type errors", func(t *testing.T) {
		var n nullTime
		require.Error(t, n.Scan(struct{}{}))
	})
}

// TestNewRecordFromScanRow exercises the type-coercion branches of
// newRecordFromScanRow.
func TestNewRecordFromScanRow(t *testing.T) {
	mkMeta := func(k kind.Kind) record.Meta {
		return record.Meta{record.NewFieldMeta(&record.ColumnTypeData{Name: "c", Kind: k}, "c")}
	}
	now := time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC)
	i64 := int64(42)
	f64 := 3.5
	b := true
	s := "hello"
	bytesVal := []byte("xyz")

	testCases := []struct {
		name string
		knd  kind.Kind
		in   any
		want any
	}{
		{name: "nil", knd: kind.Int, in: nil, want: nil},
		{name: "any_wrapping_int", knd: kind.Int, in: anyPtr(int64(7)), want: int64(7)},
		{name: "ptr_int64", knd: kind.Int, in: &i64, want: int64(42)},
		{name: "int64", knd: kind.Int, in: int64(42), want: int64(42)},
		{name: "ptr_float64_to_int", knd: kind.Int, in: &f64, want: int64(3)},
		{name: "float64_float", knd: kind.Float, in: 3.5, want: 3.5},
		{name: "ptr_bool", knd: kind.Bool, in: &b, want: true},
		{name: "bool", knd: kind.Bool, in: true, want: true},
		{name: "ptr_string", knd: kind.Text, in: &s, want: "hello"},
		{name: "string", knd: kind.Text, in: "hi", want: "hi"},
		{name: "nullint64_valid", knd: kind.Int, in: &sql.NullInt64{Int64: 9, Valid: true}, want: int64(9)},
		{name: "nullint64_invalid", knd: kind.Int, in: &sql.NullInt64{}, want: nil},
		{name: "nullstring_valid", knd: kind.Text, in: &sql.NullString{String: "v", Valid: true}, want: "v"},
		{name: "nullstring_invalid", knd: kind.Text, in: &sql.NullString{}, want: nil},
		{name: "nullfloat_valid", knd: kind.Float, in: &sql.NullFloat64{Float64: 1.5, Valid: true}, want: 1.5},
		{name: "nullfloat_invalid", knd: kind.Float, in: &sql.NullFloat64{}, want: nil},
		{name: "nullbool_valid", knd: kind.Bool, in: &sql.NullBool{Bool: true, Valid: true}, want: true},
		{name: "nullbool_invalid", knd: kind.Bool, in: &sql.NullBool{}, want: nil},
		{name: "nulltime_valid", knd: kind.Datetime, in: &sql.NullTime{Time: now, Valid: true}, want: now},
		{name: "nulltime_invalid", knd: kind.Datetime, in: &sql.NullTime{}, want: nil},
		{name: "time_value", knd: kind.Datetime, in: now, want: now},
		{name: "time_ptr", knd: kind.Datetime, in: &now, want: now},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			rec := newRecordFromScanRow(mkMeta(tc.knd), []any{tc.in})
			require.Equal(t, tc.want, rec[0])
		})
	}

	t.Run("ptr_bytes_as_bytes", func(t *testing.T) {
		rec := newRecordFromScanRow(mkMeta(kind.Bytes), []any{&bytesVal})
		require.Equal(t, []byte("xyz"), rec[0])
	})
	t.Run("ptr_bytes_empty", func(t *testing.T) {
		empty := []byte{}
		rec := newRecordFromScanRow(mkMeta(kind.Bytes), []any{&empty})
		require.Equal(t, []byte{}, rec[0])
	})
	t.Run("ptr_bytes_nil", func(t *testing.T) {
		var nilB []byte
		rec := newRecordFromScanRow(mkMeta(kind.Bytes), []any{&nilB})
		require.Nil(t, rec[0])
	})
	t.Run("ptr_bytes_as_text", func(t *testing.T) {
		rec := newRecordFromScanRow(mkMeta(kind.Text), []any{&bytesVal})
		require.Equal(t, "xyz", rec[0])
	})
	t.Run("rawbytes_as_bytes", func(t *testing.T) {
		rb := sql.RawBytes("xyz")
		rec := newRecordFromScanRow(mkMeta(kind.Bytes), []any{&rb})
		require.Equal(t, []byte("xyz"), rec[0])
	})
	t.Run("rawbytes_as_text", func(t *testing.T) {
		rb := sql.RawBytes("xyz")
		rec := newRecordFromScanRow(mkMeta(kind.Text), []any{&rb})
		require.Equal(t, "xyz", rec[0])
	})
	t.Run("rawbytes_empty_bytes", func(t *testing.T) {
		rb := sql.RawBytes{}
		rec := newRecordFromScanRow(mkMeta(kind.Bytes), []any{&rb})
		require.Equal(t, []byte{}, rec[0])
	})
	t.Run("rawbytes_empty_text", func(t *testing.T) {
		rb := sql.RawBytes{}
		rec := newRecordFromScanRow(mkMeta(kind.Text), []any{&rb})
		require.Equal(t, "", rec[0])
	})
	t.Run("decimal_nulldecimal_valid", func(t *testing.T) {
		d := decimal.RequireFromString("1.50")
		rec := newRecordFromScanRow(mkMeta(kind.Decimal), []any{&decimal.NullDecimal{Decimal: d, Valid: true}})
		require.True(t, d.Equal(rec[0].(decimal.Decimal)))
	})
	t.Run("decimal_nulldecimal_invalid", func(t *testing.T) {
		rec := newRecordFromScanRow(mkMeta(kind.Decimal), []any{&decimal.NullDecimal{}})
		require.Nil(t, rec[0])
	})
	t.Run("decimal_ptr", func(t *testing.T) {
		d := decimal.RequireFromString("2.25")
		rec := newRecordFromScanRow(mkMeta(kind.Decimal), []any{&d})
		require.True(t, d.Equal(rec[0].(decimal.Decimal)))
	})
	t.Run("decimal_value", func(t *testing.T) {
		d := decimal.RequireFromString("2.25")
		rec := newRecordFromScanRow(mkMeta(kind.Decimal), []any{d})
		require.True(t, d.Equal(rec[0].(decimal.Decimal)))
	})
	t.Run("nulltime_custom_valid_time", func(t *testing.T) {
		rec := newRecordFromScanRow(mkMeta(kind.Datetime), []any{&nullTime{Time: now, Valid: true, IsTime: true}})
		require.Equal(t, now, rec[0])
	})
	t.Run("nulltime_custom_valid_string", func(t *testing.T) {
		rec := newRecordFromScanRow(mkMeta(kind.Datetime), []any{&nullTime{String: "raw", Valid: true}})
		require.Equal(t, "raw", rec[0])
	})
	t.Run("nulltime_custom_invalid", func(t *testing.T) {
		rec := newRecordFromScanRow(mkMeta(kind.Datetime), []any{&nullTime{}})
		require.Nil(t, rec[0])
	})
	t.Run("default_passthrough", func(t *testing.T) {
		rec := newRecordFromScanRow(mkMeta(kind.Text), []any{int32(5)})
		require.Equal(t, int32(5), rec[0])
	})
}

// TestSetScanType verifies the kind-driven scan-type selection. The
// final scan type is always chosen from colType.Kind; a non-nil
// driver-supplied scan type exercises the normalization switch but does
// not override the kind-derived result.
func TestSetScanType(t *testing.T) {
	ctx := context.Background()
	testCases := []struct {
		name string
		in   reflect.Type
		knd  kind.Kind
		want reflect.Type
	}{
		{"int", sqlz.RTypeInt64, kind.Int, sqlz.RTypeNullInt64},
		{"float", sqlz.RTypeFloat64, kind.Float, sqlz.RTypeNullFloat64},
		{"string", sqlz.RTypeString, kind.Text, sqlz.RTypeNullString},
		{"bool", sqlz.RTypeBool, kind.Bool, sqlz.RTypeNullBool},
		{"time_scantype_datetime_kind", sqlz.RTypeTime, kind.Datetime, rtypeNullTime},
		{"bytes", sqlz.RTypeBytes, kind.Bytes, sqlz.RTypeBytes},
		{"first_switch_default", sqlz.RTypeAny, kind.Int, sqlz.RTypeNullInt64},
		{"nil_scantype_decimal", nil, kind.Decimal, sqlz.RTypeNullDecimal},
		{"nil_scantype_date", nil, kind.Date, rtypeNullTime},
		{"nil_scantype_time_kind", nil, kind.Time, sqlz.RTypeNullString},
		{"unknown_kind_default", nil, kind.Unknown, sqlz.RTypeAny},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ct := &record.ColumnTypeData{Name: "c", Kind: tc.knd, ScanType: tc.in}
			setScanType(ctx, ct)
			require.Equal(t, tc.want, ct.ScanType)
		})
	}
}

// TestTypesOf_Nil verifies typesOf tolerates a nil *stdlib.Rows.
func TestTypesOf_Nil(t *testing.T) {
	require.Nil(t, typesOf(nil))
}

// TestHumanMsg covers the prefix and the nil/empty-handle fallbacks.
func TestHumanMsg(t *testing.T) {
	require.Equal(t, "msg", humanMsg(nil, "msg"))
	require.Equal(t, "msg", humanMsg(&source.Source{}, "msg"))
	require.Equal(t, "@rq: msg", humanMsg(&source.Source{Handle: "@rq"}, "msg"))
}

// TestDetectConnParams_ExportedWrapper exercises the exported
// DetectConnParams (the lowercase detectConnParams core is covered
// separately with an injected transport). A plain-HTTP rqlite mock
// makes the default-transport probe succeed, so detection finds nothing
// to add.
func TestDetectConnParams_ExportedWrapper(t *testing.T) {
	var host string
	server := httptest.NewServer(newRqliteMockHandler(&host))
	t.Cleanup(server.Close)
	host = server.Listener.Addr().String()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)

	d := &driveri{}
	params, err := d.DetectConnParams(ctx, detectTestSrc("rqlite://"+host))
	require.NoError(t, err)
	require.Nil(t, params)
}

// TestQueryMethods_ErrorPaths covers the error-return branch of the
// DB-backed driver methods by pointing them at a connection that always
// fails to connect (failConnector). Each method's first query/exec
// therefore errors, exercising its errw-wrapped error path.
func TestQueryMethods_ErrorPaths(t *testing.T) {
	db := sql.OpenDB(&failConnector{err: errz.New("boom")})
	t.Cleanup(func() { _ = db.Close() })
	d := &driveri{}
	ctx := context.Background()

	_, err := d.DBProperties(ctx, db)
	require.Error(t, err)
	_, err = d.CurrentSchema(ctx, db)
	require.Error(t, err)
	_, err = d.SchemaExists(ctx, db, "main")
	require.Error(t, err)
	_, err = d.ListSchemas(ctx, db)
	require.Error(t, err)
	_, err = d.ListSchemaMetadata(ctx, db)
	require.Error(t, err)
	_, err = d.ListTableNames(ctx, db, "", true, true)
	require.Error(t, err)
	_, err = d.TableExists(ctx, db, "actor")
	require.Error(t, err)
	_, err = d.TableColumnTypes(ctx, db, "actor", []string{"id"})
	require.Error(t, err)

	tblDef := &schema.Table{
		Name: "t",
		Cols: []*schema.Column{{Name: "c", Kind: kind.Int}},
	}
	require.Error(t, d.CreateTable(ctx, db, tblDef))
	require.Error(t, d.DropTable(ctx, db, tablefq.T{Table: "t"}, true))
	require.Error(t, d.AlterTableRename(ctx, db, "t", "t2"))
	require.Error(t, d.AlterTableAddColumn(ctx, db, "t", "c", kind.Int))
	require.Error(t, d.AlterTableRenameColumn(ctx, db, "t", "c", "c2"))
	_, err = d.CopyTable(ctx, db, tablefq.T{Table: "a"}, tablefq.T{Table: "b"}, true)
	require.Error(t, err)
	require.Error(t, d.AlterTableColumnKinds(ctx, db, "t", []string{"c"}, []kind.Kind{kind.Int}))

	_, err = d.PrepareInsertStmt(ctx, db, "t", []string{"c"}, 1)
	require.Error(t, err)
	_, err = d.PrepareUpdateStmt(ctx, db, "t", []string{"c"}, "")
	require.Error(t, err)
}

// TestTruncatePing_DoOpenError covers the doOpen-failure branch of
// Truncate and Ping via a host-less location that fails before any
// network round-trip.
func TestTruncatePing_DoOpenError(t *testing.T) {
	d := &driveri{}
	ctx := context.Background()
	src := &source.Source{
		Handle:   "@rq",
		Type:     drivertype.Rqlite,
		Location: "rqlite://",
	}

	_, err := d.Truncate(ctx, src, "t", true)
	require.Error(t, err)
	require.Error(t, d.Ping(ctx, src, driver.ModeReadWrite))
}

// TestStripURLError verifies a *url.Error wrapper (which embeds the raw
// URL, including any inline password) is unwrapped to its cause, while
// other errors pass through unchanged.
func TestStripURLError(t *testing.T) {
	inner := errors.New("missing host")
	uerr := &url.Error{Op: "parse", URL: "http://user:s3cret@host", Err: inner}
	got := stripURLError(uerr)
	require.Equal(t, inner, got)
	require.NotContains(t, got.Error(), "s3cret")

	plain := errors.New("boom")
	require.Equal(t, plain, stripURLError(plain))
}

// TestInsecureConnector_ConnectRedactsCreds verifies the insecure path's
// connect error does not echo the DSN password when the underlying URL
// fails to parse (the parse fails before any network round-trip).
func TestInsecureConnector_ConnectRedactsCreds(t *testing.T) {
	c := &insecureConnector{
		dsn:    "http://user:s3cret@\x7fbad-host:4001",
		client: newInsecureHTTPClient(time.Second),
	}
	_, err := c.Connect(context.Background())
	require.Error(t, err)
	require.NotContains(t, err.Error(), "s3cret")
}
