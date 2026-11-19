package diff_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

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

// TestDiff_Data_PKAlignChangedAdjacent is the gate for the residual #947 writer
// bug where a CHANGED row that is adjacent (in PK order, same differing run) to
// an ADDED or REMOVED row had one of its diff lines silently dropped.
//
// The pre-fix text/csv writers rendered a contiguous run of differing pairs with
// TWO break-on-nil loops (a deletion loop emitting Rec1 lines and an insertion
// loop emitting Rec2 lines), then advanced the outer index by max(j,k)-1. When a
// single-sided pair (added: Rec1()==nil, removed: Rec2()==nil) preceded a changed
// pair in the same run, the corresponding loop broke at the single-sided pair and
// never reached the changed pair's line, and the index jump skipped it:
//
//   - changed adjacent to ADDED   → the changed row's "-old" line was dropped.
//   - changed adjacent to REMOVED → the changed row's "+new" line was dropped.
//
// We build a copied actor table where actor_id K is single-sided (deleted from
// one side) and the adjacent actor_id K+1 is changed (a non-key column set to a
// distinct sentinel on each side). We then assert the rendered diff contains BOTH
// the changed row's "-old" line (left sentinel) AND its "+new" line (right
// sentinel), for text AND csv.
func TestDiff_Data_PKAlignChangedAdjacent(t *testing.T) {
	const (
		k        = 50 // single-sided actor_id (added/removed)
		changeID = 51 // adjacent changed actor_id (k+1)
		leftSent = "LEFTSENT"
		rightSnt = "RIGHTSENT"
	)

	for _, format := range []string{"text", "csv"} {
		format := format
		t.Run(format, func(t *testing.T) {
			t.Run("added", func(t *testing.T) {
				// Modified copy on the LEFT: actor_id k deleted (so it surfaces as
				// "added" relative to the left), and the adjacent changed row set to
				// the left sentinel. Intact copy on the RIGHT with the right sentinel.
				got := runDiffChangedAdjacent(t, format,
					func(th *testh.Helper, left, right *source.Source) {
						th.ExecSQL(left, fmt.Sprintf(
							"DELETE FROM actor WHERE actor_id = %d", k,
						))
						th.ExecSQL(left, fmt.Sprintf(
							"UPDATE actor SET first_name = '%s' WHERE actor_id = %d", leftSent, changeID,
						))
						th.ExecSQL(right, fmt.Sprintf(
							"UPDATE actor SET first_name = '%s' WHERE actor_id = %d", rightSnt, changeID,
						))
					})

				requireHasDiffLine(t, got, "-", leftSent,
					"changed row's -old line (adjacent to an added row) must not be dropped")
				requireHasDiffLine(t, got, "+", rightSnt,
					"changed row's +new line must be present")
			})

			t.Run("removed", func(t *testing.T) {
				// Intact copy on the LEFT with the left sentinel. Modified copy on the
				// RIGHT: actor_id k deleted (so it surfaces as "removed"), adjacent
				// changed row set to the right sentinel.
				got := runDiffChangedAdjacent(t, format,
					func(th *testh.Helper, left, right *source.Source) {
						th.ExecSQL(right, fmt.Sprintf(
							"DELETE FROM actor WHERE actor_id = %d", k,
						))
						th.ExecSQL(right, fmt.Sprintf(
							"UPDATE actor SET first_name = '%s' WHERE actor_id = %d", rightSnt, changeID,
						))
						th.ExecSQL(left, fmt.Sprintf(
							"UPDATE actor SET first_name = '%s' WHERE actor_id = %d", leftSent, changeID,
						))
					})

				requireHasDiffLine(t, got, "-", leftSent,
					"changed row's -old line must be present")
				requireHasDiffLine(t, got, "+", rightSnt,
					"changed row's +new line (adjacent to a removed row) must not be dropped")
			})
		})
	}
}

// runDiffChangedAdjacent sets up two writable copies of the Sakila actor table
// (left @test_left, right @test_right), applies mutate to them, then runs
// sq diff --data at the default context with the given format and returns the
// rendered output.
func runDiffChangedAdjacent(t *testing.T, format string,
	mutate func(th *testh.Helper, left, right *source.Source),
) string {
	t.Helper()
	th := testh.New(t)

	srcLeft := th.Source(sakila.SL3) // writable temp copy

	// Build a second writable copy for the right operand.
	origPath := proj.Abs(sakila.PathSL3)
	dstPath := filepath.Join(tu.TempDir(t), "sakila_right.db")
	require.NoError(t, ioz.CopyFile(dstPath, origPath, true))
	srcRight := source.Source{
		Handle:   "@test_right",
		Type:     drivertype.SQLite,
		Location: "sqlite3://" + dstPath,
	}

	mutate(th, srcLeft, &srcRight)

	tr := testrun.New(th.Context, t, nil).Hush().Add(*srcLeft).Add(srcRight)
	args := []string{"diff", "--data", "--stop", "0"}
	if format != "text" {
		args = append(args, "-f", format)
	}
	args = append(args, srcLeft.Handle+"."+sakila.TblActor, srcRight.Handle+"."+sakila.TblActor)

	err := tr.Exec(args...)
	require.Error(t, err, "diff must report differences (exit code 1)")
	require.Equal(t, 1, errz.ExitCode(err), "sq diff exits 1 when differences exist")

	got := tr.Out.String()
	t.Log("diff output:\n" + got)
	return got
}

// TestDiff_Data_PKAlignDifferentOrdinal is the cross-table PK ordinal regression
// test. It verifies that sq diff --data correctly aligns rows by PK when the PK
// column sits at different ordinal positions in the two tables.
//
//	left:  CREATE TABLE t (id INTEGER PRIMARY KEY, name TEXT)  -- id at index 0
//	right: CREATE TABLE t (name TEXT, id INTEGER PRIMARY KEY)  -- id at index 1
//
// Pre-fix, collateByKey resolved PK column indexes from rs1 (left) only, then
// applied those same indexes to both sides. On the right table, index 0 is the
// name TEXT column, not id. compareIntKey cast rec2[0] to int64, got a string,
// and returned "PK value at index 0 in right record is string, want int64" →
// exit 2. Post-fix, keyIdxs are resolved independently per side (keyIdxs1=[0],
// keyIdxs2=[1]), so rec2[1] (the id column) is read correctly.
//
// With all three rows present on both sides, the PK merge pairs each id with
// its counterpart. Because the column order differs, record.Equal compares
// [id,name] vs [name,id] positionally and marks each pair as "changed" (not
// equal), so the diff exits 1 (differences exist), not 0. The critical
// assertion is that exit code is 1, NOT 2.
func TestDiff_Data_PKAlignDifferentOrdinal(t *testing.T) {
	const tbl = "t"

	th := testh.New(t)

	leftPath := filepath.Join(tu.TempDir(t), "left_ordinal.db")
	rightPath := filepath.Join(tu.TempDir(t), "right_ordinal.db")

	srcLeft := source.Source{
		Handle:   "@test_left",
		Type:     drivertype.SQLite,
		Location: "sqlite3://" + leftPath,
	}
	srcRight := source.Source{
		Handle:   "@test_right",
		Type:     drivertype.SQLite,
		Location: "sqlite3://" + rightPath,
	}

	// Left: id at column index 0.
	th.ExecSQL(&srcLeft, `CREATE TABLE "t" ("id" INTEGER PRIMARY KEY, "name" TEXT)`)
	th.ExecSQL(&srcLeft, `INSERT INTO "t" VALUES (1,'alice'), (2,'bob'), (3,'carol')`)

	// Right: id at column index 1 (name comes first).
	th.ExecSQL(&srcRight, `CREATE TABLE "t" ("name" TEXT, "id" INTEGER PRIMARY KEY)`)
	th.ExecSQL(&srcRight, `INSERT INTO "t" VALUES ('alice',1), ('bob',2), ('carol',3)`)

	// --unified 0 suppresses context so each diff line is exactly one data row.
	tr := testrun.New(th.Context, t, nil).Hush().Add(srcLeft, srcRight)
	err := tr.Exec(
		"diff", "--data", "--unified", "0", "--stop", "0",
		srcLeft.Handle+"."+tbl,
		srcRight.Handle+"."+tbl,
	)

	got := tr.Out.String()
	t.Log("diff output:\n" + got)

	// The tables differ (column order makes each row appear changed), so exit 1.
	// Pre-fix this would be exit 2 ("PK value ... is string, want int64").
	require.Error(t, err, "diff must report differences (column order makes rows appear changed)")
	require.Equal(t, 1, errz.ExitCode(err),
		"exit 2 means the PK index bug is still present; post-fix must exit 1")

	// Count added (+) and removed (-) data lines. With all three rows paired by
	// PK (same id values on both sides), there must be zero "added" and zero
	// "removed" lines. Each changed pair produces one "-" and one "+", so
	// added_diff_lines == removed_diff_lines == 3.
	var added, removed int
	for _, line := range strings.Split(got, "\n") {
		switch {
		case strings.HasPrefix(line, "+++") || strings.HasPrefix(line, "---"):
			// file header line
		case strings.HasPrefix(line, "+"):
			added++
		case strings.HasPrefix(line, "-"):
			removed++
		}
	}
	t.Logf("added=%d removed=%d (want added==removed==3, no single-sided pairs)", added, removed)
	require.Equal(t, 3, added,
		"each of the 3 changed pairs produces one + line; pre-fix exit 2 produces zero lines")
	require.Equal(t, 3, removed,
		"each of the 3 changed pairs produces one - line; miskeyed alignment would produce wrong counts")
	require.Equal(t, added, removed,
		"no rows are missing on either side: added and removed line counts must match")
}

// requireHasDiffLine asserts that got contains a diff data line beginning with
// the single-character prefix ("-" or "+") and containing substr, excluding the
// ---/+++ file-header lines.
func requireHasDiffLine(t *testing.T, got, prefix, substr, msg string) {
	t.Helper()
	header := prefix + prefix + prefix // "---" or "+++"
	for _, line := range strings.Split(got, "\n") {
		if strings.HasPrefix(line, header) {
			continue // file header line, not a data row
		}
		if strings.HasPrefix(line, prefix) && strings.Contains(line, substr) {
			return
		}
	}
	t.Fatalf("%s: no %q data line containing %q found in diff output:\n%s", msg, prefix, substr, got)
}

// TestDiff_Data_PKAlignSpecialCharPK is an integration gate for the D-2A/D-2B
// bug class: dataQuery formerly emitted `.name` (unquoted) for PK column
// selectors, which broke when the PK column name contained an SLQ operator
// character (e.g., `+`). The fix always emits `."name"` (double-quoted).
//
// This test creates two SQLite tables whose INTEGER PRIMARY KEY column is
// named `my+id` — a name that causes `order_by(.my+id)` to be mis-parsed
// by the SLQ engine as `.my` with ascending direction (D-2A) or to abort with
// a syntax error (D-2B), depending on whether a column named `my` exists.
// Post-fix, `order_by(."my+id")` is emitted, the SLQ lexer recognises the
// quoted NAME token, and the diff engine correctly aligns rows by their PK.
func TestDiff_Data_PKAlignSpecialCharPK(t *testing.T) {
	const tbl = "items"

	th := testh.New(t)

	// Create two fresh SQLite databases with a table whose PK column name
	// contains '+', the SLQ sort-direction operator.
	leftPath := filepath.Join(tu.TempDir(t), "left.db")
	rightPath := filepath.Join(tu.TempDir(t), "right.db")

	srcLeft := source.Source{
		Handle:   "@test_left",
		Type:     drivertype.SQLite,
		Location: "sqlite3://" + leftPath,
	}
	srcRight := source.Source{
		Handle:   "@test_right",
		Type:     drivertype.SQLite,
		Location: "sqlite3://" + rightPath,
	}

	// Create the table on both sides with a PK column named "my+id".
	const createDDL = `CREATE TABLE "items" ("my+id" INTEGER PRIMARY KEY, val TEXT)`
	th.ExecSQL(&srcLeft, createDDL)
	th.ExecSQL(&srcRight, createDDL)

	// Left: rows 1, 3, 4 (missing row 2).
	th.ExecSQL(&srcLeft, `INSERT INTO "items" VALUES (1,'a'), (3,'c'), (4,'d')`)
	// Right: rows 1, 2, 3, 4 (row 2 is the scattered-insert that should appear as "added").
	th.ExecSQL(&srcRight, `INSERT INTO "items" VALUES (1,'a'), (2,'b'), (3,'c'), (4,'d')`)

	// --unified 0 suppresses context rows so each diff line is exactly one data
	// row, keeping the assertion count unambiguous.
	tr := testrun.New(th.Context, t, nil).Hush().Add(srcLeft, srcRight)
	err := tr.Exec(
		"diff", "--data", "--unified", "0", "--stop", "0",
		srcLeft.Handle+"."+tbl,
		srcRight.Handle+"."+tbl,
	)

	require.Error(t, err, "diff must report differences (exit code 1)")
	require.Equal(t, 1, errz.ExitCode(err),
		"sq diff exits 1 when differences exist; exit 2 indicates a query error "+
			"(likely a misquoted order_by selector)")

	got := tr.Out.String()
	t.Log("diff output:\n" + got)

	var added, removed int
	for _, line := range strings.Split(got, "\n") {
		switch {
		case strings.HasPrefix(line, "+++") || strings.HasPrefix(line, "---"):
			// file header — skip
		case strings.HasPrefix(line, "+"):
			added++
		case strings.HasPrefix(line, "-"):
			removed++
		}
	}

	t.Logf("added=%d removed=%d (want added=1 removed=0)", added, removed)
	require.Equal(t, 1, added,
		"PK-aware diff must report exactly one added row for the scattered PK value; "+
			"a misquoted selector silently misaligns rows or aborts with exit 2")
	require.Zero(t, removed,
		"no rows were removed from the right table; zero removed lines expected")
}

// TestDiff_Data_NullablePKFallsBack verifies that a table whose integer
// primary key can hold NULL falls back to positional alignment instead of
// aborting. SQLite allows NULL in a primary key column that isn't the rowid
// alias, such as "id INT PRIMARY KEY". Key-aware merging can't order or compare
// a NULL key, so before the fix the diff failed with exit code 2 ("PK value
// ... is <nil>, want int64").
//
// The complementary case, an "INTEGER PRIMARY KEY" rowid alias without an
// explicit NOT NULL still getting key-aware alignment, is covered by
// TestDiff_Data_PKAlignSpecialCharPK.
func TestDiff_Data_NullablePKFallsBack(t *testing.T) {
	const tbl = "items"

	th := testh.New(t)
	srcLeft := source.Source{
		Handle:   "@test_left",
		Type:     drivertype.SQLite,
		Location: "sqlite3://" + filepath.Join(tu.TempDir(t), "left.db"),
	}
	srcRight := source.Source{
		Handle:   "@test_right",
		Type:     drivertype.SQLite,
		Location: "sqlite3://" + filepath.Join(tu.TempDir(t), "right.db"),
	}

	const createDDL = `CREATE TABLE "items" (id INT PRIMARY KEY, val TEXT)`
	th.ExecSQL(&srcLeft, createDDL)
	th.ExecSQL(&srcRight, createDDL)
	th.ExecSQL(&srcLeft, `INSERT INTO "items" VALUES (1,'a'), (NULL,'z'), (3,'c')`)
	th.ExecSQL(&srcRight, `INSERT INTO "items" VALUES (1,'a'), (2,'b'), (3,'c')`)

	tr := testrun.New(th.Context, t, nil).Hush().Add(srcLeft, srcRight)
	err := tr.Exec(
		"diff", "--data", "--stop", "0",
		srcLeft.Handle+"."+tbl,
		srcRight.Handle+"."+tbl,
	)

	got := tr.Out.String()
	t.Log("diff output:\n" + got)

	require.Error(t, err, "diff must report differences (exit code 1)")
	require.Equal(t, 1, errz.ExitCode(err),
		"sq diff exits 1 when differences exist; exit 2 means the diff aborted")
	require.Contains(t, got, "rows aligned by position",
		"a nullable primary key must fall back to positional alignment")
}

// TestDiff_Data_PKAlignAllDrivers verifies that key-aware alignment works for
// each SQL driver, not only SQLite. Each driver's actor table is diffed
// against a SQLite copy of sakila with 5 scattered actors deleted. Key-aware
// merging needs both sides to report a matching, non-nullable integer primary
// key, so this catches a driver that misreports its key metadata (for example,
// reporting actor_id as nullable) and thereby silently falls back to
// positional alignment, which cascades.
//
// Two drivers currently fall back, for reasons unrelated to the driver's
// nullability reporting, and are pinned here so that a change in either
// direction is noticed:
//
//   - ClickHouse doesn't report its sort key as a primary key.
//   - Oracle reports the key column as ACTOR_ID, which doesn't match SQLite's
//     actor_id; key-aware merging requires identical key column names.
func TestDiff_Data_PKAlignAllDrivers(t *testing.T) {
	const wantRemoved = 5
	wantFallback := map[string]bool{sakila.CH: true, sakila.Ora: true}

	for _, handle := range sakila.SQLAll() {
		t.Run(tu.Name(handle), func(t *testing.T) {
			th := testh.New(t)
			src := th.Source(handle)

			rightPath := filepath.Join(tu.TempDir(t), "right.db")
			require.NoError(t, ioz.CopyFile(rightPath, proj.Abs(sakila.PathSL3), true))
			srcRight := source.Source{
				Handle:   "@test_right",
				Type:     drivertype.SQLite,
				Location: "sqlite3://" + rightPath,
			}
			th.ExecSQL(&srcRight, "DELETE FROM actor WHERE actor_id IN (2, 4, 6, 8, 10)")

			tr := testrun.New(th.Context, t, nil).Hush().Add(*src, srcRight)
			err := tr.Exec(
				"diff", "--data", "--unified", "0", "--stop", "0",
				src.Handle+"."+sakila.TblActor,
				srcRight.Handle+"."+sakila.TblActor,
			)
			require.Error(t, err, "diff must report differences (exit code 1)")
			require.Equal(t, 1, errz.ExitCode(err))

			got := tr.Out.String()
			fellBack := strings.Contains(got, "rows aligned by position")
			if wantFallback[handle] {
				require.True(t, fellBack, "expected positional fallback for %s", handle)
				return
			}

			require.False(t, fellBack, "%s must use key-aware alignment; got:\n%s", handle, got)
			var added, removed int
			for _, line := range strings.Split(got, "\n") {
				switch {
				case strings.HasPrefix(line, "+++") || strings.HasPrefix(line, "---"):
				case strings.HasPrefix(line, "+"):
					added++
				case strings.HasPrefix(line, "-"):
					removed++
				}
			}
			require.Equal(t, wantRemoved, removed, "one removed line per deleted actor")
			require.Zero(t, added)
		})
	}
}

// TestDiff_Data_StopTrailer verifies that the --stop trailer appears only when
// a difference beyond the stop limit actually exists. Previously the trailer
// was printed whenever the diff stopped reading with rows left over, even if
// every remaining row was identical, which claimed truncation that hadn't
// happened. Both alignment paths own their own stop logic, so both are
// covered: key-aware (integer primary key) and positional (no primary key).
//
// Each table pair has exactly 3 differences, the last of which is followed by
// identical rows, so the diff stops with rows unread when --stop is 3.
func TestDiff_Data_StopTrailer(t *testing.T) {
	const trailer = "stopped after"

	testCases := []struct {
		name       string
		ddl        string
		left       string
		right      string
		stop       int
		wantDiffs  int // data lines (+ or -) expected in the output
		wantMarker bool
	}{
		{
			name:  "key_exact",
			ddl:   `CREATE TABLE t (id INTEGER PRIMARY KEY, name TEXT)`,
			left:  `INSERT INTO t VALUES (1,'a'),(3,'c'),(4,'d'),(6,'f'),(7,'g'),(9,'i'),(10,'j')`,
			right: `INSERT INTO t VALUES (1,'a'),(2,'b'),(3,'c'),(4,'d'),(5,'e'),(6,'f'),(7,'g'),(8,'h'),(9,'i'),(10,'j')`,
			stop:  3, wantDiffs: 3, wantMarker: false,
		},
		{
			name:  "key_more",
			ddl:   `CREATE TABLE t (id INTEGER PRIMARY KEY, name TEXT)`,
			left:  `INSERT INTO t VALUES (1,'a'),(3,'c'),(4,'d'),(6,'f'),(7,'g'),(9,'i'),(10,'j')`,
			right: `INSERT INTO t VALUES (1,'a'),(2,'b'),(3,'c'),(4,'d'),(5,'e'),(6,'f'),(7,'g'),(8,'h'),(9,'i'),(10,'j')`,
			stop:  2, wantDiffs: 2, wantMarker: true,
		},
		{
			name:  "positional_exact",
			ddl:   `CREATE TABLE t (id INTEGER, name TEXT)`,
			left:  `INSERT INTO t VALUES (1,'a'),(2,'b'),(3,'c'),(4,'d'),(5,'e'),(6,'f'),(7,'g'),(8,'h'),(9,'i'),(10,'j')`,
			right: `INSERT INTO t VALUES (1,'a'),(2,'B'),(3,'c'),(4,'d'),(5,'E'),(6,'f'),(7,'g'),(8,'H'),(9,'i'),(10,'j')`,
			stop:  3, wantDiffs: 6, wantMarker: false,
		},
		{
			name:  "positional_more",
			ddl:   `CREATE TABLE t (id INTEGER, name TEXT)`,
			left:  `INSERT INTO t VALUES (1,'a'),(2,'b'),(3,'c'),(4,'d'),(5,'e'),(6,'f'),(7,'g'),(8,'h'),(9,'i'),(10,'j')`,
			right: `INSERT INTO t VALUES (1,'a'),(2,'B'),(3,'c'),(4,'d'),(5,'E'),(6,'f'),(7,'g'),(8,'H'),(9,'i'),(10,'j')`,
			stop:  2, wantDiffs: 4, wantMarker: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			th := testh.New(t)
			srcLeft := source.Source{
				Handle:   "@test_left",
				Type:     drivertype.SQLite,
				Location: "sqlite3://" + filepath.Join(tu.TempDir(t), "left.db"),
			}
			srcRight := source.Source{
				Handle:   "@test_right",
				Type:     drivertype.SQLite,
				Location: "sqlite3://" + filepath.Join(tu.TempDir(t), "right.db"),
			}
			th.ExecSQL(&srcLeft, tc.ddl)
			th.ExecSQL(&srcRight, tc.ddl)
			th.ExecSQL(&srcLeft, tc.left)
			th.ExecSQL(&srcRight, tc.right)

			tr := testrun.New(th.Context, t, nil).Hush().Add(srcLeft, srcRight)
			err := tr.Exec(
				"diff", "--data", "--unified", "0", "--stop", strconv.Itoa(tc.stop),
				srcLeft.Handle+".t", srcRight.Handle+".t",
			)
			require.Error(t, err, "diff must report differences (exit code 1)")
			require.Equal(t, 1, errz.ExitCode(err))

			got := tr.Out.String()
			t.Log("diff output:\n" + got)

			var diffLines int
			for _, line := range strings.Split(got, "\n") {
				switch {
				case strings.HasPrefix(line, "+++") || strings.HasPrefix(line, "---"):
				case strings.HasPrefix(line, "+") || strings.HasPrefix(line, "-"):
					diffLines++
				}
			}
			require.Equal(t, tc.wantDiffs, diffLines)

			if tc.wantMarker {
				require.Contains(t, got, fmt.Sprintf("stopped after %d differences; more exist", tc.stop))
			} else {
				require.NotContains(t, got, trailer,
					"no difference beyond the stop limit exists, so nothing was truncated")
			}
		})
	}
}

// TestDiff_Data_MissingTable verifies that a data diff where a table exists on
// only one side completes, treating the missing table as empty: the existing
// side's rows are reported as removed (or added). Previously the query for the
// missing table returned without signaling end-of-data, so the diff blocked
// forever. That affected table-level diffs naming a missing table, and
// source-level diffs whenever the two sources' table sets differ, since
// source-level --data diffs the union of both sides' table names.
//
// Each case runs under a timeout so that a regression fails fast rather than
// hanging the test run.
func TestDiff_Data_MissingTable(t *testing.T) {
	const (
		sharedDDL  = `CREATE TABLE t (id INTEGER PRIMARY KEY, name TEXT)`
		onlyDDL    = `CREATE TABLE only_one (id INTEGER PRIMARY KEY, v TEXT)`
		onlyInsert = `INSERT INTO only_one VALUES (1,'only1'),(2,'only2'),(3,'only3')`
	)

	testCases := []struct {
		name      string
		onlyLeft  bool // only_one exists in left (true) or right (false)
		args      func(left, right string) []string
		wantPrefx string // "-" when the rows are only in left, "+" when only in right
	}{
		{
			name:     "table_right_missing",
			onlyLeft: true,
			args: func(left, right string) []string {
				return []string{left + ".only_one", right + ".only_one"}
			},
			wantPrefx: "-",
		},
		{
			name:     "table_left_missing",
			onlyLeft: false,
			args: func(left, right string) []string {
				return []string{left + ".only_one", right + ".only_one"}
			},
			wantPrefx: "+",
		},
		{
			name:     "source_extra_table",
			onlyLeft: true,
			args: func(left, right string) []string {
				return []string{left, right}
			},
			wantPrefx: "-",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			th := testh.New(t)
			srcLeft := source.Source{
				Handle:   "@test_left",
				Type:     drivertype.SQLite,
				Location: "sqlite3://" + filepath.Join(tu.TempDir(t), "left.db"),
			}
			srcRight := source.Source{
				Handle:   "@test_right",
				Type:     drivertype.SQLite,
				Location: "sqlite3://" + filepath.Join(tu.TempDir(t), "right.db"),
			}
			th.ExecSQL(&srcLeft, sharedDDL)
			th.ExecSQL(&srcRight, sharedDDL)
			th.ExecSQL(&srcLeft, `INSERT INTO t VALUES (1,'a'),(2,'b')`)
			th.ExecSQL(&srcRight, `INSERT INTO t VALUES (1,'a'),(2,'b')`)
			onlySrc := &srcRight
			if tc.onlyLeft {
				onlySrc = &srcLeft
			}
			th.ExecSQL(onlySrc, onlyDDL)
			th.ExecSQL(onlySrc, onlyInsert)

			ctx, cancel := context.WithTimeout(th.Context, 30*time.Second)
			defer cancel()

			tr := testrun.New(ctx, t, nil).Hush().Add(srcLeft, srcRight)
			args := append([]string{"diff", "--data", "--stop", "0"}, tc.args(srcLeft.Handle, srcRight.Handle)...)
			err := tr.Exec(args...)
			require.NoError(t, ctx.Err(), "diff must complete, not block")

			got := tr.Out.String()
			t.Log("diff output:\n" + got)

			require.Error(t, err, "diff must report differences (exit code 1)")
			require.Equal(t, 1, errz.ExitCode(err))
			for _, v := range []string{"only1", "only2", "only3"} {
				requireHasDiffLine(t, got, tc.wantPrefx, v, "row of the one-sided table")
			}
			// With one side empty, nothing can cascade, so the positional
			// fallback note would be noise.
			require.NotContains(t, got, "rows aligned by position")
		})
	}
}

// TestDiff_Data_HunkHeaderCounts verifies the unified-diff hunk headers that
// the native text and csv writers emit, against the exact headers GNU diff
// produces for the same line sequences (one line per row). The expected values
// were generated with "diff -U<n>" over files with one line per row.
//
// With key-aligned diffs, an ADDED pair (Rec1()==nil) contributes zero lines to
// the left side and one to the right; a REMOVED pair is the inverse. The header
// must carry per-side counts, and a zero-count range must use the GNU form, in
// which the start is the line before the (empty) range: inserting after left
// line 1 is "-1,0", and an empty side is "0,0". Tools such as patch(1) place
// the change using that start, so an off-by-one there misplaces the row.
//
// These cases each produce a single hunk. With multiple hunks, the left start
// line drifts once the sides have different row counts (a known limitation,
// tracked separately), so exact headers aren't asserted for that shape.
func TestDiff_Data_HunkHeaderCounts(t *testing.T) {
	const tbl = "items"

	testCases := []struct {
		name        string
		unified     int
		leftInsert  string
		rightInsert string
		want        []string
	}{
		{
			name:        "pure_insertion",
			leftInsert:  `INSERT INTO "items" VALUES (1,'a'),(3,'c')`,
			rightInsert: `INSERT INTO "items" VALUES (1,'a'),(2,'b'),(3,'c')`,
			want:        []string{"@@ -1,0 +2 @@"},
		},
		{
			name:        "pure_deletion",
			leftInsert:  `INSERT INTO "items" VALUES (1,'a'),(2,'b'),(3,'c')`,
			rightInsert: `INSERT INTO "items" VALUES (1,'a'),(3,'c')`,
			want:        []string{"@@ -2 +1,0 @@"},
		},
		{
			name:        "insert_at_top",
			leftInsert:  `INSERT INTO "items" VALUES (2,'b'),(3,'c')`,
			rightInsert: `INSERT INTO "items" VALUES (1,'a'),(2,'b'),(3,'c')`,
			want:        []string{"@@ -0,0 +1 @@"},
		},
		{
			name:       "right_empty",
			leftInsert: `INSERT INTO "items" VALUES (1,'a'),(2,'b')`,
			want:       []string{"@@ -1,2 +0,0 @@"},
		},
		{
			name:        "changed",
			leftInsert:  `INSERT INTO "items" VALUES (1,'a'),(2,'b'),(3,'c')`,
			rightInsert: `INSERT INTO "items" VALUES (1,'a'),(2,'B'),(3,'c')`,
			want:        []string{"@@ -2 +2 @@"},
		},
		{
			// A changed row and an added row in one hunk, with context.
			name:        "mixed_with_context",
			unified:     1,
			leftInsert:  `INSERT INTO "items" VALUES (1,'a'),(2,'b'),(4,'d')`,
			rightInsert: `INSERT INTO "items" VALUES (1,'a'),(2,'B'),(3,'c'),(4,'d')`,
			want:        []string{"@@ -1,3 +1,4 @@"},
		},
	}

	for _, format := range []string{"text", "csv"} {
		t.Run(format, func(t *testing.T) {
			for _, tc := range testCases {
				t.Run(tc.name, func(t *testing.T) {
					got := runDiffHunkHeader(t, format, tbl, tc.unified, tc.leftInsert, tc.rightInsert)
					require.Equal(t, tc.want, parseHunkHeaders(got))
				})
			}
		})
	}
}

// runDiffHunkHeader creates two SQLite databases with the given table and
// insert statements (an empty statement leaves that side's table empty), runs
// sq diff --data --unified <unified>, and returns the raw diff output. The
// table is created with an INTEGER PRIMARY KEY so the PK-aware alignment path
// is exercised.
func runDiffHunkHeader(t *testing.T, format, tbl string, unified int, leftInsert, rightInsert string) string {
	t.Helper()
	th := testh.New(t)

	leftPath := filepath.Join(tu.TempDir(t), "left_hdr.db")
	rightPath := filepath.Join(tu.TempDir(t), "right_hdr.db")

	srcLeft := source.Source{
		Handle:   "@test_left",
		Type:     drivertype.SQLite,
		Location: "sqlite3://" + leftPath,
	}
	srcRight := source.Source{
		Handle:   "@test_right",
		Type:     drivertype.SQLite,
		Location: "sqlite3://" + rightPath,
	}

	ddl := fmt.Sprintf(`CREATE TABLE "%s" ("id" INTEGER PRIMARY KEY, "val" TEXT)`, tbl)
	th.ExecSQL(&srcLeft, ddl)
	th.ExecSQL(&srcRight, ddl)
	if leftInsert != "" {
		th.ExecSQL(&srcLeft, leftInsert)
	}
	if rightInsert != "" {
		th.ExecSQL(&srcRight, rightInsert)
	}

	tr := testrun.New(th.Context, t, nil).Hush().Add(srcLeft, srcRight)
	args := []string{"diff", "--data", "--unified", strconv.Itoa(unified), "--stop", "0"}
	if format != "text" {
		args = append(args, "-f", format)
	}
	args = append(args, srcLeft.Handle+"."+tbl, srcRight.Handle+"."+tbl)

	err := tr.Exec(args...)
	require.Error(t, err, "diff must report differences (exit code 1)")
	require.Equal(t, 1, errz.ExitCode(err), "sq diff exits 1 when differences exist")

	got := tr.Out.String()
	t.Log("diff output:\n" + got)
	return got
}

// parseHunkHeaders returns all "@@ ... @@" hunk header lines from a unified
// diff output.
func parseHunkHeaders(output string) []string {
	var headers []string
	for _, line := range strings.Split(output, "\n") {
		if strings.HasPrefix(line, "@@") {
			headers = append(headers, line)
		}
	}
	return headers
}
