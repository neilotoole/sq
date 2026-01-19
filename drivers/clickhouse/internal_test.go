package clickhouse

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/libsq/core/kind"
	"github.com/neilotoole/sq/libsq/core/schema"
)

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
			gotKind := kindFromClickHouseType(tc.chType)
			require.Equal(t, tc.wantKind, gotKind, "Type %s", tc.chType)
		})
	}
}

// TestDbTypeNameFromKind tests type mapping from sq kinds to ClickHouse types.
func TestDbTypeNameFromKind(t *testing.T) {
	testCases := []struct {
		kind     kind.Kind
		wantType string
	}{
		{kind.Text, "String"},
		{kind.Int, "Int64"},
		{kind.Float, "Float64"},
		{kind.Decimal, "Decimal(18,4)"},
		{kind.Bool, "UInt8"},
		{kind.Datetime, "DateTime"},
		{kind.Date, "Date"},
		{kind.Time, "DateTime"},
		{kind.Bytes, "String"},
		{kind.Unknown, "String"},
	}

	for _, tc := range testCases {
		t.Run(tc.kind.String(), func(t *testing.T) {
			gotType := dbTypeNameFromKind(tc.kind)
			require.Equal(t, tc.wantType, gotType)
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
			got := isNullableType(tc.typeName)
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
			got := isNullableTypeUnwrapped(tc.typeName)
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
			gotKind := kindFromClickHouseType(tc.chType)
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
			gotKind := kindFromClickHouseType(tc.chType)
			require.Equal(t, tc.wantKind, gotKind)
		})
	}
}

// TestBuildCreateTableStmt tests CREATE TABLE statement generation.
func TestBuildCreateTableStmt(t *testing.T) {
	tblDef := schema.NewTable("test_table",
		[]string{"id", "name", "value"},
		[]kind.Kind{kind.Int, kind.Text, kind.Float})

	stmt := buildCreateTableStmt(tblDef)

	// Verify statement contains expected components
	require.Contains(t, stmt, "CREATE TABLE")
	require.Contains(t, stmt, "`test_table`")
	require.Contains(t, stmt, "`id`")
	require.Contains(t, stmt, "`name`")
	require.Contains(t, stmt, "`value`")
	require.Contains(t, stmt, "Int64")
	require.Contains(t, stmt, "String")
	require.Contains(t, stmt, "Float64")
	require.Contains(t, stmt, "ENGINE = MergeTree()")
	require.Contains(t, stmt, "ORDER BY `id`") // First column
}

// TestBuildInsertStmt tests INSERT statement generation.
func TestBuildInsertStmt(t *testing.T) {
	tblName := "test_table"
	cols := []*schema.Column{
		{Name: "id", Kind: kind.Int},
		{Name: "name", Kind: kind.Text},
		{Name: "value", Kind: kind.Float},
	}

	// Test single row insert
	stmt := buildInsertStmt(tblName, cols, 1)
	require.Contains(t, stmt, "INSERT INTO")
	require.Contains(t, stmt, "`test_table`")
	require.Contains(t, stmt, "`id`")
	require.Contains(t, stmt, "`name`")
	require.Contains(t, stmt, "`value`")
	require.Contains(t, stmt, "VALUES")
	require.Contains(t, stmt, "(?, ?, ?)")
	require.NotContains(t, stmt, "), (") // No second row

	// Test multi-row insert
	stmt = buildInsertStmt(tblName, cols, 3)
	require.Contains(t, stmt, "VALUES (?, ?, ?), (?, ?, ?), (?, ?, ?)")
}

// TestBuildUpdateStmt tests UPDATE statement generation.
func TestBuildUpdateStmt(t *testing.T) {
	tblName := "test_table"
	cols := []string{"name", "value"}

	// Test without WHERE clause
	stmt := buildUpdateStmt(tblName, cols, "")
	require.Contains(t, stmt, "ALTER TABLE")
	require.Contains(t, stmt, "`test_table`")
	require.Contains(t, stmt, "UPDATE")
	require.Contains(t, stmt, "`name` = ?")
	require.Contains(t, stmt, "`value` = ?")
	require.NotContains(t, stmt, "WHERE")

	// Test with WHERE clause
	stmt = buildUpdateStmt(tblName, cols, "id = 123")
	require.Contains(t, stmt, "WHERE id = 123")
}

// TestDialectPlaceholders tests that the dialect uses ? placeholders.
func TestDialectPlaceholders(t *testing.T) {
	d := &driveri{}
	dialect := d.Dialect()

	// ClickHouse should use ? placeholders
	// Test single column, single row
	require.Equal(t, "(?)", dialect.Placeholders(1, 1))

	// Test multiple columns, single row
	require.Equal(t, "(?, ?, ?)", dialect.Placeholders(3, 1))

	// Test single column, multiple rows
	require.Equal(t, "(?), (?), (?)", dialect.Placeholders(1, 3))

	// Test multiple columns, multiple rows
	require.Equal(t, "(?, ?), (?, ?)", dialect.Placeholders(2, 2))
}

// TestDialectEnquote tests backtick quoting.
func TestDialectEnquote(t *testing.T) {
	d := &driveri{}
	dialect := d.Dialect()

	testCases := []struct {
		input string
		want  string
	}{
		{"simple", "`simple`"},
		{"table_name", "`table_name`"},
		{"column", "`column`"},
		{"CamelCase", "`CamelCase`"},
		{"with space", "`with space`"},
	}

	for _, tc := range testCases {
		t.Run(tc.input, func(t *testing.T) {
			got := dialect.Enquote(tc.input)
			require.Equal(t, tc.want, got)
		})
	}
}
