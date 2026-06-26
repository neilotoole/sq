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
