// Package postgres implements the sq driver for postgres.
package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"strconv"
	"strings"

	pgx "github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/xo/dburl"

	"github.com/neilotoole/sq/libsq/ast"
	"github.com/neilotoole/sq/libsq/ast/render"
	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/jointype"
	"github.com/neilotoole/sq/libsq/core/kind"
	"github.com/neilotoole/sq/libsq/core/lg"
	"github.com/neilotoole/sq/libsq/core/lg/lga"
	"github.com/neilotoole/sq/libsq/core/lg/lgm"
	"github.com/neilotoole/sq/libsq/core/options"
	"github.com/neilotoole/sq/libsq/core/record"
	"github.com/neilotoole/sq/libsq/core/retry"
	"github.com/neilotoole/sq/libsq/core/schema"
	"github.com/neilotoole/sq/libsq/core/sqlz"
	"github.com/neilotoole/sq/libsq/core/stringz"
	"github.com/neilotoole/sq/libsq/core/tablefq"
	"github.com/neilotoole/sq/libsq/core/tuning"
	"github.com/neilotoole/sq/libsq/driver"
	"github.com/neilotoole/sq/libsq/driver/dialect"
	"github.com/neilotoole/sq/libsq/source"
	"github.com/neilotoole/sq/libsq/source/drivertype"
	"github.com/neilotoole/sq/libsq/source/location"
	"github.com/neilotoole/sq/libsq/source/metadata"
)

// Provider is the postgres implementation of driver.Provider.
type Provider struct {
	Log *slog.Logger
}

// DriverFor implements driver.Provider.
func (p *Provider) DriverFor(typ drivertype.Type) (driver.Driver, error) {
	if typ != drivertype.Pg {
		return nil, errz.Errorf("unsupported driver type {%s}", typ)
	}

	return &driveri{log: p.Log}, nil
}

var _ driver.SQLDriver = (*driveri)(nil)

// driveri is the postgres implementation of driver.Driver.
type driveri struct {
	log *slog.Logger
}

// ConnParams implements driver.SQLDriver.
func (d *driveri) ConnParams() map[string][]string {
	// https://www.postgresql.org/docs/current/libpq-connect.html#LIBPQ-PARAMKEYWORDS
	return map[string][]string{
		"channel_binding":           {"prefer", "require", "disable"},
		"connect_timeout":           {"2"},
		"application_name":          nil,
		"fallback_application_name": nil,
		"gssencmode":                {"disable", "prefer", "require"},
		"sslmode":                   {"disable", "allow", "prefer", "require", "verify-ca", "verify-full"},
	}
}

// ErrWrapFunc implements driver.SQLDriver.
func (d *driveri) ErrWrapFunc() func(error) error {
	return errw
}

// DriverMetadata implements driver.Driver.
func (d *driveri) DriverMetadata() driver.Metadata {
	return driver.Metadata{
		Type:        drivertype.Pg,
		Description: "PostgreSQL",
		Doc:         "https://github.com/jackc/pgx",
		IsSQL:       true,
		DefaultPort: 5432,
	}
}

// Dialect implements driver.SQLDriver.
func (d *driveri) Dialect() dialect.Dialect {
	return dialect.Dialect{
		Type:           drivertype.Pg,
		Placeholders:   placeholders,
		Enquote:        stringz.DoubleQuote,
		MaxBatchValues: 1000,
		Ops:            dialect.DefaultOps(),
		ExecModeFor:    dialect.DefaultExecModeFor,
		Joins:          jointype.All(),
		Catalog:        true,
	}
}

func placeholders(numCols, numRows int) string {
	rows := make([]string, numRows)

	n := 1
	var sb strings.Builder
	for i := 0; i < numRows; i++ {
		sb.Reset()
		sb.WriteRune('(')
		for j := 1; j <= numCols; j++ {
			sb.WriteRune('$')
			sb.WriteString(strconv.Itoa(n))
			n++
			if j < numCols {
				sb.WriteString(driver.Comma)
			}
		}
		sb.WriteRune(')')
		rows[i] = sb.String()
	}

	return strings.Join(rows, driver.Comma)
}

// Renderer implements driver.SQLDriver.
func (d *driveri) Renderer() *render.Renderer {
	r := render.NewDefaultRenderer()
	r.FunctionNames[ast.FuncNameSchema] = "current_schema"
	r.FunctionNames[ast.FuncNameCatalog] = "current_database"
	return r
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
	log := lg.FromContext(ctx)
	ctx = options.NewContext(ctx, src.Options)
	poolCfg, err := getPoolConfig(src, false)
	if err != nil {
		return nil, err
	}

	var opts []stdlib.OptionOpenDB
	if src.Schema != "" {
		opts = append(opts, stdlib.OptionAfterConnect(func(ctx context.Context, conn *pgx.Conn) error {
			var oldSearchPath string
			if err = conn.QueryRow(ctx, "SHOW search_path").Scan(&oldSearchPath); err != nil {
				return errw(err)
			}

			newSearchPath := stringz.DoubleQuote(src.Schema)
			if oldSearchPath != "" {
				newSearchPath += ", " + oldSearchPath
			}

			log.Debug("Setting default schema (search_path) on Postgres DB connection",
				lga.Src, src,
				lga.Conn, location.Redact(poolCfg.ConnString()),
				lga.Catalog, src.Catalog,
				lga.Schema, src.Schema,
				lga.Old, oldSearchPath,
				lga.New, newSearchPath)

			_, err = conn.Exec(ctx, "SET search_path TO "+newSearchPath)
			return errw(err)
		}))
	}

	poolCfg.ConnConfig.ConnectTimeout = driver.OptConnOpenTimeout.Get(src.Options)

	db := stdlib.OpenDB(*poolCfg.ConnConfig, opts...)
	driver.ConfigureDB(ctx, db, src.Options)

	return db, nil
}

// ValidateSource implements driver.Driver.
func (d *driveri) ValidateSource(src *source.Source) (*source.Source, error) {
	if src.Type != drivertype.Pg {
		return nil, errz.Errorf("expected driver type {%s} but got {%s}", drivertype.Pg, src.Type)
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

	return errz.Wrapf(db.PingContext(ctx), "ping %s", src.Handle)
}

// DBProperties implements driver.SQLDriver.
func (d *driveri) DBProperties(ctx context.Context, db sqlz.DB) (map[string]any, error) {
	return getPgSettings(ctx, db)
}

// Truncate implements driver.Driver.
// Note that Truncate makes a separate query to determine the
// row count of tbl before executing TRUNCATE. This row count
// query is not part of a transaction with TRUNCATE, although
// possibly it should be, as the number of rows may have changed.
func (d *driveri) Truncate(ctx context.Context, src *source.Source, tbl string, reset bool) (affected int64,
	err error,
) {
	// https://www.postgresql.org/docs/9.1/sql-truncate.html

	// RESTART IDENTITY and CASCADE/RESTRICT are from pg 8.2 onwards
	// FIXME: should first check the pg version for < pg8.2 support
	db, err := d.doOpen(ctx, src)
	if err != nil {
		return affected, errw(err)
	}

	affectedQuery := "SELECT COUNT(*) FROM " + idSanitize(tbl)
	err = db.QueryRowContext(ctx, affectedQuery).Scan(&affected)
	if err != nil {
		return 0, errw(err)
	}

	truncateQuery := "TRUNCATE TABLE " + idSanitize(tbl)
	if reset {
		// if reset & src.DBVersion >= 8.2
		truncateQuery += " RESTART IDENTITY" // default is CONTINUE IDENTITY
	}
	// We could add RESTRICT here; alternative is CASCADE
	_, err = db.ExecContext(ctx, truncateQuery)
	if err != nil {
		return 0, errw(err)
	}

	return affected, nil
}

// idSanitize sanitizes an identifier (such as table name). It will
// add surrounding quotes. For example:
//
//	table_name    -->    "table_name"
func idSanitize(s string) string {
	return pgx.Identifier([]string{s}).Sanitize()
}

// CreateSchema implements driver.SQLDriver.
func (d *driveri) CreateSchema(ctx context.Context, db sqlz.DB, schemaName string) error {
	stmt := `CREATE SCHEMA ` + idSanitize(schemaName)
	_, err := db.ExecContext(ctx, stmt)
	return errz.Wrapf(err, "failed to create schema {%s}", schemaName)
}

// DropSchema implements driver.SQLDriver.
func (d *driveri) DropSchema(ctx context.Context, db sqlz.DB, schemaName string) error {
	stmt := `DROP SCHEMA ` + idSanitize(schemaName) + ` CASCADE`
	_, err := db.ExecContext(ctx, stmt)
	return errz.Wrapf(err, "failed to drop schema {%s}", schemaName)
}

// CreateTable implements driver.SQLDriver.
func (d *driveri) CreateTable(ctx context.Context, db sqlz.DB, tblDef *schema.Table) error {
	stmt := buildCreateTableStmt(tblDef)

	_, err := db.ExecContext(ctx, stmt)
	return errw(err)
}

// CurrentSchema implements driver.SQLDriver.
func (d *driveri) CurrentSchema(ctx context.Context, db sqlz.DB) (string, error) {
	var name string
	if err := db.QueryRowContext(ctx, `SELECT CURRENT_SCHEMA()`).Scan(&name); err != nil {
		return "", errw(err)
	}

	return name, nil
}

// ListSchemas implements driver.SQLDriver. Some system schemas are
// excluded, e.g. pg_toast and pg_temp schemas.
func (d *driveri) ListSchemas(ctx context.Context, db sqlz.DB) ([]string, error) {
	log := lg.FromContext(ctx)

	const q = `SELECT schema_name FROM information_schema.schemata
 WHERE schema_name NOT LIKE 'pg_toast%' AND schema_name NOT LIKE 'pg_temp%'
 ORDER BY schema_name`
	var schemas []string
	rows, err := db.QueryContext(ctx, q)
	if err != nil {
		return nil, errw(err)
	}

	defer sqlz.CloseRows(log, rows)

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

// ListSchemaMetadata implements driver.SQLDriver. Some system schemas are
// excluded, e.g. pg_toast and pg_temp schemas.
func (d *driveri) ListSchemaMetadata(ctx context.Context, db sqlz.DB) ([]*metadata.Schema, error) {
	log := lg.FromContext(ctx)

	const q = `SELECT schema_name, catalog_name, schema_owner FROM information_schema.schemata
WHERE catalog_name = current_database()
AND schema_name NOT LIKE 'pg_toast%' AND schema_name NOT LIKE 'pg_temp%'
ORDER BY schema_name`
	var schemas []*metadata.Schema
	rows, err := db.QueryContext(ctx, q)
	if err != nil {
		return nil, errw(err)
	}

	defer sqlz.CloseRows(log, rows)

	var name string
	var catalog, owner sql.NullString

	for rows.Next() {
		if err = rows.Scan(&name, &catalog, &owner); err != nil {
			return nil, errw(err)
		}
		s := &metadata.Schema{
			Name:    name,
			Catalog: catalog.String,
			Owner:   owner.String,
		}

		schemas = append(schemas, s)
	}

	if err = rows.Err(); err != nil {
		return nil, errw(err)
	}

	return schemas, nil
}

// CurrentCatalog implements driver.SQLDriver.
func (d *driveri) CurrentCatalog(ctx context.Context, db sqlz.DB) (string, error) {
	var name string
	if err := db.QueryRowContext(ctx, `SELECT CURRENT_DATABASE()`).Scan(&name); err != nil {
		return "", errw(err)
	}

	return name, nil
}

// ListCatalogs implements driver.SQLDriver.
func (d *driveri) ListCatalogs(ctx context.Context, db sqlz.DB) ([]string, error) {
	catalogs := make([]string, 1, 3)
	if err := db.QueryRowContext(ctx, `SELECT CURRENT_DATABASE()`).Scan(&catalogs[0]); err != nil {
		return nil, errw(err)
	}

	const q = `SELECT datname FROM pg_catalog.pg_database
WHERE datistemplate = FALSE AND datallowconn = TRUE AND datname != CURRENT_DATABASE()
ORDER BY datname`

	rows, err := db.QueryContext(ctx, q)
	if err != nil {
		return nil, errw(err)
	}

	defer sqlz.CloseRows(lg.FromContext(ctx), rows)

	for rows.Next() {
		var catalog string
		if err = rows.Scan(&catalog); err != nil {
			return nil, errw(err)
		}
		catalogs = append(catalogs, catalog)
	}

	if err = rows.Err(); err != nil {
		return nil, errw(err)
	}

	return catalogs, nil
}

// SchemaExists implements driver.SQLDriver.
func (d *driveri) SchemaExists(ctx context.Context, db sqlz.DB, schma string) (bool, error) {
	if schma == "" {
		return false, nil
	}

	const q = `SELECT COUNT(schema_name) FROM information_schema.schemata
 WHERE schema_name = $1 AND catalog_name = current_database()`

	var count int
	return count > 0, errw(db.QueryRowContext(ctx, q, schma).Scan(&count))
}

// CatalogExists implements driver.SQLDriver.
func (d *driveri) CatalogExists(ctx context.Context, db sqlz.DB, catalog string) (bool, error) {
	if catalog == "" {
		return false, nil
	}

	const q = `SELECT COUNT(datname) FROM pg_catalog.pg_database
WHERE datistemplate = FALSE AND datallowconn = TRUE AND datname = $1`

	var count int
	return count > 0, errw(db.QueryRowContext(ctx, q, catalog).Scan(&count))
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
	q := fmt.Sprintf("ALTER TABLE %q ADD COLUMN %q ", tbl, col) + dbTypeNameFromKind(knd)

	_, err := db.ExecContext(ctx, q)
	if err != nil {
		return errz.Wrapf(err, "alter table: failed to add column {%s} to table {%s}", col, tbl)
	}

	return nil
}

// AlterTableColumnKinds is not yet implemented for postgres.
func (d *driveri) AlterTableColumnKinds(_ context.Context, _ sqlz.DB, _ string, _ []string, _ []kind.Kind) error {
	return errz.New("not implemented")
}

// PrepareInsertStmt implements driver.SQLDriver.
func (d *driveri) PrepareInsertStmt(ctx context.Context, db sqlz.DB, destTbl string, destColNames []string,
	numRows int,
) (*driver.StmtExecer, error) {
	// Note that the pgx driver doesn't support res.LastInsertId.
	// https://github.com/jackc/pgx/issues/411

	destColsMeta, err := d.getTableRecordMeta(ctx, db, destTbl, destColNames)
	if err != nil {
		return nil, err
	}

	stmt, err := driver.PrepareInsertStmt(ctx, d, db, destTbl, destColsMeta.Names(), numRows)
	if err != nil {
		return nil, err
	}

	execer := driver.NewStmtExecer(stmt, driver.DefaultInsertMungeFunc(destTbl, destColsMeta), newStmtExecFunc(stmt),
		destColsMeta)
	return execer, nil
}

// PrepareUpdateStmt implements driver.SQLDriver.
func (d *driveri) PrepareUpdateStmt(ctx context.Context, db sqlz.DB, destTbl string, destColNames []string,
	where string,
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

	execer := driver.NewStmtExecer(stmt, driver.DefaultInsertMungeFunc(destTbl, destColsMeta), newStmtExecFunc(stmt),
		destColsMeta)
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

// CopyTable implements driver.SQLDriver.
func (d *driveri) CopyTable(ctx context.Context, db sqlz.DB,
	fromTable, toTable tablefq.T, copyData bool,
) (int64, error) {
	stmt := fmt.Sprintf(
		"CREATE TABLE %s AS TABLE %s",
		tblfmt(toTable),
		tblfmt(fromTable),
	)

	if !copyData {
		stmt += " WITH NO DATA"
	}

	affected, err := sqlz.ExecAffected(ctx, db, stmt)
	if err != nil {
		return 0, errw(err)
	}

	return affected, nil
}

// TableExists implements driver.SQLDriver.
func (d *driveri) TableExists(ctx context.Context, db sqlz.DB, tbl string) (bool, error) {
	const query = `SELECT COUNT(*) FROM information_schema.tables
WHERE table_name = $1`

	var count int64
	err := db.QueryRowContext(ctx, query, tbl).Scan(&count)
	if err != nil {
		return false, errw(err)
	}

	return count == 1, nil
}

// ListTableNames implements driver.SQLDriver.
func (d *driveri) ListTableNames(ctx context.Context, db sqlz.DB, schma string, tables, views bool) ([]string, error) {
	var tblClause string
	switch {
	case tables && views:
		tblClause = " AND (table_type = 'BASE TABLE' OR table_type = 'VIEW')"
	case tables:
		tblClause = " AND table_type = 'BASE TABLE'"
	case views:
		tblClause = " AND table_type = 'VIEW'"
	default:
		return []string{}, nil
	}

	var args []any
	q := "SELECT table_name FROM information_schema.tables WHERE table_schema = "
	if schma == "" {
		q += "current_schema()"
	} else {
		q += "$1"
		args = append(args, schma)
	}
	q += tblClause + " ORDER BY table_name"

	rows, err := db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, errw(err)
	}

	names, err := sqlz.RowsScanColumn[string](ctx, rows)
	if err != nil {
		return nil, errw(err)
	}

	return names, nil
}

// DropTable implements driver.SQLDriver.
func (d *driveri) DropTable(ctx context.Context, db sqlz.DB, tbl tablefq.T, ifExists bool) error {
	var stmt string

	if ifExists {
		stmt = fmt.Sprintf("DROP TABLE IF EXISTS %s RESTRICT", tblfmt(tbl))
	} else {
		stmt = fmt.Sprintf("DROP TABLE %s RESTRICT", tblfmt(tbl))
	}

	_, err := db.ExecContext(ctx, stmt)
	return errw(err)
}

// TableColumnTypes implements driver.SQLDriver.
func (d *driveri) TableColumnTypes(ctx context.Context, db sqlz.DB, tblName string,
	colNames []string,
) ([]*sql.ColumnType, error) {
	// We have to do some funky stuff to get the column types
	// from when the table has no rows.
	// https://stackoverflow.com/questions/8098795/return-a-value-if-no-record-is-found

	// If tblName is "person" and we want cols "username"
	// and "email", the query will look like:
	//
	// SELECT
	// 	(SELECT username FROM person LIMIT 1) AS username,
	// 	(SELECT email FROM person LIMIT 1) AS email
	// LIMIT 1;

	enquote := d.Dialect().Enquote
	tblNameQuoted := enquote(tblName)

	var query string

	if len(colNames) == 0 {
		// When the table is empty, and colNames are not provided,
		// then we need to fetch the table col names independently.
		var err error
		colNames, err = getTableColumnNames(ctx, db, tblName)
		if err != nil {
			return nil, err
		}
	}

	var sb strings.Builder
	sb.WriteString("SELECT\n")
	for i, colName := range colNames {
		colNameQuoted := enquote(colName)
		sb.WriteString(fmt.Sprintf("  (SELECT %s FROM %s LIMIT 1) AS %s", colNameQuoted, tblNameQuoted, colNameQuoted))
		if i < len(colNames)-1 {
			sb.WriteRune(',')
		}
		sb.WriteString("\n")
	}
	sb.WriteString("LIMIT 1")
	query = sb.String()

	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, errw(err)
	}

	colTypes, err := rows.ColumnTypes()
	if err != nil {
		sqlz.CloseRows(d.log, rows)
		return nil, errw(err)
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

// getTableColumnNames consults postgres's information_schema.columns table,
// returning the names of the table's columns in ordinal order.
func getTableColumnNames(ctx context.Context, db sqlz.DB, tblName string) ([]string, error) {
	log := lg.FromContext(ctx)
	const query = `SELECT column_name FROM information_schema.columns
	WHERE table_schema = CURRENT_SCHEMA()
	AND table_name = $1
	ORDER BY ordinal_position`

	rows, err := db.QueryContext(ctx, query, tblName)
	if err != nil {
		return nil, errw(err)
	}

	var colNames []string
	var colName string

	for rows.Next() {
		err = rows.Scan(&colName)
		if err != nil {
			sqlz.CloseRows(log, rows)
			return nil, errw(err)
		}

		colNames = append(colNames, colName)
	}

	if rows.Err() != nil {
		sqlz.CloseRows(log, rows)
		return nil, errw(err)
	}

	err = rows.Close()
	if err != nil {
		return nil, errw(err)
	}

	return colNames, nil
}

// RecordMeta implements driver.SQLDriver.
func (d *driveri) RecordMeta(ctx context.Context, colTypes []*sql.ColumnType) (
	record.Meta, driver.NewRecordFunc, error,
) {
	// The jackc/pgx driver doesn't report nullability (sql.ColumnType)
	// Apparently this is due to what postgres sends over the wire.
	// See https://github.com/jackc/pgx/issues/276#issuecomment-526831493
	// So, we'll set the scan type for each column to the nullable
	// version below.

	sColTypeData := make([]*record.ColumnTypeData, len(colTypes))
	ogColNames := make([]string, len(colTypes))
	for i, colType := range colTypes {
		knd := kindFromDBTypeName(d.log, colType.Name(), colType.DatabaseTypeName())
		colTypeData := record.NewColumnTypeData(colType, knd)
		setScanType(d.log, colTypeData, knd)
		sColTypeData[i] = colTypeData
		ogColNames[i] = colTypeData.Name
	}

	mungedColNames, err := driver.MungeResultColNames(ctx, ogColNames)
	if err != nil {
		return nil, nil, err
	}

	recMeta := make(record.Meta, len(colTypes))
	for i := range sColTypeData {
		recMeta[i] = record.NewFieldMeta(sColTypeData[i], mungedColNames[i])
	}

	mungeFn := func(vals []any) (record.Record, error) {
		// postgres doesn't need to do any special munging, so we
		// just use the default munging.
		rec, skipped := driver.NewRecordFromScanRow(recMeta, vals, nil)
		if len(skipped) > 0 {
			var skippedDetails []string

			for _, skip := range skipped {
				meta := recMeta[skip]
				skippedDetails = append(skippedDetails,
					fmt.Sprintf("[%d] %s: db(%s) --> kind(%s) --> scan(%s)",
						skip, meta.Name(), meta.DatabaseTypeName(), meta.Kind(), meta.ScanType()))
			}

			return nil, errz.Errorf("expected zero skipped cols but have %d:\n  %s",
				skipped, strings.Join(skippedDetails, "\n  "))
		}
		return rec, nil
	}

	return recMeta, mungeFn, nil
}

// getPoolConfig returns the native postgres [*pgxpool.Config] for src, applying
// src's fields, such as [source.Source.Catalog] as appropriate. If
// includeConnTimeout is true, then 'connect_timeout' is included in the
// returned config; this is provided as an option, because the connection
// timeout is sometimes better handled via [context.WithTimeout].
func getPoolConfig(src *source.Source, includeConnTimeout bool) (*pgxpool.Config, error) {
	poolCfg, err := pgxpool.ParseConfig(src.Location)
	if err != nil {
		return nil, errw(err)
	}

	if src.Catalog != "" && src.Catalog != poolCfg.ConnConfig.Database {
		// The catalog differs from the database in the connection string.
		// OOTB, Postgres doesn't support cross-database references. So,
		// we'll need to change the connection string to use the catalog
		// as the database. Note that we don't modify src.Location, but it's
		// not entirely clear if that's the correct approach. Are there any
		// downsides to modifying it (as long as the modified Location is not
		// persisted back to config)?
		var u *dburl.URL
		if u, err = dburl.Parse(src.Location); err != nil {
			return nil, errw(err)
		}

		u.Path = src.Catalog
		connStr := u.String()
		poolCfg, err = pgxpool.ParseConfig(connStr)
		if err != nil {
			return nil, errw(err)
		}
	}

	if includeConnTimeout {
		srcTimeout := driver.OptConnOpenTimeout.Get(src.Options)
		// Only set connect_timeout if it's non-zero and differs from the
		// already-configured value.
		// REVISIT: We should actually always set it, otherwise the user's
		// envar PGCONNECT_TIMEOUT may override it?

		if srcTimeout > 0 || poolCfg.ConnConfig.ConnectTimeout != srcTimeout {
			var u *dburl.URL
			if u, err = dburl.Parse(poolCfg.ConnString()); err != nil {
				return nil, errw(err)
			}

			q := u.Query()
			q.Set("connect_timeout", strconv.Itoa(int(srcTimeout.Seconds())))
			u.RawQuery = q.Encode()
			poolCfg, err = pgxpool.ParseConfig(u.String())
			if err != nil {
				return nil, errw(err)
			}
		}
	}

	return poolCfg, nil
}

// doRetry executes fn with retry on isErrTooManyConnections.
func doRetry(ctx context.Context, fn func() error) error {
	maxRetryInterval := tuning.OptMaxRetryInterval.Get(options.FromContext(ctx))
	return retry.Do(ctx, maxRetryInterval, fn, isErrTooManyConnections)
}
