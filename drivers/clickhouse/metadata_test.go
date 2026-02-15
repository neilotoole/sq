package clickhouse_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/drivers/clickhouse"
	"github.com/neilotoole/sq/libsq/core/kind"
	"github.com/neilotoole/sq/libsq/core/sqlz"
	"github.com/neilotoole/sq/libsq/source/drivertype"
	"github.com/neilotoole/sq/libsq/source/metadata"
	"github.com/neilotoole/sq/testh"
	"github.com/neilotoole/sq/testh/sakila"
	"github.com/neilotoole/sq/testh/tu"
)

var _ clickhouse.Provider // Ensure package is imported

// TestBaseTypeFromClickHouseType tests unwrapping of ClickHouse type wrappers.
func TestBaseTypeFromClickHouseType(t *testing.T) {
	testCases := []struct {
		chType   string
		wantBase string
	}{
		// No wrappers.
		{"String", "String"},
		{"Int64", "Int64"},
		{"UInt16", "UInt16"},
		{"DateTime", "DateTime"},
		{"FixedString(10)", "FixedString(10)"},
		{"Decimal(18,4)", "Decimal(18,4)"},
		{"Array(String)", "Array(String)"},

		// Nullable wrapper.
		{"Nullable(UInt16)", "UInt16"},
		{"Nullable(String)", "String"},
		{"Nullable(Int64)", "Int64"},
		{"Nullable(DateTime)", "DateTime"},
		{"Nullable(FixedString(10))", "FixedString(10)"},

		// LowCardinality wrapper.
		{"LowCardinality(String)", "String"},
		{"LowCardinality(Int64)", "Int64"},

		// LowCardinality(Nullable(...)) double wrapper.
		{"LowCardinality(Nullable(String))", "String"},
		{"LowCardinality(Nullable(Int64))", "Int64"},
		{"LowCardinality(Nullable(Float64))", "Float64"},
	}

	for _, tc := range testCases {
		t.Run(tc.chType, func(t *testing.T) {
			got := clickhouse.BaseTypeFromClickHouseType(tc.chType)
			require.Equal(t, tc.wantBase, got, "Type %s", tc.chType)
		})
	}
}

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
		{"DateTime64(3)", kind.Datetime},       // Millisecond precision
		{"DateTime64(6)", kind.Datetime},       // Microsecond precision
		{"DateTime64(9)", kind.Datetime},       // Nanosecond precision
		{"DateTime64(3,'UTC')", kind.Datetime}, // With timezone
		{"DateTime('UTC')", kind.Datetime},     // DateTime with timezone

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

	// Verify Table.FQName and Column.BaseType are populated.
	for _, tbl := range md.Tables {
		require.Equal(t, "sakila."+tbl.Name, tbl.FQName,
			"Table %s: FQName should be database.table", tbl.Name)

		for _, col := range tbl.Columns {
			require.NotEmpty(t, col.BaseType,
				"Table %s, Column %s: BaseType should not be empty", tbl.Name, col.Name)
		}
	}
}

// TestMetadata_TableMetadata tests table metadata retrieval.
func TestMetadata_TableMetadata(t *testing.T) {
	tu.SkipShort(t, true)

	th := testh.New(t)
	src := th.Source(sakila.CH)

	// Create a test table with a default expression.
	th.ExecSQL(src, `CREATE TABLE IF NOT EXISTS test_table_meta (
		id Int64,
		name String DEFAULT 'unknown'
	) ENGINE = MergeTree() ORDER BY id`)
	t.Cleanup(func() { th.ExecSQL(src, "DROP TABLE IF EXISTS test_table_meta") })

	md, err := th.SourceMetadata(src)
	require.NoError(t, err)
	require.NotNil(t, md)
	require.NotEmpty(t, md.Tables, "Expected at least one table")

	// Find the test_table_meta table.
	var tbl *metadata.Table
	for _, t2 := range md.Tables {
		if t2.Name == "test_table_meta" {
			tbl = t2
			break
		}
	}
	require.NotNil(t, tbl, "test_table_meta not found in metadata")

	// Verify FQName.
	require.Equal(t, "sakila.test_table_meta", tbl.FQName)

	// Verify columns.
	require.Len(t, tbl.Columns, 2)

	idCol := tbl.Columns[0]
	require.Equal(t, "id", idCol.Name)
	require.Equal(t, "Int64", idCol.BaseType)
	require.Empty(t, idCol.DefaultValue)

	nameCol := tbl.Columns[1]
	require.Equal(t, "name", nameCol.Name)
	require.Equal(t, "String", nameCol.BaseType)
	require.Equal(t, "'unknown'", nameCol.DefaultValue)

	t.Logf("Table: %s (FQName: %s)", tbl.Name, tbl.FQName)
	for _, col := range tbl.Columns {
		t.Logf("  - %s: %s (base: %s, kind: %s, default: %q)",
			col.Name, col.ColumnType, col.BaseType, col.Kind, col.DefaultValue)
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

// TestMetadata_UnusualColumnTypes tests that metadata retrieval handles
// ClickHouse-specific types that don't have direct sq kind equivalents.
// Types like Enum8, Array, Map, and Tuple should be assigned a Kind (falling
// back to kind.Text) and have a non-empty BaseType.
//
// This test requires a live ClickHouse instance and is skipped in short mode.
func TestMetadata_UnusualColumnTypes(t *testing.T) {
	tu.SkipShort(t, true)

	th := testh.New(t)
	src := th.Source(sakila.CH)

	const tblName = "test_unusual_col_types"

	th.ExecSQL(src, "DROP TABLE IF EXISTS "+tblName)
	th.ExecSQL(src, `CREATE TABLE `+tblName+` (
		col_enum    Enum8('a' = 1, 'b' = 2, 'c' = 3),
		col_array   Array(Int32),
		col_map     Map(String, String),
		col_tuple   Tuple(String, Int64)
	) ENGINE = MergeTree() ORDER BY col_enum`)
	t.Cleanup(func() { th.ExecSQL(src, "DROP TABLE IF EXISTS "+tblName) })

	md, err := th.SourceMetadata(src)
	require.NoError(t, err)
	require.NotNil(t, md)

	// Find the test table in source metadata.
	var tbl *metadata.Table
	for _, t2 := range md.Tables {
		if t2.Name == tblName {
			tbl = t2
			break
		}
	}
	require.NotNil(t, tbl, "table %s not found in metadata", tblName)
	require.Len(t, tbl.Columns, 4)

	for _, col := range tbl.Columns {
		require.NotEmpty(t, col.BaseType,
			"column %s: BaseType should not be empty", col.Name)
		require.NotEmpty(t, col.Kind.String(),
			"column %s: Kind should be assigned", col.Name)

		t.Logf("  %s: kind=%s base_type=%s column_type=%s",
			col.Name, col.Kind, col.BaseType, col.ColumnType)
	}
}
