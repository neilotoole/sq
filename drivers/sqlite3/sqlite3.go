// Package sqlite3 implements the sq driver for SQLite.
// The backing SQL driver is mattn/sqlite3.
package sqlite3

import "C"

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3" // Import for side effect of loading the driver
	"github.com/shopspring/decimal"

	"github.com/neilotoole/sq/drivers/sqlite3/sqlparser"
	"github.com/neilotoole/sq/libsq/ast"
	"github.com/neilotoole/sq/libsq/ast/render"
	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/ioz"
	"github.com/neilotoole/sq/libsq/core/jointype"
	"github.com/neilotoole/sq/libsq/core/kind"
	"github.com/neilotoole/sq/libsq/core/langz"
	"github.com/neilotoole/sq/libsq/core/lg"
	"github.com/neilotoole/sq/libsq/core/lg/lga"
	"github.com/neilotoole/sq/libsq/core/lg/lgm"
	"github.com/neilotoole/sq/libsq/core/record"
	"github.com/neilotoole/sq/libsq/core/schema"
	"github.com/neilotoole/sq/libsq/core/sqlz"
	"github.com/neilotoole/sq/libsq/core/stringz"
	"github.com/neilotoole/sq/libsq/core/tablefq"
	"github.com/neilotoole/sq/libsq/driver"
	"github.com/neilotoole/sq/libsq/driver/dialect"
	"github.com/neilotoole/sq/libsq/source"
	"github.com/neilotoole/sq/libsq/source/drivertype"
	"github.com/neilotoole/sq/libsq/source/location"
	"github.com/neilotoole/sq/libsq/source/metadata"
)

const (

	// dbDrvr is the backing sqlite3 SQL driver impl name.
	dbDrvr = "sqlite3"

	// Prefix is the scheme+separator value "sqlite3://".
	Prefix = "sqlite3://"
)

var _ driver.Provider = (*Provider)(nil)

// Provider is the SQLite3 implementation of driver.Provider.
type Provider struct {
	Log *slog.Logger
}

// DriverFor implements driver.Provider.
func (p *Provider) DriverFor(typ drivertype.Type) (driver.Driver, error) {
	if typ != drivertype.SQLite {
		return nil, errz.Errorf("unsupported driver type {%s}", typ)
	}

	return &driveri{log: p.Log}, nil
}

var _ driver.SQLDriver = (*driveri)(nil)

// driveri is the SQLite3 implementation of driver.SQLDriver.
type driveri struct {
	log *slog.Logger
}

// ConnParams implements driver.SQLDriver.
// See: https://github.com/mattn/go-sqlite3#connection-string.
func (d *driveri) ConnParams() map[string][]string {
	return map[string][]string{
		"_auth":                     nil,
		"_auth_crypt":               {"SHA1", "SSHA1", "SHA256", "SSHA256", "SHA384", "SSHA384", "SHA512", "SSHA512"},
		"_auth_pass":                nil,
		"_auth_salt":                nil,
		"_auth_user":                nil,
		"_auto_vacuum":              {"none", "full", "incremental"},
		"_busy_timeout":             nil,
		"_cache_size":               {"-2000"},
		"_case_sensitive_like":      {"true", "false"},
		"_defer_foreign_keys":       {"true", "false"},
		"_foreign_keys":             {"true", "false"},
		"_ignore_check_constraints": {"true", "false"},
		"_journal_mode":             {"DELETE", "TRUNCATE", "PERSIST", "MEMORY", "WAL", "OFF"},
		"_loc":                      nil,
		"_locking_mode":             {"NORMAL", "EXCLUSIVE"},
		"_mutex":                    {"no", "full"},
		"_query_only":               {"true", "false"},
		"_recursive_triggers":       {"true", "false"},
		"_secure_delete":            {"true", "false", "FAST"},
		"_synchronous":              {"OFF", "NORMAL", "FULL", "EXTRA"},
		"_txlock":                   {"immediate", "deferred", "exclusive"},
		"cache":                     {"true", "false", "FAST"},
		"mode":                      {"ro", "rw", "rwc", "memory"},
	}
}

// LocationShape implements driver.SQLDriver.
func (d *driveri) LocationShape() driver.LocationShape {
	return driver.LocationShape{
		Type:    drivertype.SQLite,
		Schemes: []string{"sqlite3"},
		Segments: []driver.Segment{
			{Kind: driver.SegPathFile},
			{Kind: driver.SegConnParams, Optional: true},
		},
	}
}

// ErrWrapFunc implements driver.SQLDriver.
func (d *driveri) ErrWrapFunc() func(error) error {
	return errw
}

// DBProperties implements driver.SQLDriver.
func (d *driveri) DBProperties(ctx context.Context, db sqlz.DB) (map[string]any, error) {
	return getDBProperties(ctx, db)
}

// DriverMetadata implements driver.Driver.
func (d *driveri) DriverMetadata() driver.Metadata {
	return driver.Metadata{
		Type:        drivertype.SQLite,
		Description: "SQLite",
		Doc:         "https://github.com/mattn/go-sqlite3",
		IsSQL:       true,
	}
}

// Open implements driver.Driver.
func (d *driveri) Open(ctx context.Context, src *source.Source, _ driver.AccessMode) (driver.Grip, error) {
	lg.FromContext(ctx).Debug(lgm.OpenSrc, lga.Src, src)

	db, err := d.doOpen(ctx, src)
	if err != nil {
		return nil, err
	}

	if err = driver.OpeningPing(ctx, src, db); err != nil {
		return nil, err
	}

	return &grip{log: d.log, db: db, src: src, drvr: d}, nil
}

func (d *driveri) doOpen(ctx context.Context, src *source.Source) (*sql.DB, error) {
	dsn, err := dsnFromLocation(src.Location)
	if err != nil {
		return nil, err
	}

	db, err := sql.Open(dbDrvr, dsn)
	if err != nil {
		// Don't include dsn in the error: it may contain secret
		// connection params (e.g. _auth_pass).
		return nil, errz.Wrapf(errw(err), "failed to open sqlite3 source %s", src.Handle)
	}

	driver.ConfigureDB(ctx, db, src.Options)
	return db, nil
}

// Truncate implements driver.Driver.
func (d *driveri) Truncate(ctx context.Context, src *source.Source, tbl string, reset bool) (affected int64,
	err error,
) {
	db, err := d.doOpen(ctx, src)
	if err != nil {
		return 0, errw(err)
	}
	defer lg.WarnIfFuncError(d.log, lgm.CloseDB, db.Close)

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return 0, errw(err)
	}

	affected, err = sqlz.ExecAffected(ctx, tx, "DELETE FROM "+stringz.DoubleQuote(tbl))
	if err != nil {
		return affected, errz.Append(err, errw(tx.Rollback()))
	}

	if reset {
		// First check that the sqlite_sequence table event exists. It
		// may not exist if there are no auto-increment columns?
		const q = `SELECT COUNT(name) FROM sqlite_master WHERE type='table' AND name='sqlite_sequence'`
		var count int64
		err = tx.QueryRowContext(ctx, q).Scan(&count)
		if err != nil {
			return 0, errz.Append(err, errw(tx.Rollback()))
		}

		if count > 0 {
			_, err = tx.ExecContext(ctx, "UPDATE sqlite_sequence SET seq = 0 WHERE name = ?", tbl)
			if err != nil {
				return 0, errz.Append(err, errw(tx.Rollback()))
			}
		}
	}

	return affected, errw(tx.Commit())
}

// ValidateSource implements driver.Driver.
func (d *driveri) ValidateSource(src *source.Source) (*source.Source, error) {
	if src.Type != drivertype.SQLite {
		return nil, errz.Errorf("expected driver type {%s} but got {%s}", drivertype.SQLite, src.Type)
	}
	return src, nil
}

// Ping implements driver.Driver. SQLite does not honor read-only mode, so
// mode is ignored: doOpen always opens with SQLite's create-capable
// default. The practical effect matches DuckDB's ModeReadWrite ping, so
// "sq add" of a new .sqlite file still creates it; there is just no
// read-only variant to distinguish. See driver.Driver.Ping.
func (d *driveri) Ping(ctx context.Context, src *source.Source, _ driver.AccessMode) error {
	db, err := d.doOpen(ctx, src)
	if err != nil {
		return err
	}
	defer lg.WarnIfCloseError(d.log, lgm.CloseDB, db)

	if err = db.PingContext(ctx); err != nil {
		err = errz.Wrapf(err, "ping %s: %s", src.Handle, src.Location)
		lg.FromContext(ctx).Warn("ping failed",
			lga.Src, src,
			lga.Err, err,
		)
	}

	return nil
}

// Dialect implements driver.SQLDriver.
func (d *driveri) Dialect() dialect.Dialect {
	return dialect.Dialect{
		Type:           drivertype.SQLite,
		Placeholders:   placeholders,
		Enquote:        stringz.DoubleQuote,
		MaxBatchValues: 500,
		Ops:            dialect.DefaultOps(),
		ExecModeFor:    dialect.DefaultExecModeFor,
		Joins:          jointype.All(),
		Catalog:        false,
	}
}

func placeholders(numCols, numRows int) string {
	rows := make([]string, numRows)
	for i := 0; i < numRows; i++ {
		rows[i] = "(" + stringz.RepeatJoin("?", numCols, driver.Comma) + ")"
	}
	return strings.Join(rows, driver.Comma)
}

// Renderer implements driver.SQLDriver.
func (d *driveri) Renderer() *render.Renderer {
	r := render.NewDefaultRenderer()
	const schemaFrag = `(SELECT name FROM pragma_database_list ORDER BY seq limit 1)`
	r.FunctionOverrides[ast.FuncNameSchema] = render.FuncOverrideString(schemaFrag)

	// SQLite doesn't support catalogs, so we just return the string "default".
	// We could return empty string, but that may be even more confusing, and would
	// make SQLite the odd man out, as the other SQL drivers (even MySQL)
	// have a value for catalog.
	const catalogFrag = `(SELECT 'default')`
	r.FunctionOverrides[ast.FuncNameCatalog] = render.FuncOverrideString(catalogFrag)

	r.FunctionOverrides[ast.FuncNameContains] = renderFuncContainsInstr
	r.FunctionOverrides[ast.FuncNameStartsWith] = renderFuncStartsWithSubstr
	r.FunctionOverrides[ast.FuncNameEndsWith] = renderFuncEndsWithSubstr
	r.FunctionOverrides[ast.FuncNameIContains] = renderFuncIContainsLike
	r.FunctionOverrides[ast.FuncNameIStartsWith] = renderFuncIStartsWithLike
	r.FunctionOverrides[ast.FuncNameIEndsWith] = renderFuncIEndsWithLike
	// SQLite's default LIKE is ASCII case-insensitive, so like and
	// ilike are structurally identical — both register the same
	// renderer to make the equivalence explicit.
	r.FunctionOverrides[ast.FuncNameLike] = renderFuncLike
	r.FunctionOverrides[ast.FuncNameILike] = renderFuncLike

	return r
}

// CopyTable implements driver.SQLDriver.
//
// The implementation reads the source table's CREATE statement from
// sqlite_master, extracts the table identifier (with byte offsets) via
// the shared sqlparser package, and substitutes the destination name in
// place. Self-referential foreign keys (REFERENCES <src>(...) inside
// the same CREATE TABLE) are also rewritten to point at the destination
// so the destination's FKs resolve against itself rather than the
// source. Cross-table FKs (REFERENCES other(...) where other != src)
// are left untouched.
//
// The source table's companion objects (indexes and triggers, which
// live as separate sqlite_master rows) are also copied (gh758), with
// each companion renamed to "<orig-name>_<dest-table>" because index
// and trigger names are schema-global in SQLite. Companions are created
// after the data copy so that copied triggers don't fire on (or mutate)
// the rows being copied. See copyTableCompanionDDL for the rewrite
// details and limitations.
func (d *driveri) CopyTable(ctx context.Context, db sqlz.DB,
	fromTbl, toTbl tablefq.T, copyData bool,
) (int64, error) {
	// Per https://stackoverflow.com/questions/12730390/copy-table-structure-to-new-table-in-sqlite3
	// It is possible to copy the table structure with a simple statement:
	//  CREATE TABLE copied AS SELECT * FROM mytable WHERE 0
	// However, this does not keep the type information as desired. Thus
	// we need to do something more complicated.

	// First we get the original CREATE TABLE statement.
	masterTbl := tablefq.T{Schema: fromTbl.Schema, Table: "sqlite_master"}
	q := fmt.Sprintf("SELECT sql FROM %s WHERE type='table' AND name=%s",
		masterTbl.Render(stringz.DoubleQuote),
		stringz.SingleQuote(fromTbl.Table))
	var ogTblCreateStmt string
	err := db.QueryRowContext(ctx, q).Scan(&ogTblCreateStmt)
	if err != nil {
		return 0, errw(err)
	}

	// Extract the table identifier (with byte offsets) from the
	// CREATE TABLE statement. Offsets let us splice the new identifier
	// without strings.Replace, which is fragile when the identifier
	// recurs elsewhere in the DDL (CHECK exprs, default literals, etc.).
	ogIdent, err := sqlparser.ExtractTableIdentFromCreateTableStmt(ogTblCreateStmt)
	if err != nil {
		return 0, errz.Wrap(err, "sqlite3: copy table")
	}

	identStart := ogIdent.TableOffset
	if ogIdent.SchemaOffset >= 0 {
		identStart = ogIdent.SchemaOffset
	}
	identEnd := ogIdent.TableOffset + len(ogIdent.RawTable)
	destQuoted := toTbl.Render(stringz.DoubleQuote)

	edits := []sqlparser.Edit{{
		Start:       identStart,
		End:         identEnd,
		Replacement: destQuoted,
	}}

	// Rewrite self-FKs so the destination's REFERENCES point at itself
	// rather than the source (gh759). Cross-table FKs are left alone.
	// SQLite's foreign_table grammar rule is a single any_name (no
	// schema qualification permitted), so the replacement here is the
	// destination's bare table token even when destQuoted carries a
	// "schema"."table" prefix for the CREATE TABLE identifier edit.
	destTableQuoted := stringz.DoubleQuote(toTbl.Table)
	fkRefs, err := sqlparser.ExtractForeignTableRefsFromCreateTableStmt(ogTblCreateStmt)
	if err != nil {
		return 0, errz.Wrap(err, "sqlite3: copy table")
	}
	for _, r := range fkRefs {
		if !strings.EqualFold(r.Table, ogIdent.Table) {
			continue
		}
		edits = append(edits, sqlparser.Edit{
			Start:       r.TableOffset,
			End:         r.TableOffset + len(r.RawTable),
			Replacement: destTableQuoted,
		})
	}

	destTblCreateStmt, err := sqlparser.ApplyEdits(ogTblCreateStmt, edits)
	if err != nil {
		return 0, errz.Wrap(err, "sqlite3: copy table: failed to apply DDL rewrites")
	}

	// Read and rewrite the companion (index and trigger) DDL up front,
	// before any writes, so a rewrite failure aborts the copy cleanly.
	companionStmts, err := copyTableCompanionDDL(ctx, db, fromTbl, toTbl)
	if err != nil {
		return 0, err
	}

	_, err = db.ExecContext(ctx, destTblCreateStmt)
	if err != nil {
		return 0, errw(err)
	}

	var affected int64
	if copyData {
		stmt := fmt.Sprintf("INSERT INTO %s SELECT * FROM %s", toTbl, fromTbl)
		if affected, err = sqlz.ExecAffected(ctx, db, stmt); err != nil {
			return 0, errw(err)
		}
	}

	// Companions are created after the data copy: a copied trigger must
	// not fire on the rows being copied.
	for _, stmt := range companionStmts {
		if _, err = db.ExecContext(ctx, stmt); err != nil {
			return 0, errw(err)
		}
	}

	return affected, nil
}

// copyTableCompanionDDL reads the DDL of fromTbl's companion objects
// (indexes and triggers) from sqlite_master and returns each statement
// rewritten to apply to toTbl. Rows with NULL sql are automatic indexes
// (e.g. backing a UNIQUE constraint or PRIMARY KEY) that the rewritten
// CREATE TABLE already recreates, so the query excludes them.
//
// Index and trigger names are schema-global in SQLite, so each
// companion is renamed by appending the destination table name:
// "<orig-name>_<dest-table>". A trigger's ON <table> target, and any
// table references in its body that name the source table, are
// rewritten to the destination; references to other tables are left
// untouched. See sqlparser.RewriteCreateIndexStmt and
// sqlparser.RewriteCreateTriggerStmt.
func copyTableCompanionDDL(ctx context.Context, db sqlz.DB,
	fromTbl, toTbl tablefq.T,
) ([]string, error) {
	masterTbl := tablefq.T{Schema: fromTbl.Schema, Table: "sqlite_master"}
	q := fmt.Sprintf("SELECT name, type, sql FROM %s WHERE tbl_name = ? "+
		"AND type IN ('index','trigger') AND sql IS NOT NULL ORDER BY name",
		masterTbl.Render(stringz.DoubleQuote))
	rows, err := db.QueryContext(ctx, q, fromTbl.Table)
	if err != nil {
		return nil, errw(err)
	}
	defer sqlz.CloseRows(lg.FromContext(ctx), rows)

	destTblIdent := stringz.DoubleQuote(toTbl.Table)
	var stmts []string
	for rows.Next() {
		var name, typ, ddl string
		if err = rows.Scan(&name, &typ, &ddl); err != nil {
			return nil, errw(err)
		}

		// The companion lives in the destination table's schema.
		newIdent := tablefq.T{Schema: toTbl.Schema, Table: name + "_" + toTbl.Table}.
			Render(stringz.DoubleQuote)

		var rewritten string
		switch typ {
		case "index":
			rewritten, err = sqlparser.RewriteCreateIndexStmt(ddl, newIdent, destTblIdent)
		case "trigger":
			rewritten, err = sqlparser.RewriteCreateTriggerStmt(ddl, newIdent, destTblIdent)
		default:
			// Unreachable given the query predicate.
			continue
		}
		if err != nil {
			return nil, errz.Wrapf(err, "sqlite3: copy table: rewrite %s {%s}", typ, name)
		}
		stmts = append(stmts, rewritten)
	}
	if err = rows.Err(); err != nil {
		return nil, errw(err)
	}
	return stmts, nil
}

// RecordMeta implements driver.SQLDriver.
func (d *driveri) RecordMeta(ctx context.Context, colTypes []*sql.ColumnType) (
	record.Meta, driver.NewRecordFunc, error,
) {
	recMeta, err := recordMetaFromColumnTypes(ctx, colTypes)
	if err != nil {
		return nil, nil, errw(err)
	}

	mungeFn := func(vals []any) (record.Record, error) {
		rec := newRecordFromScanRow(recMeta, vals)
		return rec, nil
	}

	return recMeta, mungeFn, nil
}

// newRecordFromScanRow iterates over the elements of the row slice
// from rows.Scan, and returns a new (record) slice, replacing any
// wrapper types such as sql.NullString with the unboxed value,
// and other similar sanitization. For example, it will
// make a copy of any sql.RawBytes. The row slice
// can be reused by rows.Scan after this function returns.
//
// Note that this function can modify the kind of the record.Meta elements
// if the kind is currently unknown. That is, if meta[0].Kind() returns
// kind.Unknown, but this function detects that row[0] is an *int64, then
// the kind will be set to kind.Int.
//
//nolint:funlen,gocognit,gocyclo,cyclop
func newRecordFromScanRow(meta record.Meta, row []any) (rec record.Record) {
	rec = make([]any, len(row))

	for i := 0; i < len(row); i++ {
		if row[i] == nil {
			rec[i] = nil
			continue
		}

		// Dereference *any before the switch
		col := row[i]
		if ptr, ok := col.(*any); ok {
			col = *ptr
		}

		switch col := col.(type) {
		default:
			// Shouldn't happen
			// TODO: We really should log here
			rec[i] = col
			continue
		case nil:
			rec[i] = nil
		case *int64:
			record.SetKindIfUnknown(meta, i, kind.Int)
			rec[i] = *col
		case int64:
			record.SetKindIfUnknown(meta, i, kind.Int)
			rec[i] = col
		case *float64:
			record.SetKindIfUnknown(meta, i, kind.Float)
			rec[i] = *col
		case float64:
			record.SetKindIfUnknown(meta, i, kind.Float)
			rec[i] = col
		case decimal.Decimal:
			record.SetKindIfUnknown(meta, i, kind.Decimal)
			rec[i] = col
		case *decimal.Decimal:
			record.SetKindIfUnknown(meta, i, kind.Decimal)
			rec[i] = *col

		case *bool:
			record.SetKindIfUnknown(meta, i, kind.Bool)
			rec[i] = *col
		case bool:
			record.SetKindIfUnknown(meta, i, kind.Bool)
			rec[i] = col
		case *string:
			record.SetKindIfUnknown(meta, i, kind.Text)
			rec[i] = *col
		case string:
			record.SetKindIfUnknown(meta, i, kind.Text)
			rec[i] = col
		case *[]byte:
			if col == nil || *col == nil {
				rec[i] = nil
				continue
			}

			if meta[i].Kind() != kind.Bytes {
				// We only want to use []byte for kind.Bytes. Otherwise
				// switch to a string.
				s := string(*col)
				rec[i] = s
				record.SetKindIfUnknown(meta, i, kind.Text)
				continue
			}

			if len(*col) == 0 {
				rec[i] = []byte{}
			} else {
				dest := make([]byte, len(*col))
				copy(dest, *col)
				rec[i] = dest
			}
			record.SetKindIfUnknown(meta, i, kind.Bytes)
		case *sql.NullInt64:
			if col.Valid {
				rec[i] = col.Int64
			} else {
				rec[i] = nil
			}
			record.SetKindIfUnknown(meta, i, kind.Int)
		case *decimal.NullDecimal:
			if col.Valid {
				rec[i] = col.Decimal
			} else {
				rec[i] = nil
			}
		case *sql.NullString:
			if col.Valid {
				rec[i] = col.String
			} else {
				rec[i] = nil
			}
			record.SetKindIfUnknown(meta, i, kind.Text)
		case *sql.RawBytes:
			if col == nil || *col == nil {
				// Explicitly set rec[i] so that its type becomes nil
				rec[i] = nil
				continue
			}

			knd := meta[i].Kind()

			// If RawBytes is of length zero, there's no
			// need to copy.
			if len(*col) == 0 {
				if knd == kind.Bytes {
					rec[i] = []byte{}
				} else {
					// Else treat it as an empty string
					var s string
					rec[i] = s
					record.SetKindIfUnknown(meta, i, kind.Text)
				}

				continue
			}

			dest := make([]byte, len(*col))
			copy(dest, *col)

			if knd == kind.Bytes {
				rec[i] = dest
			} else {
				s := string(dest)
				rec[i] = s
				record.SetKindIfUnknown(meta, i, kind.Text)
			}

		case *sql.NullFloat64:
			if col.Valid {
				rec[i] = col.Float64
			} else {
				rec[i] = nil
			}
			record.SetKindIfUnknown(meta, i, kind.Float)
		case *sql.NullBool:
			if col.Valid {
				rec[i] = col.Bool
			} else {
				rec[i] = nil
			}
			record.SetKindIfUnknown(meta, i, kind.Bool)
		case *sqlz.NullBool:
			// This custom NullBool type is only used by sqlserver at this time.
			// Possibly this code should skip this item, and allow
			// the sqlserver munge func handle the conversion?
			if col.Valid {
				rec[i] = col.Bool
			} else {
				rec[i] = nil
			}
			record.SetKindIfUnknown(meta, i, kind.Bool)
		case *sql.NullTime:
			if col.Valid {
				rec[i] = col.Time
			} else {
				rec[i] = nil
			}
			record.SetKindIfUnknown(meta, i, kind.Datetime)
		case *nullTime:
			// No SetKindIfUnknown call (unlike the sibling cases): a *nullTime
			// dest is only allocated for columns already classified as
			// kind.Datetime/kind.Date, so the kind is never unknown here. The
			// value is the parsed time, or the raw string when the stored text
			// didn't match a known datetime layout.
			switch {
			case !col.Valid:
				rec[i] = nil
			case col.IsTime:
				rec[i] = col.Time
			default:
				rec[i] = col.String
			}
		case *time.Time:
			rec[i] = *col
			record.SetKindIfUnknown(meta, i, kind.Datetime)
		case time.Time:
			rec[i] = col
			record.SetKindIfUnknown(meta, i, kind.Datetime)

		// REVISIT: We probably don't need any of the below cases
		// for sqlite?
		case *int:
			rec[i] = int64(*col)
			record.SetKindIfUnknown(meta, i, kind.Int)

		case int:
			rec[i] = int64(col)
			record.SetKindIfUnknown(meta, i, kind.Int)

		case *int8:
			rec[i] = int64(*col)
			record.SetKindIfUnknown(meta, i, kind.Int)

		case int8:
			rec[i] = int64(col)
			record.SetKindIfUnknown(meta, i, kind.Int)

		case *int16:
			rec[i] = int64(*col)
			record.SetKindIfUnknown(meta, i, kind.Int)

		case int16:
			rec[i] = int64(col)
			record.SetKindIfUnknown(meta, i, kind.Int)

		case *int32:
			rec[i] = int64(*col)
			record.SetKindIfUnknown(meta, i, kind.Int)

		case int32:
			rec[i] = int64(col)
			record.SetKindIfUnknown(meta, i, kind.Int)

		case *uint:
			rec[i] = int64(*col)
			record.SetKindIfUnknown(meta, i, kind.Int)

		case uint:
			rec[i] = int64(col)
			record.SetKindIfUnknown(meta, i, kind.Int)

		case *uint8:
			rec[i] = int64(*col)
			record.SetKindIfUnknown(meta, i, kind.Int)

		case uint8:
			rec[i] = int64(col)
			record.SetKindIfUnknown(meta, i, kind.Int)

		case *uint16:
			rec[i] = int64(*col)
			record.SetKindIfUnknown(meta, i, kind.Int)

		case uint16:
			rec[i] = int64(col)
			record.SetKindIfUnknown(meta, i, kind.Int)

		case *uint32:
			rec[i] = int64(*col)
			record.SetKindIfUnknown(meta, i, kind.Int)

		case uint32:
			rec[i] = int64(col)
			record.SetKindIfUnknown(meta, i, kind.Int)

		case *float32:
			rec[i] = float64(*col)
			record.SetKindIfUnknown(meta, i, kind.Int)

		case float32:
			rec[i] = int64(col)
			record.SetKindIfUnknown(meta, i, kind.Int)
		}
	}

	return rec
}

// DropTable implements driver.SQLDriver.
func (d *driveri) DropTable(ctx context.Context, db sqlz.DB, tbl tablefq.T, ifExists bool) error {
	var stmt string

	if ifExists {
		stmt = fmt.Sprintf("DROP TABLE IF EXISTS %s", tbl)
	} else {
		stmt = fmt.Sprintf("DROP TABLE %s", tbl)
	}

	_, err := db.ExecContext(ctx, stmt)
	return errw(err)
}

// CreateSchema implements driver.SQLDriver. This is implemented for SQLite
// by attaching a new database to db, using ATTACH DATABASE. This attached
// database is only available on the connection on which the ATTACH DATABASE
// command was issued. Thus, db must be a *sql.Conn or *sql.Tx, as
// per sqlz.RequireSingleConn. The same constraint applies to DropSchema.
//
// See: https://www.sqlite.org/lang_attach.html
func (d *driveri) CreateSchema(ctx context.Context, db sqlz.DB, schemaName string) error {
	if err := sqlz.RequireSingleConn(db); err != nil {
		return errz.Wrapf(err, "create schema {%s}: ATTACH DATABASE requires single connection", schemaName)
	}

	// NOTE: Empty string for DATABASE creates a temporary database.
	// We may want to change this to create a more permanent database, perhaps
	// in the same directory as the existing db?
	stmt := `ATTACH DATABASE "" AS ` + stringz.DoubleQuote(schemaName)
	if _, err := db.ExecContext(ctx, stmt); err != nil {
		return errz.Wrapf(err, "create schema {%s}", schemaName)
	}

	return nil
}

// DropSchema implements driver.SQLDriver. As per CreateSchema, db must
// be a *sql.Conn or *sql.Tx.
//
// See https://www.sqlite.org/lang_detach.html
func (d *driveri) DropSchema(ctx context.Context, db sqlz.DB, schemaName string) error {
	if err := sqlz.RequireSingleConn(db); err != nil {
		return errz.Wrapf(err, "drop schema {%s}: DETACH DATABASE requires single connection", schemaName)
	}

	stmt := `DETACH DATABASE ` + stringz.DoubleQuote(schemaName)
	if _, err := db.ExecContext(ctx, stmt); err != nil {
		return errz.Wrapf(err, "drop schema {%s}", schemaName)
	}

	return nil
}

// CreateTable implements driver.SQLDriver.
func (d *driveri) CreateTable(ctx context.Context, db sqlz.DB, tblDef *schema.Table) error {
	query := buildCreateTableStmt(tblDef)

	stmt, err := db.PrepareContext(ctx, query)
	if err != nil {
		return errw(err)
	}

	_, err = stmt.ExecContext(ctx)
	if err != nil {
		lg.WarnIfCloseError(d.log, lgm.CloseDBStmt, stmt)
		return errw(err)
	}

	return errw(stmt.Close())
}

// CurrentSchema implements driver.SQLDriver.
func (d *driveri) CurrentSchema(ctx context.Context, db sqlz.DB) (string, error) {
	const q = `SELECT name FROM pragma_database_list ORDER BY seq limit 1`
	var name string
	if err := db.QueryRowContext(ctx, q).Scan(&name); err != nil {
		return "", errw(err)
	}

	return name, nil
}

// SchemaExists implements driver.SQLDriver.
func (d *driveri) SchemaExists(ctx context.Context, db sqlz.DB, schma string) (bool, error) {
	if schma == "" {
		return false, nil
	}

	const q = `SELECT COUNT(name) FROM pragma_database_list WHERE name = ?`

	var count int
	return count > 0, errw(db.QueryRowContext(ctx, q, schma).Scan(&count))
}

// ListSchemas implements driver.SQLDriver.
func (d *driveri) ListSchemas(ctx context.Context, db sqlz.DB) ([]string, error) {
	log := lg.FromContext(ctx)

	const q = `SELECT name FROM pragma_database_list ORDER BY name`
	var schemas []string
	rows, err := db.QueryContext(ctx, q)
	if err != nil {
		return nil, errz.Err(err)
	}
	defer sqlz.CloseRows(log, rows)

	for rows.Next() {
		var schma string
		if err = rows.Scan(&schma); err != nil {
			return nil, errz.Err(err)
		}
		schemas = append(schemas, schma)
	}

	if err = rows.Err(); err != nil {
		return nil, errz.Err(err)
	}

	return schemas, nil
}

// ListTableNames implements driver.SQLDriver. The returned names exclude
// any sqlite_ internal tables.
func (d *driveri) ListTableNames(ctx context.Context, db sqlz.DB, schma string, tables, views bool) ([]string, error) {
	var tblClause string
	switch {
	case tables && views:
		tblClause = " WHERE (type = 'table' OR type = 'view')"
	case tables:
		tblClause = " WHERE type = 'table'"
	case views:
		tblClause = " WHERE type = 'view'"
	default:
		return []string{}, nil
	}

	tblClause += " AND name NOT LIKE 'sqlite_%'"

	q := "SELECT name FROM "
	if schma == "" {
		q += "sqlite_master"
	} else {
		q += stringz.DoubleQuote(schma) + ".sqlite_master"
	}
	q += tblClause + " ORDER BY name"

	rows, err := db.QueryContext(ctx, q)
	if err != nil {
		return nil, errw(err)
	}

	names, err := sqlz.RowsScanColumn[string](ctx, rows)
	if err != nil {
		return nil, errw(err)
	}

	return names, nil
}

// ListSchemaMetadata implements driver.SQLDriver.
// The returned metadata.Schema instances will have a Catalog
// value of "default", and an empty Owner value.
func (d *driveri) ListSchemaMetadata(ctx context.Context, db sqlz.DB) ([]*metadata.Schema, error) {
	names, err := d.ListSchemas(ctx, db)
	if err != nil {
		return nil, err
	}

	schemas := make([]*metadata.Schema, len(names))
	for i, name := range names {
		schemas[i] = &metadata.Schema{
			Name:    name,
			Catalog: "default",
		}
	}
	return schemas, nil
}

// CatalogExists implements driver.SQLDriver. SQLite does not support catalogs,
// so this method always returns an error.
func (d *driveri) CatalogExists(_ context.Context, _ sqlz.DB, _ string) (bool, error) {
	return false, errz.New("sqlite3: catalog mechanism not supported")
}

// CurrentCatalog implements driver.SQLDriver. SQLite does not support catalogs,
// so this method returns an error.
func (d *driveri) CurrentCatalog(_ context.Context, _ sqlz.DB) (string, error) {
	return "", errz.New("sqlite3: catalog mechanism not supported")
}

// ListCatalogs implements driver.SQLDriver. SQLite does not support catalogs,
// so this method returns an error.
func (d *driveri) ListCatalogs(_ context.Context, _ sqlz.DB) ([]string, error) {
	return nil, errz.New("sqlite3: catalog mechanism not supported")
}

// TableExists implements driver.SQLDriver.
func (d *driveri) TableExists(ctx context.Context, db sqlz.DB, tbl string) (bool, error) {
	const query = `SELECT COUNT(*) FROM sqlite_master WHERE name = ? AND type='table'`

	var count int64
	err := db.QueryRowContext(ctx, query, tbl).Scan(&count)
	if err != nil {
		return false, errw(err)
	}

	return count == 1, nil
}

// PrepareInsertStmt implements driver.SQLDriver.
func (d *driveri) PrepareInsertStmt(ctx context.Context, db sqlz.DB, destTbl string, destColNames []string,
	numRows int,
) (*driver.StmtExecer, error) {
	destColsMeta, err := d.getTableRecordMeta(ctx, db, destTbl, destColNames)
	if err != nil {
		return nil, err
	}

	stmt, err := driver.PrepareInsertStmt(ctx, d, db, destTbl, destColsMeta.Names(), numRows)
	if err != nil {
		return nil, err
	}

	execer := driver.NewStmtExecer(stmt, driver.DefaultInsertMungeFunc(destTbl, destColsMeta),
		newStmtExecFunc(stmt), destColsMeta)
	return execer, nil
}

// NewBatchInsert implements driver.SQLDriver.
func (d *driveri) NewBatchInsert(ctx context.Context, msg string, db sqlz.DB,
	_ *source.Source, destTbl string, destColNames []string,
) (*driver.BatchInsert, error) {
	return driver.DefaultNewBatchInsert(ctx, msg, d, db, destTbl, destColNames)
}

// PrepareUpdateStmt implements driver.SQLDriver.
func (d *driveri) PrepareUpdateStmt(ctx context.Context, db sqlz.DB, destTbl string,
	destColNames []string, where string,
) (*driver.StmtExecer, error) {
	destColsMeta, err := d.getTableRecordMeta(ctx, db, destTbl, destColNames)
	if err != nil {
		return nil, err
	}

	query, err := buildUpdateStmt(destTbl, destColNames, where)
	if err != nil {
		return nil, err
	}

	stmt, err := db.PrepareContext(ctx, query)
	if err != nil {
		return nil, err
	}

	execer := driver.NewStmtExecer(stmt, driver.DefaultInsertMungeFunc(destTbl, destColsMeta),
		newStmtExecFunc(stmt), destColsMeta)
	return execer, nil
}

func newStmtExecFunc(stmt *sql.Stmt) driver.StmtExecFunc {
	return func(ctx context.Context, args ...any) (int64, error) {
		res, err := stmt.ExecContext(ctx, args...)
		if err != nil {
			return 0, errw(err)
		}
		affected, err := res.RowsAffected()
		return affected, errw(err)
	}
}

// TableColumnTypes implements driver.SQLDriver.
func (d *driveri) TableColumnTypes(ctx context.Context, db sqlz.DB, tblName string,
	colNames []string,
) ([]*sql.ColumnType, error) {
	// Given the dynamic behavior of sqlite's rows.ColumnTypes,
	// this query selects a single row, as that'll give us more
	// accurate column type info than no rows. For other db
	// impls, LIMIT can be 0.
	const queryTpl = "SELECT %s FROM %s LIMIT 1"

	enquote := d.Dialect().Enquote
	tblNameQuoted := enquote(tblName)

	colsClause := "*"
	if len(colNames) > 0 {
		colNamesQuoted := langz.Apply(colNames, enquote)
		colsClause = strings.Join(colNamesQuoted, driver.Comma)
	}

	query := fmt.Sprintf(queryTpl, colsClause, tblNameQuoted)
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, errw(err)
	}

	// We invoke rows.ColumnTypes twice.
	// The first time is to cover the scenario where the table
	// is empty (no rows), so that we at least get some
	// column type info.
	colTypes, err := rows.ColumnTypes()
	if err != nil {
		sqlz.CloseRows(d.log, rows)
		return nil, errw(err)
	}

	// If the table does have rows, we invoke rows.ColumnTypes again,
	// as on this invocation the column type info will be more
	// accurate (col nullability will be reported etc).
	if rows.Next() {
		colTypes, err = rows.ColumnTypes()
		if err != nil {
			sqlz.CloseRows(d.log, rows)
			return nil, errw(err)
		}
	}

	err = rows.Err()
	if err != nil {
		sqlz.CloseRows(d.log, rows)
		return nil, errw(err)
	}

	err = rows.Close()
	if err != nil {
		return nil, errw(err)
	}

	return colTypes, nil
}

func (d *driveri) getTableRecordMeta(ctx context.Context, db sqlz.DB, tblName string, colNames []string) (
	record.Meta, error,
) {
	colTypes, err := d.TableColumnTypes(ctx, db, tblName, colNames)
	if err != nil {
		return nil, err
	}

	destCols, _, err := d.RecordMeta(ctx, colTypes)
	if err != nil {
		return nil, err
	}

	return destCols, nil
}

var _ driver.ScratchSrcFunc = NewScratchSource

// NewScratchSource returns a new scratch src. The supplied fpath
// must be the absolute path to the location to create the SQLite DB file,
// typically in the user cache dir.
// The returned clnup func will delete the dB file.
func NewScratchSource(ctx context.Context, fpath string) (src *source.Source, clnup func() error, err error) {
	log := lg.FromContext(ctx)
	src = &source.Source{
		Type:     drivertype.SQLite,
		Handle:   source.ScratchHandle,
		Location: Prefix + fpath,
		// The path is an internally constructed literal, not a
		// placeholder template: mark it so resolution is a no-op.
		SecretsResolved: true,
	}

	clnup = func() error {
		if journal := filepath.Join(fpath, ".db-journal"); ioz.FileAccessible(journal) {
			lg.WarnIfError(log, "Delete sqlite3 db journal file", os.Remove(journal))
		}

		log.Debug("Delete sqlite3 scratchdb file", lga.Src, src, lga.Path, fpath)
		if err := os.Remove(fpath); err != nil {
			log.Warn("Delete sqlite3 scratchdb file", lga.Err, err)
			return errz.Err(err)
		}

		return nil
	}

	return src, clnup, nil
}

// PathFromLocation returns the absolute file path from the source location,
// which must have the "sqlite3://" prefix. Any "?key=val&..." connection-string
// suffix is stripped; callers that need the full DSN should use
// dsnFromLocation instead.
func PathFromLocation(src *source.Source) (string, error) {
	if src.Type != drivertype.SQLite {
		return "", errz.Errorf("driver {%s} does not support {%s}", drivertype.SQLite, src.Type)
	}

	if !strings.HasPrefix(src.Location, Prefix) {
		return "", errz.Errorf("sqlite3 source location must begin with {%s} but was: %s", Prefix, src.RedactedLocation())
	}

	loc := filePathFromLocation(src.Location)
	if len(loc) < 2 {
		return "", errz.Errorf("sqlite3 source location is too short: %s", src.RedactedLocation())
	}

	return filepath.Clean(loc), nil
}

// dsnFromLocation converts an sq location string
// ("sqlite3:///path/to/foo.db?mode=ro") into the DSN form expected by
// mattn/go-sqlite3. Requires the "sqlite3://" prefix; preserves the rest
// verbatim (including any "?key=val&..." query suffix). mattn/go-sqlite3
// parses the query suffix itself; see
// https://github.com/mattn/go-sqlite3#connection-string.
func dsnFromLocation(loc string) (string, error) {
	if !strings.HasPrefix(loc, Prefix) {
		// Don't echo loc: it may carry secret connection params
		// (e.g. _auth_pass) in a "?key=val" suffix.
		return "", errz.Errorf("invalid sqlite3 location: missing %q prefix", Prefix)
	}
	return loc[len(Prefix):], nil
}

// filePathFromLocation returns the on-disk file path for a sqlite3
// location, or "" for a malformed location or one whose path component
// is empty. Any "?key=val&..." DSN query suffix is stripped. The
// returned path is neither normalized nor absolutized; callers that
// need normalization should run it through filepath.Clean, and callers
// that need an absolute path should use filepath.Abs.
func filePathFromLocation(loc string) string {
	if !strings.HasPrefix(loc, Prefix) {
		return ""
	}
	p := loc[len(Prefix):]
	if i := strings.IndexByte(p, '?'); i >= 0 {
		p = p[:i]
	}
	return p
}

// MungeLocation takes a location argument (as received from the user)
// and builds a sqlite3 location URL. Each of these forms are allowed:
//
//	sqlite3:///path/to/sakila.db	--> sqlite3:///path/to/sakila.db
//	sqlite3:sakila.db 						--> sqlite3:///current/working/dir/sakila.db
//	sqlite3:/sakila.db 						--> sqlite3:///sakila.db
//	sqlite3:./sakila.db 					--> sqlite3:///current/working/dir/sakila.db
//	sqlite3:sakila.db 						--> sqlite3:///current/working/dir/sakila.db
//	sakila.db											--> sqlite3:///current/working/dir/sakila.db
//	/path/to/sakila.db						--> sqlite3:///path/to/sakila.db
//
// An optional "?key=val[&...]" connection-string suffix is preserved
// verbatim. The first '?' is always treated as the path/query separator,
// so paths whose POSIX filename legally contains '?' are not supported.
//
// The final form is particularly nice for shell completion etc.
//
// Note that this function is OS-dependent, due to the use of pkg filepath.
// Thus, on Windows, this is seen:
//
//	C:/Users/sq/sakila.db 				--> sqlite3://C:/Users/sq/sakila.db
//
// But that input location gets mangled on non-Windows OSes. This probably
// isn't a problem in practice, but longer-term it may make sense to rewrite
// MungeLocation to be OS-independent.
//
// MungeLocation is idempotent, and is a thin wrapper around
// location.MungeTemplateForDriver: the user-typed location is a
// placeholder template, so cwd bytes spliced in by absolutization are
// escaped (gh #797). Locations resolved from secret placeholders at
// connect time are literal bytes and take the no-escape path via
// location.MungeForDriver (driver.ResolveSourceSecrets).
func MungeLocation(loc string) (string, error) {
	return location.MungeTemplateForDriver(drivertype.SQLite, loc)
}
