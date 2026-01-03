// Package oracle implements the sq driver for Oracle Database.
package oracle

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"strconv"
	"strings"

	"github.com/godror/godror"

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
	"github.com/neilotoole/sq/libsq/core/sqlz"
	"github.com/neilotoole/sq/libsq/core/stringz"
	"github.com/neilotoole/sq/libsq/core/tablefq"
	"github.com/neilotoole/sq/libsq/driver"
	"github.com/neilotoole/sq/libsq/driver/dialect"
	"github.com/neilotoole/sq/libsq/source"
	"github.com/neilotoole/sq/libsq/source/drivertype"
	"github.com/neilotoole/sq/libsq/source/metadata"
)

// Provider is the Oracle implementation of driver.Provider.
type Provider struct {
	Log *slog.Logger
}

// DriverFor implements driver.Provider.
func (p *Provider) DriverFor(typ drivertype.Type) (driver.Driver, error) {
	if typ != drivertype.Oracle {
		return nil, errz.Errorf("unsupported driver type {%s}", typ)
	}

	return &driveri{log: p.Log}, nil
}

var _ driver.SQLDriver = (*driveri)(nil)

// driveri is the Oracle implementation of driver.Driver.
type driveri struct {
	log *slog.Logger
}

// ConnParams implements driver.SQLDriver.
func (d *driveri) ConnParams() map[string][]string {
	// Oracle connection parameters
	// godror supports many Oracle-specific parameters
	return map[string][]string{
		"connectionClass":      nil,
		"poolMinSessions":      {"0"},
		"poolMaxSessions":      {"1000"},
		"poolIncrement":        {"1"},
		"timezone":             nil,
		"standaloneConnection": {"0", "1"},
	}
}

// ErrWrapFunc implements driver.SQLDriver.
func (d *driveri) ErrWrapFunc() func(error) error {
	return errw
}

// DriverMetadata implements driver.Driver.
func (d *driveri) DriverMetadata() driver.Metadata {
	return driver.Metadata{
		Type:        drivertype.Oracle,
		Description: "Oracle Database",
		Doc:         "https://github.com/godror/godror",
		IsSQL:       true,
		DefaultPort: 1521,
	}
}

// Dialect implements driver.SQLDriver.
func (d *driveri) Dialect() dialect.Dialect {
	return dialect.Dialect{
		Type:           drivertype.Oracle,
		Placeholders:   placeholders,
		Enquote:        enquoteOracle,
		MaxBatchValues: 1000,
		Ops:            dialect.DefaultOps(),
		Joins:          jointype.All(),
		Catalog:        false, // Oracle uses schemas only
	}
}

// enquoteOracle wraps an identifier in double quotes and uppercases it.
// Oracle stores quoted identifiers case-sensitively, so we uppercase them
// to match Oracle's convention of uppercase unquoted identifiers.
func enquoteOracle(s string) string {
	return stringz.DoubleQuote(strings.ToUpper(s))
}

// placeholders generates Oracle-style placeholders: (:1, :2, :3), (:4, :5, :6), ...
func placeholders(numCols, numRows int) string {
	rows := make([]string, numRows)

	n := 1
	var sb strings.Builder
	for i := 0; i < numRows; i++ {
		sb.Reset()
		sb.WriteRune('(')
		for j := 1; j <= numCols; j++ {
			sb.WriteRune(':')
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
	r.FunctionNames[ast.FuncNameSchema] = "SYS_CONTEXT('USERENV', 'CURRENT_SCHEMA')"
	r.FunctionOverrides[ast.FuncNameCatalog] = doRenderFuncCatalog
	r.FunctionOverrides[ast.FuncNameRowNum] = renderFuncRowNum
	return r
}

// doRenderFuncCatalog renders the catalog function. Oracle doesn't have catalogs,
// so we return NULL.
func doRenderFuncCatalog(_ *render.Context, _ *ast.FuncNode) (string, error) {
	return "NULL", nil
}

// renderFuncRowNum renders the row number function.
func renderFuncRowNum(_ *render.Context, _ *ast.FuncNode) (string, error) {
	return "ROWNUM", nil
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

// doOpen opens a connection to the Oracle database.
func (d *driveri) doOpen(ctx context.Context, src *source.Source) (*sql.DB, error) {
	ctx = options.NewContext(ctx, src.Options)

	// Parse Oracle connection string
	// godror expects: user/password@host:port/service_name
	// or TNS alias: user/password@tnsalias
	params, err := godror.ParseConnString(src.Location)
	if err != nil {
		return nil, errw(err)
	}

	// Open database connection using godror connector
	// Note: Connection timeout is handled via context timeout at higher levels
	db := sql.OpenDB(godror.NewConnector(params))
	driver.ConfigureDB(ctx, db, src.Options)

	return db, nil
}

// ValidateSource implements driver.Driver.
func (d *driveri) ValidateSource(src *source.Source) (*source.Source, error) {
	if src.Type != drivertype.Oracle {
		return nil, errz.Errorf("expected driver type {%s} but got {%s}", drivertype.Oracle, src.Type)
	}
	return src, nil
}

// Ping implements driver.Driver.
func (d *driveri) Ping(ctx context.Context, src *source.Source) error {
	db, err := d.doOpen(ctx, src)
	if err != nil {
		return err
	}
	defer db.Close()

	return errw(db.PingContext(ctx))
}

// CurrentSchema implements driver.SQLDriver.
func (d *driveri) CurrentSchema(ctx context.Context, db sqlz.DB) (string, error) {
	var name string
	err := db.QueryRowContext(ctx,
		"SELECT SYS_CONTEXT('USERENV', 'CURRENT_SCHEMA') FROM DUAL").Scan(&name)
	return name, errw(err)
}

// ListSchemas implements driver.SQLDriver.
func (d *driveri) ListSchemas(ctx context.Context, db sqlz.DB) ([]string, error) {
	const query = `SELECT username
FROM all_users
WHERE username NOT IN ('SYS', 'SYSTEM', 'OUTLN', 'DBSNMP', 'APPQOSSYS',
                       'WMSYS', 'EXFSYS', 'CTXSYS', 'XDB', 'ANONYMOUS',
                       'ORACLE_OCM', 'MDSYS', 'OLAPSYS', 'ORDDATA', 'ORDSYS')
ORDER BY username`

	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, errw(err)
	}
	defer rows.Close()

	var schemas []string
	for rows.Next() {
		var schema string
		if err = rows.Scan(&schema); err != nil {
			return nil, errw(err)
		}
		schemas = append(schemas, schema)
	}

	if err = rows.Err(); err != nil {
		return nil, errw(err)
	}

	return schemas, nil
}

// CurrentCatalog implements driver.SQLDriver.
// Oracle doesn't have the concept of catalogs like PostgreSQL.
func (d *driveri) CurrentCatalog(_ context.Context, _ sqlz.DB) (string, error) {
	return "", errz.New("Oracle does not support catalogs")
}

// ListCatalogs implements driver.SQLDriver.
// Oracle doesn't have the concept of catalogs like PostgreSQL.
func (d *driveri) ListCatalogs(_ context.Context, _ sqlz.DB) ([]string, error) {
	return nil, errz.New("Oracle does not support catalogs")
}

// CreateSchema implements driver.SQLDriver.
// In Oracle, schemas are tied to users. Use CREATE USER instead.
func (d *driveri) CreateSchema(_ context.Context, _ sqlz.DB, _ string) error {
	return errz.New("Oracle does not support CREATE SCHEMA; use CREATE USER instead")
}

// DropSchema implements driver.SQLDriver.
// In Oracle, schemas are tied to users. Use DROP USER instead.
func (d *driveri) DropSchema(_ context.Context, _ sqlz.DB, _ string) error {
	return errz.New("Oracle does not support DROP SCHEMA; use DROP USER instead")
}

// SchemaExists implements driver.SQLDriver.
func (d *driveri) SchemaExists(ctx context.Context, db sqlz.DB, schma string) (bool, error) {
	const query = `SELECT COUNT(*) FROM all_users WHERE username = :1`

	var count int
	err := db.QueryRowContext(ctx, query, strings.ToUpper(schma)).Scan(&count)
	if err != nil {
		return false, errw(err)
	}

	return count > 0, nil
}

// CatalogExists implements driver.SQLDriver.
// Oracle doesn't have catalogs.
func (d *driveri) CatalogExists(_ context.Context, _ sqlz.DB, _ string) (bool, error) {
	return false, errz.New("Oracle does not support catalogs")
}

// TableExists implements driver.SQLDriver.
func (d *driveri) TableExists(ctx context.Context, db sqlz.DB, tbl string) (bool, error) {
	const query = `SELECT COUNT(*) FROM user_tables WHERE table_name = :1`

	var count int
	err := db.QueryRowContext(ctx, query, strings.ToUpper(tbl)).Scan(&count)
	if err != nil {
		return false, errw(err)
	}

	return count > 0, nil
}

// ListTableNames implements driver.SQLDriver.
func (d *driveri) ListTableNames(ctx context.Context, db sqlz.DB, _ string, tables, views bool) ([]string, error) {
	names := []string{}

	if tables {
		const queryTables = `SELECT table_name FROM user_tables WHERE temporary = 'N' ORDER BY table_name`
		rows, err := db.QueryContext(ctx, queryTables)
		if err != nil {
			return nil, errw(err)
		}
		defer rows.Close()

		for rows.Next() {
			var name string
			if err = rows.Scan(&name); err != nil {
				return nil, errw(err)
			}
			names = append(names, name)
		}

		if err = rows.Err(); err != nil {
			return nil, errw(err)
		}
	}

	if views {
		const queryViews = `SELECT view_name FROM user_views ORDER BY view_name`
		rows, err := db.QueryContext(ctx, queryViews)
		if err != nil {
			return nil, errw(err)
		}
		defer rows.Close()

		for rows.Next() {
			var name string
			if err = rows.Scan(&name); err != nil {
				return nil, errw(err)
			}
			names = append(names, name)
		}

		if err = rows.Err(); err != nil {
			return nil, errw(err)
		}
	}

	return names, nil
}

// DropTable implements driver.SQLDriver.
func (d *driveri) DropTable(ctx context.Context, db sqlz.DB, tbl tablefq.T, ifExists bool) error {
	var stmt string
	tblName := stringz.DoubleQuote(strings.ToUpper(tbl.Table))

	if ifExists {
		// Oracle 12c+ supports IF EXISTS
		stmt = fmt.Sprintf("DROP TABLE IF EXISTS %s CASCADE CONSTRAINTS", tblName)
	} else {
		stmt = fmt.Sprintf("DROP TABLE %s CASCADE CONSTRAINTS", tblName)
	}

	_, err := db.ExecContext(ctx, stmt)
	return errw(err)
}

// Truncate implements driver.SQLDriver.
func (d *driveri) Truncate(ctx context.Context, src *source.Source, tbl string, reset bool) (int64, error) {
	db, err := d.doOpen(ctx, src)
	if err != nil {
		return 0, errw(err)
	}
	defer db.Close()

	// Get row count before truncate
	var affected int64
	tblName := stringz.DoubleQuote(strings.ToUpper(tbl))
	err = db.QueryRowContext(ctx, "SELECT COUNT(*) FROM "+tblName).Scan(&affected)
	if err != nil {
		return 0, errw(err)
	}

	// TRUNCATE with optional storage reset
	truncateQuery := "TRUNCATE TABLE " + tblName
	if reset {
		truncateQuery += " DROP STORAGE" // Also resets sequences in Oracle
	} else {
		truncateQuery += " REUSE STORAGE"
	}

	_, err = db.ExecContext(ctx, truncateQuery)
	if err != nil {
		return 0, errw(err)
	}

	return affected, nil
}

// AlterTableAddColumn implements driver.SQLDriver.
func (d *driveri) AlterTableAddColumn(ctx context.Context, db sqlz.DB, tbl, col string, knd kind.Kind) error {
	q := fmt.Sprintf(`ALTER TABLE "%s" ADD "%s" %s`,
		strings.ToUpper(tbl), strings.ToUpper(col), dbTypeNameFromKind(knd))
	_, err := db.ExecContext(ctx, q)
	return errw(err)
}

// AlterTableRename implements driver.SQLDriver.
func (d *driveri) AlterTableRename(ctx context.Context, db sqlz.DB, tbl, newName string) error {
	q := fmt.Sprintf(`ALTER TABLE "%s" RENAME TO "%s"`, strings.ToUpper(tbl), strings.ToUpper(newName))
	_, err := db.ExecContext(ctx, q)
	return errw(err)
}

// AlterTableRenameColumn implements driver.SQLDriver.
func (d *driveri) AlterTableRenameColumn(ctx context.Context, db sqlz.DB, tbl, col, newName string,
) error {
	q := fmt.Sprintf(`ALTER TABLE "%s" RENAME COLUMN "%s" TO "%s"`,
		strings.ToUpper(tbl), strings.ToUpper(col), strings.ToUpper(newName))
	_, err := db.ExecContext(ctx, q)
	return errw(err)
}

// AlterTableColumnKinds implements driver.SQLDriver.
func (d *driveri) AlterTableColumnKinds(_ context.Context, _ sqlz.DB, _ string, _ []string, _ []kind.Kind) error {
	return errz.New("AlterTableColumnKinds: not implemented")
}

// CopyTable implements driver.SQLDriver.
func (d *driveri) CopyTable(
	ctx context.Context, db sqlz.DB, fromTable, toTable tablefq.T, copyData bool,
) (int64, error) {
	fromTblName := stringz.DoubleQuote(strings.ToUpper(fromTable.Table))
	toTblName := stringz.DoubleQuote(strings.ToUpper(toTable.Table))

	var stmt string
	if copyData {
		stmt = fmt.Sprintf("CREATE TABLE %s AS SELECT * FROM %s", toTblName, fromTblName)
	} else {
		stmt = fmt.Sprintf("CREATE TABLE %s AS SELECT * FROM %s WHERE 1=0", toTblName, fromTblName)
	}

	result, err := db.ExecContext(ctx, stmt)
	if err != nil {
		return 0, errw(err)
	}

	if !copyData {
		return 0, nil
	}

	affected, err := result.RowsAffected()
	return affected, errw(err)
}

// TableColumnTypes implements driver.SQLDriver.
func (d *driveri) TableColumnTypes(
	ctx context.Context, db sqlz.DB, tblName string, colNames []string,
) ([]*sql.ColumnType, error) {
	// If colNames is empty, get all columns
	if len(colNames) == 0 {
		var err error
		colNames, err = d.getTableColumnNames(ctx, db, tblName)
		if err != nil {
			return nil, err
		}
	}

	enquote := d.Dialect().Enquote

	// Build column list for SELECT
	colsClause := make([]string, len(colNames))
	for i, colName := range colNames {
		colsClause[i] = enquote(colName)
	}

	// Use a subquery to get column types from an empty result set
	query := fmt.Sprintf("SELECT %s FROM %s WHERE 1=0",
		strings.Join(colsClause, ", "),
		enquote(tblName))

	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, errw(err)
	}
	defer rows.Close()

	colTypes, err := rows.ColumnTypes()
	return colTypes, errw(err)
}

// getTableColumnNames returns the column names for a table.
func (d *driveri) getTableColumnNames(ctx context.Context, db sqlz.DB, tblName string) ([]string, error) {
	const query = `SELECT column_name
FROM user_tab_columns
WHERE table_name = :1
ORDER BY column_id`

	rows, err := db.QueryContext(ctx, query, strings.ToUpper(tblName))
	if err != nil {
		return nil, errw(err)
	}
	defer rows.Close()

	var colNames []string
	for rows.Next() {
		var colName string
		if err = rows.Scan(&colName); err != nil {
			return nil, errw(err)
		}
		colNames = append(colNames, colName)
	}

	return colNames, errw(rows.Err())
}

// RecordMeta implements driver.SQLDriver.
func (d *driveri) RecordMeta(
	ctx context.Context, colTypes []*sql.ColumnType,
) (record.Meta, driver.NewRecordFunc, error) {
	sColTypeData := make([]*record.ColumnTypeData, len(colTypes))
	ogColNames := make([]string, len(colTypes))

	for i, colType := range colTypes {
		knd := kindFromDBTypeName(d.log, colType.Name(), colType.DatabaseTypeName())
		colTypeData := record.NewColumnTypeData(colType, knd)
		d.setScanType(colTypeData, knd)
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

	mungeFn := func(row []any) (record.Record, error) {
		// Oracle doesn't need special munging, so we use default munging.
		rec, skipped := driver.NewRecordFromScanRow(recMeta, row, nil)
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

// setScanType sets the appropriate scan type for Oracle column types.
func (d *driveri) setScanType(colTypeData *record.ColumnTypeData, knd kind.Kind) {
	// For nullable columns, use nullable scan types
	switch knd {
	case kind.Null, kind.Text, kind.Unknown:
		colTypeData.ScanType = sqlz.RTypeNullString
	case kind.Int:
		colTypeData.ScanType = sqlz.RTypeNullInt64
	case kind.Float:
		colTypeData.ScanType = sqlz.RTypeNullFloat64
	case kind.Decimal:
		colTypeData.ScanType = sqlz.RTypeNullDecimal
	case kind.Bool:
		// Oracle BOOLEAN is NUMBER(1,0)
		colTypeData.ScanType = sqlz.RTypeNullInt64
	case kind.Datetime, kind.Date, kind.Time:
		colTypeData.ScanType = sqlz.RTypeNullTime
	case kind.Bytes:
		colTypeData.ScanType = sqlz.RTypeBytes
	}
}

// getTableRecordMeta returns the record metadata for the specified columns of a table.
func (d *driveri) getTableRecordMeta(ctx context.Context, db sqlz.DB, tblName string, colNames []string) (
	record.Meta, error,
) {
	colTypes, err := d.TableColumnTypes(ctx, db, tblName, colNames)
	if err != nil {
		return nil, err
	}

	recMeta, _, err := d.RecordMeta(ctx, colTypes)
	return recMeta, err
}

// newStmtExecFunc returns a StmtExecFunc that wraps stmt.ExecContext.
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
func (d *driveri) PrepareUpdateStmt(ctx context.Context, db sqlz.DB, destTbl string, destColNames []string,
	where string,
) (*driver.StmtExecer, error) {
	destColsMeta, err := d.getTableRecordMeta(ctx, db, destTbl, destColNames)
	if err != nil {
		return nil, err
	}

	// Build UPDATE statement
	enquote := d.Dialect().Enquote
	destTblQuoted := enquote(destTbl)

	setClause := make([]string, len(destColNames))
	for i, colName := range destColNames {
		setClause[i] = fmt.Sprintf("%s = :%d", enquote(colName), i+1)
	}

	query := fmt.Sprintf("UPDATE %s SET %s", destTblQuoted, strings.Join(setClause, ", "))
	if where != "" {
		query += " WHERE " + where
	}

	stmt, err := db.PrepareContext(ctx, query)
	if err != nil {
		return nil, errw(err)
	}

	execer := driver.NewStmtExecer(stmt, driver.DefaultInsertMungeFunc(destTbl, destColsMeta),
		newStmtExecFunc(stmt), destColsMeta)
	return execer, nil
}

// DBProperties implements driver.SQLDriver.
func (d *driveri) DBProperties(ctx context.Context, db sqlz.DB) (map[string]any, error) {
	const query = `SELECT
    (SELECT SYS_CONTEXT('USERENV', 'DB_NAME') FROM DUAL) AS db_name,
    (SELECT version FROM v$instance) AS version,
    (SELECT SYS_CONTEXT('USERENV', 'CURRENT_SCHEMA') FROM DUAL) AS current_schema
FROM DUAL`

	props := make(map[string]any)

	var dbName, version, currentSchema string
	err := db.QueryRowContext(ctx, query).Scan(&dbName, &version, &currentSchema)
	if err != nil {
		return nil, errw(err)
	}

	props["db_name"] = dbName
	props["version"] = version
	props["current_schema"] = currentSchema

	return props, nil
}

// ListSchemaMetadata implements driver.SQLDriver.
func (d *driveri) ListSchemaMetadata(ctx context.Context, db sqlz.DB) ([]*metadata.Schema, error) {
	// For Oracle, schemas are users, so this is similar to ListSchemas
	// but returns metadata.Schema objects
	schemaNames, err := d.ListSchemas(ctx, db)
	if err != nil {
		return nil, err
	}

	schemas := make([]*metadata.Schema, len(schemaNames))
	for i, name := range schemaNames {
		schemas[i] = &metadata.Schema{
			Name: name,
			// Oracle doesn't have a catalog concept, so leave it empty
		}
	}

	return schemas, nil
}
