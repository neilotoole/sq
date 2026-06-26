// cli/diff/keymerge_internal_test.go
package diff

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/libsq/core/kind"
	"github.com/neilotoole/sq/libsq/core/record"
	"github.com/neilotoole/sq/libsq/source"
	"github.com/neilotoole/sq/libsq/source/metadata"
)

// newFieldMeta constructs a *record.FieldMeta whose Name() returns name.
// Only the name matters for TestPKColIndexes.
func newFieldMeta(name string) *record.FieldMeta {
	return record.NewFieldMeta(&record.ColumnTypeData{Name: name}, "")
}

func tbl(cols ...*metadata.Column) *metadata.Table {
	return &metadata.Table{Columns: cols}
}

func col(name string, k kind.Kind, pk bool) *metadata.Column {
	return &metadata.Column{Name: name, Kind: k, PrimaryKey: pk}
}

func TestPKMergeKey(t *testing.T) {
	intPK := tbl(col("payment_id", kind.Int, true), col("amount", kind.Decimal, false))
	names, ok := pkMergeKey(intPK, intPK)
	require.True(t, ok)
	require.Equal(t, []string{"payment_id"}, names)

	// Composite int PK is eligible.
	comp := tbl(col("actor_id", kind.Int, true), col("film_id", kind.Int, true))
	names, ok = pkMergeKey(comp, comp)
	require.True(t, ok)
	require.Equal(t, []string{"actor_id", "film_id"}, names)

	// No PK -> ineligible.
	noPK := tbl(col("a", kind.Int, false))
	_, ok = pkMergeKey(noPK, noPK)
	require.False(t, ok)

	// String PK -> ineligible in v1.
	strPK := tbl(col("code", kind.Text, true))
	_, ok = pkMergeKey(strPK, strPK)
	require.False(t, ok)

	// Mismatched PK columns between the two tables -> ineligible.
	other := tbl(col("rental_id", kind.Int, true))
	_, ok = pkMergeKey(intPK, other)
	require.False(t, ok)

	// nil metadata -> ineligible, no panic.
	_, ok = pkMergeKey(nil, intPK)
	require.False(t, ok)
}

func TestPKColIndexes(t *testing.T) {
	rm := record.Meta{
		newFieldMeta("payment_id"),
		newFieldMeta("amount"),
		newFieldMeta("rental_id"),
	}
	idxs, ok := pkColIndexes(rm, []string{"payment_id"})
	require.True(t, ok)
	require.Equal(t, []int{0}, idxs)

	idxs, ok = pkColIndexes(rm, []string{"rental_id", "payment_id"})
	require.True(t, ok)
	require.Equal(t, []int{2, 0}, idxs)

	_, ok = pkColIndexes(rm, []string{"missing"})
	require.False(t, ok)
}

func TestCompareIntKey(t *testing.T) {
	idxs := []int{0}
	lt, err := compareIntKey(record.Record{int64(1)}, record.Record{int64(2)}, idxs)
	require.NoError(t, err)
	require.Equal(t, -1, lt)

	eq, err := compareIntKey(record.Record{int64(5)}, record.Record{int64(5)}, idxs)
	require.NoError(t, err)
	require.Equal(t, 0, eq)

	gt, err := compareIntKey(record.Record{int64(9)}, record.Record{int64(3)}, idxs)
	require.NoError(t, err)
	require.Equal(t, 1, gt)

	// Composite: first column ties, second decides.
	comp := []int{0, 1}
	c, err := compareIntKey(record.Record{int64(1), int64(7)}, record.Record{int64(1), int64(9)}, comp)
	require.NoError(t, err)
	require.Equal(t, -1, c)

	// Non-int keyed value -> error.
	_, err = compareIntKey(record.Record{"x"}, record.Record{int64(1)}, idxs)
	require.Error(t, err)
}

// drainMerge feeds left/right records into mergeRecordsByKey and returns the
// emitted pairs as compact "k:side" tokens: "added=K" (left nil), "removed=K"
// (right nil), "same=K" (equal), "chg=K" (both present, differ). Key is the
// first PK column's int64 value, taken from whichever side is non-nil.
func drainMerge(t *testing.T, keyIdxs []int, leftRows, rightRows []record.Record) []string {
	t.Helper()
	left := make(chan record.Record, len(leftRows)+1)
	right := make(chan record.Record, len(rightRows)+1)
	for _, r := range leftRows {
		left <- r
	}
	close(left)
	for _, r := range rightRows {
		right <- r
	}
	close(right)

	var got []string
	err := mergeRecordsByKey(context.Background(), left, right, keyIdxs, func(rp record.Pair) bool {
		var k int64
		switch {
		case rp.Rec1() != nil:
			k = rp.Rec1()[keyIdxs[0]].(int64)
		default:
			k = rp.Rec2()[keyIdxs[0]].(int64)
		}
		switch {
		case rp.Rec1() == nil:
			got = append(got, fmt.Sprintf("added=%d", k))
		case rp.Rec2() == nil:
			got = append(got, fmt.Sprintf("removed=%d", k))
		case rp.Equal():
			got = append(got, fmt.Sprintf("same=%d", k))
		default:
			got = append(got, fmt.Sprintf("chg=%d", k))
		}
		return true
	})
	require.NoError(t, err)
	return got
}

func rowID(id int64, rest ...any) record.Record {
	return append(record.Record{id}, rest...)
}

func TestMergeRecordsByKey_ScatteredInserts(t *testing.T) {
	// Issue #947 minimal shape: right has 3 extra rows (2,5,8) scattered
	// through the key range. Expect 3 clean "added", everything else "same".
	left := []record.Record{rowID(1), rowID(3), rowID(4), rowID(6), rowID(7), rowID(9), rowID(10)}
	right := []record.Record{
		rowID(1), rowID(2), rowID(3), rowID(4), rowID(5),
		rowID(6), rowID(7), rowID(8), rowID(9), rowID(10),
	}
	got := drainMerge(t, []int{0}, left, right)
	require.Equal(t, []string{
		"same=1", "added=2", "same=3", "same=4", "added=5",
		"same=6", "same=7", "added=8", "same=9", "same=10",
	}, got)
}

func TestMergeRecordsByKey_Removed(t *testing.T) {
	left := []record.Record{rowID(1), rowID(2), rowID(3)}
	right := []record.Record{rowID(2)}
	got := drainMerge(t, []int{0}, left, right)
	require.Equal(t, []string{"removed=1", "same=2", "removed=3"}, got)
}

func TestMergeRecordsByKey_Changed(t *testing.T) {
	// Same key, differing non-key column -> "chg", not removed+added.
	left := []record.Record{rowID(1, "a"), rowID(2, "b")}
	right := []record.Record{rowID(1, "a"), rowID(2, "B")}
	got := drainMerge(t, []int{0}, left, right)
	require.Equal(t, []string{"same=1", "chg=2"}, got)
}

func TestMergeRecordsByKey_OneSideEmpty(t *testing.T) {
	got := drainMerge(t, []int{0}, nil, []record.Record{rowID(1), rowID(2)})
	require.Equal(t, []string{"added=1", "added=2"}, got)

	got = drainMerge(t, []int{0}, []record.Record{rowID(1), rowID(2)}, nil)
	require.Equal(t, []string{"removed=1", "removed=2"}, got)
}

func TestMergeRecordsByKey_EmitStop(t *testing.T) {
	left := []record.Record{rowID(1), rowID(2), rowID(3)}
	right := []record.Record{rowID(1), rowID(2), rowID(3)}
	left2 := make(chan record.Record, 4)
	right2 := make(chan record.Record, 4)
	for _, r := range left {
		left2 <- r
	}
	close(left2)
	for _, r := range right {
		right2 <- r
	}
	close(right2)

	var n int
	err := mergeRecordsByKey(context.Background(), left2, right2, []int{0}, func(record.Pair) bool {
		n++
		return n < 2 // stop after emitting 2 pairs
	})
	require.NoError(t, err)
	require.Equal(t, 2, n)
}

func TestDataQuery(t *testing.T) {
	td := source.Table{Handle: "@a", Name: "payment"}

	// No PK -> bare query, unchanged from the positional path.
	require.Equal(t, `@a."payment"`, dataQuery(td, nil))

	// Single int PK -> ordered.
	require.Equal(t, `@a."payment" | order_by(.payment_id)`, dataQuery(td, []string{"payment_id"}))

	// Composite PK -> ordered by both, in order.
	fa := source.Table{Handle: "@a", Name: "film_actor"}
	require.Equal(t,
		`@a."film_actor" | order_by(.actor_id, .film_id)`,
		dataQuery(fa, []string{"actor_id", "film_id"}),
	)
}
