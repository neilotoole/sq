package duckdb

import (
	"context"
	"fmt"

	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/kind"
	"github.com/neilotoole/sq/libsq/core/sqlz"
)

// alterTableRename renames a table.
func alterTableRename(ctx context.Context, db sqlz.DB, oldName, newName string) error {
	q := fmt.Sprintf(`ALTER TABLE %q RENAME TO %q`, oldName, newName)
	_, err := db.ExecContext(ctx, q)
	return errz.Wrapf(errz.Err(err),
		"duckdb: alter table: failed to rename table {%s} to {%s}", oldName, newName)
}

// alterTableAddColumn adds a column of the given kind to an existing table.
func alterTableAddColumn(ctx context.Context, db sqlz.DB, tblName, colName string, k kind.Kind) error {
	dbType := dbTypeNameFromKind(k)
	q := fmt.Sprintf(`ALTER TABLE %q ADD COLUMN %q %s`, tblName, colName, dbType)
	_, err := db.ExecContext(ctx, q)
	return errz.Wrapf(errz.Err(err),
		"duckdb: alter table: failed to add column {%s} to table {%s}", colName, tblName)
}

// alterTableRenameColumn renames a column in an existing table.
func alterTableRenameColumn(ctx context.Context, db sqlz.DB, tblName, oldCol, newCol string) error {
	q := fmt.Sprintf(`ALTER TABLE %q RENAME COLUMN %q TO %q`, tblName, oldCol, newCol)
	_, err := db.ExecContext(ctx, q)
	return errz.Wrapf(errz.Err(err),
		"duckdb: alter table: failed to rename column {%s.%s} to {%s}", tblName, oldCol, newCol)
}

// alterTableColumnKinds changes column types. Each (column, kind) pair is
// applied as a separate ALTER COLUMN statement; the operation is not atomic
// across columns.
//
// Note that colNames and kinds must be the same length.
func alterTableColumnKinds(
	ctx context.Context, db sqlz.DB, tblName string, colNames []string, kinds []kind.Kind,
) error {
	if len(colNames) != len(kinds) {
		return errz.Errorf("duckdb: alter table: mismatched count of columns (%d) and kinds (%d)",
			len(colNames), len(kinds))
	}
	for i, col := range colNames {
		dbType := dbTypeNameFromKind(kinds[i])
		q := fmt.Sprintf(`ALTER TABLE %q ALTER COLUMN %q SET DATA TYPE %s`, tblName, col, dbType)
		if _, err := db.ExecContext(ctx, q); err != nil {
			return errz.Wrapf(errz.Err(err),
				"duckdb: alter table: failed to set data type of column {%s.%s} to {%s}",
				tblName, col, dbType)
		}
	}
	return nil
}

// dbTypeNameFromKind returns the DuckDB column type name for a kind.Kind.
// The mapping mirrors kindFromDBTypeName (in metadata.go) to ensure
// round-trip consistency.
func dbTypeNameFromKind(k kind.Kind) string {
	switch k {
	case kind.Bool:
		return "BOOLEAN"
	case kind.Int:
		return "BIGINT"
	case kind.Float:
		return "DOUBLE"
	case kind.Decimal:
		return "DECIMAL(38,9)"
	case kind.Text:
		return "VARCHAR"
	case kind.Bytes:
		return "BLOB"
	case kind.Date:
		return "DATE"
	case kind.Time:
		return "TIME"
	case kind.Datetime:
		return "TIMESTAMP"
	case kind.Unknown, kind.Null:
		fallthrough
	default:
		return "VARCHAR"
	}
}
