// cli/diff/keymerge.go
package diff

import (
	"context"
	"strings"

	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/kind"
	"github.com/neilotoole/sq/libsq/core/record"
	"github.com/neilotoole/sq/libsq/core/stringz"
	"github.com/neilotoole/sq/libsq/source"
	"github.com/neilotoole/sq/libsq/source/metadata"
)

// pkMergeKey reports whether md1 and md2 share a primary key that is eligible
// for key-aware row merging, and if so returns the ordered PK column names.
//
// v1 eligibility (see plan Global Constraints): both tables must expose a
// non-empty primary key, the PK column names must match in order, and every PK
// column must be integer-kind. Integer ordering is unambiguous across drivers,
// so the DB-side ORDER BY is guaranteed to agree with compareIntKey. Non-int
// PKs fall back to positional alignment.
func pkMergeKey(md1, md2 *metadata.Table) (pkColNames []string, ok bool) {
	pk1 := md1.PKCols()
	pk2 := md2.PKCols()
	if len(pk1) == 0 || len(pk1) != len(pk2) {
		return nil, false
	}

	names := make([]string, len(pk1))
	for i := range pk1 {
		if pk1[i].Name != pk2[i].Name {
			return nil, false
		}
		if pk1[i].Kind != kind.Int || pk2[i].Kind != kind.Int {
			return nil, false
		}
		names[i] = pk1[i].Name
	}
	return names, true
}

// pkColIndexes maps PK column names to their positions in a record described by
// recMeta, preserving the order of pkColNames. ok is false if any name is not
// found.
func pkColIndexes(recMeta record.Meta, pkColNames []string) (idxs []int, ok bool) {
	names := recMeta.Names()
	idxs = make([]int, len(pkColNames))
	for i, want := range pkColNames {
		found := -1
		for j, have := range names {
			if have == want {
				found = j
				break
			}
		}
		if found == -1 {
			return nil, false
		}
		idxs[i] = found
	}
	return idxs, true
}

// mergeRecordsByKey reads PK-ordered records from left and right and emits
// record.Pair values in key order. Both channels must deliver records sorted
// ascending by the PK columns at keyIdxs (the caller guarantees this via an
// ORDER BY query). A receive of nil from a channel signals that side is
// exhausted.
//
// For each step the lower-keyed record is emitted: equal keys produce a pair of
// both records (equal/changed decided by record.Equal inside NewPair); a
// left-only key produces (rec, nil) (removed); a right-only key produces
// (nil, rec) (added). emit is called once per pair; if it returns false the
// merge stops and returns nil. row is a monotonic counter used as the pair's
// nominal row index, matching the positional path's semantics.
func mergeRecordsByKey(ctx context.Context, left, right <-chan record.Record,
	keyIdxs []int, emit func(rp record.Pair) bool,
) error {
	var (
		rec1, rec2   record.Record
		advance1     = true
		advance2     = true
		have1, have2 bool
		row          int
	)

	recv := func(ch <-chan record.Record) (record.Record, bool, error) {
		select {
		case <-ctx.Done():
			return nil, false, errz.Err(context.Cause(ctx))
		case r := <-ch:
			return r, r != nil, nil
		}
	}

	for {
		if ctx.Err() != nil {
			return errz.Err(context.Cause(ctx))
		}

		var err error
		if advance1 {
			if rec1, have1, err = recv(left); err != nil {
				return err
			}
			advance1 = false
		}
		if advance2 {
			if rec2, have2, err = recv(right); err != nil {
				return err
			}
			advance2 = false
		}

		var rp record.Pair
		switch {
		case !have1 && !have2:
			return nil // both exhausted
		case !have2:
			rp = record.NewPair(row, rec1, nil) // removed
			advance1 = true
		case !have1:
			rp = record.NewPair(row, nil, rec2) // added
			advance2 = true
		default:
			c, cmpErr := compareIntKey(rec1, rec2, keyIdxs)
			if cmpErr != nil {
				return cmpErr
			}
			switch {
			case c == 0:
				rp = record.NewPair(row, rec1, rec2) // same or changed
				advance1 = true
				advance2 = true
			case c < 0:
				rp = record.NewPair(row, rec1, nil) // removed (left key is lower)
				advance1 = true
			default:
				rp = record.NewPair(row, nil, rec2) // added (right key is lower)
				advance2 = true
			}
		}

		row++
		if !emit(rp) {
			return nil
		}
	}
}

// compareIntKey returns -1, 0 or 1 comparing the integer PK tuples of rec1 and
// rec2 at the given column indexes. It returns an error if any keyed value is
// not an int64 (which should not happen for an integer-kind PK column, but is
// guarded so a surprising value triggers a clean fallback rather than a panic).
func compareIntKey(rec1, rec2 record.Record, idxs []int) (int, error) {
	for _, idx := range idxs {
		v1, ok := rec1[idx].(int64)
		if !ok {
			return 0, errz.Errorf("diff: PK value at index %d in left record is %T, want int64", idx, rec1[idx])
		}
		v2, ok := rec2[idx].(int64)
		if !ok {
			return 0, errz.Errorf("diff: PK value at index %d in right record is %T, want int64", idx, rec2[idx])
		}
		switch {
		case v1 < v2:
			return -1, nil
		case v1 > v2:
			return 1, nil
		}
	}
	return 0, nil
}

// dataQuery returns the SLQ query that the diff engine executes for one side of
// a table-data diff. With no pkColNames it returns the bare table query,
// identical to the legacy positional path. With pkColNames it appends an
// order_by so the rows arrive PK-sorted, which the key-merge collation requires.
func dataQuery(td source.Table, pkColNames []string) string {
	q := td.Handle + "." + stringz.DoubleQuote(td.Name)
	if len(pkColNames) == 0 {
		return q
	}

	sel := make([]string, len(pkColNames))
	for i, name := range pkColNames {
		sel[i] = slqQuotedSelector(name)
	}
	return q + " | order_by(" + strings.Join(sel, ", ") + ")"
}

// slqQuotedSelector returns a double-quoted SLQ selector for the given column
// name, suitable for use in order_by and similar SLQ expressions. Examples:
//
//	actor_id  →  ."actor_id"
//	my id     →  ."my id"
//	a.b       →  ."a.b"
//	a+b       →  ."a+b"
//
// The SLQ STRING token (libsq/ast/internal/slq/SLQLexer.interp, ATN charset
// analysis) admits any character except `"` and `\` literally, and treats `\"`
// and `\\` as escape sequences (JSON-style). All other characters — including
// space, `.`, `+`, `-`, `|`, `(`, `)` — are safe unescaped inside the quotes.
//
// Note: libsq/ast/selector.go extractSelVal strips the outer quotes but does
// not unescape backslash sequences. Column names containing a literal `"` or
// `\` are correctly escaped here so the SLQ lexer accepts the input, but the
// extracted column name seen by the SQL renderer will retain the backslash.
// This is a pre-existing limitation of extractSelVal; the common special
// characters (space, `.`, `+`, `-`, `|`, `(`, `)`) are fully unaffected.
func slqQuotedSelector(name string) string {
	// Escape `\` before `"` to avoid double-escaping.
	escaped := strings.ReplaceAll(name, `\`, `\\`)
	escaped = strings.ReplaceAll(escaped, `"`, `\"`)
	return `."` + escaped + `"`
}
