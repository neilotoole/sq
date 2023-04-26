// Package drivers is the parent package of the
// concrete sq driver implementations.
package drivers

import "github.com/neilotoole/sq/libsq/core/options"

// OptIngestHeader specifies whether ingested data has a header row or not.
// If not set, the ingester *may* try to detect if the input has a header.
var OptIngestHeader = options.NewBool(
	"ingest.header",
	false,
	"",
	"source",
)
