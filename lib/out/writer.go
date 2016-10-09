package out

import (
	"github.com/neilotoole/sq/lib/common"
	"github.com/neilotoole/sq/lib/driver"
)

type ResultWriter interface {
	Open() error
	ResultRows(rows []*common.ResultRow) error
	Close() error
	Metadata(meta *driver.SourceMetadata) error
}

type MetadataWriter interface {
	Metadata(meta *driver.SourceMetadata) error
}
