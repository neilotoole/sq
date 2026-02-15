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

	"github.com/neilotoole/sq/libsq/core/kind"
	"github.com/neilotoole/sq/libsq/core/schema"
)

// Exported variables for testing unexported functions from external test packages.
// These allow tests in *_test.go files (package clickhouse_test) to access
// unexported functionality without modifying the public API.

// render.go exports.
var (
	DbTypeNameFromKind   = dbTypeNameFromKind
	BuildCreateTableStmt = buildCreateTableStmt
	BuildUpdateStmt      = buildUpdateStmt
)

// metadata.go exports.
var (
	BaseTypeFromClickHouseType = baseTypeFromClickHouseType
	KindFromClickHouseType     = kindFromClickHouseType
	IsNullableType             = isNullableType
	IsNullableTypeUnwrapped    = isNullableTypeUnwrapped
	TableTypeFromEngine        = tableTypeFromEngine
	ConvertArrayToString       = convertArrayToString
)

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

// Type aliases for function signatures used in tests.
type (
	// KindFromClickHouseTypeFunc is the signature of kindFromClickHouseType.
	KindFromClickHouseTypeFunc = func(chType string) kind.Kind

	// BuildCreateTableStmtFunc is the signature of buildCreateTableStmt.
	BuildCreateTableStmtFunc = func(tblDef *schema.Table) string
)
