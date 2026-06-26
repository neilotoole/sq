// Package clickhouse internal_test.go provides test exports and internal tests.
//
// This file serves two purposes:
//
//  1. Export Bridge: Provides exported aliases for unexported functions so that
//     external test packages (package clickhouse_test in *_test.go files) can
//     test internal functionality without being in the same package.
//
//  2. Internal Tests: Contains tests that need direct access to unexported
//     functions or types and are simpler to write in the same package.
//
// # Export Convention
//
// Unexported functions are exported with capitalized names:
//
//	dbTypeNameFromKind   -> DbTypeNameFromKind
//	buildCreateTableStmt -> BuildCreateTableStmt
//	kindFromClickHouseType -> KindFromClickHouseType
//
// This is a common Go testing pattern that maintains encapsulation while
// enabling thorough testing of internal behavior.
package clickhouse

import (
	"testing"

	clickhouse "github.com/ClickHouse/clickhouse-go/v2"
	"github.com/stretchr/testify/require"
)

// Exported variables for testing unexported functions from external test packages.
// These allow tests in *_test.go files (package clickhouse_test) to access
// unexported functionality without modifying the public API.

// render.go exports.
var (
	DbTypeNameFromKind       = dbTypeNameFromKind
	BuildCreateTableStmt     = buildCreateTableStmt
	BuildCreateTableStmtName = buildCreateTableStmtName
	BuildUpdateStmt          = buildUpdateStmt
)

// metadata.go exports.
var (
	BaseTypeFromClickHouseType = baseTypeFromClickHouseType
	KindFromClickHouseType     = kindFromClickHouseType
	IsNullableType             = isNullableType
	IsNullableTypeUnwrapped    = isNullableTypeUnwrapped
	TableTypeFromEngine        = tableTypeFromEngine
	ConvertArrayToString       = convertArrayToString
	ResolveQualifiedColNames   = resolveQualifiedColNames
)

// TestExtractClickHouseCheckConstraints tests the pure DDL parser used to
// extract CHECK constraints from system.tables.create_table_query.
func TestExtractClickHouseCheckConstraints(t *testing.T) {
	testCases := []struct {
		name     string
		ddl      string
		tblName  string
		wantLen  int
		wantCons []struct{ name, clause string }
	}{
		{
			name:    "no constraints",
			ddl:     "CREATE TABLE t (`id` Int64, `name` String) ENGINE = MergeTree ORDER BY id",
			tblName: "t",
			wantLen: 0,
		},
		{
			name:    "single simple constraint",
			ddl:     "CREATE TABLE t (`id` Int64, `price` Decimal(10, 2), CONSTRAINT chk_price CHECK (price > 0)) ENGINE = MergeTree ORDER BY id",
			tblName: "t",
			wantLen: 1,
			wantCons: []struct{ name, clause string }{
				{"chk_price", "price > 0"},
			},
		},
		{
			name:    "multiple constraints with nested parens",
			ddl:     "CREATE TABLE t (`id` Int64, `age` Int32, CONSTRAINT chk_price CHECK (price > 0), CONSTRAINT chk_age CHECK ((age >= 0) AND (age <= 150))) ENGINE = MergeTree ORDER BY id",
			tblName: "t",
			wantLen: 2,
			wantCons: []struct{ name, clause string }{
				{"chk_price", "price > 0"},
				{"chk_age", "(age >= 0) AND (age <= 150)"},
			},
		},
		{
			name:    "empty DDL",
			ddl:     "",
			tblName: "t",
			wantLen: 0,
		},
		{
			// A closing paren inside a string literal must not close the CHECK
			// expression. Without string-literal awareness, balancedParenContents
			// exits at the ')' inside ')', truncating the clause to "name != '".
			name:    "paren inside single-quoted string literal",
			ddl:     "CREATE TABLE t (`id` Int64, `name` String, CONSTRAINT chk_name CHECK (name != ')')) ENGINE = MergeTree ORDER BY id",
			tblName: "t",
			wantLen: 1,
			wantCons: []struct{ name, clause string }{
				{"chk_name", "name != ')'"},
			},
		},
		{
			// Escaped quote (SQL-standard '' pair) inside a string that also
			// contains a closing paren must not prematurely end the expression.
			name:    "escaped quote and paren inside string literal",
			ddl:     "CREATE TABLE t (`id` Int64, CONSTRAINT chk_id CHECK (id != 0)) ENGINE = MergeTree ORDER BY id",
			tblName: "t",
			wantLen: 1,
			wantCons: []struct{ name, clause string }{
				{"chk_id", "id != 0"},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := extractClickHouseCheckConstraints(tc.ddl, tc.tblName)
			require.Len(t, got, tc.wantLen)
			for i, want := range tc.wantCons {
				require.Equal(t, tc.tblName, got[i].Table)
				require.Equal(t, want.name, got[i].Name)
				require.Equal(t, want.clause, got[i].Clause)
			}
		})
	}
}

// TestExtractViewSelectFromCHDDL tests the pure-function DDL parser used to
// extract the SELECT text from a CREATE [MATERIALIZED] VIEW DDL string.
func TestExtractViewSelectFromCHDDL(t *testing.T) {
	testCases := []struct {
		name string
		ddl  string
		want string
	}{
		{
			// Standard view DDL: the common case.
			name: "standard view",
			ddl:  "CREATE VIEW sakila.actor_info AS SELECT id, name FROM actor",
			want: "SELECT id, name FROM actor",
		},
		{
			// Materialized view with TO target: " AS " appears once, before SELECT.
			name: "materialized view with TO target",
			ddl:  "CREATE MATERIALIZED VIEW sakila.mv TO sakila.mv_target AS SELECT id FROM actor",
			want: "SELECT id FROM actor",
		},
		{
			// " AS SELECT " match is preferred when the DDL has an earlier " AS "
			// (e.g. a column alias in the header), avoiding a false split.
			name: "AS SELECT preferred over earlier AS",
			ddl:  "CREATE VIEW db.v AS SELECT id AS alias_id FROM t",
			want: "SELECT id AS alias_id FROM t",
		},
		{
			// No " AS " in the DDL: must return "" rather than the raw DDL blob.
			// The old code returned the full string unchanged, which is wrong.
			name: "no AS returns empty string",
			ddl:  "CREATE TABLE t (id Int64) ENGINE = MergeTree ORDER BY id",
			want: "",
		},
		{
			// Empty input.
			name: "empty DDL",
			ddl:  "",
			want: "",
		},
		{
			// Lowercase keywords: case-insensitive matching.
			name: "lowercase as select",
			ddl:  "create view db.v as select id from t",
			want: "select id from t",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := extractViewSelectFromCHDDL(tc.ddl)
			require.Equal(t, tc.want, got)
		})
	}
}

// clickhouse.go exports.
var LocationWithDefaultPort = locationWithDefaultPort

// errors.go exports.
var (
	HasErrCode             = hasErrCode
	IsErrUnknownTable      = isErrUnknownTable
	ErrCodeUnknownTable    = errCodeUnknownTable
	IsErrUnknownDatabase   = isErrUnknownDatabase
	ErrCodeUnknownDatabase = errCodeUnknownDatabase
)

// TestHasErrCode verifies that hasErrCode correctly identifies ClickHouse errors
// by their error code, even when the error has been wrapped by errw or other
// error wrappers.
//
// This is critical for error classification - sq needs to detect specific
// database errors (like "table not found") to provide appropriate responses.
// The test verifies:
//
//  1. Direct *clickhouse.Exception errors are recognized by code
//  2. Wrapped errors (via errw) still match by code using errors.As
func TestHasErrCode(t *testing.T) {
	// Create a ClickHouse exception with UNKNOWN_TABLE error code (60)
	var err error
	err = &clickhouse.Exception{
		Code:    60,
		Message: "Table sakila.does_not_exist doesn't exist. (UNKNOWN_TABLE)",
	}

	// Verify direct error detection
	require.True(t, hasErrCode(err, errCodeUnknownTable))

	// Verify wrapped error detection - errw wraps errors for stack traces
	// but hasErrCode should still find the underlying ClickHouse error
	err = errw(err)
	require.True(t, hasErrCode(err, errCodeUnknownTable))
}

// TestHasErrCode_UnknownDatabase verifies that hasErrCode correctly identifies
// ClickHouse UNKNOWN_DATABASE errors (code 81), even when wrapped.
func TestHasErrCode_UnknownDatabase(t *testing.T) {
	var err error
	err = &clickhouse.Exception{
		Code:    81,
		Message: "Database no_such_db does not exist. (UNKNOWN_DATABASE)",
	}

	// Verify direct error detection.
	require.True(t, hasErrCode(err, errCodeUnknownDatabase))
	require.True(t, isErrUnknownDatabase(err))
	require.False(t, isErrUnknownTable(err))

	// Verify wrapped error detection.
	err = errw(err)
	require.True(t, hasErrCode(err, errCodeUnknownDatabase))
	require.True(t, isErrUnknownDatabase(err))
}
