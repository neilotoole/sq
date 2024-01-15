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

	"github.com/neilotoole/sq/drivers/sqlite3/internal/sqlparser"
	"github.com/neilotoole/sq/libsq/ast"
	"github.com/neilotoole/sq/libsq/ast/render"
	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/ioz"
	"github.com/neilotoole/sq/libsq/core/jointype"
	"github.com/neilotoole/sq/libsq/core/kind"
	"github.com/neilotoole/sq/libsq/core/lg"
	"github.com/neilotoole/sq/libsq/core/lg/lga"
	"github.com/neilotoole/sq/libsq/core/lg/lgm"
	"github.com/neilotoole/sq/libsq/core/loz"
	"github.com/neilotoole/sq/libsq/core/record"
	"github.com/neilotoole/sq/libsq/core/sqlmodel"
	"github.com/neilotoole/sq/libsq/core/sqlz"
	"github.com/neilotoole/sq/libsq/core/stringz"
	"github.com/neilotoole/sq/libsq/core/tablefq"
	"github.com/neilotoole/sq/libsq/driver"
	"github.com/neilotoole/sq/libsq/driver/dialect"
	"github.com/neilotoole/sq/libsq/source"
	"github.com/neilotoole/sq/libsq/source/drivertype"
	"github.com/neilotoole/sq/libsq/source/metadata"
)

const (
	// Type is the sqlite3 source driver type.
	Type drivertype.Type = "sqlite3"

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
	if typ != Type {
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
		Type:        Type,
		Description: "SQLite",
		Doc:         "https://github.com/mattn/go-sqlite3",
		IsSQL:       true,
	}
}

// Open implements driver.Driver.
func (d *driveri) Open(ctx context.Context, src *source.Source) (driver.Grip, error) {
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
	fp, err := PathFromLocation(src)
	if err != nil {
		return nil, err
	}

	db, err := sql.Open(dbDrvr, fp)
	if err != nil {
		return nil, errz.Wrapf(errw(err), "failed to open sqlite3 source with DSN: %s", fp)
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

	affected, err = sqlz.ExecAffected(ctx, tx, fmt.Sprintf("DELETE FROM %q", tbl))
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
	if src.Type != Type {
		return nil, errz.Errorf("expected driver type {%s} but got {%s}", Type, src.Type)
	}
	return src, nil
}

// Ping implements driver.Driver.
func (d *driveri) Ping(ctx context.Context, src *source.Source) error {
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
		Type:           Type,
		Placeholders:   placeholders,
		Enquote:        stringz.DoubleQuote,
		MaxBatchValues: 500,
		Ops:            dialect.DefaultOps(),
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

	return r
}

// CopyTable implements driver.SQLDriver.
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

	// Next, we extract the table identifier from the CREATE TABLE statement.
	// For example, "main"."actor". Note that the schema part may be empty.
	ogSchema, ogTbl, err := sqlparser.ExtractTableIdentFromCreateTableStmt(ogTblCreateStmt,
		false)
	if err != nil {
		return 0, errw(err)
	}

	// Now we know what text to replace in ogTblCreateStmt.
	replaceTarget := ogTbl
	if ogSchema != "" {
		replaceTarget = ogSchema + "." + ogTbl
	}

	// Replace the old table identifier with the new one, and, voila,
	// we have our new CREATE TABLE statement.
	destTblCreateStmt := strings.Replace(
		ogTblCreateStmt,
		replaceTarget,
		toTbl.Render(stringz.DoubleQuote),
		1,
	)

	_, err = db.ExecContext(ctx, destTblCreateStmt)
	if err != nil {
		return 0, errw(err)
	}

	if !copyData {
		return 0, nil
	}

	stmt := fmt.Sprintf("INSERT INTO %s SELECT * FROM %s", toTbl, fromTbl)
	affected, err := sqlz.ExecAffected(ctx, db, stmt)
	if err != nil {
		return 0, errw(err)
	}

	return affected, nil
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
func (d *driveri) CreateTable(ctx context.Context, db sqlz.DB, tblDef *sqlmodel.TableDef) error {
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

// ListSchemas implements driver.SQLDriver.
func (d *driveri) ListSchemas(ctx context.Context, db sqlz.DB) ([]string, error) {
	log := lg.FromContext(ctx)

	const q = `SELECT name FROM pragma_database_list ORDER BY name`
	var schemas []string
	rows, err := db.QueryContext(ctx, q)
	if err != nil {
		return nil, errz.Err(err)
	}
	defer lg.WarnIfCloseError(log, lgm.CloseDBRows, rows)

	for rows.Next() {
		var schema string
		if err = rows.Scan(&schema); err != nil {
			return nil, errz.Err(err)
		}
		schemas = append(schemas, schema)
	}

	if err = rows.Err(); err != nil {
		return nil, errz.Err(err)
	}

	return schemas, nil
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
		colNamesQuoted := loz.Apply(colNames, enquote)
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
		lg.WarnIfFuncError(d.log, lgm.CloseDBRows, rows.Close)
		return nil, errw(err)
	}

	// If the table does have rows, we invoke rows.ColumnTypes again,
	// as on this invocation the column type info will be more
	// accurate (col nullability will be reported etc).
	if rows.Next() {
		colTypes, err = rows.ColumnTypes()
		if err != nil {
			lg.WarnIfFuncError(d.log, lgm.CloseDBRows, rows.Close)
			return nil, errw(err)
		}
	}

	err = rows.Err()
	if err != nil {
		lg.WarnIfFuncError(d.log, lgm.CloseDBRows, rows.Close)
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
		Type:     Type,
		Handle:   source.ScratchHandle,
		Location: Prefix + fpath,
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
// which should have the "sqlite3://" prefix.
func PathFromLocation(src *source.Source) (string, error) {
	// FIXME: Does this actually work with query params in the path?
	// Probably not? Maybe refactor use dburl.Parse or such.
	if src.Type != Type {
		return "", errz.Errorf("driver {%s} does not support {%s}", Type, src.Type)
	}

	if !strings.HasPrefix(src.Location, Prefix) {
		return "", errz.Errorf("sqlite3 source location must begin with {%s} but was: %s", Prefix, src.RedactedLocation())
	}

	loc := strings.TrimPrefix(src.Location, Prefix)
	if len(loc) < 2 {
		return "", errz.Errorf("sqlite3 source location is too short: %s", src.RedactedLocation())
	}

	loc = filepath.Clean(loc)
	return loc, nil
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
func MungeLocation(loc string) (string, error) {
	loc2 := strings.TrimSpace(loc)
	if loc2 == "" {
		return "", errz.New("location must not be empty")
	}

	loc2 = strings.TrimPrefix(loc2, "sqlite3://")
	loc2 = strings.TrimPrefix(loc2, "sqlite3:")

	fp, err := filepath.Abs(loc2)
	if err != nil {
		return "", errz.Wrapf(errw(err), "invalid location: %s", loc)
	}

	fp = filepath.ToSlash(fp)
	return "sqlite3://" + fp, nil
}
