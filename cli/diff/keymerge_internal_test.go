// cli/diff/keymerge_internal_test.go
package diff

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/libsq/core/kind"
	"github.com/neilotoole/sq/libsq/core/record"
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
