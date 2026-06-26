package diff

import (
	"context"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/libsq/core/diffdoc"
	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/record"
)

// These tests exercise recordDiffer.exec, which is the sole consumer of the
// github.com/neilotoole/tailbuf package in sq. The tailbuf holds the trailing
// window of record pairs so that exec can emit the "before" context lines that
// precede a differing record, via tailbuf.SliceNominal. Each test feeds a
// hand-crafted sequence of record pairs and asserts on the resulting diff
// hunks. The most important case is clear_tailbuf_on_hunk_max_size, which
// covers the previously-untested tb.Clear() path: tailbuf's Clear() semantics
// changed between v0.0.4 and v0.1.1, and these tests pin the behavior across
// that upgrade.

// capturedHunk records what recordDiffer.exec passed to the RecordHunkWriter
// for a single hunk: the nominal offset, plus the row index and equality of
// each record pair in the hunk. This captures exactly the tailbuf-driven record
// window, free of output-format noise.
type capturedHunk struct {
	offset int
	rows   []int
	equal  []bool
}

// fakeHunkWriter is a RecordHunkWriter that captures the pairs and offset for
// each hunk rather than rendering diff text. It isolates the tailbuf-driven
// record-window selection from output formatting (and avoids an import cycle
// that real output writers, which depend on this package, would introduce).
type fakeHunkWriter struct {
	hunks []capturedHunk
}

var _ RecordHunkWriter = (*fakeHunkWriter)(nil)

// WriteHunk implements RecordHunkWriter.
func (f *fakeHunkWriter) WriteHunk(_ context.Context, hunk *diffdoc.Hunk,
	_, _ record.Meta, pairs []record.Pair,
) {
	ch := capturedHunk{offset: hunk.Offset()}
	var body strings.Builder
	for _, p := range pairs {
		ch.rows = append(ch.rows, p.Row())
		ch.equal = append(ch.equal, p.Equal())
		fmt.Fprintf(&body, "%d:%t\n", p.Row(), p.Equal())
	}
	f.hunks = append(f.hunks, ch)

	// Write the body before sealing; it's a programming error to write after
	// Seal. The header is arbitrary deterministic text; exec doesn't inspect it.
	_, _ = hunk.Write([]byte(body.String()))
	hunk.Seal([]byte(fmt.Sprintf("@@ offset=%d @@\n", hunk.Offset())), nil)
}

// runRecordDiffer drives recordDiffer.exec with a synthetic stream of record
// pairs. The equal slice describes the stream: equal[i] == true produces a
// matching pair at row i, false produces a differing pair. It returns the
// captured hunks and the fully-rendered (sealed) diff document.
func runRecordDiffer(t *testing.T, numLines, hunkMaxSize int, equal []bool) (*fakeHunkWriter, string) {
	t.Helper()

	fake := &fakeHunkWriter{}
	rd := &recordDiffer{
		cfg: &Config{
			Lines:            numLines,
			HunkMaxSize:      hunkMaxSize,
			RecordHunkWriter: fake,
		},
		// The fake writer ignores record.Meta, so recMetaFn may return nil.
		recMetaFn: func() (rm1, rm2 record.Meta) { return nil, nil },
	}

	doc := diffdoc.NewHunkDoc(
		diffdoc.Titlef(nil, "test diff"),
		diffdoc.Headerf(nil, "left", "right"),
	)

	ch := make(chan record.Pair, len(equal)+1)
	for i, eq := range equal {
		if eq {
			rec := record.Record{fmt.Sprintf("r%d", i)}
			ch <- record.NewPair(i, rec, rec)
			continue
		}
		ch <- record.NewPair(i, record.Record{fmt.Sprintf("a%d", i)}, record.Record{fmt.Sprintf("b%d", i)})
	}
	close(ch)

	err := rd.exec(context.Background(), ch, doc)
	require.NoError(t, err)

	doc.Seal(nil)
	require.NoError(t, doc.Err())

	// Read the sealed doc to confirm hunk sealing/reading works end-to-end and
	// doesn't deadlock.
	out, err := io.ReadAll(doc)
	require.NoError(t, err)

	return fake, string(out)
}

// TestRecordDifferExec_CancelMidLookahead is the regression test for issue
// #906. When the context is canceled while exec is in its look-ahead loop, exec
// has already created a hunk (via doc.NewHunk) but has not yet populated it (the
// only path that seals it). exec must seal that orphaned hunk before returning,
// otherwise a reader that traverses the doc's hunks blocks forever on the
// unsealed hunk.
func TestRecordDifferExec_CancelMidLookahead(t *testing.T) {
	fake := &fakeHunkWriter{}
	rd := &recordDiffer{
		cfg: &Config{
			// numLines=1 means the look-ahead needs two contiguous matching pairs
			// before it stops, so a single matching pair leaves exec blocked in the
			// look-ahead select, waiting for more.
			Lines:            1,
			HunkMaxSize:      100,
			RecordHunkWriter: fake,
		},
		recMetaFn: func() (rm1, rm2 record.Meta) { return nil, nil },
	}

	doc := diffdoc.NewHunkDoc(
		diffdoc.Titlef(nil, "test diff"),
		diffdoc.Headerf(nil, "left", "right"),
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Unbuffered channel: each send completes only once exec receives it, which
	// makes the sequence below deterministic with no timing assumptions.
	ch := make(chan record.Pair)
	execErrCh := make(chan error, 1)
	go func() {
		execErrCh <- rd.exec(ctx, ch, doc)
	}()

	// A differing pair makes exec create a hunk and enter the look-ahead loop.
	ch <- record.NewPair(0, record.Record{"a0"}, record.Record{"b0"})
	// A matching pair is consumed inside the look-ahead select; with numLines=1
	// it is not enough to stop, so exec loops back to the select and waits.
	ch <- record.NewPair(1, record.Record{"r1"}, record.Record{"r1"})
	// Now exec is committed to the look-ahead select with an empty channel, so
	// canceling makes it take the ctx-done branch (break LOOP) with a hunk
	// created but not populated.
	cancel()

	err := <-execErrCh
	require.Error(t, err, "exec should return the context error")

	// The hunk was orphaned mid-look-ahead: it was created but populateHunk (the
	// only path that calls the writer) never ran. Asserting the writer saw no
	// hunks confirms we are exercising the orphaned-hunk path, not a normally
	// populated one, so the test cannot pass for the wrong reason.
	require.Empty(t, fake.hunks, "populateHunk must not have run on the cancel path")

	// Seal the doc without an error to force the reader to traverse the hunks.
	// (The production caller seals with the error, which short-circuits Read and
	// masks an unsealed hunk; sealing nil here exposes it.)
	doc.Seal(nil)

	type readResult struct {
		err error
	}
	done := make(chan readResult, 1)
	go func() {
		_, readErr := io.ReadAll(doc)
		done <- readResult{err: readErr}
	}()

	select {
	case res := <-done:
		// Reader completed (did not hang): the orphaned hunk was sealed. It was
		// sealed with the context error, so reading it surfaces that error, which
		// also proves a hunk really was created and is now readable.
		require.Error(t, res.err, "the orphaned hunk should be sealed with the context error")
	case <-time.After(10 * time.Second):
		t.Fatal("io.ReadAll hung: exec left a hunk unsealed (issue #906)")
	}
}

// panicHunkWriter mimics recordHunkWriterAdapter.WriteHunk's contract: it always
// seals the hunk via a defer (even when it panics), then panics. It is used to
// prove that exec does not double-seal the hunk on a populateHunk panic, which
// would replace the original panic with a confusing "already sealed" panic.
type panicHunkWriter struct{}

var _ RecordHunkWriter = panicHunkWriter{}

func (panicHunkWriter) WriteHunk(_ context.Context, hunk *diffdoc.Hunk,
	_, _ record.Meta, _ []record.Pair,
) {
	defer hunk.Seal(nil, nil)
	panic("boom in WriteHunk")
}

// TestRecordDifferExec_WriteHunkPanicNoDoubleSeal guards the #906 fix against
// regressing to a deferred seal. populateHunk (via WriteHunk) seals the hunk in
// its own defer even on panic, so exec must not also seal it on the panic path,
// or it double-seals and the original panic is masked.
func TestRecordDifferExec_WriteHunkPanicNoDoubleSeal(t *testing.T) {
	rd := &recordDiffer{
		cfg: &Config{
			// numLines=0 makes a single matching pair after the difference enough
			// to close the look-ahead and reach populateHunk.
			Lines:            0,
			HunkMaxSize:      100,
			RecordHunkWriter: panicHunkWriter{},
		},
		recMetaFn: func() (rm1, rm2 record.Meta) { return nil, nil },
	}

	doc := diffdoc.NewHunkDoc(
		diffdoc.Titlef(nil, "test diff"),
		diffdoc.Headerf(nil, "left", "right"),
	)

	ch := make(chan record.Pair, 2)
	ch <- record.NewPair(0, record.Record{"a0"}, record.Record{"b0"}) // differs: creates a hunk
	ch <- record.NewPair(1, record.Record{"r1"}, record.Record{"r1"}) // matches: closes look-ahead
	close(ch)

	defer func() {
		r := recover()
		require.NotNil(t, r, "exec should propagate the original WriteHunk panic")
		require.Equal(t, "boom in WriteHunk", r,
			"exec must not convert the WriteHunk panic into a double-seal panic")
	}()

	_ = rd.exec(context.Background(), ch, doc)
}

// collectPairs runs collateByKey over two synthetic PK-ordered streams and
// returns the emitted pairs. It builds a record.Meta whose column names map
// back to keyIdxs, then passes the corresponding pkColNames so that collateByKey
// resolves indices via pkColIndexes exactly as production does.
func collectPairs(t *testing.T, keyIdxs []int, left, right []record.Record) []record.Pair {
	t.Helper()
	rs1 := &dbResults{recCh: make(chan record.Record, len(left)+1), opened: make(chan struct{})}
	rs2 := &dbResults{recCh: make(chan record.Record, len(right)+1), opened: make(chan struct{})}
	for _, r := range left {
		rs1.recCh <- r
	}
	rs1.recCh <- nil // end-of-data sentinel used by the collation loop
	for _, r := range right {
		rs2.recCh <- r
	}
	rs2.recCh <- nil

	// Build a record.Meta with one named column per record position, so
	// pkColIndexes(recMeta, pkColNames) resolves back to keyIdxs.
	nCols := 0
	for _, r := range append(append([]record.Record{}, left...), right...) {
		if len(r) > nCols {
			nCols = len(r)
		}
	}
	names := make([]string, nCols)
	rm := make(record.Meta, nCols)
	for i := range names {
		names[i] = fmt.Sprintf("col%d", i)
		rm[i] = newFieldMeta(names[i])
	}
	rs1.recMeta = rm
	rs2.recMeta = rm
	// Signal that recMeta is ready, mimicking dbResults.Open.
	close(rs1.opened)
	close(rs2.opened)

	pkColNames := make([]string, len(keyIdxs))
	for i, idx := range keyIdxs {
		pkColNames[i] = names[idx]
	}

	recPairsCh := make(chan record.Pair, len(left)+len(right)+2)
	cfg := &Config{Lines: 3, StopAfter: 0}
	err := collateByKey(context.Background(), rs1, rs2, pkColNames, recPairsCh, cfg,
		func(int64) {}, func(error) {})
	require.NoError(t, err)

	var got []record.Pair
	for rp := range recPairsCh {
		got = append(got, rp)
	}
	return got
}

func TestCollateByKey_ScatteredInserts(t *testing.T) {
	left := []record.Record{{int64(1)}, {int64(3)}, {int64(4)}}
	right := []record.Record{{int64(1)}, {int64(2)}, {int64(3)}, {int64(4)}}
	pairs := collectPairs(t, []int{0}, left, right)

	require.Len(t, pairs, 4)
	// Only the inserted key (2) is an "added" pair; the rest are equal.
	require.True(t, pairs[0].Equal())     // 1
	require.Nil(t, pairs[1].Rec1())       // 2 added
	require.Equal(t, int64(2), pairs[1].Rec2()[0])
	require.True(t, pairs[2].Equal()) // 3
	require.True(t, pairs[3].Equal()) // 4
}

func TestRecordDifferExec(t *testing.T) {
	testCases := []struct {
		name        string
		numLines    int
		hunkMaxSize int
		equal       []bool
		want        []capturedHunk
	}{
		{
			name:        "all_equal_no_hunks",
			numLines:    1,
			hunkMaxSize: 100,
			equal:       []bool{true, true, true, true},
			want:        nil,
		},
		{
			name:        "empty_stream_no_hunks",
			numLines:    1,
			hunkMaxSize: 100,
			equal:       []bool{},
			want:        nil,
		},
		{
			name:        "single_diff_mid_stream",
			numLines:    2,
			hunkMaxSize: 100,
			equal:       []bool{true, true, true, false, true, true, true, true},
			want: []capturedHunk{
				{offset: 1, rows: []int{1, 2, 3, 4, 5}, equal: []bool{true, true, false, true, true}},
			},
		},
		{
			name:        "diff_at_row_zero_clamps_before_context",
			numLines:    1,
			hunkMaxSize: 100,
			equal:       []bool{false, true, true},
			want: []capturedHunk{
				{offset: 0, rows: []int{0, 1}, equal: []bool{false, true}},
			},
		},
		{
			name:        "diff_at_final_row_truncates_after_context",
			numLines:    1,
			hunkMaxSize: 100,
			equal:       []bool{true, true, false},
			want: []capturedHunk{
				{offset: 1, rows: []int{1, 2}, equal: []bool{true, false}},
			},
		},
		{
			name:        "back_to_back_diffs_single_hunk",
			numLines:    1,
			hunkMaxSize: 100,
			equal:       []bool{true, false, false, true, true, true},
			want: []capturedHunk{
				{offset: 0, rows: []int{0, 1, 2, 3}, equal: []bool{true, false, false, true}},
			},
		},
		{
			name:        "zero_context_lines",
			numLines:    0,
			hunkMaxSize: 100,
			equal:       []bool{true, false, true, true},
			want: []capturedHunk{
				{offset: 1, rows: []int{1}, equal: []bool{false}},
			},
		},
		{
			// The critical case: a run of differing pairs long enough to hit
			// HunkMaxSize forces tb.Clear(), and the next difference must start a
			// fresh hunk with no carried-over (duplicate) context. This pins the
			// tailbuf Clear()/SliceNominal interaction across the v0.0.4 -> v0.1.1
			// upgrade.
			name:        "clear_tailbuf_on_hunk_max_size",
			numLines:    1,
			hunkMaxSize: 3,
			equal:       []bool{true, false, false, false, false, true, true, true},
			want: []capturedHunk{
				{offset: 0, rows: []int{0, 1, 2}, equal: []bool{true, false, false}},
				{offset: 3, rows: []int{3, 4, 5}, equal: []bool{false, false, true}},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			fake, out := runRecordDiffer(t, tc.numLines, tc.hunkMaxSize, tc.equal)

			require.Equal(t, tc.want, fake.hunks)

			// The offset reported to NewHunk must equal the nominal row of the
			// first pair in each hunk; this is the invariant that tailbuf's
			// nominal indexing underpins.
			for _, h := range fake.hunks {
				require.NotEmpty(t, h.rows)
				require.Equal(t, h.rows[0], h.offset,
					"hunk offset must equal the row of its first pair")
			}

			if len(tc.want) == 0 {
				require.Empty(t, out, "no hunks should render an empty doc")
			}
		})
	}
}

// makeAllDiff returns a []bool of length n where every element is false
// (all record pairs differ). Used to construct an all-differing stream for
// stop-trailer tests.
func makeAllDiff(n int) []bool {
	// Zero value of bool is false, so a plain make is enough.
	return make([]bool, n)
}

// runRecordDifferStop is the stop-trailer analogue of runRecordDiffer. It
// accepts a stopAfter parameter and simulates the production stop signal by
// cancelling a synthetic dbCtx with errz.ErrStop (mirroring what
// collateByKey/collatePositional do when the threshold is reached). It drives
// recordDiffer.execAndSeal, which is the method that owns the trailer logic,
// so the assertion exercises the real production code path.
func runRecordDifferStop(t *testing.T, numLines, hunkMaxSize, stopAfter int, equal []bool) (*fakeHunkWriter, string) {
	t.Helper()

	fake := &fakeHunkWriter{}
	rd := &recordDiffer{
		cfg: &Config{
			Lines:            numLines,
			HunkMaxSize:      hunkMaxSize,
			StopAfter:        stopAfter,
			RecordHunkWriter: fake,
		},
		recMetaFn: func() (rm1, rm2 record.Meta) { return nil, nil },
	}

	doc := diffdoc.NewHunkDoc(
		diffdoc.Titlef(nil, "test diff"),
		diffdoc.Headerf(nil, "left", "right"),
	)

	ch := make(chan record.Pair, len(equal)+1)
	for i, eq := range equal {
		if eq {
			rec := record.Record{fmt.Sprintf("r%d", i)}
			ch <- record.NewPair(i, rec, rec)
			continue
		}
		ch <- record.NewPair(i, record.Record{fmt.Sprintf("a%d", i)}, record.Record{fmt.Sprintf("b%d", i)})
	}
	close(ch)

	// Simulate the stop signal. In production, collateByKey/collatePositional
	// call dbCancel(errz.ErrStop) when the stop threshold is reached, cancelling
	// dbCtx (not the outer ctx). We pre-cancel dbCtx here to mimic that. When
	// stopAfter is 0 no stop is configured and dbCtx is left uncancelled.
	dbCtx, dbCancel := context.WithCancelCause(context.Background())
	defer dbCancel(nil)
	if stopAfter > 0 {
		dbCancel(errz.ErrStop)
	}

	rd.execAndSeal(context.Background(), dbCtx, ch, doc)

	out, err := io.ReadAll(doc)
	require.NoError(t, err)

	return fake, string(out)
}

// TestStopTrailer_PresentWhenTruncated verifies that execAndSeal appends a
// "stopped after N differences" trailer when the db context was cancelled with
// errz.ErrStop, and emits no trailer when StopAfter is 0.
func TestStopTrailer_PresentWhenTruncated(t *testing.T) {
	// 10 differing pairs, StopAfter=2 -> stop occurred -> trailer present.
	_, out := runRecordDifferStop(t, 0, 5000, 2, makeAllDiff(10))
	require.Contains(t, out, "stopped after")

	// No StopAfter -> no stop -> no trailer.
	_, out = runRecordDifferStop(t, 0, 5000, 0, makeAllDiff(10))
	require.NotContains(t, out, "stopped after")
}
