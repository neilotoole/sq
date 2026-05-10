package driver

import (
	"strings"

	"github.com/neilotoole/sq/libsq/core/errz"
)

// ResolveTableColumnsFold returns a slice the same length as src, where each
// entry is the canonical-case match for the corresponding entry in src,
// looked up in actual via case-insensitive comparison. The order of src is
// preserved. It returns an error naming the first src entry that has no
// match in actual.
//
// This is used by [SQLDriver.TableColumnTypes] implementations of
// case-sensitive destinations (e.g. Postgres, ClickHouse) to translate
// column names emitted by source drivers that store identifiers in a
// different case — Oracle, for example, emits UPPERCASE — back to the
// destination table's stored case before they are quoted into a query.
// Without this translation, a cross-source `--insert=@dest.tbl` would
// reference, say, "ACTOR_ID" against a column stored as "actor_id" and
// fail at the wire level (Postgres SQLSTATE 42703, ClickHouse code 47).
func ResolveTableColumnsFold(actual, src []string) ([]string, error) {
	out := make([]string, len(src))
	for i, name := range src {
		var canonical string
		for _, a := range actual {
			if strings.EqualFold(a, name) {
				canonical = a
				break
			}
		}
		if canonical == "" {
			return nil, errz.Errorf("column %q does not exist", name)
		}
		out[i] = canonical
	}
	return out, nil
}
