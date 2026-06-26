// cli/diff/keymerge.go
package diff

import (
	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/kind"
	"github.com/neilotoole/sq/libsq/core/record"
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
