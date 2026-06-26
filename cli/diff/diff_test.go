package diff_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/cli/testrun"
	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/ioz"
	"github.com/neilotoole/sq/libsq/source"
	"github.com/neilotoole/sq/libsq/source/drivertype"
	"github.com/neilotoole/sq/testh"
	"github.com/neilotoole/sq/testh/proj"
	"github.com/neilotoole/sq/testh/sakila"
	"github.com/neilotoole/sq/testh/tu"
)

// TestDiff_Data_ReadOnly is a regression guard for the diff --data path
// opening sources read-only. The table-data comparison runs SLQ via a
// run.QueryContext; if its AccessMode is left at the ModeReadWrite
// default, DuckDB sources are opened read-write and take a write lock.
//
// The test makes the DuckDB file read-only on disk (0444), so a
// read-write open fails (DuckDB can't take a write lock on it) while a
// read-only open succeeds. diff is wholly read-only, so it must succeed.
func TestDiff_Data_ReadOnly(t *testing.T) {
	tu.SkipReadOnlyFileUnenforceable(t)
	th := testh.New(t)
	src := th.Source(sakila.Duck)
	path := strings.TrimPrefix(src.Location, "duckdb://")

	// th.Source returns a per-test copy, so chmod-ing it is safe.
	require.NoError(t, os.Chmod(path, 0o444))
	t.Cleanup(func() { _ = os.Chmod(path, 0o644) }) // let TempDir cleanup remove it

	// Diff a table against itself: no differences (clean exit), but the
	// data path still opens the source and runs the row queries. With a
	// read-write open this fails on the 0444 file; read-only succeeds.
	tr := testrun.New(th.Context, t, nil).Hush().Add(*src)
	require.NoError(t, tr.Exec("diff", "--data",
		src.Handle+".actor", src.Handle+".actor"),
		"sq diff --data must open the source read-only (no write lock)")
}

func TestSchemaDiff(t *testing.T) {
	th := testh.New(t)

	tr := testrun.New(th.Context, t, nil)
	_ = tr.Reset().Exec("config", "set", "debug.progress.force", "true")

	tr = tr.Add(
		source.Source{
			Handle:   "@test_a",
			Type:     drivertype.SQLite,
			Location: "sqlite3://" + proj.Abs("cli/diff/testdata/sakila_a.db"),
		},
		source.Source{
			Handle:   "@test_b",
			Type:     drivertype.SQLite,
			Location: "sqlite3://" + proj.Abs("cli/diff/testdata/sakila_b.db"),
		},
	)

	err := tr.Reset().Exec("diff", "@test_a", "@test_b", "--schema")

	require.Error(t, err)
	require.Equal(t, 1, errz.ExitCode(err), "should be exit code 1 on differences")
	fmt.Fprintln(os.Stdout, tr.Out.String())
}

// TestDiff_unsupportedFormat is a regression guard for the format validation
// that moved out of OptDiffDataFormat's (globally-shared) validFn into
// getDiffRecordWriter. diff renders data only for text-based formats; xlsx,
// raw, and the inspect-only mermaid-erd format must still be rejected with a
// clear error.
func TestDiff_unsupportedFormat(t *testing.T) {
	th := testh.New(t)
	tr := testrun.New(th.Context, t, nil).Add(
		source.Source{
			Handle:   "@test_a",
			Type:     drivertype.SQLite,
			Location: "sqlite3://" + proj.Abs("cli/diff/testdata/sakila_a.db"),
		},
		source.Source{
			Handle:   "@test_b",
			Type:     drivertype.SQLite,
			Location: "sqlite3://" + proj.Abs("cli/diff/testdata/sakila_b.db"),
		},
	)

	for _, fm := range []string{"xlsx", "raw", "mermaid-erd"} {
		t.Run(fm, func(t *testing.T) {
			err := tr.Reset().Exec("diff", "@test_a", "@test_b", "--data", "-f", fm)
			require.Error(t, err)
			require.Contains(t, err.Error(), "diff does not support output format")
		})
	}
}

// TestDiff_Data_PKAlignScatteredInserts is the end-to-end gate for issue #947.
// It confirms that sq diff --data, when both tables share an integer primary
// key, reports exactly one clean "added" line per row that exists in the right
// table but is absent (at a scattered PK position) in the left table, rather
// than the positional cascade the pre-#947 code produced.
//
// Fixture: the Sakila actor table (actor_id INTEGER PRIMARY KEY, 200 rows).
// We delete 5 scattered actor_ids from a left-side copy, so the right-side
// (intact) copy has those 5 rows as scattered inserts.
func TestDiff_Data_PKAlignScatteredInserts(t *testing.T) {
	// wantAdded is the count of rows present in the right table but not in
	// the left table — exactly one per scattered actor_id we delete below.
	const wantAdded = 5

	th := testh.New(t)
	srcA := th.Source(sakila.SL3) // writable temp copy of sakila; handle @sakila_sl3

	// Remove 5 scattered rows from the left copy. After this, srcA.actor has
	// 195 rows, and the intact right copy has 5 extra rows at those scattered
	// PK positions, mirroring the payment-table scenario from issue #947.
	th.ExecSQL(srcA, "DELETE FROM actor WHERE actor_id IN (2, 4, 6, 8, 10)")
	require.Equal(t, int64(sakila.TblActorCount-wantAdded), th.RowCount(srcA, sakila.TblActor))

	// Build the right-side source: an intact copy of the original Sakila
	// fixture with all 200 actor rows.
	origPath := proj.Abs(sakila.PathSL3)
	dstPath := filepath.Join(tu.TempDir(t), "sakila_right.db")
	require.NoError(t, ioz.CopyFile(dstPath, origPath, true))
	srcB := source.Source{
		Handle:   "@test_right",
		Type:     drivertype.SQLite,
		Location: "sqlite3://" + dstPath,
	}

	// --unified 0 suppresses context so each diff line is exactly one data row
	// (no surrounding context), keeping the assertion count unambiguous.
	// --stop 0 disables truncation.
	tr := testrun.New(th.Context, t, nil).Hush().Add(*srcA).Add(srcB)
	err := tr.Exec(
		"diff", "--data", "--unified", "0", "--stop", "0",
		srcA.Handle+"."+sakila.TblActor,
		srcB.Handle+"."+sakila.TblActor,
	)

	// sq diff exits with code 1 when differences exist; that surfaces as a
	// non-nil error here.
	require.Error(t, err, "diff must report differences (exit code 1)")
	require.Equal(t, 1, errz.ExitCode(err), "sq diff exits 1 when differences exist")

	got := tr.Out.String()
	t.Log("diff output:\n" + got) // diagnostic: confirms clean adds in test log

	// Count added (+) and removed (-) data lines. Skip the file-header lines
	// (--- / +++) and hunk-header lines (@@ ... @@).
	var added, removed int
	for _, line := range strings.Split(got, "\n") {
		switch {
		case strings.HasPrefix(line, "+++") || strings.HasPrefix(line, "---"):
			// file header line, not a data row
		case strings.HasPrefix(line, "+"):
			added++
		case strings.HasPrefix(line, "-"):
			removed++
		}
	}

	t.Logf("added=%d removed=%d (want added=%d removed=0)", added, removed, wantAdded)

	// The critical assertion: PK-aware diff produces exactly wantAdded "+"
	// lines — one per scattered insert. The pre-#947 positional alignment
	// would have cascaded to ~190 diff lines for the same input.
	require.Equal(t, wantAdded, added,
		"PK-aware diff must report exactly one added line per scattered insert; "+
			"positional alignment would cascade to far more diff lines")
	require.Zero(t, removed,
		"no rows were removed from the right table; zero removed lines expected")
}

// TestDiff_Data_PKAlignDefaultContext is the default-context (3 lines) gate for
// issue #947. Unlike TestDiff_Data_PKAlignScatteredInserts, which pins
// --unified 0 (hiding the writer's mid-stream single-sided cascade behind the
// absence of context lines), this test runs with the DEFAULT context so that
// the unchanged neighbour rows of each scattered insert/delete are rendered.
//
// The bug it guards: the text/csv hunk writers pre-render only the non-nil
// records into per-side buffers, then walk the pairs scanning one line per
// non-equal pair WITHOUT checking which side is nil. A single-sided pair
// (added: Rec1()==nil, removed: Rec2()==nil) made the scanner consume the line
// belonging to a later pair, mislabeling an unchanged neighbour as a deletion
// or insertion and cascading down the hunk.
//
// We assert, for both the text and csv formats:
//   - ADDED scenario  (right has scattered rows the left lacks): exactly N "+"
//     lines, ZERO "-" lines, and unchanged neighbours appear as context lines.
//   - REMOVED scenario (left has scattered rows the right lacks): exactly N "-"
//     lines, ZERO "+" lines, neighbours as context.
func TestDiff_Data_PKAlignDefaultContext(t *testing.T) {
	// Scattered actor_ids, spaced far apart so each forms its own hunk with a
	// full 3-line context window on each side (no clamping, no hunk overlap).
	ids := []int{50, 100, 150}
	const wantN = 3

	for _, format := range []string{"text", "csv"} {
		format := format
		t.Run(format, func(t *testing.T) {
			t.Run("added", func(t *testing.T) {
				// Modified copy on the LEFT (missing the rows); intact copy on the
				// RIGHT. The right rows absent from the left are "added".
				added, removed, ctxLines := runDiffDefaultContext(t, format, ids, false)
				t.Logf("added=%d removed=%d context=%d (want added=%d removed=0)",
					added, removed, ctxLines, wantN)
				require.Equal(t, wantN, added,
					"PK-aware diff must report exactly one added line per scattered insert")
				require.Zero(t, removed,
					"no rows removed; the writer must not mislabel neighbours as deletions")
				require.Positive(t, ctxLines,
					"unchanged neighbours must render as context lines, not diff lines")
			})

			t.Run("removed", func(t *testing.T) {
				// Intact copy on the LEFT; modified copy (missing the rows) on the
				// RIGHT. The left rows absent from the right are "removed".
				added, removed, ctxLines := runDiffDefaultContext(t, format, ids, true)
				t.Logf("added=%d removed=%d context=%d (want removed=%d added=0)",
					added, removed, ctxLines, wantN)
				require.Equal(t, wantN, removed,
					"PK-aware diff must report exactly one removed line per scattered delete")
				require.Zero(t, added,
					"no rows added; the writer must not mislabel neighbours as insertions")
				require.Positive(t, ctxLines,
					"unchanged neighbours must render as context lines, not diff lines")
			})
		})
	}
}

// runDiffDefaultContext sets up an intact Sakila actor table and a modified
// copy with the given actor_ids deleted, then runs sq diff --data at the
// default context with the given output format. If intactLeft is true the
// intact copy is the left diff operand (so the deleted rows surface as
// removals); otherwise the modified copy is on the left (deleted rows surface
// as additions). It returns the count of "+" (added), "-" (removed), and " "
// (context) data lines, excluding the ---/+++ file headers and @@ hunk headers.
func runDiffDefaultContext(t *testing.T, format string, ids []int, intactLeft bool,
) (added, removed, ctxLines int) {
	t.Helper()
	th := testh.New(t)

	// modified: writable temp copy of sakila with the scattered rows deleted.
	srcMod := th.Source(sakila.SL3)
	idStrs := make([]string, len(ids))
	for i, id := range ids {
		idStrs[i] = strconv.Itoa(id)
	}
	th.ExecSQL(srcMod, "DELETE FROM actor WHERE actor_id IN ("+strings.Join(idStrs, ", ")+")")
	require.Equal(t, int64(sakila.TblActorCount-len(ids)), th.RowCount(srcMod, sakila.TblActor))

	// intact: fresh untouched copy of the sakila fixture (all 200 actor rows).
	origPath := proj.Abs(sakila.PathSL3)
	dstPath := filepath.Join(tu.TempDir(t), "sakila_intact.db")
	require.NoError(t, ioz.CopyFile(dstPath, origPath, true))
	srcIntact := source.Source{
		Handle:   "@test_intact",
		Type:     drivertype.SQLite,
		Location: "sqlite3://" + dstPath,
	}

	left, right := srcMod.Handle, srcIntact.Handle
	if intactLeft {
		left, right = srcIntact.Handle, srcMod.Handle
	}

	tr := testrun.New(th.Context, t, nil).Hush().Add(*srcMod).Add(srcIntact)
	args := []string{"diff", "--data", "--stop", "0"}
	if format != "text" {
		args = append(args, "-f", format)
	}
	args = append(args, left+"."+sakila.TblActor, right+"."+sakila.TblActor)

	err := tr.Exec(args...)
	require.Error(t, err, "diff must report differences (exit code 1)")
	require.Equal(t, 1, errz.ExitCode(err), "sq diff exits 1 when differences exist")

	got := tr.Out.String()
	t.Log("diff output:\n" + got)

	for _, line := range strings.Split(got, "\n") {
		switch {
		case strings.HasPrefix(line, "+++") || strings.HasPrefix(line, "---"):
			// file header line, not a data row
		case strings.HasPrefix(line, "@@"):
			// hunk header line, not a data row
		case strings.HasPrefix(line, "+"):
			added++
		case strings.HasPrefix(line, "-"):
			removed++
		case strings.HasPrefix(line, " "):
			ctxLines++
		}
	}
	return added, removed, ctxLines
}
