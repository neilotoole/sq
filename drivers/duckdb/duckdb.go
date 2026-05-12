package duckdb

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"math/big"
	"time"

	duckdbdriver "github.com/duckdb/duckdb-go/v2" // also registers the "duckdb" sql driver
	"github.com/shopspring/decimal"

	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/kind"
	"github.com/neilotoole/sq/libsq/core/lg"
	"github.com/neilotoole/sq/libsq/core/lg/lga"
	"github.com/neilotoole/sq/libsq/core/lg/lgm"
	"github.com/neilotoole/sq/libsq/core/record"
	"github.com/neilotoole/sq/libsq/core/schema"
	"github.com/neilotoole/sq/libsq/core/sqlz"
	"github.com/neilotoole/sq/libsq/core/tablefq"
	"github.com/neilotoole/sq/libsq/driver"
	"github.com/neilotoole/sq/libsq/source"
	"github.com/neilotoole/sq/libsq/source/drivertype"
	"github.com/neilotoole/sq/libsq/source/metadata"
)

const (
	// dbDrvr is the backing duckdb SQL driver impl name.
	dbDrvr = "duckdb"

	// Prefix is the scheme+separator value "duckdb://".
	Prefix = "duckdb://"
)

var _ driver.Provider = (*Provider)(nil)

// Provider is the DuckDB implementation of driver.Provider.
type Provider struct {
	Log *slog.Logger
}

// DriverFor implements driver.Provider.
func (p *Provider) DriverFor(typ drivertype.Type) (driver.Driver, error) {
	if typ != drivertype.DuckDB {
		return nil, errz.Errorf("unsupported driver type {%s}", typ)
	}
	return &driveri{log: p.Log}, nil
}

var _ driver.SQLDriver = (*driveri)(nil)

// driveri is the DuckDB implementation of driver.SQLDriver.
type driveri struct {
	log *slog.Logger
}

// ConnParams implements driver.SQLDriver.
// Whitelist of DuckDB settings that drive shell completion.
// See https://duckdb.org/docs/configuration/overview.
func (d *driveri) ConnParams() map[string][]string {
	return map[string][]string{
		"access_mode":            {"READ_ONLY", "READ_WRITE"},
		"memory_limit":           nil,
		"threads":                nil,
		"default_order":          {"ASC", "DESC"},
		"default_null_order":     {"NULLS_FIRST", "NULLS_LAST"},
		"enable_external_access": {"true", "false"},
		"enable_object_cache":    {"true", "false"},
		"temp_directory":         nil,
		"wal_autocheckpoint":     nil,
	}
}

// DriverMetadata implements driver.Driver.
func (d *driveri) DriverMetadata() driver.Metadata {
	return driver.Metadata{
		Type:        drivertype.DuckDB,
		Description: "DuckDB",
		Doc:         "https://duckdb.org",
		IsSQL:       true,
	}
}

// Open implements driver.Driver.
func (d *driveri) Open(ctx context.Context, src *source.Source) (driver.Grip, error) {
	lg.FromContext(ctx).Debug(lgm.OpenSrc, lga.Src, src)
	db, err := d.doOpen(ctx, src)
	if err != nil {
		return nil, errz.Err(err)
	}
	if err = driver.OpeningPing(ctx, src, db); err != nil {
		return nil, err
	}
	return &grip{log: d.log, db: db, src: src, drvr: d}, nil
}

func (d *driveri) doOpen(_ context.Context, src *source.Source) (*sql.DB, error) {
	dsn, err := dsnFromLocation(src.Location)
	if err != nil {
		return nil, err
	}
	db, err := sql.Open(dbDrvr, dsn)
	if err != nil {
		return nil, errz.Err(err)
	}
	return db, nil
}

// dsnFromLocation converts an sq location string ("duckdb:///path/to.duckdb?param=val")
// into the DSN form expected by go-duckdb.
// go-duckdb accepts either "" (in-memory) or a file path with optional "?key=val&..."
// query suffix. Strip the "duckdb://" prefix; preserve query string.
func dsnFromLocation(loc string) (string, error) {
	if loc == Prefix || loc == Prefix+":memory:" {
		return "", nil
	}
	if len(loc) < len(Prefix) {
		return "", errz.Errorf("invalid duckdb location: %q", loc)
	}
	return loc[len(Prefix):], nil
}

// Ping implements driver.Driver.
func (d *driveri) Ping(ctx context.Context, src *source.Source) error {
	return errz.New("not implemented")
}

// ValidateSource implements driver.Driver.
func (d *driveri) ValidateSource(src *source.Source) (*source.Source, error) {
	return nil, errz.New("not implemented")
}

// ErrWrapFunc implements driver.SQLDriver.
func (d *driveri) ErrWrapFunc() func(error) error {
	return func(err error) error { return errz.Err(err) }
}

// CurrentSchema implements driver.SQLDriver.
func (d *driveri) CurrentSchema(ctx context.Context, db sqlz.DB) (string, error) {
	var name string
	if err := db.QueryRowContext(ctx, stmtCurrentSchema).Scan(&name); err != nil {
		return "", errz.Err(err)
	}
	return name, nil
}

// ListSchemas implements driver.SQLDriver.
func (d *driveri) ListSchemas(ctx context.Context, db sqlz.DB) ([]string, error) {
	return listSchemas(ctx, db)
}

// ListSchemaMetadata implements driver.SQLDriver.
func (d *driveri) ListSchemaMetadata(ctx context.Context, db sqlz.DB) ([]*metadata.Schema, error) {
	return listSchemaMetadata(ctx, db)
}

// CurrentCatalog implements driver.SQLDriver.
func (d *driveri) CurrentCatalog(ctx context.Context, db sqlz.DB) (string, error) {
	var name string
	if err := db.QueryRowContext(ctx, stmtCurrentCatalog).Scan(&name); err != nil {
		return "", errz.Err(err)
	}
	return name, nil
}

// ListCatalogs implements driver.SQLDriver.
// DuckDB treats each database file as a single catalog; we return only the
// current one.
func (d *driveri) ListCatalogs(ctx context.Context, db sqlz.DB) ([]string, error) {
	name, err := d.CurrentCatalog(ctx, db)
	if err != nil {
		return nil, err
	}
	return []string{name}, nil
}

// TableColumnTypes implements driver.SQLDriver.
func (d *driveri) TableColumnTypes(_ context.Context, _ sqlz.DB, _ string, _ []string) ([]*sql.ColumnType, error) {
	return nil, errz.New("not implemented")
}

// RecordMeta implements driver.SQLDriver.
//
// go-duckdb returns native Go values (not sql.Null* wrappers). NULL columns
// are represented as nil in the driver.Value slice. The munge function
// converts duckdb-specific types (Decimal, Interval, *big.Int, composites)
// to sq's canonical record types.
func (d *driveri) RecordMeta(ctx context.Context, colTypes []*sql.ColumnType) (
	record.Meta, driver.NewRecordFunc, error,
) {
	ctData := make([]*record.ColumnTypeData, len(colTypes))
	ogColNames := make([]string, len(colTypes))
	for i, ct := range colTypes {
		dbTypeName := ct.DatabaseTypeName()
		knd := kindFromDBTypeName(dbTypeName)
		colTypeData := record.NewColumnTypeData(ct, knd)
		// Always use *any as the scan target. go-duckdb delivers native Go
		// values (int32, float32, string, time.Time, duckdb.Decimal, etc.)
		// directly via the database/sql driver protocol. When the scan target
		// is *any, database/sql stores the raw driver value inside the any,
		// which our munge function then normalises into sq canonical types.
		// Using typed scan targets (e.g. *int32) would require us to handle
		// every pointer-to-concrete-type variant, which is fragile.
		colTypeData.ScanType = sqlz.RTypeAny
		ctData[i] = colTypeData
		ogColNames[i] = colTypeData.Name
	}

	mungedNames, err := driver.MungeResultColNames(ctx, ogColNames)
	if err != nil {
		return nil, nil, errz.Err(err)
	}

	recMeta := make(record.Meta, len(colTypes))
	for i := range ctData {
		recMeta[i] = record.NewFieldMeta(ctData[i], mungedNames[i])
	}

	mungeFn := newRecordFuncForDuckDB(recMeta)
	return recMeta, mungeFn, nil
}

// newRecordFuncForDuckDB returns a driver.NewRecordFunc that converts a row of
// raw scan values returned by go-duckdb into a sq record.Record containing only
// canonical types: nil, bool, int64, float64, decimal.Decimal, string, []byte,
// time.Time.
//
// go-duckdb delivers native Go values directly (no sql.Null* wrappers); NULL
// columns arrive as untyped nil. Exotic DuckDB-specific types are converted to
// their closest sq canonical representation.
func newRecordFuncForDuckDB(recMeta record.Meta) driver.NewRecordFunc {
	return func(rowVals []any) (record.Record, error) {
		rec := make(record.Record, len(rowVals))
		for i, val := range rowVals {
			if val == nil {
				rec[i] = nil
				continue
			}

			// Unwrap *any (used when ScanType is RTypeAny / unknown columns).
			if ptr, ok := val.(*any); ok {
				if ptr == nil || *ptr == nil {
					rec[i] = nil
					continue
				}
				val = *ptr
			}

			switch v := val.(type) {
			// ---- boolean ----
			case bool:
				record.SetKindIfUnknown(recMeta, i, kind.Bool)
				rec[i] = v

			// ---- integers (go-duckdb uses fixed-width ints) ----
			case int8:
				record.SetKindIfUnknown(recMeta, i, kind.Int)
				rec[i] = int64(v)
			case int16:
				record.SetKindIfUnknown(recMeta, i, kind.Int)
				rec[i] = int64(v)
			case int32:
				record.SetKindIfUnknown(recMeta, i, kind.Int)
				rec[i] = int64(v)
			case int64:
				record.SetKindIfUnknown(recMeta, i, kind.Int)
				rec[i] = v
			case uint8:
				record.SetKindIfUnknown(recMeta, i, kind.Int)
				rec[i] = int64(v)
			case uint16:
				record.SetKindIfUnknown(recMeta, i, kind.Int)
				rec[i] = int64(v)
			case uint32:
				record.SetKindIfUnknown(recMeta, i, kind.Int)
				rec[i] = int64(v)
			case uint64:
				record.SetKindIfUnknown(recMeta, i, kind.Int)
				// uint64 may overflow int64 for very large values; truncate.
				rec[i] = int64(v) //nolint:gosec

			// ---- HUGEINT / UHUGEINT / BIGNUM (*big.Int) ----
			// These can exceed int64 range; truncate to int64 (best effort).
			case *big.Int:
				record.SetKindIfUnknown(recMeta, i, kind.Int)
				rec[i] = v.Int64()

			// ---- floats ----
			case float32:
				record.SetKindIfUnknown(recMeta, i, kind.Float)
				rec[i] = float64(v)
			case float64:
				record.SetKindIfUnknown(recMeta, i, kind.Float)
				rec[i] = v

			// ---- DECIMAL (duckdb.Decimal → shopspring decimal.Decimal) ----
			case duckdbdriver.Decimal:
				record.SetKindIfUnknown(recMeta, i, kind.Decimal)
				// Convert via the string representation to avoid float precision loss.
				d, err := decimal.NewFromString(v.String())
				if err != nil {
					// Fallback: use float64 approximation.
					rec[i] = decimal.NewFromFloat(v.Float64())
				} else {
					rec[i] = d
				}

			// ---- string / text ----
			case string:
				record.SetKindIfUnknown(recMeta, i, kind.Text)
				rec[i] = v

			// ---- bytes / blob ----
			case []byte:
				if recMeta[i].Kind() == kind.Bytes {
					b := make([]byte, len(v))
					copy(b, v)
					rec[i] = b
				} else {
					// UUID and similar types scan as []byte but are treated as text.
					record.SetKindIfUnknown(recMeta, i, kind.Text)
					rec[i] = string(v)
				}

			// ---- time.Time (TIMESTAMP, DATE, TIME) ----
			case time.Time:
				record.SetKindIfUnknown(recMeta, i, kind.Datetime)
				rec[i] = v

			// ---- INTERVAL (duckdb.Interval → string, deferred to Task 4.4) ----
			case duckdbdriver.Interval:
				record.SetKindIfUnknown(recMeta, i, kind.Text)
				rec[i] = fmt.Sprintf("%d months %d days %d μs", v.Months, v.Days, v.Micros)

			// ---- composite types: LIST ([]any), ARRAY ----
			case []any:
				record.SetKindIfUnknown(recMeta, i, kind.Text)
				rec[i] = fmt.Sprintf("%v", v)

			// ---- STRUCT (map[string]any) ----
			case map[string]any:
				record.SetKindIfUnknown(recMeta, i, kind.Text)
				rec[i] = fmt.Sprintf("%v", v)

			// ---- MAP (duckdb.OrderedMap) ----
			case duckdbdriver.OrderedMap:
				record.SetKindIfUnknown(recMeta, i, kind.Text)
				rec[i] = v.String()

			// ---- UNION (duckdb.Union) ----
			case duckdbdriver.Union:
				record.SetKindIfUnknown(recMeta, i, kind.Text)
				rec[i] = fmt.Sprintf("%v", v.Value)

			default:
				// Unknown type: stringify as best-effort.
				record.SetKindIfUnknown(recMeta, i, kind.Text)
				rec[i] = fmt.Sprintf("%v", v)
			}
		}
		return rec, nil
	}
}

// PrepareInsertStmt implements driver.SQLDriver.
func (d *driveri) PrepareInsertStmt(_ context.Context, _ sqlz.DB, _ string, _ []string, _ int) (*driver.StmtExecer, error) {
	return nil, errz.New("not implemented")
}

// NewBatchInsert implements driver.SQLDriver.
func (d *driveri) NewBatchInsert(_ context.Context, _ string, _ sqlz.DB, _ *source.Source, _ string, _ []string) (*driver.BatchInsert, error) {
	return nil, errz.New("not implemented")
}

// PrepareUpdateStmt implements driver.SQLDriver.
func (d *driveri) PrepareUpdateStmt(_ context.Context, _ sqlz.DB, _ string, _ []string, _ string) (*driver.StmtExecer, error) {
	return nil, errz.New("not implemented")
}

// CreateTable implements driver.SQLDriver.
func (d *driveri) CreateTable(_ context.Context, _ sqlz.DB, _ *schema.Table) error {
	return errz.New("not implemented")
}

// CreateSchema implements driver.SQLDriver.
func (d *driveri) CreateSchema(_ context.Context, _ sqlz.DB, _ string) error {
	return errz.New("not implemented")
}

// DropSchema implements driver.SQLDriver.
func (d *driveri) DropSchema(_ context.Context, _ sqlz.DB, _ string) error {
	return errz.New("not implemented")
}

// CatalogExists implements driver.SQLDriver.
func (d *driveri) CatalogExists(_ context.Context, _ sqlz.DB, _ string) (bool, error) {
	return false, errz.New("not implemented")
}

// SchemaExists implements driver.SQLDriver.
func (d *driveri) SchemaExists(ctx context.Context, db sqlz.DB, schma string) (bool, error) {
	return schemaExists(ctx, db, schma)
}

// Truncate implements driver.SQLDriver.
func (d *driveri) Truncate(_ context.Context, _ *source.Source, _ string, _ bool) (int64, error) {
	return 0, errz.New("not implemented")
}

// TableExists implements driver.SQLDriver.
func (d *driveri) TableExists(ctx context.Context, db sqlz.DB, tbl string) (bool, error) {
	return tableExists(ctx, db, tbl)
}

// ListTableNames implements driver.SQLDriver.
func (d *driveri) ListTableNames(ctx context.Context, db sqlz.DB, schma string, tables, views bool) ([]string, error) {
	return listTableNames(ctx, db, schma, tables, views)
}

// CopyTable implements driver.SQLDriver.
func (d *driveri) CopyTable(_ context.Context, _ sqlz.DB, _, _ tablefq.T, _ bool) (int64, error) {
	return 0, errz.New("not implemented")
}

// DropTable implements driver.SQLDriver.
func (d *driveri) DropTable(_ context.Context, _ sqlz.DB, _ tablefq.T, _ bool) error {
	return errz.New("not implemented")
}

// AlterTableRename implements driver.SQLDriver.
func (d *driveri) AlterTableRename(ctx context.Context, db sqlz.DB, oldName, newName string) error {
	return alterTableRename(ctx, db, oldName, newName)
}

// AlterTableAddColumn implements driver.SQLDriver.
func (d *driveri) AlterTableAddColumn(ctx context.Context, db sqlz.DB, tblName, colName string, k kind.Kind) error {
	return alterTableAddColumn(ctx, db, tblName, colName, k)
}

// AlterTableRenameColumn implements driver.SQLDriver.
func (d *driveri) AlterTableRenameColumn(ctx context.Context, db sqlz.DB, tblName, oldCol, newCol string) error {
	return alterTableRenameColumn(ctx, db, tblName, oldCol, newCol)
}

// AlterTableColumnKinds implements driver.SQLDriver.
func (d *driveri) AlterTableColumnKinds(
	ctx context.Context, db sqlz.DB, tblName string, colNames []string, kinds []kind.Kind,
) error {
	return alterTableColumnKinds(ctx, db, tblName, colNames, kinds)
}

// DBProperties implements driver.SQLDriver.
func (d *driveri) DBProperties(_ context.Context, _ sqlz.DB) (map[string]any, error) {
	return nil, errz.New("not implemented")
}

// grip is a minimal placeholder for the DuckDB Grip implementation.
// It will be replaced by a full implementation in Task 1.4.
type grip struct {
	log  *slog.Logger
	db   *sql.DB
	src  *source.Source
	drvr *driveri
}

var _ driver.Grip = (*grip)(nil)

// DB implements driver.Grip.
func (g *grip) DB(_ context.Context) (*sql.DB, error) {
	return g.db, nil
}

// SQLDriver implements driver.Grip.
func (g *grip) SQLDriver() driver.SQLDriver {
	return g.drvr
}

// Source implements driver.Grip.
func (g *grip) Source() *source.Source {
	return g.src
}

// SourceMetadata implements driver.Grip.
func (g *grip) SourceMetadata(ctx context.Context, noSchema bool) (*metadata.Source, error) {
	return getSourceMetadata(ctx, g.src, g.db, noSchema)
}

// TableMetadata implements driver.Grip.
func (g *grip) TableMetadata(ctx context.Context, tblName string) (*metadata.Table, error) {
	schemaName, err := g.drvr.CurrentSchema(ctx, g.db)
	if err != nil {
		return nil, err
	}
	return getTableMetadata(ctx, g.db, schemaName, tblName)
}

// Close implements driver.Grip.
func (g *grip) Close() error {
	return errz.Err(g.db.Close())
}
