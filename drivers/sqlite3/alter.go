package sqlite3

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/neilotoole/sq/drivers/sqlite3/sqlparser"
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
// AUTOINCREMENT sequence continuity is preserved across the rebuild (gh757):
// the table's sqlite_sequence row is captured before the rebuild and restored
// afterward, so the next insert picks seq+1 rather than MAX(rowid)+1.
//
// Note that colNames and kinds must be the same length.
func (d *driveri) AlterTableColumnKinds(ctx context.Context, db sqlz.DB,
	tblName string, colNames []string, kinds []kind.Kind,
) (err error) {
	if len(colNames) != len(kinds) {
		return errz.New("sqlite3: alter table: mismatched count of columns and kinds")
	}

	// It's recommended to disable foreign keys before this alter procedure.
	if restorePragmaFK, fkErr := pragmaDisableForeignKeys(ctx, db); fkErr != nil {
		return fkErr
	} else if restorePragmaFK != nil {
		defer restorePragmaFK()
	}

	q := "SELECT sql FROM sqlite_master WHERE type='table' AND name=?"
	var ogDDL string
	if err = db.QueryRowContext(ctx, q, tblName).Scan(&ogDDL); err != nil {
		return errz.Wrapf(err, "sqlite3: alter table: failed to read original DDL")
	}

	// Capture the table's sqlite_sequence row (if any) before the rebuild.
	// The DROP TABLE below removes the row, so without a restore the next
	// AUTOINCREMENT insert would pick MAX(rowid)+1 rather than seq+1,
	// silently reusing rowids of previously deleted rows (gh757).
	srcSeq, err := readSqliteSequence(ctx, db, tblName)
	if err != nil {
		return err
	}

	allColDefs, err := sqlparser.ExtractCreateTableStmtColDefs(ogDDL)
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

	// Locate the table identifier in the original DDL so its byte offset
	// is known. Avoids unanchored strings.Replace, which can misfire when
	// the table name also appears as a column-name prefix or inside a
	// comment / default literal.
	tblIdent, err := sqlparser.ExtractTableIdentFromCreateTableStmt(ogDDL)
	if err != nil {
		return errz.Wrap(err, "sqlite3: alter table: failed to extract table identifier from DDL")
	}

	nuTblName := "tmp_tbl_alter_" + stringz.Uniq32()
	edits := make([]sqlparser.Edit, 0, len(colDefs)+1)
	for i, colDef := range colDefs {
		edits = append(edits, sqlparser.Edit{
			Start:       colDef.RawTypeOffset,
			End:         colDef.RawTypeOffset + len(colDef.RawType),
			Replacement: DBTypeForKind(kinds[i]),
		})
	}
	edits = append(edits, sqlparser.Edit{
		Start:       tblIdent.TableOffset,
		End:         tblIdent.TableOffset + len(tblIdent.RawTable),
		Replacement: stringz.DoubleQuote(nuTblName),
	})

	nuDDL, err := sqlparser.ApplyEdits(ogDDL, edits)
	if err != nil {
		return errz.Wrap(err, "sqlite3: alter table: failed to apply DDL rewrites")
	}

	if _, err = db.ExecContext(ctx, nuDDL); err != nil {
		return errz.Wrapf(err, "sqlite3: alter table: failed to create temporary table")
	}

	copyStmt := fmt.Sprintf(
		"INSERT INTO %s SELECT * FROM %s",
		stringz.DoubleQuote(nuTblName),
		stringz.DoubleQuote(tblName),
	)
	if _, err = db.ExecContext(ctx, copyStmt); err != nil {
		return errz.Wrapf(err, "sqlite3: alter table: failed to copy data to temporary table")
	}

	// Drop old table
	if _, err = db.ExecContext(ctx, "DROP TABLE "+stringz.DoubleQuote(tblName)); err != nil {
		return errz.Wrapf(err, "sqlite3: alter table: failed to drop original table")
	}

	// Rename new table to old table name
	if _, err = db.ExecContext(ctx, fmt.Sprintf(
		"ALTER TABLE %s RENAME TO %s",
		stringz.DoubleQuote(nuTblName),
		stringz.DoubleQuote(tblName),
	)); err != nil {
		return errz.Wrapf(err, "sqlite3: alter table: failed to rename temporary table")
	}

	if srcSeq.Valid {
		// Restore AUTOINCREMENT continuity (gh757). At this point the
		// rebuilt table's sqlite_sequence row, created by the copy and
		// renamed along with the table, holds MAX(rowid) of the copied
		// rows (0 if the copy moved no rows) rather than the original
		// seq. The UPDATE takes max(seq, captured) so the restore never
		// lowers the value already in place, and the conditional INSERT
		// covers the case of no row existing. Neither statement destroys
		// state, so a failure here can't lose the live sequence value.
		// sqlite_sequence has no unique constraint on name, which rules
		// out INSERT OR REPLACE.
		if _, err = db.ExecContext(ctx,
			"UPDATE sqlite_sequence SET seq = max(seq, ?) WHERE name = ?",
			srcSeq.Int64, tblName); err != nil {
			return errz.Wrapf(errw(err),
				"sqlite3: alter table: table {%s} was rebuilt, but updating sqlite_sequence to %d failed",
				tblName, srcSeq.Int64)
		}
		if _, err = db.ExecContext(ctx,
			"INSERT INTO sqlite_sequence (name, seq) SELECT ?, ? "+
				"WHERE NOT EXISTS (SELECT 1 FROM sqlite_sequence WHERE name = ?)",
			tblName, srcSeq.Int64, tblName); err != nil {
			return errz.Wrapf(errw(err),
				"sqlite3: alter table: table {%s} was rebuilt, but restoring sqlite_sequence to %d failed",
				tblName, srcSeq.Int64)
		}
	}

	return nil
}

// readSqliteSequence returns the sqlite_sequence row for tbl. The result is
// invalid (and err nil) if the sqlite_sequence table doesn't exist, or has
// no row for tbl.
func readSqliteSequence(ctx context.Context, db sqlz.DB, tbl string) (sql.NullInt64, error) {
	var seq sql.NullInt64

	// sqlite_sequence only exists once an AUTOINCREMENT table has been
	// created in the DB; querying it blindly would error.
	var n int
	if err := db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='sqlite_sequence'",
	).Scan(&n); err != nil {
		return seq, errz.Wrap(errw(err),
			"sqlite3: alter table: failed to check for sqlite_sequence table")
	}
	if n == 0 {
		return seq, nil
	}

	if err := db.QueryRowContext(ctx,
		"SELECT seq FROM sqlite_sequence WHERE name=?", tbl,
	).Scan(&seq); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return seq, errz.Wrapf(errw(err),
			"sqlite3: alter table: failed to read sqlite_sequence for {%s}", tbl)
	}

	return seq, nil
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
