package out

import (
	"github.com/neilotoole/sq/libsq/drvr"
	"github.com/neilotoole/sq/libsq/drvr/sqlh"
)

// RecordWriter outputs query results. The caller must invoke Close() when
// all records are written.
type RecordWriter interface {
	Records(records []*sqlh.Record) error
	Close() error
}

// MetadataWriter can output data source metadata.
type MetadataWriter interface {
	Metadata(meta *drvr.SourceMetadata) error
}

// SourceWriter can output data source details.
type SourceWriter interface {
	SourceSet(srcs *drvr.SourceSet, active *drvr.Source) error
	Source(src *drvr.Source) error
}

// ErrorWriter can output errors.
type ErrorWriter interface {
	Error(err error)
}

// HelpWriter can output user help.
type HelpWriter interface {
	Help(help string) error
}
