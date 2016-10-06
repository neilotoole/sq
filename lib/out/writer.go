package out

import (
	"github.com/neilotoole/sq/lib/common"
	"github.com/neilotoole/sq/lib/driver"
)

type ResultWriter interface {
	ResultRows(rows []*common.ResultRow) error
	Metadata(meta *driver.SourceMetadata) error
}
