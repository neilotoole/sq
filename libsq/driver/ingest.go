package driver

import (
	"context"

	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/options"
	"github.com/neilotoole/sq/libsq/core/stringz"
)

// OptIngestHeader specifies whether ingested data has a header row or not.
// If not set, the ingester *may* try to detect if the input has a header.
var OptIngestHeader = options.NewBool(
	"ingest.header",
	"",
	false,
	0,
	false,
	"Ingest data has a header row",
	`Specifies whether ingested data has a header row or not.
If not set, the ingester *may* try to detect if the input has a header.
Generally it is best to leave this option unset and allow the ingester
to detect the header.`,
	options.TagSource,
	options.TagIngestMutate,
)

// OptIngestCache specifies whether ingested data is cached or not.
var OptIngestCache = options.NewBool(
	"ingest.cache",
	"",
	false,
	0,
	true,
	"Ingest data is cached",
	`Specifies whether ingested data is cached or not. When data is ingested
from a document source, it is stored in a cache DB. Subsequent uses of that same
source will use that cached DB instead of ingesting the data again, unless this
option is set to false, in which case, the data is ingested each time.`,
	options.TagSource,
)

// OptIngestSampleSize specifies the number of samples that a detector
// should take to determine ingest data type.
var OptIngestSampleSize = options.NewInt(
	"ingest.sample-size",
	"",
	0,
	256,
	"Ingest data sample size for type detection",
	`Specify the number of samples that a detector should take to determine type.`,
	options.TagSource,
	options.TagIngestMutate,
)

// OptIngestColRename transforms a column name in ingested data.
var OptIngestColRename = options.NewString(
	"ingest.column.rename",
	"",
	0,
	"{{.Name}}{{with .Recurrence}}_{{.}}{{end}}",
	func(s string) error {
		return stringz.ValidTemplate("ingest.column.rename", s)
	},
	"Template to rename ingest columns",
	`This Go text template is executed on ingested column names.
Its primary purpose is to rename duplicate header column names in the
ingested data. For example, given a CSV file with header row:

  actor_id, first_name, actor_id

The default template renames the columns to:

  actor_id, first_name, actor_id_1

The fields available in the template are:

  .Name         column header name
  .Index        zero-based index of the column in the header row
  .Alpha        alphabetical index of the column, e.g. [A, B ... Z, AA, AB]
  .Recurrence   nth recurrence of the colum name in the header row

For a unique column name, e.g. "first_name" above, ".Recurrence" will be 0.
For duplicate column names, ".Recurrence" will be 0 for the first instance,
then 1 for the next instance, and so on.`,
	options.TagSource,
	options.TagIngestMutate,
)

// MungeIngestColNames transforms ingest data column names, per the template
// defined in the option driver.OptIngestColRename found on the context.
// It is the ingest counterpart of MungeResultColNames.
//
// For example, given a CSV file with header [actor_id, first_name, actor_id],
// the columns might be renamed to [actor_id, first_name, actor_id_1].
//
// MungeIngestColNames should be invoked by each ingester impl that may
// encounter duplicate col names in the ingest data.
func MungeIngestColNames(ctx context.Context, ogColNames []string) (colNames []string, err error) {
	if len(ogColNames) == 0 {
		return ogColNames, nil
	}

	o := options.FromContext(ctx)
	tplText := OptIngestColRename.Get(o)
	if tplText == "" {
		return ogColNames, nil
	}

	tpl, err := stringz.NewTemplate(OptIngestColRename.Key(), tplText)
	if err != nil {
		return nil, errz.Wrap(err, "config: ")
	}

	return doMungeColNames(tpl, ogColNames)
}
