// Package output provides interfaces and implementations for
// outputting data and messages. All sq command output should be
// via one of the writer interfaces defined in this package.
// The RecordWriterAdapter type provides a bridge between the
// asynchronous libsq.RecordWriter interface and the simpler
// synchronous RecordWriter interface defined here.
package output

import (
	"context"
	"io"
	"time"

	"github.com/neilotoole/sq/cli/buildinfo"
	"github.com/neilotoole/sq/cli/config"
	"github.com/neilotoole/sq/cli/hostinfo"
	"github.com/neilotoole/sq/libsq/core/options"
	"github.com/neilotoole/sq/libsq/core/record"
	"github.com/neilotoole/sq/libsq/driver"
	"github.com/neilotoole/sq/libsq/source"
	"github.com/neilotoole/sq/libsq/source/metadata"
)

// RecordWriter is an interface for writing records to a destination.
// In effect, it is a synchronous counterpart to the asynchronous
// libsq.RecordWriter interface. Being a synchronous interface, it is
// less tricky to implement than libsq.RecordWriter. The RecordWriterAdapter
// type defined in this package bridges the two interfaces.
//
// The Open method must be invoked before WriteRecords. Close must be
// invoked when all records are written. The Flush method advises the
// writer to flush any internal buffers.
type RecordWriter interface {
	// Open instructs the writer to prepare to write records
	// described by recMeta.
	Open(ctx context.Context, recMeta record.Meta) error

	// WriteRecords writes rec to the destination.
	WriteRecords(ctx context.Context, recs []record.Record) error

	// Flush advises the writer to flush any internal
	// buffer. Note that the writer may implement an independent
	// flushing strategy, or may not buffer at all.
	Flush(ctx context.Context) error

	// Close closes the writer after flushing any internal buffer.
	Close(ctx context.Context) error
}

// RecordInsertWriter outputs details of record insertion into a destination
// table.
//
// One could ask: why not add the RecordInsertWriter.RecordsInserted method to
// the RecordWriter interface, instead of creating a new interface? This is
// because RecordInsertWriter is effectively a user-facing logger, for which sq
// only guarantees text (tablew) and JSON (jsonw). It doesn't really make sense
// to force the Excel writer (xlsxw) to implement a "N rows affected" mechanism.
//
// Note that RecordInsertWriter is distinct from StmtExecWriter. StmtExecWriter
// generically outputs the details of any SQL statement execution, which could
// be an INSERT, but also could be UPDATE, CREATE, etc.). Meanwhile,
// RecordInsertWriter outputs the results of the "sq --insert" mechanism, which
// pipes the results (records) of a query to a destination source/table.
type RecordInsertWriter interface {
	// RecordsInserted outputs record insertion details, indicating that a count
	// of rowsInserted rows were inserted into tbl in destination target.
	RecordsInserted(ctx context.Context, target *source.Source, tbl string,
		rowsInserted int64, elapsed time.Duration) error
}

// StmtExecWriter outputs details of a successfully executed SQL statement.
//
// Note that StmtExecWriter is distinct from RecordInsertWriter. StmtExecWriter
// generically outputs the details of any SQL statement execution, which could
// be an INSERT, but also could be UPDATE, CREATE, etc.). Meanwhile,
// RecordInsertWriter outputs the results of the "sq --insert" mechanism, which
// pipes the results (records) of a query to a destination source/table.
type StmtExecWriter interface {
	// StmtExecuted writes SQL statement execution details.
	StmtExecuted(ctx context.Context, target *source.Source, affected int64, elapsed time.Duration) error
}

// MetadataWriter can output metadata.
type MetadataWriter interface {
	// TableMetadata writes the table metadata.
	TableMetadata(tblMeta *metadata.Table) error

	// SourceMetadata writes the source metadata.
	SourceMetadata(srcMeta *metadata.Source, showSchema bool) error

	// DBProperties writes the DB properties.
	DBProperties(props map[string]any) error

	// DriverMetadata writes the metadata for the drivers.
	DriverMetadata(drvrs []driver.Metadata) error

	// Catalogs writes the list of catalogs.
	Catalogs(currentCatalog string, catalogs []string) error

	// Schemata writes the list of schemas.
	Schemata(currentSchema string, schemas []*metadata.Schema) error
}

// SourceWriter can output data source details.
type SourceWriter interface {
	// Collection outputs details of the collection. Specifically it prints
	// the sources from coll's active group.
	Collection(coll *source.Collection) error

	// Source outputs details of the source.
	Source(coll *source.Collection, src *source.Source) error

	// Added is called when src is added to the collection.
	Added(coll *source.Collection, src *source.Source) error

	// Removed is called when sources are removed from the collection.
	Removed(srcs ...*source.Source) error

	// Moved is called when a source is moved from old to nu.
	Moved(coll *source.Collection, old, nu *source.Source) error

	// Group prints the group.
	Group(group *source.Group) error

	// SetActiveGroup is called when the group is set.
	SetActiveGroup(group *source.Group) error

	// Groups prints a list of groups.
	Groups(tree *source.Group) error
}

// ErrorWriter outputs errors.
type ErrorWriter interface {
	// Error outputs error conditions. It's possible that systemErr and
	// humanErr differ; systemErr is the error that occurred, and humanErr
	// is the error that should be presented to the user.
	Error(systemErr, humanErr error)
}

// PingWriter writes ping results.
type PingWriter interface {
	// Open opens the writer to write the supplied sources.
	Open(srcs []*source.Source) error

	// Result prints a ping result. The ping succeeded if
	// err is nil. If err is context.DeadlineExceeded, the d
	// arg will be the timeout value.
	Result(src *source.Source, d time.Duration, err error) error

	// Close is called after all results have been received.
	Close() error
}

// VersionWriter prints the CLI version.
type VersionWriter interface {
	// Version prints version info. Arg latestVersion is the latest
	// version available from the homebrew repository. The value
	// may be empty.
	Version(bi buildinfo.Info, latestVersion string, si hostinfo.Info) error
}

// ConfigWriter prints config.
type ConfigWriter interface {
	// Location prints the config location. The origin may be empty, or one
	// of "flag", "env", "default".
	Location(loc string, origin config.Origin) error

	// Opt prints a single options.Opt.
	Opt(o options.Options, opt options.Opt) error

	// Options prints config options.
	Options(reg *options.Registry, o options.Options) error

	// SetOption is called when an option is set.
	SetOption(o options.Options, opt options.Opt) error

	// UnsetOption is called when an option is unset.
	UnsetOption(opt options.Opt) error

	// CacheLocation prints the cache location.
	CacheLocation(loc string) error

	// CacheStat prints cache info. Set arg size to -1 to indicate
	// that the size of the cache could not be calculated.
	CacheStat(loc string, enabled bool, size int64) error
}

// Writers is a container for the various output Writers.
type Writers struct {
	// PrOut is the printing config for stdout.
	PrOut *Printing

	// PrErr is the printing config for stderr.
	PrErr *Printing

	Record       RecordWriter
	RecordInsert RecordInsertWriter
	StmtExec     StmtExecWriter
	Metadata     MetadataWriter
	Source       SourceWriter
	Error        ErrorWriter
	Ping         PingWriter
	Version      VersionWriter
	Config       ConfigWriter
	SQL          SQLWriter
	Keyring      KeyringWriter
}

// KeyringRef is one row of "sq config keyring ls" output. Each row
// describes one keyring entry: either a ${keyring:<path>} reference
// reachable from the active source collection, or an entry found in the
// OS keyring. Status classifies the row.
type KeyringRef struct {
	Status string `json:"status"`
	Path   string `json:"path"`
	Handle string `json:"handle"`
	Driver string `json:"driver"`
}

// KeyringStatus* enumerate the status values surfaced by
// KeyringWriter.List. The string forms are part of the JSON contract
// and must not change casually.
const (
	KeyringStatusReferenced = "referenced" // in config and present in the keyring
	KeyringStatusOrphan     = "orphan"     // in the keyring, referenced by no source
	KeyringStatusMissing    = "missing"    // referenced by a source, absent from the keyring
)

// KeyringMigrateStatus enumerates the status values surfaced by
// KeyringWriter.Migrate. The string forms are part of the JSON
// contract and must not change casually.
const (
	KeyringMigrateStatusPlanned  = "planned"  // dry-run: source would be migrated
	KeyringMigrateStatusSkip     = "skip"     // not eligible (no password, malformed, etc.)
	KeyringMigrateStatusMigrated = "migrated" // applied: keyring entry written, YAML updated
	KeyringMigrateStatusFailed   = "failed"   // applied: a step failed (mint/write/save)
)

// KeyringMigrateRow describes one source's outcome in a migrate plan
// (dry-run) or migrate result (applied). Status takes one of the
// KeyringMigrateStatus* constants.
type KeyringMigrateRow struct {
	Handle      string `json:"handle"`
	Status      string `json:"status"`
	Reason      string `json:"reason,omitempty"`       // populated for "skip"
	NewLocation string `json:"new_location,omitempty"` // populated for "migrated"
	Error       string `json:"error,omitempty"`        // populated for "failed"
}

// KeyringPruneStatus* enumerate the status values surfaced by
// KeyringWriter.Prune. The string forms are part of the JSON contract.
const (
	KeyringPruneStatusPlanned = "planned" // dry-run: entry would be deleted
	KeyringPruneStatusDeleted = "deleted" // applied: entry was deleted
	KeyringPruneStatusFailed  = "failed"  // applied: deletion failed
)

// KeyringKind* classify a keyring entry's path shape, for display only.
const (
	KeyringKindID    = "id"    // sq-minted opaque Crockford ID
	KeyringKindNamed = "named" // user-named entry
)

// KeyringPruneRow describes one orphaned entry in a prune plan (dry-run)
// or result (applied). Kind is informational; Status takes one of the
// KeyringPruneStatus* constants.
type KeyringPruneRow struct {
	Path   string `json:"path"`
	Kind   string `json:"kind"`
	Status string `json:"status"`
	Error  string `json:"error,omitempty"` // populated for "failed"
}

// KeyringWriter prints output for the "sq config keyring" command group.
// Implementations live in cli/output/tablew (text/table) and
// cli/output/jsonw (JSON).
type KeyringWriter interface {
	// List prints the result of "sq config keyring ls".
	List(refs []KeyringRef) error

	// Get prints the result of "sq config keyring get". When revealed
	// is false, the writer should omit the secret value (printing only
	// metadata) so callers can pass it through verbatim regardless of
	// the --reveal flag.
	Get(path, value string, revealed bool) error

	// Created prints confirmation of "sq config keyring create".
	Created(path string) error

	// Updated prints confirmation of "sq config keyring update".
	Updated(path string) error

	// Rm prints confirmation of "sq config keyring rm". Deleting a
	// non-existent entry still calls this — rm is idempotent.
	Rm(path string) error

	// Migrate prints per-source migration outcomes. When dryRun is true
	// the rows describe a plan (statuses: "planned" or "skip"); when
	// false the rows describe applied outcomes (statuses: "migrated",
	// "skip", or "failed").
	Migrate(rows []KeyringMigrateRow, dryRun bool) error

	// Prune prints the orphaned entries removed by "sq config keyring
	// prune". When dryRun is true every row's status is "planned"; when
	// false no row is "planned" — each is "deleted" or "failed".
	Prune(rows []KeyringPruneRow, dryRun bool) error
}

// NewRecordWriterFunc is a func type that returns an output.RecordWriter.
type NewRecordWriterFunc func(out io.Writer, pr *Printing) RecordWriter
