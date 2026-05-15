package tablew

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/libsq/source/metadata"
)

// TestIndexEntriesByColumn_TagMatrix pins the verbose-text dedup
// contract for [indexEntriesByColumn]:
//
//   - PK-backing index entries are dropped (Column.PrimaryKey already
//     marks the participating columns under the PK column).
//   - UC-backing index entries are KEPT but tagged backing=true, so
//     the renderer can mute them (parens + Subdued color). Column-set
//     match is used (not name match) so SQLite's auto-named
//     sqlite_autoindex_* entries dedupe correctly.
//   - Standalone CREATE UNIQUE INDEX definitions (no matching UC) are
//     kept with backing=false.
//   - Plain non-unique secondary indexes are kept with backing=false.
//   - At most one matching unique index is bound per UC: a second
//     unique index over the same columns stays as backing=false.
func TestIndexEntriesByColumn_TagMatrix(t *testing.T) {
	tbl := &metadata.Table{
		Name: "demo",
		Columns: []*metadata.Column{
			{Name: "id"},
			{Name: "email"},
			{Name: "first_name"},
			{Name: "last_name"},
			{Name: "nickname"},
		},
		UniqueConstraints: []*metadata.UniqueConstraint{
			{
				Name:    "demo_email_key",
				Table:   "demo",
				Columns: []string{"email"},
			},
			{
				Name:    "uniq_full_name",
				Table:   "demo",
				Columns: []string{"first_name", "last_name"},
			},
		},
		Indexes: []*metadata.Index{
			// PK-backing index — must be filtered entirely.
			{
				Name:    "demo_pkey",
				Table:   "demo",
				Columns: []string{"id"},
				Unique:  true,
				Primary: true,
			},
			// UC-backing index whose name matches the UC — kept,
			// tagged backing=true.
			{
				Name:    "demo_email_key",
				Table:   "demo",
				Columns: []string{"email"},
				Unique:  true,
			},
			// UC-backing index whose name DIFFERS from the UC name
			// (mirrors SQLite's sqlite_autoindex_*). Still tagged
			// backing=true because the column-set matches a UC.
			{
				Name:    "sqlite_autoindex_demo_1",
				Table:   "demo",
				Columns: []string{"first_name", "last_name"},
				Unique:  true,
			},
			// Standalone CREATE UNIQUE INDEX with no matching UC —
			// kept, backing=false (the demo_email_key UC was already
			// bound to the previous index).
			{
				Name:    "idx_solo_unique",
				Table:   "demo",
				Columns: []string{"email"},
				Unique:  true,
			},
			// Plain secondary index — kept, backing=false.
			{
				Name:    "idx_demo_nickname",
				Table:   "demo",
				Columns: []string{"nickname"},
			},
		},
	}

	got := indexEntriesByColumn(tbl)

	require.Empty(t, got["id"], "PK-backing index must not surface in INDEXES (PK column conveys it)")

	require.Equal(t, []indexEntry{
		{name: "demo_email_key", backing: true},
		{name: "idx_solo_unique", backing: false},
	}, got["email"], "UC-backing entry tagged; standalone unique index untagged")

	require.Equal(t, []indexEntry{
		{name: "sqlite_autoindex_demo_1", backing: true},
	}, got["first_name"],
		"auto-named backing index on (first_name,last_name) tagged via column-set match")
	require.Equal(t, []indexEntry{
		{name: "sqlite_autoindex_demo_1", backing: true},
	}, got["last_name"], "same as first_name — composite UC member")

	require.Equal(t, []indexEntry{
		{name: "idx_demo_nickname", backing: false},
	}, got["nickname"], "plain non-unique secondary index always present and untagged")
}

// TestUniqueNamesByColumn pins the UNIQUE CONSTRAINTS column's
// per-column rendering: each composite UC's name shows under every
// member column.
func TestUniqueNamesByColumn(t *testing.T) {
	tbl := &metadata.Table{
		Columns: []*metadata.Column{{Name: "first_name"}, {Name: "last_name"}, {Name: "email"}},
		UniqueConstraints: []*metadata.UniqueConstraint{
			{Name: "demo_email_key", Columns: []string{"email"}},
			{Name: "uniq_full_name", Columns: []string{"first_name", "last_name"}},
			nil, // tolerated; skipped.
		},
	}
	got := uniqueNamesByColumn(tbl)
	require.Equal(t, []string{"uniq_full_name"}, got["first_name"])
	require.Equal(t, []string{"uniq_full_name"}, got["last_name"])
	require.Equal(t, []string{"demo_email_key"}, got["email"])
}

// TestOutgoingFKByColumn pins the per-column FK lookup. For composite
// FKs, every member column maps to the same FK pointer.
func TestOutgoingFKByColumn(t *testing.T) {
	composite := &metadata.ForeignKey{
		Name:       "fk_demo_composite",
		Table:      "demo",
		Columns:    []string{"a", "b"},
		RefTable:   "parent",
		RefColumns: []string{"x", "y"},
	}
	single := &metadata.ForeignKey{
		Name:       "fk_demo_simple",
		Table:      "demo",
		Columns:    []string{"c"},
		RefTable:   "other",
		RefColumns: []string{"id"},
	}

	tbl := &metadata.Table{
		Columns: []*metadata.Column{{Name: "a"}, {Name: "b"}, {Name: "c"}},
		FK:      metadata.NewFKGroup([]*metadata.ForeignKey{composite, single}, nil),
	}
	got := outgoingFKByColumn(tbl)
	require.Same(t, composite, got["a"])
	require.Same(t, composite, got["b"],
		"composite FK columns must share the same *ForeignKey pointer")
	require.Same(t, single, got["c"])
}

// TestIndexEntriesByColumn_NoUCs verifies that when the table has no
// UNIQUE constraints, every non-PK index is kept and tagged
// backing=false. The PK-backing index is still filtered.
func TestIndexEntriesByColumn_NoUCs(t *testing.T) {
	tbl := &metadata.Table{
		Columns: []*metadata.Column{{Name: "id"}, {Name: "email"}},
		Indexes: []*metadata.Index{
			{Name: "demo_pkey", Columns: []string{"id"}, Unique: true, Primary: true},
			{Name: "idx_email", Columns: []string{"email"}, Unique: true},
		},
	}
	got := indexEntriesByColumn(tbl)
	require.Empty(t, got["id"], "PK still filtered without UCs")
	require.Equal(t, []indexEntry{
		{name: "idx_email", backing: false},
	}, got["email"], "unique index with no matching UC is kept untagged")
}

// TestFormatFKRef_CrossCatalog pins the qualification format for
// cross-catalog and cross-schema FK references.
func TestFormatFKRef_CrossCatalog(t *testing.T) {
	testCases := []struct {
		name string
		fk   *metadata.ForeignKey
		want string
	}{
		{name: "nil", fk: nil, want: ""},
		{
			name: "same_source",
			fk:   &metadata.ForeignKey{RefTable: "language", RefColumns: []string{"language_id"}},
			want: "language(language_id)",
		},
		{
			name: "cross_schema",
			fk: &metadata.ForeignKey{
				RefSchema:  "other",
				RefTable:   "language",
				RefColumns: []string{"language_id"},
			},
			want: "other.language(language_id)",
		},
		{
			name: "cross_catalog",
			fk: &metadata.ForeignKey{
				RefCatalog: "remotedb",
				RefSchema:  "other",
				RefTable:   "language",
				RefColumns: []string{"language_id"},
			},
			want: "remotedb.other.language(language_id)",
		},
		{
			name: "composite",
			fk: &metadata.ForeignKey{
				RefTable:   "parent",
				RefColumns: []string{"a", "b"},
			},
			want: "parent(a, b)",
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, formatFKRef(tc.fk))
		})
	}
}
