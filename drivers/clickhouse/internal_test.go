package clickhouse

import (
	"github.com/neilotoole/sq/libsq/core/kind"
	"github.com/neilotoole/sq/libsq/core/schema"
)

// Exported variables for testing unexported functions from external test packages.
// These allow tests in *_test.go files (package clickhouse_test) to access
// unexported functionality. The naming convention is to capitalize the first
// letter of the unexported function name (e.g., buildCreateTableStmt becomes
// BuildCreateTableStmt).

// render.go exports.
var (
	DbTypeNameFromKind   = dbTypeNameFromKind
	BuildCreateTableStmt = buildCreateTableStmt
	BuildUpdateStmt      = buildUpdateStmt
)

// metadata.go exports.
var (
	KindFromClickHouseType  = kindFromClickHouseType
	IsNullableType          = isNullableType
	IsNullableTypeUnwrapped = isNullableTypeUnwrapped
	TableTypeFromEngine     = tableTypeFromEngine
)

// Type aliases for function signatures used in tests.
type (
	// KindFromClickHouseTypeFunc is the signature of kindFromClickHouseType.
	KindFromClickHouseTypeFunc = func(chType string) kind.Kind

	// BuildCreateTableStmtFunc is the signature of buildCreateTableStmt.
	BuildCreateTableStmtFunc = func(tblDef *schema.Table) string
)
