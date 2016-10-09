package out

import (
	"github.com/neilotoole/sq/lib/common"
	"github.com/neilotoole/sq/lib/drvr"
)

type ResultWriter interface {
	Open() error
	ResultRows(rows []*common.ResultRow) error
	Close() error
	Metadata(meta *drvr.SourceMetadata) error
}

type MetadataWriter interface {
	Metadata(meta *drvr.SourceMetadata) error
}
