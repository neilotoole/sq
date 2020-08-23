// Package output provides interfaces and implementations for
// outputting data and messages. All sq command output should be
// via one of the writer interfaces defined in this package.
// The RecordWriterAdapter type provides a bridge between the
// asynchronous libsq.RecordWriter interface and the simpler
// synchronous RecordWriter interface defined here.
package output

import (
	"time"

	"github.com/neilotoole/sq/libsq/core/sqlz"
	"github.com/neilotoole/sq/libsq/driver"
	"github.com/neilotoole/sq/libsq/notify"
	"github.com/neilotoole/sq/libsq/source"
)

// RecordWriter is an interface for writing records to a destination.
// In effect it is a synchronous counterpart to the asynchronous
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
	Open(recMeta sqlz.RecordMeta) error

	// WriteRecords writes rec to the destination.
	WriteRecords(recs []sqlz.Record) error

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
	SourceMetadata(srcMeta *source.Metadata) error

	// DriverMetadata writes the metadata for the drivers.
	DriverMetadata(drvrs []driver.Metadata) error
}

// SourceWriter can output data source details.
type SourceWriter interface {
	// SourceSet outputs details of the source set.
	SourceSet(srcs *source.Set) error

	// Source outputs details of the source.
	Source(src *source.Source) error
}

// NotificationWriter outputs notification destination details.
type NotificationWriter interface {
	// NotifyDestinations outputs details of the notification
	// destinations.
	NotifyDestinations(dests []notify.Destination) error
}

// ErrorWriter outputs errors.
type ErrorWriter interface {
	// Error outputs err.
	Error(err error)
}

// PingWriter writes ping results.
type PingWriter interface {
	// Open opens the writer to write the supplied sources.
	Open(srcs []*source.Source)

	// Result prints a ping result. The ping succeeded if
	// err is nil. If err is context.DeadlineExceeded, the d
	// arg will be the timeout value.
	Result(src *source.Source, d time.Duration, err error)

	// Close is called after all results have been received.
	Close() error
}

// FlushThreshold is the size in bytes after which a writer
// should flush any internal buffer.
const FlushThreshold = 1000
