package typetest

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/cli/output"
	"github.com/neilotoole/sq/libsq"
	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/kind"
	"github.com/neilotoole/sq/libsq/source"
	"github.com/neilotoole/sq/libsq/source/metadata"
	"github.com/neilotoole/sq/testh"
)

// EnvDump is the envar that turns on dump mode: when it's set to true (1,
// true), Run logs what every column actually reports, before
// checking anything. It's for writing or updating a case table: run the test
// with it set, against each server version, and read the expectations off
// the log.
const EnvDump = "SQ_TYPETEST_DUMP"

// dumpActual logs formatActual for each column of tbl.
func dumpActual(t *testing.T, th *testh.Helper, src *source.Source, fold func(string) string, serverVer,
	tbl string, cols []Column,
) {
	t.Helper()
	tblMeta, err := th.TableMetadata(src, fold(tbl))
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
		md := byName[fold(col.Name)]
		if md == nil {
			fmt.Fprintf(&sb, "%s: not in TableMetadata\n", col.Name)
			continue
		}

		queryKind, vals, queryErr := dumpQuery(th, src, SelectColumn(tbl, col))
		sb.WriteString(formatActual(col, md, fold(tbl), queryKind, vals, queryErr) + "\n")
	}
	t.Log(sb.String())
}

// dumpQuery runs query and returns the result kind and values of its second
// field, the column, in row order, or the query error. Like checkColumnQuery,
// it waits for the record writer before reading the sink.
func dumpQuery(th *testh.Helper, src *source.Source, query string) (kind.Kind, []any, error) {
	sink := &testh.RecordSink{}
	recw := output.NewRecordWriterAdapter(th.Context, sink)
	err := libsq.QuerySQL(th.Context, th.Open(src), nil, recw, nil, query)
	if _, waitErr := recw.Wait(); err == nil {
		err = waitErr
	}
	if err != nil {
		return kind.Unknown, nil, err
	}
	if len(sink.RecMeta) < 2 {
		return kind.Unknown, nil, errz.Errorf("dump: want 2 result columns, got %d", len(sink.RecMeta))
	}

	vals := make([]any, 0, len(sink.Recs))
	for _, rec := range sink.Recs {
		vals = append(vals, rec[1])
	}
	return sink.RecMeta[1].Kind(), vals, nil
}

// formatActual formats what col actually reports: its inspect metadata md,
// its query result kind, and its values in row order as Go literals, or
// queryErr if the query failed. In default_value, tbl, the table name as the
// database reports it, is replaced by TableToken, so that the output can be
// pasted as an expectation.
func formatActual(col Column, md *metadata.Column, tbl string, queryKind kind.Kind, vals []any,
	queryErr error,
) string {
	valStr := "<query failed>"
	if queryErr != nil {
		valStr = "<query failed: " + queryErr.Error() + ">"
	}
	if queryErr == nil && vals != nil {
		parts := make([]string, len(vals))
		for i, v := range vals {
			parts[i] = fmtGoValue(v)
		}
		valStr = "[" + strings.Join(parts, " ") + "]"
	}

	return fmt.Sprintf("%s (%s, %s): base_type %q, column_type %q, kind %s, nullable %t, "+
		"default_value %q, query kind %s, values %s",
		col.Name, col.Case.Decl, col.Variant, md.BaseType, md.ColumnType, md.Kind, md.Nullable,
		defaultWithToken(md.DefaultValue, tbl), queryKind, valStr)
}

// fmtGoValue formats a record value as the dump and the value checks show
// it, close to how it's written in a case table: a basic number or bool with
// its type (%#v drops it, and the comparison is type-sensitive), a value
// with a GoString method by it (time.Time prints as time.Date(...)), one
// with only a String method as its type and string, such as
// decimal.Decimal(999.99), and anything else by %#v.
func fmtGoValue(v any) string {
	switch v := v.(type) {
	case nil:
		return "<nil>"
	case int64, uint64, float64, bool:
		return fmt.Sprintf("%T(%#v)", v, v)
	case fmt.GoStringer:
		return v.GoString()
	case fmt.Stringer:
		return fmt.Sprintf("%T(%s)", v, v)
	default:
		return fmt.Sprintf("%#v", v)
	}
}

// defaultWithToken returns def with tbl replaced by TableToken. An empty tbl
// leaves def unchanged.
func defaultWithToken(def, tbl string) string {
	if tbl == "" {
		return def
	}
	return strings.ReplaceAll(def, tbl, TableToken)
}

// dumpEnabled reports whether dump mode is on, failing t if EnvDump isn't a
// boolean.
func dumpEnabled(t *testing.T) bool {
	t.Helper()
	on, err := envBool(EnvDump)
	require.NoError(t, err)
	return on
}

// dumpPlaceholderDefault is the expected default_value that dump mode gives a
// Defaulted column whose case sets none (see withDumpPlaceholderDefaults).
const dumpPlaceholderDefault = "<unset: see the dump>"

// dumpConfig returns a copy of cfg for dump mode, with its cases passed
// through withDumpPlaceholderDefaults. Run uses it for every path, the ddl
// subtest's rendering included.
func dumpConfig(cfg Config) Config {
	cfg.Cases = withDumpPlaceholderDefaults(cfg.Cases)
	return cfg
}

// withDumpPlaceholderDefaults returns a copy of cases in which each case that
// sets no expected default_value for its Defaulted column, in Want or
// PerVariant.Defaulted, expects dumpPlaceholderDefault instead. Resolve
// requires that expectation, and dump mode is how it's drafted, so without
// this the dump would never run. The placeholder can't match a real
// default_value, so the checks after the dump still fail.
func withDumpPlaceholderDefaults(cases []Case) []Case {
	out := slices.Clone(cases)
	for i := range out {
		c := &out[i]
		if !c.Want.Default.Set && !c.PerVariant.Defaulted.Default.Set {
			c.PerVariant.Defaulted.Default = Is(dumpPlaceholderDefault)
		}
	}
	return out
}
