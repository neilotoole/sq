package oracle

import (
	"context"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/libsq/ast"
	"github.com/neilotoole/sq/libsq/core/kind"
	"github.com/neilotoole/sq/libsq/core/record"
	"github.com/neilotoole/sq/libsq/core/sqlz"
	"github.com/neilotoole/sq/libsq/driver"
	"github.com/neilotoole/sq/libsq/source"
	"github.com/neilotoole/sq/libsq/source/drivertype"
)

// TestProvider_DriverFor verifies the provider returns the Oracle driver for
// the Oracle type and rejects any other type.
func TestProvider_DriverFor(t *testing.T) {
	t.Parallel()
	p := &Provider{Log: slog.Default()}

	drvr, err := p.DriverFor(drivertype.Oracle)
	require.NoError(t, err)
	require.NotNil(t, drvr)

	_, err = p.DriverFor(drivertype.Pg)
	require.Error(t, err, "non-Oracle type must be rejected")
}

// TestDriveri_ValidateSource covers both the happy path and the type-mismatch
// rejection.
func TestDriveri_ValidateSource(t *testing.T) {
	t.Parallel()
	d := &driveri{log: slog.Default()}

	src := &source.Source{Handle: "@ora", Type: drivertype.Oracle}
	got, err := d.ValidateSource(src)
	require.NoError(t, err)
	require.Same(t, src, got)

	_, err = d.ValidateSource(&source.Source{Handle: "@bad", Type: drivertype.Pg})
	require.Error(t, err, "wrong source type must be rejected")
}

// TestDriveri_DriverMetadata pins the static driver metadata.
func TestDriveri_DriverMetadata(t *testing.T) {
	t.Parallel()
	md := (&driveri{}).DriverMetadata()
	require.Equal(t, drivertype.Oracle, md.Type)
	require.True(t, md.IsSQL)
	require.Equal(t, 1521, md.DefaultPort)
	require.NotEmpty(t, md.Description)
}

// TestDriveri_Dialect pins the dialect configuration the renderer and batch
// machinery rely on.
func TestDriveri_Dialect(t *testing.T) {
	t.Parallel()
	dlct := (&driveri{}).Dialect()
	require.Equal(t, drivertype.Oracle, dlct.Type)
	require.False(t, dlct.Catalog, "Oracle uses schemas only")
	require.True(t, dlct.IntBool, "Oracle emulates BOOLEAN as NUMBER(1,0)")
	require.Equal(t, 1000, dlct.MaxBatchValues)
	require.NotNil(t, dlct.Placeholders)
	require.NotNil(t, dlct.Enquote)
}

// TestDriveri_ConnParams confirms the advertised go-ora connection parameters.
func TestDriveri_ConnParams(t *testing.T) {
	t.Parallel()
	params := (&driveri{}).ConnParams()
	require.Contains(t, params, "SSL")
	require.Contains(t, params, "wallet")
	require.Contains(t, params, "CONNECTION TIMEOUT")
}

// TestDriveri_LocationShape confirms the location grammar segments.
func TestDriveri_LocationShape(t *testing.T) {
	t.Parallel()
	shape := (&driveri{}).LocationShape()
	require.Equal(t, drivertype.Oracle, shape.Type)
	require.Equal(t, []string{"oracle"}, shape.Schemes)
	require.NotEmpty(t, shape.Segments)
}

// TestDriveri_ErrWrapFunc verifies the wrap func is wired and passes nil
// through unchanged.
func TestDriveri_ErrWrapFunc(t *testing.T) {
	t.Parallel()
	fn := (&driveri{}).ErrWrapFunc()
	require.NotNil(t, fn)
	require.NoError(t, fn(nil))
}

// TestEnquoteOracle verifies identifiers are double-quoted and upper-cased.
func TestEnquoteOracle(t *testing.T) {
	t.Parallel()
	require.Equal(t, `"ACTOR"`, enquoteOracle("actor"))
	require.Equal(t, `"ACTOR"`, enquoteOracle("Actor"))
	require.Equal(t, `"FIRST_NAME"`, enquoteOracle("first_name"))
}

// TestOracleSubstWherePlaceholders verifies "?" markers are rewritten to
// Oracle :N binds starting at the given offset, leaving other text intact.
func TestOracleSubstWherePlaceholders(t *testing.T) {
	t.Parallel()
	require.Equal(t, `"ID" = :3`, oracleSubstWherePlaceholders(`"ID" = ?`, 3))
	require.Equal(t,
		`"A" = :2 AND "B" = :3`,
		oracleSubstWherePlaceholders(`"A" = ? AND "B" = ?`, 2))
	require.Equal(t, `"ID" > 0`, oracleSubstWherePlaceholders(`"ID" > 0`, 1),
		"text without ? is unchanged")
}

// TestDriveri_Renderer verifies the renderer is assembled with the Oracle
// row-range, pre-render hook, and the function overrides/result-kind pins that
// keep computed NUMBER columns scanning correctly (issues #594, #839, #844).
func TestDriveri_Renderer(t *testing.T) {
	t.Parallel()
	r := (&driveri{}).Renderer()
	require.NotNil(t, r)
	require.NotNil(t, r.Range)
	require.NotEmpty(t, r.PreRender)

	require.Contains(t, r.FunctionOverrides, ast.FuncNameSchema)
	require.Contains(t, r.FunctionOverrides, ast.FuncNameCatalog)
	require.Contains(t, r.FunctionOverrides, ast.FuncNameAvg)
	require.Contains(t, r.FunctionOverrides, ast.FuncNameSum)

	require.Equal(t, kind.Int, r.FunctionResultKinds[ast.FuncNameCount])
	require.Equal(t, kind.Int, r.FunctionResultKinds[ast.FuncNameCountUnique])
	require.Equal(t, kind.Int, r.FunctionResultKinds[ast.FuncNameRowNum])
}

// TestDriveri_setScanType maps every kind to its expected scan type, including
// the Oracle-specific Bool→int64 mapping (NUMBER(1,0)).
func TestDriveri_setScanType(t *testing.T) {
	t.Parallel()
	d := &driveri{}
	testCases := []struct {
		knd  kind.Kind
		want any
	}{
		{kind.Null, sqlz.RTypeNullString},
		{kind.Text, sqlz.RTypeNullString},
		{kind.Unknown, sqlz.RTypeNullString},
		{kind.Int, sqlz.RTypeNullInt64},
		{kind.Float, sqlz.RTypeNullFloat64},
		{kind.Decimal, sqlz.RTypeNullDecimal},
		{kind.Bool, sqlz.RTypeNullInt64},
		{kind.Datetime, sqlz.RTypeNullTime},
		{kind.Date, sqlz.RTypeNullTime},
		{kind.Time, sqlz.RTypeNullTime},
		{kind.Bytes, sqlz.RTypeBytes},
	}
	for _, tc := range testCases {
		t.Run(tc.knd.String(), func(t *testing.T) {
			t.Parallel()
			ctd := &record.ColumnTypeData{}
			d.setScanType(ctd, tc.knd)
			require.Equal(t, tc.want, ctd.ScanType)
		})
	}
}

// TestDbTypeNameFromKind_Null covers the kind.Null branch not exercised by the
// internal test's main table.
func TestDbTypeNameFromKind_Null(t *testing.T) {
	t.Parallel()
	require.Equal(t, "VARCHAR2(4000)", dbTypeNameFromKind(kind.Null))
}

// TestKindFromDBTypeName_RemainingBranches covers the type names not exercised
// by TestKindFromDBTypeName, plus the unknown-type warning path (non-nil log).
func TestKindFromDBTypeName_RemainingBranches(t *testing.T) {
	t.Parallel()
	testCases := map[string]kind.Kind{
		"TIMETZ":         kind.Time,
		"ROWID":          kind.Text,
		"UROWID":         kind.Text,
		"VARCHAR":        kind.Text,
		"LONG":           kind.Text,
		"LONGVARCHAR":    kind.Text,
		"OCICLOBLOCATOR": kind.Text,
		"OCISTRING":      kind.Text,
		"OCIBLOBLOCATOR": kind.Bytes,
		"OCIFILELOCATOR": kind.Bytes,
		"VARRAW":         kind.Bytes,
		"LONGRAW":        kind.Bytes,
		"LONGVARRAW":     kind.Bytes,
		"OCIDATE":        kind.Datetime,
		"TIMESTAMPTZ":    kind.Datetime,
		"TIMESTAMPELTZ":  kind.Datetime,
		"BFLOAT":         kind.Float,
		"BDOUBLE":        kind.Float,
		"INTERVALYM":     kind.Text,
		"INTERVALDS":     kind.Text,
	}
	for dbTypeName, want := range testCases {
		t.Run(dbTypeName, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, want, kindFromDBTypeName(nil, "col", dbTypeName))
		})
	}

	// Unknown type with a non-nil logger exercises the log.Warn branch.
	require.Equal(t, kind.Unknown,
		kindFromDBTypeName(slog.Default(), "col", "SOME_UNKNOWN_TYPE"))
}

// TestDriveri_UnsupportedFeatures pins the error-returning stubs for features
// Oracle doesn't model the sq way (catalogs, CREATE/DROP SCHEMA, and the
// not-yet-implemented AlterTableColumnKinds). None of these touch the DB, so a
// nil sqlz.DB is fine.
func TestDriveri_UnsupportedFeatures(t *testing.T) {
	t.Parallel()
	d := &driveri{}
	ctx := context.Background()

	_, err := d.CurrentCatalog(ctx, nil)
	require.Error(t, err)

	_, err = d.ListCatalogs(ctx, nil)
	require.Error(t, err)

	require.Error(t, d.CreateSchema(ctx, nil, "x"))
	require.Error(t, d.DropSchema(ctx, nil, "x"))

	exists, err := d.CatalogExists(ctx, nil, "x")
	require.Error(t, err)
	require.False(t, exists)

	require.Error(t, d.AlterTableColumnKinds(ctx, nil, "t", nil, nil))
}

// Compile-time assertion mirroring the production var, kept local so the test
// file documents the contract it exercises.
var _ driver.SQLDriver = (*driveri)(nil)
