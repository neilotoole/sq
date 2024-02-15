package diff

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	udiff "github.com/neilotoole/sq/cli/diff/internal/go-udiff"
	"github.com/neilotoole/sq/cli/diff/internal/go-udiff/myers"
	"github.com/neilotoole/sq/cli/output"
)

//nolint:lll,unused
const (
	alphaBefore = "a: 1\nb: 2\nc: 3\nd: 4\ne: 5\nf: 6\ng: 7\nh: 8\ni: 9\nj: 10\nk: 11\nl: 12\nm: 13\nn: 14\no: 15\np: 16\nq: 17\nr: 18\ns: 19\nt: 20\nu: 21\nv: 22\nw: 23\nx: 24\ny: 25\nz: 26\n"
	alphaAfter  = "a: 1\nb: 2\nc: 3\nd: 4\ne: 5\nf: 6\ng: X\nh: 8\ni: 9\nj: 10\nk: 11\nl: 12\nm: 13\nn: 14\no: 15\np: 16\nq: 17\nr: 18\ns: 19\nt: 20\nhuzzah\nu: 21\nv: 22\nw: 23\nx: 24\ny: 25\nz: 26\n"

	alphaShortBefore = "a: 1\nb: 2\nc: 3\nd: 4\ne: 5\nf: 6\ng: 7\nh: 8\ni: 9\nj: 10\n"
	alphaShortAfter  = "a: 1\nb: 2\nc: 3\nd: 4\ne: 5\nf: X\ng: 7\nh: 8\ni: 9\nj: 10\n"

	numLines = 3
)

func TestMyers(t *testing.T) {
	const left, right = alphaShortBefore, alphaShortAfter
	edits := myers.ComputeEdits(left, right)
	result, err := udiff.ToUnified(
		"before",
		"after",
		left,
		edits,
		numLines,
	)

	require.NoError(t, err)

	buf := &bytes.Buffer{}
	err = Print2(
		context.Background(),
		buf,
		output.NewPrinting(),
		"diff before after",
		strings.NewReader(result),
	)
	require.NoError(t, err)

	t.Logf("\n" + buf.String())
}

func TestMyersDiff(t *testing.T) {
	const left, right = alphaShortBefore, alphaShortAfter
	edits := myers.ComputeEdits(left, right)
	dff, err := udiff.ToUnifiedDiff(
		"before",
		"after",
		left,
		edits,
		numLines,
	)

	require.NoError(t, err)
	buf := &bytes.Buffer{}
	for _, h := range dff.Hunks {
		fmt.Fprintf(buf, "hunk: -%d, +%d\n", h.FromLine, h.ToLine)
		for _, l := range h.Lines {
			fmt.Fprintf(buf, "%s %q\n", l.Kind, l.Content)
		}
	}

	t.Log("\n" + buf.String())
}
