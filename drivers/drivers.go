// Package drivers is the parent package of the
// concrete sq driver implementations.
package drivers

import "github.com/neilotoole/sq/libsq/core/options"

// OptIngestHeader specifies whether ingested data has a header row or not.
// If not set, the ingester *may* try to detect if the input has a header.
var OptIngestHeader = options.NewBool(
	"ingest.header",
	"",
	0,
	false,
	"Ingest data has a header row",
	`Specifies whether ingested data has a header row or not.
If not set, the ingester *may* try to detect if the input has a header.
Generally it is best to leave this option unset and allow the ingester
to detect the header.`,
	"source",
)

// OptIngestSampleSize specifies the number of samples that a detector
// should take to determine type.
var OptIngestSampleSize = options.NewInt(
	"ingest.sample-size",
	"",
	0,
	1024,
	"Ingest data sample size for type detection",
	`Specify the number of samples that a detector should take to determine type.`,
	"source",
)
