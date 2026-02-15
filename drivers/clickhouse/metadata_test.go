package clickhouse_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/drivers/clickhouse"
	"github.com/neilotoole/sq/libsq/core/kind"
	"github.com/neilotoole/sq/libsq/core/sqlz"
	"github.com/neilotoole/sq/libsq/source/drivertype"
	"github.com/neilotoole/sq/testh"
	"github.com/neilotoole/sq/testh/sakila"
	"github.com/neilotoole/sq/testh/tu"
)

var _ clickhouse.Provider // Ensure package is imported

// TestKindFromClickHouseType tests type mapping from ClickHouse to sq kinds.
func TestKindFromClickHouseType(t *testing.T) {
	testCases := []struct {
		chType   string
		wantKind kind.Kind
	}{
		// Integer types
		{"UInt8", kind.Int},
		{"UInt16", kind.Int},
		{"UInt32", kind.Int},
		{"UInt64", kind.Int},
		{"Int8", kind.Int},
		{"Int16", kind.Int},
		{"Int32", kind.Int},
		{"Int64", kind.Int},

		// Float types
		{"Float32", kind.Float},
		{"Float64", kind.Float},

		// String types
		{"String", kind.Text},
		{"FixedString", kind.Text},      // Base type (unlikely in practice)
		{"FixedString(10)", kind.Text},  // ClickHouse returns this format
		{"FixedString(255)", kind.Text}, // Various lengths
		{"FixedString(1)", kind.Text},   // Minimum length

		// Boolean
		{"Bool", kind.Bool},

		// Date/Time types
		{"Date", kind.Date},
		{"Date32", kind.Date},
		{"DateTime", kind.Datetime},
		{"DateTime64", kind.Datetime},

		// UUID
		{"UUID", kind.Text},

		// Decimal
		{"Decimal(18,4)", kind.Decimal},
		{"Decimal(10,2)", kind.Decimal},

		// Array (mapped to text)
		{"Array(String)", kind.Text},
		{"Array(Int32)", kind.Text},

		// Nullable wrapper
		{"Nullable(String)", kind.Text},
		{"Nullable(Int64)", kind.Int},
		{"Nullable(Float64)", kind.Float},
		{"Nullable(DateTime)", kind.Datetime},
		{"Nullable(FixedString(10))", kind.Text}, // FixedString with Nullable

		// LowCardinality wrapper
		{"LowCardinality(String)", kind.Text},
		{"LowCardinality(Nullable(String))", kind.Text},
		{"LowCardinality(FixedString(10))", kind.Text}, // FixedString with LowCardinality

		// Unknown types default to text
		{"SomeUnknownType", kind.Text},
	}

	for _, tc := range testCases {
		t.Run(tc.chType, func(t *testing.T) {
			gotKind := clickhouse.KindFromClickHouseType(tc.chType)
			require.Equal(t, tc.wantKind, gotKind, "Type %s", tc.chType)
		})
	}
}

// TestIsNullableType tests nullable type detection.
func TestIsNullableType(t *testing.T) {
	testCases := []struct {
		typeName string
		want     bool
	}{
		{"String", false},
		{"Int64", false},
		{"Nullable(String)", true},
		{"Nullable(Int64)", true},
		{"Nullable(DateTime)", true},
		{"LowCardinality(String)", false},
		{"LowCardinality(Nullable(String))", false}, // Outer wrapper is not Nullable
		{"", false},
		{"Nullabl", false}, // Too short
	}

	for _, tc := range testCases {
		t.Run(tc.typeName, func(t *testing.T) {
			got := clickhouse.IsNullableType(tc.typeName)
			require.Equal(t, tc.want, got)
		})
	}
}

// TestIsNullableTypeUnwrapped tests nullable detection after stripping wrappers.
// This is critical for LowCardinality(Nullable(T)) types where the outer wrapper
// is not Nullable but the inner type is.
func TestIsNullableTypeUnwrapped(t *testing.T) {
	testCases := []struct {
		typeName string
		want     bool
	}{
		// Non-nullable types
		{"String", false},
		{"Int64", false},
		{"LowCardinality(String)", false},
		{"LowCardinality(Int64)", false},
		{"", false},

		// Nullable types (direct)
		{"Nullable(String)", true},
		{"Nullable(Int64)", true},
		{"Nullable(DateTime)", true},

		// Nullable types wrapped in LowCardinality - the key case this function handles
		{"LowCardinality(Nullable(String))", true},
		{"LowCardinality(Nullable(Int64))", true},
		{"LowCardinality(Nullable(Float64))", true},
		{"LowCardinality(Nullable(DateTime))", true},
	}

	for _, tc := range testCases {
		t.Run(tc.typeName, func(t *testing.T) {
			got := clickhouse.IsNullableTypeUnwrapped(tc.typeName)
			require.Equal(t, tc.want, got,
				"isNullableTypeUnwrapped(%q) = %v, want %v", tc.typeName, got, tc.want)
		})
	}
}

// TestLowCardinalityWrapperStripping specifically tests that LowCardinality
// wrappers are correctly stripped to extract the inner type.
// This test catches the off-by-one bug where chType[:14] was compared against
// "LowCardinality(" (15 chars), which would never match.
func TestLowCardinalityWrapperStripping(t *testing.T) {
	testCases := []struct {
		chType   string
		wantKind kind.Kind
		desc     string
	}{
		// These would return kind.Text with or without the fix (String default)
		{"LowCardinality(String)", kind.Text, "LowCardinality wrapping String"},

		// These REQUIRE correct wrapper stripping to return the right kind
		{"LowCardinality(Int64)", kind.Int, "LowCardinality wrapping Int64"},
		{"LowCardinality(Int32)", kind.Int, "LowCardinality wrapping Int32"},
		{"LowCardinality(UInt64)", kind.Int, "LowCardinality wrapping UInt64"},
		{"LowCardinality(Float64)", kind.Float, "LowCardinality wrapping Float64"},
		{"LowCardinality(Float32)", kind.Float, "LowCardinality wrapping Float32"},
		{"LowCardinality(DateTime)", kind.Datetime, "LowCardinality wrapping DateTime"},
		{"LowCardinality(Date)", kind.Date, "LowCardinality wrapping Date"},

		// Nested wrappers: LowCardinality(Nullable(...))
		{"LowCardinality(Nullable(Int64))", kind.Int, "LowCardinality+Nullable wrapping Int64"},
		{"LowCardinality(Nullable(Float64))", kind.Float, "LowCardinality+Nullable wrapping Float64"},
		{"LowCardinality(Nullable(DateTime))", kind.Datetime, "LowCardinality+Nullable wrapping DateTime"},

		// Edge cases
		{"LowCardinality(UUID)", kind.Text, "LowCardinality wrapping UUID"},
		{"LowCardinality(Bool)", kind.Bool, "LowCardinality wrapping Bool"},
	}

	for _, tc := range testCases {
		t.Run(tc.chType, func(t *testing.T) {
			gotKind := clickhouse.KindFromClickHouseType(tc.chType)
			require.Equal(t, tc.wantKind, gotKind,
				"%s: expected %s but got %s", tc.desc, tc.wantKind, gotKind)
		})
	}
}

// TestNullableWrapperStripping tests that Nullable wrappers are correctly stripped.
func TestNullableWrapperStripping(t *testing.T) {
	testCases := []struct {
		chType   string
		wantKind kind.Kind
	}{
		{"Nullable(Int64)", kind.Int},
		{"Nullable(Int32)", kind.Int},
		{"Nullable(UInt64)", kind.Int},
		{"Nullable(Float64)", kind.Float},
		{"Nullable(Float32)", kind.Float},
		{"Nullable(String)", kind.Text},
		{"Nullable(DateTime)", kind.Datetime},
		{"Nullable(Date)", kind.Date},
		{"Nullable(Bool)", kind.Bool},
		{"Nullable(UUID)", kind.Text},
	}

	for _, tc := range testCases {
		t.Run(tc.chType, func(t *testing.T) {
			gotKind := clickhouse.KindFromClickHouseType(tc.chType)
			require.Equal(t, tc.wantKind, gotKind)
		})
	}
}

// TestTableTypeFromEngine tests that ClickHouse engine names are correctly
// mapped to canonical table types (table vs view).
func TestTableTypeFromEngine(t *testing.T) {
	testCases := []struct {
		engine   string
		wantType string
	}{
		// View engines
		{"View", sqlz.TableTypeView},
		{"MaterializedView", sqlz.TableTypeView},

		// Table engines (MergeTree family)
		{"MergeTree", sqlz.TableTypeTable},
		{"ReplacingMergeTree", sqlz.TableTypeTable},
		{"SummingMergeTree", sqlz.TableTypeTable},
		{"AggregatingMergeTree", sqlz.TableTypeTable},
		{"CollapsingMergeTree", sqlz.TableTypeTable},
		{"VersionedCollapsingMergeTree", sqlz.TableTypeTable},

		// Other table engines
		{"Log", sqlz.TableTypeTable},
		{"TinyLog", sqlz.TableTypeTable},
		{"Memory", sqlz.TableTypeTable},
		{"Buffer", sqlz.TableTypeTable},
		{"Distributed", sqlz.TableTypeTable},
		{"Dictionary", sqlz.TableTypeTable},

		// Unknown/future engines should default to table
		{"SomeNewEngine", sqlz.TableTypeTable},
		{"", sqlz.TableTypeTable},
	}

	for _, tc := range testCases {
		t.Run(tc.engine, func(t *testing.T) {
			got := clickhouse.TableTypeFromEngine(tc.engine)
			require.Equal(t, tc.wantType, got,
				"tableTypeFromEngine(%q) = %q, want %q", tc.engine, got, tc.wantType)
		})
	}
}

// TestMetadata_SourceMetadata tests source metadata retrieval.
func TestMetadata_SourceMetadata(t *testing.T) {
	tu.SkipShort(t, true)

	th := testh.New(t)
	src := th.Source(sakila.CH)

	md, err := th.SourceMetadata(src)
	require.NoError(t, err)
	require.NotNil(t, md)

	require.Equal(t, sakila.CH, md.Handle)
	require.NotEmpty(t, md.Location)
	require.Equal(t, "sakila", md.Name)
	require.Equal(t, md.Name, md.FQName)
	require.Equal(t, md.Name, md.Schema)

	require.Equal(t, drivertype.ClickHouse, md.Driver)
	require.Equal(t, drivertype.ClickHouse, md.DBDriver)

	// sq only supports ClickHouse v25+.
	require.True(t, strings.HasPrefix(md.DBVersion, "25."),
		"expected DBVersion to start with '25.', got: %s", md.DBVersion)
	require.True(t, strings.HasPrefix(md.DBProduct, "ClickHouse 25."),
		"expected DBProduct to start with 'ClickHouse 25.', got: %s", md.DBProduct)
	require.Equal(t, "ClickHouse "+md.DBVersion, md.DBProduct)

	require.Equal(t, "sakila", md.User)
	require.Greater(t, md.Size, int64(0))

	require.NotEmpty(t, md.Tables)
	require.Greater(t, md.TableCount, int64(0))
	require.Greater(t, md.ViewCount, int64(0))
	require.Equal(t, int64(len(md.Tables)), md.TableCount+md.ViewCount)
}

// TestMetadata_TableMetadata tests table metadata retrieval.
func TestMetadata_TableMetadata(t *testing.T) {
	tu.SkipShort(t, true)

	th := testh.New(t)
	src := th.Source(sakila.CH)

	// Create a test table so metadata has something to retrieve
	th.ExecSQL(src, "CREATE TABLE IF NOT EXISTS test_table_meta (id Int64, name String) ENGINE = MergeTree() ORDER BY id")
	t.Cleanup(func() { th.ExecSQL(src, "DROP TABLE IF EXISTS test_table_meta") })

	md, err := th.SourceMetadata(src)
	require.NoError(t, err)
	require.NotNil(t, md)
	require.NotEmpty(t, md.Tables, "Expected at least one table")

	// Check first table metadata
	tbl := md.Tables[0]
	require.NotEmpty(t, tbl.Name)
	require.NotEmpty(t, tbl.Columns)

	t.Logf("Table: %s", tbl.Name)
	t.Logf("Columns: %d", len(tbl.Columns))
	for _, col := range tbl.Columns {
		t.Logf("  - %s: %s (kind: %s, nullable: %v)",
			col.Name, col.ColumnType, col.Kind, col.Nullable)
	}
}

// TestMetadata_Catalogs tests catalog/database listing.
func TestMetadata_Catalogs(t *testing.T) {
	tu.SkipShort(t, true)

	th, src, drvr, _, db := testh.NewWith(t, sakila.CH)

	catalogs, err := drvr.ListCatalogs(th.Context, db)
	require.NoError(t, err)
	require.NotEmpty(t, catalogs)
	t.Logf("Catalogs: %v", catalogs)

	// Test catalog exists
	currentCatalog, err := drvr.CurrentCatalog(th.Context, db)
	require.NoError(t, err)
	require.NotEmpty(t, currentCatalog)
	t.Logf("Current catalog: %s", currentCatalog)

	exists, err := drvr.CatalogExists(th.Context, db, currentCatalog)
	require.NoError(t, err)
	require.True(t, exists)

	// Test non-existent catalog
	exists, err = drvr.CatalogExists(th.Context, db, "nonexistent_db_12345")
	require.NoError(t, err)
	require.False(t, exists)

	_ = src
}

// TestMetadata_Schemas tests schema listing.
func TestMetadata_Schemas(t *testing.T) {
	tu.SkipShort(t, true)

	th, src, drvr, _, db := testh.NewWith(t, sakila.CH)

	schemas, err := drvr.ListSchemas(th.Context, db)
	require.NoError(t, err)
	require.NotEmpty(t, schemas)
	t.Logf("Schemas: %v", schemas)

	// Test schema exists
	currentSchema, err := drvr.CurrentSchema(th.Context, db)
	require.NoError(t, err)
	require.NotEmpty(t, currentSchema)
	t.Logf("Current schema: %s", currentSchema)

	exists, err := drvr.SchemaExists(th.Context, db, currentSchema)
	require.NoError(t, err)
	require.True(t, exists)

	_ = src
}

// TestMetadata_TableNames tests table name listing.
func TestMetadata_TableNames(t *testing.T) {
	tu.SkipShort(t, true)

	th, src, drvr, _, db := testh.NewWith(t, sakila.CH)

	// List all tables
	tables, err := drvr.ListTableNames(th.Context, db, "", true, false)
	require.NoError(t, err)
	t.Logf("Tables: %v", tables)

	// List all views
	views, err := drvr.ListTableNames(th.Context, db, "", false, true)
	require.NoError(t, err)
	t.Logf("Views: %v", views)

	// List both tables and views
	all, err := drvr.ListTableNames(th.Context, db, "", true, true)
	require.NoError(t, err)
	t.Logf("All objects: %v", all)
	require.GreaterOrEqual(t, len(all), len(tables))
	require.GreaterOrEqual(t, len(all), len(views))

	_ = src
}
