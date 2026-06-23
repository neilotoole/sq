package parquet

import (
	"context"
	"database/sql"
	"log/slog"
	"net/url"
	"path"

	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/lg/lga"
	"github.com/neilotoole/sq/libsq/core/lg/lgm"
	"github.com/neilotoole/sq/libsq/driver"
	"github.com/neilotoole/sq/libsq/files"
	"github.com/neilotoole/sq/libsq/source"
	"github.com/neilotoole/sq/libsq/source/drivertype"
	"github.com/neilotoole/sq/libsq/source/location"
	"github.com/neilotoole/sq/libsq/source/metadata"
)

var _ driver.Grip = (*grip)(nil)

// grip implements driver.Grip for a Parquet source. It wraps an in-memory
// DuckDB grip that holds a "data" view pointing at read_parquet(<path>).
// SourceMetadata is rewritten so the source reports as drivertype.Parquet
// rather than the underlying DuckDB.
type grip struct {
	log    *slog.Logger
	src    *source.Source
	files  *files.Files
	dbGrip driver.Grip
}

// DB implements driver.Grip.
func (g *grip) DB(ctx context.Context) (*sql.DB, error) {
	db, err := g.dbGrip.DB(ctx)
	return db, errw(err)
}

// SQLDriver implements driver.Grip.
func (g *grip) SQLDriver() driver.SQLDriver {
	return g.dbGrip.SQLDriver()
}

// Source implements driver.Grip.
func (g *grip) Source() *source.Source {
	return g.src
}

// SourceMetadata implements driver.Grip.
func (g *grip) SourceMetadata(ctx context.Context, noSchema bool) (*metadata.Source, error) {
	md, err := g.dbGrip.SourceMetadata(ctx, noSchema)
	if err != nil {
		return nil, errw(err)
	}

	md.Handle = g.src.Handle
	md.Driver = drivertype.Parquet
	md.Location = g.src.Location

	if isNonHTTPRemote(g.src.Location) {
		// location.Filename and files.Filesize treat non-HTTP remotes
		// (s3://, gs://, etc.) as local file paths and fail. Derive the name
		// from the URL path instead, and leave Size nil: determining the
		// object size would require re-reading the remote via DuckDB httpfs.
		md.Name = remoteFileName(g.src.Location)
		md.FQName = md.Name
		return md, nil
	}

	if md.Name, err = location.Filename(g.src.Location); err != nil {
		return nil, err
	}
	md.FQName = md.Name

	var size int64
	if size, err = g.files.Filesize(ctx, g.src); err != nil {
		return nil, err
	}
	md.Size = &size

	return md, nil
}

// remoteFileName returns the final path segment of a remote URL location,
// stripping any query string. It falls back to loc itself if the URL does
// not parse or has no path.
func remoteFileName(loc string) string {
	u, err := url.Parse(loc)
	if err != nil || u.Path == "" || u.Path == "/" {
		return loc
	}
	return path.Base(u.Path)
}

// TableMetadata implements driver.Grip.
func (g *grip) TableMetadata(ctx context.Context, tblName string) (*metadata.Table, error) {
	if tblName != source.MonotableName {
		return nil, errz.Errorf("parquet: table name should be %q, but got: %s",
			source.MonotableName, tblName)
	}
	md, err := g.dbGrip.TableMetadata(ctx, tblName)
	return md, errw(err)
}

// Close implements driver.Grip.
func (g *grip) Close() error {
	g.log.Debug(lgm.CloseDB, lga.Handle, g.src.Handle)
	return errw(g.dbGrip.Close())
}
