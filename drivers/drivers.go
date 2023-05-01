// Package drivers is the parent package of the
// concrete sq driver implementations.
package drivers

import "github.com/neilotoole/sq/libsq/core/options"

// OptIngestHeader specifies whether ingested data has a header row or not.
// If not set, the ingester *may* try to detect if the input has a header.
var OptIngestHeader = options.NewBool(
	"ingest.header",
	false,
	`Specifies whether ingested data has a header row or not.
If not set, the ingester *may* try to detect if the input has a header.`,
	"source",
)

// OptIngestSampleSize specifies the number of samples that a detector
// should take to determine type.
var OptIngestSampleSize = options.NewInt(
	"ingest.sample-size",
	1024,
	`Specify the number of samples that a detector should take to determine type.`,
	"source")
