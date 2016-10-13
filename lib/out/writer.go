package out

import (
	"github.com/neilotoole/sq/lib/common"
	"github.com/neilotoole/sq/lib/drvr"
)

// ResultWriter can output query results. The caller must invoke Close() when
// all rows are written.
type ResultWriter interface {
	ResultRows(rows []*common.ResultRow) error
	Close() error
	Metadata(meta *drvr.SourceMetadata) error // TODO: duplicate
}

// MetadataWriter can output data source metadata.
type MetadataWriter interface {
	Metadata(meta *drvr.SourceMetadata) error
}

// SourceWriter can output data source details.
type SourceWriter interface {
	Sources(srcs *drvr.SourceSet) error
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
