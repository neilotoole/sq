// Package parquet implements the sq driver for Apache Parquet files.
// Parquet is a columnar binary format; this driver delegates reads to an
// in-memory DuckDB grip via the bundled "parquet" and "httpfs" extensions.
package parquet

import (
	"context"
	"log/slog"
	"strings"

	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/lg"
	"github.com/neilotoole/sq/libsq/core/lg/lga"
	"github.com/neilotoole/sq/libsq/core/lg/lgm"
	"github.com/neilotoole/sq/libsq/driver"
	"github.com/neilotoole/sq/libsq/files"
	"github.com/neilotoole/sq/libsq/source"
	"github.com/neilotoole/sq/libsq/source/drivertype"
)

// Provider implements driver.Provider.
type Provider struct {
	Log      *slog.Logger
	Registry *driver.Registry
	Files    *files.Files
}

// DriverFor implements driver.Provider.
func (p *Provider) DriverFor(typ drivertype.Type) (driver.Driver, error) {
	if typ != drivertype.Parquet {
		return nil, errz.Errorf("unsupported driver type {%s}", typ)
	}
	return &driveri{
		log:      p.Log,
		registry: p.Registry,
		files:    p.Files,
	}, nil
}

// driveri implements driver.Driver for Parquet files.
type driveri struct {
	log      *slog.Logger
	registry *driver.Registry
	files    *files.Files
}

// DriverMetadata implements driver.Driver.
func (d *driveri) DriverMetadata() driver.Metadata {
	return driver.Metadata{
		Type:        drivertype.Parquet,
		Description: "Apache Parquet",
		Doc:         "https://parquet.apache.org",
		Monotable:   true,
	}
}

// Open implements driver.Driver.
func (d *driveri) Open(ctx context.Context, src *source.Source) (driver.Grip, error) {
	log := lg.FromContext(ctx).With(lga.Src, src)
	log.Debug(lgm.OpenSrc, lga.Src, src)

	parquetPath, dsnQuery, err := parseLocation(src.Location)
	if err != nil {
		return nil, errw(err)
	}

	// Build an in-memory DuckDB source whose DSN forwards the user's options.
	memLoc := "duckdb://:memory:"
	if dsnQuery != "" {
		memLoc += "?" + dsnQuery
	}
	memSrc := &source.Source{
		Type:     drivertype.DuckDB,
		Handle:   src.Handle + "_pq",
		Location: memLoc,
	}

	duckdbDrvr, err := d.registry.DriverFor(drivertype.DuckDB)
	if err != nil {
		return nil, errw(err)
	}

	dbGrip, err := duckdbDrvr.Open(ctx, memSrc)
	if err != nil {
		return nil, errw(err)
	}

	if err := createParquetView(ctx, dbGrip, parquetPath); err != nil {
		_ = dbGrip.Close()
		return nil, err
	}

	log.Debug("Opened parquet source", lga.Src, src)
	return &grip{
		log:    d.log,
		src:    src,
		files:  d.files,
		dbGrip: dbGrip,
	}, nil
}

// createParquetView runs CREATE VIEW "data" AS SELECT * FROM
// read_parquet('<path>') on dbGrip, then forces a DESCRIBE so any parquet
// footer / file-existence errors surface at Open time rather than first
// query. The path is escaped for splicing into a single-quoted SQL literal.
func createParquetView(ctx context.Context, dbGrip driver.Grip, parquetPath string) error {
	db, err := dbGrip.DB(ctx)
	if err != nil {
		return errw(err)
	}

	// parquetPath is user-controlled (it comes from src.Location), but
	// escapeSingleQuotes implements SQL-standard '' escaping; DuckDB does not
	// treat backslash as an escape character in single-quoted string literals,
	// so doubling the quote is sufficient to neutralize any injection attempt.
	//nolint:gosec // G202: path is escaped via escapeSingleQuotes; see comment above.
	stmt := `CREATE VIEW "data" AS SELECT * FROM read_parquet('` +
		escapeSingleQuotes(parquetPath) + `')`
	if _, err := db.ExecContext(ctx, stmt); err != nil {
		return errz.Wrapf(err, "parquet: create view for %q", parquetPath)
	}

	// Force eager footer read so errors surface here rather than at first query.
	if _, err := db.ExecContext(ctx, `DESCRIBE "data"`); err != nil {
		return errz.Wrapf(err, "parquet: describe view for %q", parquetPath)
	}

	return nil
}

// ValidateSource implements driver.Driver.
func (d *driveri) ValidateSource(src *source.Source) (*source.Source, error) {
	d.log.Debug("Validating source", lga.Src, src)
	if src.Type != drivertype.Parquet {
		return nil, errz.Errorf("expected driver type {%s} but got {%s}",
			drivertype.Parquet, src.Type)
	}
	return src, nil
}

// Ping implements driver.Driver.
func (d *driveri) Ping(ctx context.Context, src *source.Source) error {
	return d.files.Ping(ctx, src)
}

// errw wraps err with the package's standard boundary prefix. Errors crossing
// from DuckDB or the filesystem into sq go through here so the stack trace
// anchors at the parquet-side caller.
func errw(err error) error {
	return errz.Wrap(err, "parquet")
}

// parseLocation splits a parquet source location into the file/URL path that
// will be passed to read_parquet(...) and the DSN query string forwarded to
// the underlying DuckDB connection. A "?key=val&..." suffix on the location
// becomes the dsnQuery; everything before it is the path.
func parseLocation(loc string) (path, dsnQuery string, err error) {
	if loc == "" {
		return "", "", errz.New("parquet: location must not be empty")
	}
	if i := strings.LastIndex(loc, "?"); i >= 0 {
		return loc[:i], loc[i+1:], nil
	}
	return loc, "", nil
}

// escapeSingleQuotes doubles every ' in s, suitable for splicing inside a
// SQL single-quoted string literal. This is defense-in-depth: callers should
// have already validated path is a clean file path or parsed URL before
// reaching here.
func escapeSingleQuotes(s string) string {
	return strings.ReplaceAll(s, "'", "''")
}
