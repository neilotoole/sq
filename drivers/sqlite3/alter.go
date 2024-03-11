package sqlite3

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/neilotoole/sq/drivers/sqlite3/internal/sqlparser"
	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/kind"
	"github.com/neilotoole/sq/libsq/core/lg"
	"github.com/neilotoole/sq/libsq/core/lg/lga"
	"github.com/neilotoole/sq/libsq/core/sqlz"
	"github.com/neilotoole/sq/libsq/core/stringz"
)

// AlterTableRename implements driver.SQLDriver.
func (d *driveri) AlterTableRename(ctx context.Context, db sqlz.DB, tbl, newName string) error {
	q := fmt.Sprintf(`ALTER TABLE %q RENAME TO %q`, tbl, newName)
	_, err := db.ExecContext(ctx, q)
	return errz.Wrapf(errw(err), "alter table: failed to rename table {%s} to {%s}", tbl, newName)
}

// AlterTableRenameColumn implements driver.SQLDriver.
func (d *driveri) AlterTableRenameColumn(ctx context.Context, db sqlz.DB, tbl, col, newName string) error {
	q := fmt.Sprintf("ALTER TABLE %q RENAME COLUMN %q TO %q", tbl, col, newName)
	_, err := db.ExecContext(ctx, q)
	return errz.Wrapf(errw(err), "alter table: failed to rename column {%s.%s} to {%s}", tbl, col, newName)
}

// AlterTableAddColumn implements driver.SQLDriver.
func (d *driveri) AlterTableAddColumn(ctx context.Context, db sqlz.DB, tbl, col string, knd kind.Kind) error {
	q := fmt.Sprintf("ALTER TABLE %q ADD COLUMN %q ", tbl, col) + DBTypeForKind(knd)

	_, err := db.ExecContext(ctx, q)
	if err != nil {
		return errz.Wrapf(errw(err), "alter table: failed to add column {%s} to table {%s}", col, tbl)
	}

	return nil
}

// AlterTableColumnKinds implements driver.SQLDriver. Note that SQLite doesn't
// really support altering column types, so this is a hacky implementation.
// It's not guaranteed that indices, constraints, etc. will be preserved. See:
//
//   - https://www.sqlite.org/lang_altertable.html
//   - https://www.sqlite.org/faq.html#q11
//   - https://www.sqlitetutorial.net/sqlite-alter-table/
//
// Note that colNames and kinds must be the same length.
func (d *driveri) AlterTableColumnKinds(ctx context.Context, db sqlz.DB,
	tblName string, colNames []string, kinds []kind.Kind,
) (err error) {
	if len(colNames) != len(kinds) {
		return errz.New("sqlite3: alter table: mismatched count of columns and kinds")
	}

	if restorePragmaFK, fkErr := pragmaDisableForeignKeys(ctx, db); fkErr != nil {
		return fkErr
	} else if restorePragmaFK != nil {
		defer restorePragmaFK()
	}

	var tx *sql.Tx
	if tx, err = getTx(ctx, db); err != nil {
		return err
	}

	defer func() {
		if tx != nil {
			_ = tx.Rollback()
		}
	}()

	q := "SELECT sql FROM sqlite_master WHERE type='table' AND name=?"
	var ogDDL string
	if err = tx.QueryRowContext(ctx, q, tblName).Scan(&ogDDL); err != nil {
		return errz.Wrapf(err, "sqlite3: alter table: failed to read original DDL")
	}

	allColDefs, err := sqlparser.ExtractCreateStmtColDefs(ogDDL)
	if err != nil {
		return errz.Wrapf(err, "sqlite3: alter table: failed to extract column definitions from DDL")
	}

	var colDefs []*sqlparser.ColDef
	for i, colName := range colNames {
		for _, cd := range allColDefs {
			if cd.Name == colName {
				colDefs = append(colDefs, cd)
				break
			}
		}
		if len(colDefs) != i+1 {
			return errz.Errorf("sqlite3: alter table: column {%s} not found in table DDL", colName)
		}
	}

	nuDDL := ogDDL
	for i, colDef := range colDefs {
		wantType := DBTypeForKind(kinds[i])
		wantColDefText := strings.Replace(colDef.Raw, colDef.RawType, wantType, 1)
		nuDDL = strings.Replace(nuDDL, colDef.Raw, wantColDefText, 1)
	}

	nuTblName := "tmp_tbl_alter_" + stringz.Uniq32()
	nuDDL = strings.Replace(nuDDL, tblName, nuTblName, 1)

	if _, err = tx.ExecContext(ctx, nuDDL); err != nil {
		return errz.Wrapf(err, "sqlite3: alter table: failed to create temporary table")
	}

	copyStmt := fmt.Sprintf( //nolint:gosec
		"INSERT INTO %s SELECT * FROM %s",
		stringz.DoubleQuote(nuTblName),
		stringz.DoubleQuote(tblName),
	)
	if _, err = tx.ExecContext(ctx, copyStmt); err != nil {
		return errz.Wrapf(err, "sqlite3: alter table: failed to copy data to temporary table")
	}

	// Drop old table
	if _, err = tx.ExecContext(ctx, "DROP TABLE "+stringz.DoubleQuote(tblName)); err != nil {
		return errz.Wrapf(err, "sqlite3: alter table: failed to drop original table")
	}

	// Rename new table to old table name
	if _, err = tx.ExecContext(ctx, fmt.Sprintf(
		"ALTER TABLE %s RENAME TO %s",
		stringz.DoubleQuote(nuTblName),
		stringz.DoubleQuote(tblName),
	)); err != nil {
		return errz.Wrapf(err, "sqlite3: alter table: failed to rename temporary table")
	}

	err = tx.Commit()
	tx = nil
	if err != nil {
		return errz.Wrapf(err, "sqlite3: alter table: failed to commit transaction")
	}

	return nil
}

func getTx(ctx context.Context, db sqlz.DB) (tx *sql.Tx, err error) {
	var ok bool
	if tx, ok = db.(*sql.Tx); !ok {
		var sqlDB *sql.DB
		if sqlDB, ok = db.(*sql.DB); !ok {
			return nil, errz.Errorf("sqlite3: expected *sql.DB or *sql.Tx but got: %T", db)
		}

		tx, err = sqlDB.BeginTx(ctx, nil)
		if err != nil {
			return nil, errz.Wrapf(err, "sqlite3: failed to begin transaction")
		}
	}

	return tx, nil
}

// pragmaDisableForeignKeys disables foreign keys, returning a function that
// restores the original state of the foreign_keys pragma. If an error occurs,
// the returned restore function will be nil.
func pragmaDisableForeignKeys(ctx context.Context, db sqlz.DB) (restore func(), err error) {
	pragmaFkExisting, err := readPragma(ctx, db, "foreign_keys")
	if err != nil {
		return nil, errz.Wrapf(err, "sqlite3: alter table: failed to read foreign_keys pragma")
	}

	if _, err = db.ExecContext(ctx, "PRAGMA foreign_keys=off"); err != nil {
		return nil, errz.Wrapf(err, "sqlite3: alter table: failed to disable foreign_keys pragma")
	}

	return func() {
		_, restoreErr := db.ExecContext(ctx, fmt.Sprintf("PRAGMA foreign_keys=%v", pragmaFkExisting))
		if restoreErr != nil {
			lg.FromContext(ctx).Error("sqlite3: alter table: failed to restore foreign_keys pragma", lga.Err, restoreErr)
		}
	}, nil
}
