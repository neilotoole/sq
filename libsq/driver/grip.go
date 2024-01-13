package driver

import (
	"context"
	"database/sql"

	"github.com/neilotoole/sq/libsq/source"
	"github.com/neilotoole/sq/libsq/source/metadata"
)

// Grip is the link between a source and its database connection.
// Why is it named Grip? TLDR: all the other names were taken,
// including Handle, Conn, DB, Source, etc.
//
// Grip is conceptually equivalent to stdlib sql.DB, and in fact
// encapsulates a sql.DB instance. The realized sql.DB instance can be
// accessed via the DB method.
type Grip interface {
	// DB returns the sql.DB object for this Grip.
	// This operation may take a long time if opening the DB requires
	// an ingest of data (but note that when an ingest step occurs is
	// driver-dependent).
	DB(ctx context.Context) (*sql.DB, error)

	// SQLDriver returns the underlying database driver. The type of the SQLDriver
	// may be different from the driver type reported by the Source.
	SQLDriver() SQLDriver

	// FIXME: Add a method: SourceDriver() Driver.

	// Source returns the source for which this Grip was opened.
	Source() *source.Source

	// SourceMetadata returns metadata about the Grip.
	// If noSchema is true, schema details are not populated
	// on the returned metadata.Source.
	SourceMetadata(ctx context.Context, noSchema bool) (*metadata.Source, error)

	// TableMetadata returns metadata for the specified table in the Grip.
	TableMetadata(ctx context.Context, tblName string) (*metadata.Table, error)

	// Close is invoked to close and release any underlying resources.
	Close() error
}

// GripOpenIngester opens a Grip via an ingest function.
type GripOpenIngester interface {
	// OpenIngest opens a Grip for src by executing ingestFn, which is
	// responsible for ingesting data into dest. If allowCache is false,
	// ingest always occurs; if true, the cache is consulted first (and
	// ingestFn may not be invoked).
	OpenIngest(ctx context.Context, src *source.Source, allowCache bool,
		ingestFn func(ctx context.Context, dest Grip) error) (Grip, error)
}
