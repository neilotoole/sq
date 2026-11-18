package typetest

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/neilotoole/sq/libsq/core/kind"
	"github.com/neilotoole/sq/libsq/source"
	"github.com/neilotoole/sq/libsq/source/metadata"
	"github.com/neilotoole/sq/testh"
)

// DumpEnvar is the envar that turns on dump mode: when it's set to a
// non-empty value, Run logs what every column actually reports, before
// checking anything. It's for writing or updating a case table: run the test
// with it set, against each server version, and read the expectations off
// the log.
const DumpEnvar = "SQ_TYPETEST_DUMP"

// dumpActual logs formatActual for each column of tbl.
func dumpActual(t *testing.T, th *testh.Helper, src *source.Source, serverVer, tbl string, cols []Column) {
	t.Helper()
	tblMeta, err := th.TableMetadata(src, tbl)
	if err != nil {
		t.Logf("dump: server %s: TableMetadata: %v", serverVer, err)
		return
	}

	byName := map[string]*metadata.Column{}
	for _, c := range tblMeta.Columns {
		byName[c.Name] = c
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "dump: server %s:\n", serverVer)
	for _, col := range cols {
		md := byName[col.Name]
		if md == nil {
			fmt.Fprintf(&sb, "%s: not in TableMetadata\n", col.Name)
			continue
		}

		var (
			queryKind kind.Kind
			vals      []any
		)
		if sink, err := th.QuerySQL(src, nil, SelectColumn(tbl, col)); err == nil {
			queryKind = sink.RecMeta[1].Kind()
			for _, rec := range sink.Recs {
				vals = append(vals, rec[1])
			}
		}
		sb.WriteString(formatActual(col, md, queryKind, vals) + "\n")
	}
	t.Log(sb.String())
}

// formatActual formats what col actually reports: its inspect metadata md,
// its query result kind, and its values in row order, as Go literals. A nil
// vals means the query failed.
func formatActual(col Column, md *metadata.Column, queryKind kind.Kind, vals []any) string {
	valStr := "<query failed>"
	if vals != nil {
		parts := make([]string, len(vals))
		for i, v := range vals {
			switch v.(type) {
			case nil:
				parts[i] = "<nil>"
			case int64, uint64, float64, bool:
				// %#v drops the type of a basic number or bool, and the
				// comparison is type-sensitive.
				parts[i] = fmt.Sprintf("%T(%#v)", v, v)
			default:
				parts[i] = fmt.Sprintf("%#v", v)
			}
		}
		valStr = "[" + strings.Join(parts, " ") + "]"
	}

	return fmt.Sprintf("%s (%s, %s): base_type %q, column_type %q, kind %s, nullable %t, "+
		"default_value %q, query kind %s, values %s",
		col.Name, col.Case.Decl, col.Variant, md.BaseType, md.ColumnType, md.Kind, md.Nullable,
		md.DefaultValue, queryKind, valStr)
}

// dumpEnabled reports whether dump mode is on.
func dumpEnabled() bool {
	return os.Getenv(DumpEnvar) != ""
}
