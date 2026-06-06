// Package rqlite implements the sq driver for rqlite, the distributed
// SQLite-based database (see https://rqlite.io). The backing SQL driver
// is github.com/rqlite/gorqlite's stdlib (database/sql) adapter.
//
// Unlike sq's sqlite3 driver, rqlite is networked: there is no file
// mode. The source location is an HTTP URL using one of two schemes,
//
//	rqlite://user:pass@host:4001    (HTTP)
//	rqlites://user:pass@host:4001   (HTTPS)
//
// which are translated to gorqlite's expected http(s):// URLs at Open
// time.
//
// rqlite's HTTP API does not support interactive transactions, only
// atomic batches via /db/execute. gorqlite's database/sql driver
// surfaces Begin/Commit/Rollback as no-ops, which means sq code paths
// that rely on a real Tx (notably Truncate and NewBatchInsert) need
// rqlite-specific replacements rather than reusing the standard
// Tx-wrapped helpers.
package rqlite

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"net/url"
	"strings"

	"github.com/rqlite/gorqlite"
	_ "github.com/rqlite/gorqlite/stdlib" // Import for side effect of registering the "rqlite" sql driver.

	"github.com/neilotoole/sq/libsq/ast"
	"github.com/neilotoole/sq/libsq/ast/render"
	"github.com/neilotoole/sq/libsq/core/errz"
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
	"github.com/neilotoole/sq/libsq/source/metadata"
)

const (
	// dbDrvr is the name registered by gorqlite/stdlib via
	// sql.Register("rqlite", ...). It is intentionally identical to
	// drivertype.Rqlite's string value.
	dbDrvr = "rqlite"

	// Prefix is the scheme+separator for plain-HTTP rqlite sources.
	Prefix = "rqlite://"

	// PrefixSecure is the scheme+separator for HTTPS rqlite sources.
	PrefixSecure = "rqlites://"

	defaultPort = 4001
)

var _ driver.Provider = (*Provider)(nil)

// Provider is the rqlite implementation of driver.Provider.
type Provider struct {
	Log *slog.Logger
}

// DriverFor implements driver.Provider.
func (p *Provider) DriverFor(typ drivertype.Type) (driver.Driver, error) {
	if typ != drivertype.Rqlite {
		return nil, errz.Errorf("unsupported driver type {%s}", typ)
	}

	return &driveri{log: p.Log}, nil
}

var _ driver.SQLDriver = (*driveri)(nil)

// driveri is the rqlite implementation of driver.SQLDriver.
type driveri struct {
	log *slog.Logger
}

// ConnParams implements driver.SQLDriver. The values come from rqlite's
// connection URL grammar (see gorqlite/conn.go) and the small set of
// rqlite features sq actively cares about.
func (d *driveri) ConnParams() map[string][]string {
	return map[string][]string{
		"level":                   {"none", "weak", "linearizable", "strong"},
		"disableClusterDiscovery": {"true", "false"},
	}
}

// ErrWrapFunc implements driver.SQLDriver.
func (d *driveri) ErrWrapFunc() func(error) error {
	return errw
}

// DBProperties implements driver.SQLDriver. rqlite v10 surfaces a
// SQLite version and a small cluster-level status payload over its
// HTTP API rather than via pragmas, so this implementation returns
// just the SQLite version. Richer cluster status (leader address,
// node count) lands in a follow-up.
func (d *driveri) DBProperties(ctx context.Context, db sqlz.DB) (map[string]any, error) {
	const q = `SELECT sqlite_version()`
	var v string
	if err := db.QueryRowContext(ctx, q).Scan(&v); err != nil {
		return nil, errw(err)
	}
	return map[string]any{"sqlite_version": v}, nil
}

// DriverMetadata implements driver.Driver.
func (d *driveri) DriverMetadata() driver.Metadata {
	return driver.Metadata{
		Type:        drivertype.Rqlite,
		Description: "rqlite",
		Doc:         "https://rqlite.io",
		IsSQL:       true,
		DefaultPort: defaultPort,
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
	dsn, err := dsnFromLocation(src.Location)
	if err != nil {
		return nil, err
	}

	db, err := sql.Open(dbDrvr, dsn)
	if err != nil {
		// Don't include dsn in the error: it may carry credentials.
		return nil, errz.Wrapf(errw(err), "failed to open rqlite source %s", src.Handle)
	}

	driver.ConfigureDB(ctx, db, src.Options)
	return db, nil
}

// Truncate implements driver.Driver.
//
// rqlite has no interactive transactions. We emit DELETE FROM tbl
// through the standard database/sql path; one HTTP call, atomic
// at the rqlite layer for that single statement. When reset is true
// and the sqlite_sequence table exists, we follow with a separate
// UPDATE to clear the AUTOINCREMENT counter. The DELETE and the
// counter UPDATE are NOT atomic relative to each other; this is
// intentional. The simpler non-batch path reports the deleted-row
// count accurately, and atomic DELETE+UPDATE for Truncate is reserved
// for the cases where it materially matters (CopyTable,
// AlterTableColumnKinds).
func (d *driveri) Truncate(ctx context.Context, src *source.Source, tbl string, reset bool) (int64, error) {
	db, err := d.doOpen(ctx, src)
	if err != nil {
		return 0, errw(err)
	}
	defer lg.WarnIfFuncError(d.log, lgm.CloseDB, db.Close)

	affected, err := sqlz.ExecAffected(ctx, db, fmt.Sprintf("DELETE FROM %q", tbl))
	if err != nil {
		return affected, errw(err)
	}

	if reset {
		const seqProbe = `SELECT COUNT(name) FROM sqlite_master WHERE type='table' AND name='sqlite_sequence'`
		var seqCount int64
		if err = db.QueryRowContext(ctx, seqProbe).Scan(&seqCount); err != nil {
			return affected, errw(err)
		}
		if seqCount > 0 {
			if _, err = db.ExecContext(ctx,
				"UPDATE sqlite_sequence SET seq = 0 WHERE name = ?", tbl); err != nil {
				return affected, errw(err)
			}
		}
	}

	return affected, nil
}

// ValidateSource implements driver.Driver.
func (d *driveri) ValidateSource(src *source.Source) (*source.Source, error) {
	if src.Type != drivertype.Rqlite {
		return nil, errz.Errorf("expected driver type {%s} but got {%s}", drivertype.Rqlite, src.Type)
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
		return errz.Wrapf(errw(err), "ping %s: %s", src.Handle, src.Location)
	}

	return nil
}

// Dialect implements driver.SQLDriver. rqlite speaks SQLite SQL, so the
// dialect mirrors the sqlite3 driver's.
func (d *driveri) Dialect() dialect.Dialect {
	return dialect.Dialect{
		Type:           drivertype.Rqlite,
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

// Renderer implements driver.SQLDriver. The SLQ function overrides
// mirror the sqlite3 driver's since rqlite executes SQLite SQL.
func (d *driveri) Renderer() *render.Renderer {
	r := render.NewDefaultRenderer()

	// rqlite/SQLite has no real concept of schemas or catalogs; surface
	// the same conventional values the sqlite3 driver uses so the
	// rendered SQL doesn't fail on `schema()` / `catalog()`.
	const schemaFrag = `(SELECT name FROM pragma_database_list ORDER BY seq limit 1)`
	r.FunctionOverrides[ast.FuncNameSchema] = render.FuncOverrideString(schemaFrag)
	const catalogFrag = `(SELECT 'default')`
	r.FunctionOverrides[ast.FuncNameCatalog] = render.FuncOverrideString(catalogFrag)

	r.FunctionOverrides[ast.FuncNameContains] = renderFuncContainsInstr
	r.FunctionOverrides[ast.FuncNameStartsWith] = renderFuncStartsWithSubstr
	r.FunctionOverrides[ast.FuncNameEndsWith] = renderFuncEndsWithSubstr
	r.FunctionOverrides[ast.FuncNameIContains] = renderFuncIContainsLike
	r.FunctionOverrides[ast.FuncNameIStartsWith] = renderFuncIStartsWithLike
	r.FunctionOverrides[ast.FuncNameIEndsWith] = renderFuncIEndsWithLike
	// SQLite's default LIKE is ASCII case-insensitive, so like and ilike
	// register the same renderer.
	r.FunctionOverrides[ast.FuncNameLike] = renderFuncLike
	r.FunctionOverrides[ast.FuncNameILike] = renderFuncLike

	return r
}

// dsnFromLocation translates an rqlite:// or rqlites:// source location
// into the http(s):// URL that gorqlite.Open expects.
func dsnFromLocation(loc string) (string, error) {
	var scheme string
	switch {
	case strings.HasPrefix(loc, PrefixSecure):
		scheme = "https"
	case strings.HasPrefix(loc, Prefix):
		scheme = "http"
	default:
		return "", errz.Errorf("rqlite: location must start with %q or %q, got: %s",
			Prefix, PrefixSecure, loc)
	}

	u, err := url.Parse(loc)
	if err != nil {
		return "", errz.Wrapf(err, "rqlite: invalid location: %s", loc)
	}

	u.Scheme = scheme
	return u.String(), nil
}

// CopyTable implements driver.SQLDriver.
//
// rqlite has no interactive transactions, so we use the introspect
// and rebuild strategy: read the source table's metadata, convert
// to a *schema.Table, and emit a fresh CREATE TABLE via
// buildCreateTableStmt. When copyData=true, the CREATE and the
// INSERT-FROM-SELECT are sent as one atomic batch via writeAtomic.
//
// This is a lossy copy. UNIQUE, FOREIGN KEY, AUTOINCREMENT, CHECK,
// indexes, triggers, and the original DEFAULT values are NOT
// preserved. Only Name, Kind, NotNull, PRIMARY KEY (single column),
// and the presence (not value) of column defaults are carried over.
// For faithful CREATE TABLE preservation, use rqlite's sqlite_master
// text directly via a SQL parser (deferred, see follow-up).
func (d *driveri) CopyTable(ctx context.Context, db sqlz.DB,
	fromTbl, toTbl tablefq.T, copyData bool,
) (int64, error) {
	srcMd, err := getTableMetadata(ctx, db, fromTbl.Table)
	if err != nil {
		return 0, errw(err)
	}

	dstTblDef := tableMetadataToSchema(srcMd, toTbl.Table)
	createStmt := buildCreateTableStmt(dstTblDef)

	if !copyData {
		if _, err = db.ExecContext(ctx, createStmt); err != nil {
			return 0, errw(err)
		}
		return 0, nil
	}

	// Atomic CREATE + INSERT-FROM-SELECT.
	stmts := []gorqlite.ParameterizedStatement{
		{Query: createStmt},
		{Query: fmt.Sprintf(`INSERT INTO %q SELECT * FROM %q`, toTbl.Table, fromTbl.Table)},
	}
	results, err := writeAtomic(ctx, db, stmts...)
	if err != nil {
		return 0, err
	}
	// rqlite reports rows_affected as changes() at result time, which
	// is stale for the CREATE statement. The INSERT-SELECT at
	// results[1] carries the real inserted-row count.
	return results[1].RowsAffected, nil
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
		return newRecordFromScanRow(recMeta, vals), nil
	}

	return recMeta, mungeFn, nil
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

// CreateSchema implements driver.SQLDriver.
func (d *driveri) CreateSchema(_ context.Context, _ sqlz.DB, _ string) error {
	return errz.New("rqlite: CREATE SCHEMA is not supported (SQLite has no schemas)")
}

// DropSchema implements driver.SQLDriver.
func (d *driveri) DropSchema(_ context.Context, _ sqlz.DB, _ string) error {
	return errz.New("rqlite: DROP SCHEMA is not supported (SQLite has no schemas)")
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
	const q = `SELECT name FROM pragma_database_list ORDER BY seq LIMIT 1`
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
	const q = `SELECT name FROM pragma_database_list ORDER BY name`
	rows, err := db.QueryContext(ctx, q)
	if err != nil {
		return nil, errw(err)
	}
	defer sqlz.CloseRows(d.log, rows)

	var schemas []string
	for rows.Next() {
		var schma string
		if err = rows.Scan(&schma); err != nil {
			return nil, errw(err)
		}
		schemas = append(schemas, schma)
	}
	if err = rows.Err(); err != nil {
		return nil, errw(err)
	}
	return schemas, nil
}

// ListTableNames implements driver.SQLDriver. System tables
// (sqlite_master, sqlite_sequence, etc.) are filtered out.
func (d *driveri) ListTableNames(ctx context.Context, db sqlz.DB, schma string,
	tables, views bool,
) ([]string, error) {
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
	return names, errw(err)
}

// ListSchemaMetadata implements driver.SQLDriver. The returned schemas
// carry the conventional catalog value "default".
func (d *driveri) ListSchemaMetadata(ctx context.Context, db sqlz.DB) ([]*metadata.Schema, error) {
	names, err := d.ListSchemas(ctx, db)
	if err != nil {
		return nil, err
	}
	schemas := make([]*metadata.Schema, len(names))
	for i, name := range names {
		schemas[i] = &metadata.Schema{Name: name, Catalog: "default"}
	}
	return schemas, nil
}

// CatalogExists implements driver.SQLDriver.
func (d *driveri) CatalogExists(_ context.Context, _ sqlz.DB, _ string) (bool, error) {
	return false, nil
}

// CurrentCatalog implements driver.SQLDriver.
func (d *driveri) CurrentCatalog(_ context.Context, _ sqlz.DB) (string, error) {
	return "", errz.New("rqlite: catalogs are not supported (SQLite has no catalogs)")
}

// ListCatalogs implements driver.SQLDriver.
func (d *driveri) ListCatalogs(_ context.Context, _ sqlz.DB) ([]string, error) {
	return nil, errz.New("rqlite: catalogs are not supported (SQLite has no catalogs)")
}

// TableExists implements driver.SQLDriver.
func (d *driveri) TableExists(ctx context.Context, db sqlz.DB, tbl string) (bool, error) {
	const query = `SELECT COUNT(*) FROM sqlite_master WHERE name = ? AND type='table'`
	var count int64
	if err := db.QueryRowContext(ctx, query, tbl).Scan(&count); err != nil {
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

// newStmtExecFunc adapts a *sql.Stmt to driver.StmtExecFunc.
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

// NewBatchInsert implements driver.SQLDriver. Each batch is one
// multi-row INSERT statement, which gorqlite stdlib sends as one
// HTTP POST to /db/execute, atomic per batch at the rqlite layer.
// No rqlite-specific batching machinery is needed; we delegate to
// the standard framework.
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
		return nil, errw(err)
	}

	execer := driver.NewStmtExecer(stmt, driver.DefaultInsertMungeFunc(destTbl, destColsMeta),
		newStmtExecFunc(stmt), destColsMeta)
	return execer, nil
}

// TableColumnTypes implements driver.SQLDriver. The implementation
// mirrors the sqlite3 driver: SELECT a single row from the table so
// rows.ColumnTypes returns richer info than it would for an empty
// result set. When the table is empty we fall back to the
// no-rows ColumnTypes call.
func (d *driveri) TableColumnTypes(ctx context.Context, db sqlz.DB, tblName string,
	colNames []string,
) ([]*sql.ColumnType, error) {
	const queryTpl = "SELECT %s FROM %s LIMIT 1"

	enquote := d.Dialect().Enquote
	tblNameQuoted := enquote(tblName)

	colsClause := "*"
	if len(colNames) > 0 {
		colsClause = strings.Join(langz.Apply(colNames, enquote), driver.Comma)
	}

	query := fmt.Sprintf(queryTpl, colsClause, tblNameQuoted)
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, errw(err)
	}

	colTypes, err := rows.ColumnTypes()
	if err != nil {
		sqlz.CloseRows(d.log, rows)
		return nil, errw(err)
	}

	if rows.Next() {
		colTypes, err = rows.ColumnTypes()
		if err != nil {
			sqlz.CloseRows(d.log, rows)
			return nil, errw(err)
		}
	}

	if err = rows.Err(); err != nil {
		sqlz.CloseRows(d.log, rows)
		return nil, errw(err)
	}

	if err = rows.Close(); err != nil {
		return nil, errw(err)
	}

	return colTypes, nil
}

// getTableRecordMeta returns the record.Meta for the named columns of
// tblName. If colNames is empty, all columns are returned.
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

// AlterTableRename implements driver.SQLDriver.
func (d *driveri) AlterTableRename(ctx context.Context, db sqlz.DB, tbl, newName string) error {
	q := fmt.Sprintf(`ALTER TABLE %q RENAME TO %q`, tbl, newName)
	_, err := db.ExecContext(ctx, q)
	return errz.Wrapf(errw(err), "rqlite: alter table: failed to rename table {%s} to {%s}", tbl, newName)
}

// AlterTableAddColumn implements driver.SQLDriver.
func (d *driveri) AlterTableAddColumn(ctx context.Context, db sqlz.DB, tbl, col string, knd kind.Kind) error {
	q := fmt.Sprintf("ALTER TABLE %q ADD COLUMN %q %s", tbl, col, DBTypeForKind(knd))
	_, err := db.ExecContext(ctx, q)
	if err != nil {
		return errz.Wrapf(errw(err), "rqlite: alter table: failed to add column {%s} to table {%s}", col, tbl)
	}
	return nil
}

// AlterTableRenameColumn implements driver.SQLDriver.
func (d *driveri) AlterTableRenameColumn(ctx context.Context, db sqlz.DB, tbl, col, newName string) error {
	q := fmt.Sprintf("ALTER TABLE %q RENAME COLUMN %q TO %q", tbl, col, newName)
	_, err := db.ExecContext(ctx, q)
	return errz.Wrapf(errw(err), "rqlite: alter table: failed to rename column {%s.%s} to {%s}", tbl, col, newName)
}

// AlterTableColumnKinds implements driver.SQLDriver.
//
// SQLite has no ALTER COLUMN TYPE, so we rebuild the table via an
// atomic batch:
//
//	PRAGMA foreign_keys=off
//	CREATE TABLE <tmp> (... new kinds ...)
//	INSERT INTO <tmp> SELECT * FROM <original>
//	DROP TABLE <original>
//	ALTER TABLE <tmp> RENAME TO <original>
//	PRAGMA foreign_keys=on
//
// All six statements ride one /db/execute HTTP call and are atomic
// at rqlite.
//
// Schema is reconstructed from *metadata.Table (the source of truth
// for what sq tracks). This is a lossy rebuild. UNIQUE, FOREIGN KEY,
// AUTOINCREMENT, CHECK, indexes, triggers, and the original DEFAULT
// values are NOT preserved. Only Name, Kind, NotNull, PRIMARY KEY
// (single column), and the presence (not value) of column defaults
// are carried over. For faithful CREATE TABLE preservation, use
// rqlite's sqlite_master text directly via a SQL parser (deferred,
// see follow-up).
func (d *driveri) AlterTableColumnKinds(ctx context.Context, db sqlz.DB,
	tbl string, colNames []string, kinds []kind.Kind,
) error {
	if len(colNames) != len(kinds) {
		return errz.New("rqlite: alter table: mismatched count of columns and kinds")
	}

	srcMd, err := getTableMetadata(ctx, db, tbl)
	if err != nil {
		return errw(err)
	}

	dstTblDef := tableMetadataToSchema(srcMd, tbl)
	// Apply the requested kind swaps.
	for i, colName := range colNames {
		col, ferr := dstTblDef.FindCol(colName)
		if ferr != nil {
			return errz.Wrapf(ferr, "rqlite: alter table: column not found")
		}
		col.Kind = kinds[i]
	}

	tmpName := "tmp_tbl_alter_" + stringz.Uniq8()
	dstTblDef.Name = tmpName
	createStmt := buildCreateTableStmt(dstTblDef)

	stmts := []gorqlite.ParameterizedStatement{
		{Query: "PRAGMA foreign_keys=off"},
		{Query: createStmt},
		{Query: fmt.Sprintf(`INSERT INTO %q SELECT * FROM %q`, tmpName, tbl)},
		{Query: fmt.Sprintf(`DROP TABLE %q`, tbl)},
		{Query: fmt.Sprintf(`ALTER TABLE %q RENAME TO %q`, tmpName, tbl)},
		{Query: "PRAGMA foreign_keys=on"},
	}

	_, err = writeAtomic(ctx, db, stmts...)
	return err
}
