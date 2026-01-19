package clickhouse

import (
	"github.com/neilotoole/sq/libsq/core/kind"
	"github.com/neilotoole/sq/libsq/core/schema"
)

// Exported variables for testing unexported functions from external test packages.
// These allow tests in *_test.go files (package clickhouse_test) to access
// unexported functionality.

// render.go exports.
var (
	ExportDbTypeNameFromKind   = dbTypeNameFromKind
	ExportBuildCreateTableStmt = buildCreateTableStmt
	ExportBuildUpdateStmt      = buildUpdateStmt
)

// metadata.go exports.
var (
	ExportKindFromClickHouseType  = kindFromClickHouseType
	ExportIsNullableType          = isNullableType
	ExportIsNullableTypeUnwrapped = isNullableTypeUnwrapped
	ExportTableTypeFromEngine     = tableTypeFromEngine
)

// Type aliases for function signatures used in tests.
type (
	// KindFromClickHouseTypeFunc is the signature of kindFromClickHouseType.
	KindFromClickHouseTypeFunc = func(chType string) kind.Kind

	// BuildCreateTableStmtFunc is the signature of buildCreateTableStmt.
	BuildCreateTableStmtFunc = func(tblDef *schema.Table) string
)
