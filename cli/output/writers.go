// Package output provides interfaces and implementations for
// outputting data and messages. All sq command output should be
// via one of the writer interfaces defined in this package.
// The RecordWriterAdapter type provides a bridge between the
// asynchronous libsq.RecordWriter interface and the simpler
// synchronous RecordWriter interface defined here.
package output

import (
	"io"
	"time"

	"github.com/neilotoole/sq/cli/buildinfo"
	"github.com/neilotoole/sq/cli/hostinfo"
	"github.com/neilotoole/sq/libsq/core/options"
	"github.com/neilotoole/sq/libsq/core/record"
	"github.com/neilotoole/sq/libsq/driver"
	"github.com/neilotoole/sq/libsq/source"
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
	Open(recMeta record.Meta) error

	// WriteRecords writes rec to the destination.
	WriteRecords(recs []record.Record) error

	// Flush advises the writer to flush any internal
	// buffer. Note that the writer may implement an independent
	// flushing strategy, or may not buffer at all.
	Flush() error

	// Close closes the writer after flushing any internal buffer.
	Close() error
}

// MetadataWriter can output metadata.
type MetadataWriter interface {
	// TableMetadata writes the table metadata.
	TableMetadata(tblMeta *source.TableMetadata) error

	// SourceMetadata writes the source metadata.
	SourceMetadata(srcMeta *source.Metadata, showSchema bool) error

	// DBProperties writes the DB properties.
	DBProperties(props map[string]any) error

	// DriverMetadata writes the metadata for the drivers.
	DriverMetadata(drvrs []driver.Metadata) error
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
	// Error outputs err.
	Error(err error)
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
	Version(bi buildinfo.BuildInfo, latestVersion string, si hostinfo.Info) error
}

// ConfigWriter prints config.
type ConfigWriter interface {
	// Location prints the config location. The origin may be empty, or one
	// of "flag", "env", "default".
	Location(loc, origin string) error

	// Opt prints a single options.Opt.
	Opt(o options.Options, opt options.Opt) error

	// Options prints config options.
	Options(reg *options.Registry, o options.Options) error

	// SetOption is called when an option is set.
	SetOption(o options.Options, opt options.Opt) error

	// UnsetOption is called when an option is unset.
	UnsetOption(opt options.Opt) error
}

// Writers is a container for the various output Writers.
type Writers struct {
	Printing *Printing

	Record   RecordWriter
	Metadata MetadataWriter
	Source   SourceWriter
	Error    ErrorWriter
	Ping     PingWriter
	Version  VersionWriter
	Config   ConfigWriter
}

// NewRecordWriterFunc is a func type that returns an output.RecordWriter.
type NewRecordWriterFunc func(out io.Writer, pr *Printing) RecordWriter
