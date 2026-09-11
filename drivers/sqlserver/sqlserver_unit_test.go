package sqlserver_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/drivers/sqlserver"
	"github.com/neilotoole/sq/libsq/ast"
	"github.com/neilotoole/sq/libsq/core/kind"
	"github.com/neilotoole/sq/libsq/core/lg/lgt"
	"github.com/neilotoole/sq/libsq/driver"
	"github.com/neilotoole/sq/libsq/source"
	"github.com/neilotoole/sq/libsq/source/drivertype"
)

// newDriver returns a SQL Server driver instance without opening a database
// connection. It's for exercising driver methods that don't touch the DB.
func newDriver(t *testing.T) driver.SQLDriver {
	t.Helper()
	p := &sqlserver.Provider{Log: lgt.New(t)}
	drvr, err := p.DriverFor(drivertype.MSSQL)
	require.NoError(t, err)
	sqlDrvr, ok := drvr.(driver.SQLDriver)
	require.True(t, ok)
	return sqlDrvr
}

func TestProvider_DriverFor(t *testing.T) {
	t.Parallel()

	p := &sqlserver.Provider{Log: lgt.New(t)}

	drvr, err := p.DriverFor(drivertype.MSSQL)
	require.NoError(t, err)
	require.NotNil(t, drvr)

	// An unsupported driver type must error.
	drvr, err = p.DriverFor(drivertype.Pg)
	require.Error(t, err)
	require.Nil(t, drvr)
}

func TestDriver_DriverMetadata(t *testing.T) {
	t.Parallel()

	md := newDriver(t).DriverMetadata()
	require.Equal(t, drivertype.MSSQL, md.Type)
	require.True(t, md.IsSQL)
	require.False(t, md.IsEmbeddedSQL)
	require.Equal(t, 1433, md.DefaultPort)
	require.NotEmpty(t, md.Description)
	require.NotEmpty(t, md.Doc)
}

func TestDriver_Dialect(t *testing.T) {
	t.Parallel()

	d := newDriver(t).Dialect()
	require.Equal(t, drivertype.MSSQL, d.Type)
	require.True(t, d.Catalog)
	require.Equal(t, 1000, d.MaxBatchValues)

	// Placeholders must use the @pN form.
	require.Equal(t, "(@p1, @p2)", d.Placeholders(2, 1))
	// Enquote must double-quote.
	require.Equal(t, `"col"`, d.Enquote("col"))
}

func TestDriver_ConnParams(t *testing.T) {
	t.Parallel()

	params := newDriver(t).ConnParams()
	require.NotEmpty(t, params)
	// Spot-check a couple of well-known SQL Server connection parameters.
	require.Contains(t, params, "database")
	require.Contains(t, params, "encrypt")
	require.Equal(t, []string{"disable", "false", "true"}, params["encrypt"])
}

func TestDriver_LocationShape(t *testing.T) {
	t.Parallel()

	shape := newDriver(t).LocationShape()
	require.Equal(t, drivertype.MSSQL, shape.Type)
	require.Contains(t, shape.Schemes, "sqlserver")
	require.NotEmpty(t, shape.Segments)
}

func TestDriver_ErrWrapFunc(t *testing.T) {
	t.Parallel()

	fn := newDriver(t).ErrWrapFunc()
	require.NotNil(t, fn)
	// A nil error must wrap to nil.
	require.NoError(t, fn(nil))
}

func TestDriver_Renderer(t *testing.T) {
	t.Parallel()

	r := newDriver(t).Renderer()
	require.NotNil(t, r)
	require.NotNil(t, r.Range)
	require.NotNil(t, r.Literal)

	// SQL Server-specific function name and override wiring.
	require.Equal(t, "SCHEMA_NAME", r.FunctionNames[ast.FuncNameSchema])
	require.Equal(t, "DB_NAME", r.FunctionNames[ast.FuncNameCatalog])
	require.Contains(t, r.FunctionOverrides, ast.FuncNameAvg)
	require.Contains(t, r.FunctionOverrides, ast.FuncNameSum)
	require.Contains(t, r.FunctionOverrides, ast.FuncNameRowNum)
}

func TestDriver_ValidateSource(t *testing.T) {
	t.Parallel()

	drvr := newDriver(t)

	src := &source.Source{
		Handle:   "@ms",
		Type:     drivertype.MSSQL,
		Location: "sqlserver://sa:pw@localhost?database=sakila",
	}
	got, err := drvr.ValidateSource(src)
	require.NoError(t, err)
	require.Same(t, src, got)

	// Wrong driver type must error.
	bad := &source.Source{Handle: "@pg", Type: drivertype.Pg}
	got, err = drvr.ValidateSource(bad)
	require.Error(t, err)
	require.Nil(t, got)
}

func TestDriver_AlterTableColumnKinds_NotImplemented(t *testing.T) {
	t.Parallel()

	// AlterTableColumnKinds is not yet implemented for SQL Server; it must
	// return an error without touching the (nil) DB.
	err := newDriver(t).AlterTableColumnKinds(
		t.Context(), nil, "tbl", []string{"col"}, []kind.Kind{kind.Int},
	)
	require.Error(t, err)
}
