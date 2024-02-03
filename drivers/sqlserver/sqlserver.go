// Package sqlserver implements the sq driver for SQL Server.
package sqlserver

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"strconv"
	"strings"

	"github.com/microsoft/go-mssqldb/msdsn"

	"github.com/neilotoole/sq/libsq/ast"
	"github.com/neilotoole/sq/libsq/ast/render"
	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/jointype"
	"github.com/neilotoole/sq/libsq/core/kind"
	"github.com/neilotoole/sq/libsq/core/lg"
	"github.com/neilotoole/sq/libsq/core/lg/lga"
	"github.com/neilotoole/sq/libsq/core/lg/lgm"
	"github.com/neilotoole/sq/libsq/core/loz"
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
	// dbDrvr is the backing SQL Server driver impl name.
	dbDrvr = "sqlserver"
)

var _ driver.Provider = (*Provider)(nil)

// Provider is the SQL Server implementation of driver.Provider.
type Provider struct {
	Log *slog.Logger
}

// DriverFor implements driver.Provider.
func (p *Provider) DriverFor(typ drivertype.Type) (driver.Driver, error) {
	if typ != drivertype.MSSQL {
		return nil, errz.Errorf("unsupported driver type {%s}}", typ)
	}

	return &driveri{log: p.Log}, nil
}

var _ driver.SQLDriver = (*driveri)(nil)

// driveri is the SQL Server implementation of driver.Driver.
type driveri struct {
	log *slog.Logger
}

// ConnParams implements driver.SQLDriver.
func (d *driveri) ConnParams() map[string][]string {
	// https://github.com/microsoft/go-mssqldb#connection-parameters-and-dsn.
	return map[string][]string{
		"ApplicationIntent":      {"ReadOnly"},
		"ServerSPN":              nil,
		"TrustServerCertificate": {"false", "true"},
		"Workstation ID":         nil,
		"app name":               {"sq"},
		"certificate":            nil,
		"connection timeout":     {"0"},
		"database":               nil,
		"dial timeout":           {"0"},
		"encrypt":                {"disable", "false", "true"},
		"failoverpartner":        nil,
		"failoverport":           {"1433"},
		"hostNameInCertificate":  nil,
		"keepAlive":              {"0", "30"},
		"log":                    {"0", "1", "2", "4", "8", "16", "32", "64", "128", "255"},
		"packet size":            {"512", "4096", "16383", "32767"},
		"protocol":               nil,
		"tlsmin":                 {"1.0", "1.1", "1.2", "1.3"},
		"user id":                nil,
	}
}

// ErrWrapFunc implements driver.SQLDriver.
func (d *driveri) ErrWrapFunc() func(error) error {
	return errw
}

// DriverMetadata implements driver.SQLDriver.
func (d *driveri) DriverMetadata() driver.Metadata {
	return driver.Metadata{
		Type:        drivertype.MSSQL,
		Description: "Microsoft SQL Server / Azure SQL Edge",
		Doc:         "https://github.com/microsoft/go-mssqldb",
		IsSQL:       true,
		DefaultPort: 1433,
	}
}

// Dialect implements driver.SQLDriver.
func (d *driveri) Dialect() dialect.Dialect {
	return dialect.Dialect{
		Type:           drivertype.MSSQL,
		Placeholders:   placeholders,
		Enquote:        stringz.DoubleQuote,
		MaxBatchValues: 1000,
		Ops:            dialect.DefaultOps(),
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
			sb.WriteString("@p")
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

	// Custom functions for SQLServer-specific stuff.
	r.Range = renderRange
	r.PreRender = append(r.PreRender, preRender)

	r.FunctionNames[ast.FuncNameSchema] = "SCHEMA_NAME"
	r.FunctionNames[ast.FuncNameCatalog] = "DB_NAME"
	r.FunctionOverrides[ast.FuncNameRowNum] = renderFuncRowNum

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
	loc := src.Location
	cfg, err := msdsn.Parse(loc)
	if err != nil {
		return nil, errw(err)
	}
	if src.Catalog != "" {
		cfg.Database = src.Catalog
		loc = cfg.URL().String()

		log.Debug("Using catalog as database in connection string",
			lga.Src, src,
			lga.Catalog, src.Catalog,
			lga.Conn, location.Redact(loc),
		)
	}

	cfg.DialTimeout = driver.OptConnOpenTimeout.Get(src.Options)
	loc = cfg.URL().String()

	db, err := sql.Open(dbDrvr, loc)
	if err != nil {
		return nil, errw(err)
	}

	driver.ConfigureDB(ctx, db, src.Options)
	return db, nil
}

// ValidateSource implements driver.Driver.
func (d *driveri) ValidateSource(src *source.Source) (*source.Source, error) {
	if src.Type != drivertype.MSSQL {
		return nil, errz.Errorf("expected driver type %q but got %q", drivertype.MSSQL, src.Type)
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

	err = db.PingContext(ctx)
	return errz.Wrapf(errw(err), "ping %s", src.Handle)
}

// DBProperties implements driver.SQLDriver.
func (d *driveri) DBProperties(ctx context.Context, db sqlz.DB) (map[string]any, error) {
	return getDBProperties(ctx, db)
}

// Truncate implements driver.Driver. Due to a quirk of SQL Server, the
// operation is implemented in two statements. First "DELETE FROM tbl" to
// delete all rows. Then, if reset is true, the table sequence counter
// is reset via RESEED.
//
//nolint:lll
func (d *driveri) Truncate(ctx context.Context, src *source.Source, tbl string, reset bool,
) (affected int64, err error) {
	// https://docs.microsoft.com/en-us/sql/t-sql/statements/truncate-table-transact-sql?view=sql-server-ver15

	// When there are foreign key constraints on mssql tables,
	// it's not possible to TRUNCATE the table. An alternative is
	// to delete all rows and reseed the identity column.
	//
	//  DELETE FROM "table1"; DBCC CHECKIDENT ('table1', RESEED, 1);
	//
	// See: https://stackoverflow.com/questions/253849/cannot-truncate-table-because-it-is-being-referenced-by-a-foreign-key-constraint

	db, err := d.doOpen(ctx, src)
	if err != nil {
		return 0, errw(err)
	}
	defer lg.WarnIfFuncError(d.log, lgm.CloseDB, db.Close)

	affected, err = sqlz.ExecAffected(ctx, db, fmt.Sprintf("DELETE FROM %q", tbl))
	if err != nil {
		return affected, errz.Wrapf(errw(err), "truncate: failed to delete from %q", tbl)
	}

	if reset {
		_, err = db.ExecContext(ctx, fmt.Sprintf("DBCC CHECKIDENT ('%s', RESEED, 1)", tbl))
		if err != nil {
			if hasErrCode(err, errNoIdentityColumn) {
				// The table has no identity column, so we can't reseed.
				lg.FromContext(ctx).Warn("truncate: table has no identity column, so cannot reseed",
					lga.Src, src, lga.Table, tbl, lga.Err, errw(err))
				return affected, nil
			}
			return affected, errz.Wrapf(errw(err), "truncate: deleted %d rows from %q but RESEED failed", affected, tbl)
		}
	}

	return affected, nil
}

// TableColumnTypes implements driver.SQLDriver.
func (d *driveri) TableColumnTypes(ctx context.Context, db sqlz.DB, tblName string,
	colNames []string,
) ([]*sql.ColumnType, error) {
	// SQLServer has this unusual incantation for its LIMIT equivalent:
	//
	// SELECT username, email, address_id FROM person
	// ORDER BY (SELECT 0) OFFSET 0 ROWS FETCH NEXT 1 ROWS ONLY;
	const queryTpl = "SELECT %s FROM %s ORDER BY (SELECT 0) OFFSET 0 ROWS FETCH NEXT 1 ROWS ONLY"

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

	colTypes, err := rows.ColumnTypes()
	if err != nil {
		lg.WarnIfFuncError(d.log, lgm.CloseDBRows, rows.Close)
		return nil, errw(err)
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

// RecordMeta implements driver.SQLDriver.
func (d *driveri) RecordMeta(ctx context.Context, colTypes []*sql.ColumnType) (
	record.Meta, driver.NewRecordFunc, error,
) {
	sColTypeData := make([]*record.ColumnTypeData, len(colTypes))
	ogColNames := make([]string, len(colTypes))
	for i, colType := range colTypes {
		knd := kindFromDBTypeName(d.log, colType.Name(), colType.DatabaseTypeName())
		colTypeData := record.NewColumnTypeData(colType, knd)
		setScanType(colTypeData, knd)
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
		// sqlserver doesn't need to do any special munging, so we
		// just use the default munging.
		rec, skipped := driver.NewRecordFromScanRow(recMeta, vals, nil)
		if len(skipped) > 0 {
			return nil, errz.Errorf("expected zero skipped cols but have %d", skipped)
		}
		return rec, nil
	}

	return recMeta, mungeFn, nil
}

// TableExists implements driver.SQLDriver.
func (d *driveri) TableExists(ctx context.Context, db sqlz.DB, tbl string) (bool, error) {
	const query = `SELECT COUNT(*) FROM information_schema.tables
WHERE table_schema = schema_name() AND table_name = @p1`

	var count int64
	if err := db.QueryRowContext(ctx, query, tbl).Scan(&count); err != nil {
		return false, errw(err)
	}

	return count == 1, nil
}

// CurrentSchema implements driver.SQLDriver.
func (d *driveri) CurrentSchema(ctx context.Context, db sqlz.DB) (string, error) {
	var name string
	if err := db.QueryRowContext(ctx, `SELECT SCHEMA_NAME()`).Scan(&name); err != nil {
		return "", errw(err)
	}

	return name, nil
}

// ListSchemas implements driver.SQLDriver.
func (d *driveri) ListSchemas(ctx context.Context, db sqlz.DB) ([]string, error) {
	log := lg.FromContext(ctx)

	const q = `SELECT name FROM sys.schemas ORDER BY name`
	var schemas []string
	rows, err := db.QueryContext(ctx, q)
	if err != nil {
		return nil, errz.Err(err)
	}
	defer lg.WarnIfCloseError(log, lgm.CloseDBRows, rows)

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

// SchemaExists implements driver.SQLDriver.
func (d *driveri) SchemaExists(ctx context.Context, db sqlz.DB, schma string) (bool, error) {
	if schma == "" {
		return false, nil
	}

	const q = `SELECT COUNT(SCHEMA_NAME) FROM INFORMATION_SCHEMA.SCHEMATA
WHERE SCHEMA_NAME = @p1 AND CATALOG_NAME = DB_NAME()`

	var count int
	return count > 0, errw(db.QueryRowContext(ctx, q, schma).Scan(&count))
}

// ListSchemaMetadata implements driver.SQLDriver.
func (d *driveri) ListSchemaMetadata(ctx context.Context, db sqlz.DB) ([]*metadata.Schema, error) {
	log := lg.FromContext(ctx)

	const q = `SELECT schema_name, catalog_name, schema_owner FROM information_schema.schemata
WHERE catalog_name = DB_NAME()
ORDER BY schema_name`
	var schemas []*metadata.Schema
	rows, err := db.QueryContext(ctx, q)
	if err != nil {
		return nil, errw(err)
	}

	defer lg.WarnIfCloseError(log, lgm.CloseDBRows, rows)

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
	if err := db.QueryRowContext(ctx, `SELECT DB_NAME()`).Scan(&name); err != nil {
		return "", errw(err)
	}

	return name, nil
}

// CatalogExists implements driver.SQLDriver.
func (d *driveri) CatalogExists(ctx context.Context, db sqlz.DB, catalog string) (bool, error) {
	if catalog == "" {
		return false, nil
	}

	const q = `SELECT COUNT(name) FROM sys.databases WHERE name = @p1`

	var count int
	return count > 0, errw(db.QueryRowContext(ctx, q, catalog).Scan(&count))
}

// ListCatalogs implements driver.SQLDriver.
func (d *driveri) ListCatalogs(ctx context.Context, db sqlz.DB) ([]string, error) {
	catalogs := make([]string, 1, 3)
	if err := db.QueryRowContext(ctx, `SELECT DB_NAME()`).Scan(&catalogs[0]); err != nil {
		return nil, errw(err)
	}

	const q = `SELECT name FROM sys.databases WHERE name != DB_NAME() ORDER BY name`

	rows, err := db.QueryContext(ctx, q)
	if err != nil {
		return nil, errw(err)
	}

	defer lg.WarnIfCloseError(lg.FromContext(ctx), lgm.CloseDBRows, rows)

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

// CreateSchema implements driver.SQLDriver.
func (d *driveri) CreateSchema(ctx context.Context, db sqlz.DB, schemaName string) error {
	stmt := `CREATE SCHEMA ` + stringz.DoubleQuote(schemaName)
	if _, err := db.ExecContext(ctx, stmt); err != nil {
		return errz.Wrapf(err, "failed to create schema {%s}", schemaName)
	}

	lg.FromContext(ctx).Debug("Created schema", lga.Schema, schemaName)
	return nil
}

// DropSchema implements driver.SQLDriver.
func (d *driveri) DropSchema(ctx context.Context, db sqlz.DB, schemaName string) error {
	dropObjectsStmt := genDropSchemaObjectsStmt(schemaName)

	if _, err := db.ExecContext(ctx, dropObjectsStmt); err != nil {
		return errz.Wrapf(err, "failed to drop objects in schema {%s}", schemaName)
	}

	dropSchemaStmt := `DROP SCHEMA [` + schemaName + `]`
	if _, err := db.ExecContext(ctx, dropSchemaStmt); err != nil {
		return errz.Wrapf(err, "failed to drop schema {%s}", schemaName)
	}

	lg.FromContext(ctx).Debug("Dropped schema", lga.Schema, schemaName)
	return nil
}

// ListTableNames implements driver.SQLDriver.
func (d *driveri) ListTableNames(ctx context.Context, db sqlz.DB, schma string, tables, views bool) ([]string, error) {
	var tblClause string

	switch {
	case tables && views:
		tblClause = " AND (TABLE_TYPE = 'BASE TABLE' OR TABLE_TYPE = 'VIEW')"
	case tables:
		tblClause = " AND TABLE_TYPE = 'BASE TABLE'"
	case views:
		tblClause = " AND TABLE_TYPE = 'VIEW'"
	default:
		return []string{}, nil
	}

	var args []any
	q := "SELECT TABLE_NAME FROM INFORMATION_SCHEMA.TABLES WHERE table_schema = "
	if schma == "" {
		q += "SCHEMA_NAME()"
	} else {
		q += "@p1"
		args = append(args, schma)
	}
	q += tblClause + " ORDER BY TABLE_NAME"

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

// CreateTable implements driver.SQLDriver.
func (d *driveri) CreateTable(ctx context.Context, db sqlz.DB, tblDef *schema.Table) error {
	stmt := buildCreateTableStmt(tblDef)

	_, err := db.ExecContext(ctx, stmt)
	return errw(err)
}

// AlterTableAddColumn implements driver.SQLDriver.
func (d *driveri) AlterTableAddColumn(ctx context.Context, db sqlz.DB, tbl, col string, knd kind.Kind) error {
	q := fmt.Sprintf("ALTER TABLE %q ADD %q ", tbl, col) + dbTypeNameFromKind(knd)

	_, err := db.ExecContext(ctx, q)
	return errz.Wrapf(errw(err), "alter table: failed to add column %q to table %q", col, tbl)
}

// AlterTableRename implements driver.SQLDriver.
func (d *driveri) AlterTableRename(ctx context.Context, db sqlz.DB, tbl, newName string) error {
	schma, err := d.CurrentSchema(ctx, db)
	if err != nil {
		return err
	}

	q := fmt.Sprintf(`exec sp_rename '[%s].[%s]', '%s'`, schma, tbl, newName)
	_, err = db.ExecContext(ctx, q)
	return errz.Wrapf(errw(err), "alter table: failed to rename table %q to %q", tbl, newName)
}

// AlterTableRenameColumn implements driver.SQLDriver.
func (d *driveri) AlterTableRenameColumn(ctx context.Context, db sqlz.DB, tbl, col, newName string) error {
	schma, err := d.CurrentSchema(ctx, db)
	if err != nil {
		return err
	}

	q := fmt.Sprintf(`exec sp_rename '[%s].[%s].[%s]', '%s'`, schma, tbl, col, newName)
	_, err = db.ExecContext(ctx, q)
	return errz.Wrapf(errw(err), "alter table: failed to rename column {%s.%s.%s} to {%s}", schma, tbl, col, newName)
}

// CopyTable implements driver.SQLDriver.
func (d *driveri) CopyTable(ctx context.Context, db sqlz.DB,
	fromTable, toTable tablefq.T, copyData bool,
) (int64, error) {
	var stmt string

	if copyData {
		stmt = fmt.Sprintf("SELECT * INTO %s FROM %s", tblfmt(toTable), tblfmt(fromTable))
	} else {
		stmt = fmt.Sprintf("SELECT TOP(0) * INTO %s FROM %s", tblfmt(toTable), tblfmt(fromTable))
	}

	affected, err := sqlz.ExecAffected(ctx, db, stmt)
	if err != nil {
		return 0, errw(err)
	}

	return affected, nil
}

// DropTable implements driver.SQLDriver.
func (d *driveri) DropTable(ctx context.Context, db sqlz.DB, tbl tablefq.T, ifExists bool) error {
	var stmt string

	// We don't want the catalog for this part.
	tbl.Catalog = ""
	tblID := tblfmt(tbl)

	if ifExists {
		stmt = fmt.Sprintf("IF OBJECT_ID('%s', 'U') IS NOT NULL DROP TABLE %s", tblID, tblID)
	} else {
		stmt = fmt.Sprintf("DROP TABLE %s", tblID)
	}

	_, err := db.ExecContext(ctx, stmt)
	return errw(err)
}

// PrepareInsertStmt implements driver.SQLDriver.
func (d *driveri) PrepareInsertStmt(ctx context.Context, db sqlz.DB, destTbl string, destColNames []string,
	numRows int,
) (*driver.StmtExecer, error) {
	destColsMeta, err := d.getTableColsMeta(ctx, db, destTbl, destColNames)
	if err != nil {
		return nil, err
	}

	stmt, err := driver.PrepareInsertStmt(ctx, d, db, destTbl, destColsMeta.Names(), numRows)
	if err != nil {
		return nil, err
	}

	execer := driver.NewStmtExecer(stmt, driver.DefaultInsertMungeFunc(destTbl, destColsMeta),
		newStmtExecFunc(stmt, db, destTbl), destColsMeta)
	return execer, nil
}

// PrepareUpdateStmt implements driver.SQLDriver.
func (d *driveri) PrepareUpdateStmt(ctx context.Context, db sqlz.DB, destTbl string, destColNames []string,
	where string,
) (*driver.StmtExecer, error) {
	destColsMeta, err := d.getTableColsMeta(ctx, db, destTbl, destColNames)
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
		newStmtExecFunc(stmt, db, destTbl), destColsMeta)
	return execer, nil
}

func (d *driveri) getTableColsMeta(ctx context.Context, db sqlz.DB, tblName string, colNames []string) (
	record.Meta, error,
) {
	// SQLServer has this unusual incantation for its LIMIT equivalent:
	//
	// SELECT username, email, address_id FROM person
	// ORDER BY (SELECT 0) OFFSET 0 ROWS FETCH NEXT 1 ROWS ONLY;
	const queryTpl = "SELECT %s FROM %s ORDER BY (SELECT 0) OFFSET 0 ROWS FETCH NEXT 1 ROWS ONLY"

	enquote := d.Dialect().Enquote
	tblNameQuoted := enquote(tblName)
	colNamesQuoted := loz.Apply(colNames, enquote)
	colsJoined := strings.Join(colNamesQuoted, driver.Comma)

	query := fmt.Sprintf(queryTpl, colsJoined, tblNameQuoted)
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, errw(err)
	}

	colTypes, err := rows.ColumnTypes()
	if err != nil {
		lg.WarnIfFuncError(d.log, lgm.CloseDBRows, rows.Close)
		return nil, errw(err)
	}

	if rows.Err() != nil {
		return nil, errw(rows.Err())
	}

	destCols, _, err := d.RecordMeta(ctx, colTypes)
	if err != nil {
		lg.WarnIfFuncError(d.log, lgm.CloseDBRows, rows.Close)
		return nil, errw(err)
	}

	err = rows.Close()
	if err != nil {
		return nil, errw(err)
	}

	return destCols, nil
}

// newStmtExecFunc returns a StmtExecFunc that has logic to deal with
// the "identity insert" error. If the error is encountered, setIdentityInsert
// is called and stmt is executed again.
func newStmtExecFunc(stmt *sql.Stmt, db sqlz.DB, tbl string) driver.StmtExecFunc {
	return func(ctx context.Context, args ...any) (int64, error) {
		res, err := stmt.ExecContext(ctx, args...)
		if err == nil {
			var affected int64
			affected, err = res.RowsAffected()
			return affected, errw(err)
		}

		if !hasErrCode(err, errCodeIdentityInsert) {
			return 0, errw(err)
		}

		idErr := setIdentityInsert(ctx, db, tbl, true)
		if idErr != nil {
			return 0, errz.Append(errw(err), idErr)
		}

		res, err = stmt.ExecContext(ctx, args...)
		if err != nil {
			return 0, errw(err)
		}

		affected, err := res.RowsAffected()
		return affected, errw(err)
	}
}

// setIdentityInsert enables (or disables) "identity insert" for tbl on db.
// SQLServer is fussy about inserting values to the identity col. This
// error can be returned from the driver:
//
//	mssql: Cannot insert explicit value for identity column in table 'payment' when IDENTITY_INSERT is set to OFF
//
// The solution is "SET IDENTITY_INSERT tbl ON".
//
// See: https://docs.microsoft.com/en-us/sql/t-sql/statements/set-identity-insert-transact-sql?view=sql-server-ver15
func setIdentityInsert(ctx context.Context, db sqlz.DB, tbl string, on bool) error {
	mode := "ON"
	if !on {
		mode = "OFF"
	}

	query := fmt.Sprintf("SET IDENTITY_INSERT %s %s", tblfmt(tbl), mode)
	_, err := db.ExecContext(ctx, query)
	return errz.Wrapf(errw(err), "failed to SET IDENTITY INSERT %s %s", tblfmt(tbl), mode)
}

// tblfmt formats a table name for use in a query. The arg can be a string,
// or a tablefq.T.
func tblfmt[T string | tablefq.T](tbl T) string {
	tfq := tablefq.From(tbl)
	return tfq.Render(stringz.DoubleQuote)
}

// genDropSchemaObjectsStmt generates a SQL statement that drops all
// objects in the named schema. It is used by driveri.DropSchema.
// This statement is necessary because SQLServer
// doesn't support "DROP SCHEMA [NAME] CASCADE".
// Note that script may not be comprehensive; there could be other
// objects that we haven't considered. But it works on all that
// that's been tested so far.
//
// See: https://stackoverflow.com/a/8150428
//
//nolint:lll
func genDropSchemaObjectsStmt(schemaName string) string {
	const tpl = `
declare @SchemaName nvarchar(100) = '%s'
declare @SchemaID int = schema_id(@SchemaName)

declare @n char(1)
set @n = char(10)
declare @stmt nvarchar(max)

-- procedures
select @stmt = isnull( @stmt + @n, '' ) +
               'drop procedure [' + @SchemaName + '].[' + name + ']'
from sys.procedures where schema_id = @SchemaID


-- check constraints
select @stmt = isnull( @stmt + @n, '' ) +
               'alter table [' + @SchemaName + '].[' + object_name( parent_object_id ) + ']    drop constraint [' + name + ']'
from sys.check_constraints where schema_id = @SchemaID

-- functions
select @stmt = isnull( @stmt + @n, '' ) +
               'drop function [' + @SchemaName + '].[' + name + ']'
from sys.objects
where schema_id = @SchemaID and type in ( 'FN', 'IF', 'TF' )
--
-- views
select @stmt = isnull( @stmt + @n, '' ) +
               'drop view [' + @SchemaName + '].[' + name + ']'
from sys.views  where schema_id = @SchemaID
--
-- foreign keys
select @stmt = isnull( @stmt + @n, '' ) +
               'alter table [' + @SchemaName + '].[' + object_name( parent_object_id ) + '] drop constraint [' + name + ']'
from sys.foreign_keys where schema_id = @SchemaID

-- tables
select @stmt = isnull( @stmt + @n, '' ) +
               'drop table [' + @SchemaName + '].[' + name + ']'
from sys.tables where schema_id = @SchemaID

-- user defined types
select @stmt = isnull( @stmt + @n, '' ) +
               'drop type [' + @SchemaName + '].[' + name + ']'
from sys.types
where schema_id = @SchemaID and is_user_defined = 1

exec sp_executesql @stmt
`

	return fmt.Sprintf(tpl, schemaName)
}
