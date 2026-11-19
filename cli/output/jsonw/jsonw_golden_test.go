package jsonw_test

import (
	"bufio"
	"bytes"
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/cli/output"
	"github.com/neilotoole/sq/cli/output/jsonw"
	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/lg/lgt"
	"github.com/neilotoole/sq/libsq/core/record"
	"github.com/neilotoole/sq/testh"
	"github.com/neilotoole/sq/testh/fixt"
)

// updateGolden regenerates goldenPath instead of asserting against it:
//
//	go test ./cli/output/jsonw/ -update
//
// Review the resulting diff before committing. Because the golden file stores
// each case as a Go-quoted string, a changed ANSI escape shows up as readable
// text (\x1b[32m) rather than as raw terminal control bytes.
var updateGolden = flag.Bool("update", false, "update the golden file in testdata")

// goldenPath holds the exact bytes each jsonw writer produces, for every
// combination in the matrix built by goldenCases.
const goldenPath = "testdata/writers_golden.txt"

// TestWritersGolden asserts the exact output bytes of the JSON writers,
// colored and monochrome. The rest of the package's tests either run
// monochrome or assert only that some ANSI escape is present, which cannot
// catch a token changing color, a stray or missing reset, or a control
// character changing spelling. A jsoncolor upgrade did exactly that
// (see #1136) and the suite stayed green.
//
// To regenerate after an intentional change, run with -update and review the
// diff.
func TestWritersGolden(t *testing.T) {
	got := goldenCases(t)

	if *updateGolden {
		writeGolden(t, got)
		t.Logf("updated %s with %d cases", goldenPath, len(got))
		return
	}

	want := readGolden(t)

	// Compare per case, so a failure names the case that moved rather than
	// dumping the whole matrix.
	names := make([]string, 0, len(got))
	for name := range got {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		t.Run(name, func(t *testing.T) {
			wantCase, ok := want[name]
			require.True(t, ok,
				"case %q missing from %s; rerun with -update", name, goldenPath)
			require.Equal(t, wantCase, got[name],
				"output changed for %q; if intentional, rerun with -update", name)
		})
	}

	for name := range want {
		_, ok := got[name]
		require.True(t, ok,
			"case %q in %s is no longer produced; rerun with -update", name, goldenPath)
	}
}

// goldenCases renders every writer/color/pretty combination, keyed by case
// name. The record fixture deliberately includes a row of control characters,
// since that is the class of change that slipped through previously.
func goldenCases(t *testing.T) map[string]string {
	t.Helper()
	ctx := context.Background()
	out := map[string]string{}

	colNames, kinds := fixt.ColNamePerKind(false, false, false)
	recMeta := testh.NewRecordMeta(colNames, kinds)

	tm := time.Unix(0, 0).UTC()
	full := record.Record{
		int64(64), 64.64, "10000000000000000.99", true, "hello", tm, tm, tm, []byte("hello"),
	}
	nulls := record.Record{nil, nil, nil, nil, nil, nil, nil, nil, nil}
	ctrl := record.Record{
		int64(64), 64.64, "10000000000000000.99", true,
		"ctrl:\b\f\t\n\r\x01:end", tm, tm, tm, []byte("hello"),
	}
	recs := []record.Record{full, nulls, ctrl}

	writers := []struct {
		name string
		fn   func(w *bytes.Buffer, pr *output.Printing) output.RecordWriter
	}{
		{"std", func(w *bytes.Buffer, pr *output.Printing) output.RecordWriter {
			return jsonw.NewStdRecordWriter(w, pr)
		}},
		{"object", func(w *bytes.Buffer, pr *output.Printing) output.RecordWriter {
			return jsonw.NewObjectRecordWriter(w, pr)
		}},
		{"array", func(w *bytes.Buffer, pr *output.Printing) output.RecordWriter {
			return jsonw.NewArrayRecordWriter(w, pr)
		}},
	}

	for _, wr := range writers {
		for _, color := range []bool{true, false} {
			for _, pretty := range []bool{true, false} {
				buf := &bytes.Buffer{}
				pr := output.NewPrinting()
				pr.EnableColor(color)
				pr.Compact = !pretty

				w := wr.fn(buf, pr)
				require.NoError(t, w.Open(ctx, recMeta))
				require.NoError(t, w.WriteRecords(ctx, recs))
				require.NoError(t, w.Close(ctx))

				key := fmt.Sprintf("record/%s/color=%v/pretty=%v", wr.name, color, pretty)
				out[key] = buf.String()
			}
		}
	}

	// The error writer, non-verbose only. Verbose output embeds absolute
	// source paths and stdlib line numbers from the stack trace, so it cannot
	// be a stable fixture across machines or Go versions.
	for _, color := range []bool{true, false} {
		buf := &bytes.Buffer{}
		pr := output.NewPrinting()
		pr.EnableColor(color)

		err := errz.New("the error message")
		jsonw.NewErrorWriter(lgt.New(t), buf, pr).Error(err, err)
		out[fmt.Sprintf("error/color=%v", color)] = buf.String()
	}

	return out
}

// writeGolden serializes cases to goldenPath, one "### name" header per case
// followed by the Go-quoted output.
func writeGolden(t *testing.T, cases map[string]string) {
	t.Helper()

	names := make([]string, 0, len(cases))
	for name := range cases {
		names = append(names, name)
	}
	sort.Strings(names)

	buf := &bytes.Buffer{}
	buf.WriteString("# Generated by TestWritersGolden -update. Do not edit by hand.\n")
	buf.WriteString("# Each case is the exact writer output, Go-quoted so ANSI escapes stay readable.\n\n")
	for _, name := range names {
		fmt.Fprintf(buf, "### %s\n%q\n\n", name, cases[name])
	}

	require.NoError(t, os.MkdirAll(filepath.Dir(goldenPath), 0o750))
	require.NoError(t, os.WriteFile(goldenPath, buf.Bytes(), 0o600))
}

// readGolden parses goldenPath back into a map of case name to expected output.
func readGolden(t *testing.T) map[string]string {
	t.Helper()

	f, err := os.Open(goldenPath)
	require.NoError(t, err, "missing %s; generate it with -update", goldenPath)
	defer func() { _ = f.Close() }()

	out := map[string]string{}
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)

	var name string
	for sc.Scan() {
		line := sc.Text()
		switch {
		case strings.HasPrefix(line, "### "):
			name = strings.TrimPrefix(line, "### ")
		case line == "" || strings.HasPrefix(line, "#"):
			continue
		default:
			require.NotEmpty(t, name, "value line before any ### header in %s", goldenPath)
			val, err := strconv.Unquote(line)
			require.NoError(t, err, "malformed quoted value for %q in %s", name, goldenPath)
			out[name] = val
			name = ""
		}
	}
	require.NoError(t, sc.Err())
	require.NotEmpty(t, out, "%s contains no cases", goldenPath)

	return out
}
