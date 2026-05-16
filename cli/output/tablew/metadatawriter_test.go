package tablew

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/cli/output"
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
// FKs, every member column maps to the same FK pointer. Columns that
// participate in multiple outgoing FK constraints retain every entry
// in declaration order.
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
	// Second outgoing FK that also constrains column "c" — exercises
	// the multi-FK-per-column case the renderer must preserve.
	overlap := &metadata.ForeignKey{
		Name:       "fk_demo_overlap",
		Table:      "demo",
		Columns:    []string{"c"},
		RefTable:   "alt",
		RefColumns: []string{"id"},
	}

	tbl := &metadata.Table{
		Columns: []*metadata.Column{{Name: "a"}, {Name: "b"}, {Name: "c"}},
		FK:      metadata.NewFKGroup([]*metadata.ForeignKey{composite, single, overlap}, nil),
	}
	got := outgoingFKByColumn(tbl)
	require.Equal(t, []*metadata.ForeignKey{composite}, got["a"])
	require.Equal(t, []*metadata.ForeignKey{composite}, got["b"],
		"composite FK columns must share the same *ForeignKey pointer")
	require.Same(t, composite, got["a"][0])
	require.Same(t, composite, got["b"][0])
	require.Equal(t, []*metadata.ForeignKey{single, overlap}, got["c"],
		"a column constrained by multiple FKs retains every entry in declaration order")
}

// TestFormatFKRefs covers joining of multi-FK column entries.
func TestFormatFKRefs(t *testing.T) {
	a := &metadata.ForeignKey{RefTable: "parent", RefColumns: []string{"x"}}
	b := &metadata.ForeignKey{RefTable: "alt", RefColumns: []string{"id"}}

	require.Empty(t, formatFKRefs(nil))
	require.Empty(t, formatFKRefs([]*metadata.ForeignKey{}))
	require.Equal(t, "parent(x)", formatFKRefs([]*metadata.ForeignKey{a}))
	require.Equal(t, "parent(x), alt(id)",
		formatFKRefs([]*metadata.ForeignKey{a, b}))
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

// TestPrintTablesVerbose_COLSPlainNumeric exercises the full
// printTablesVerbose path to lock the round-3 fix that emits the COLS
// cell as a plain numeric string. The column-3 transformer is
// pr.Number; pre-styling the cell would double-wrap ANSI codes. Tests
// run with color disabled so the assertion is on the raw character
// content of the cell (no escape-code interleaving), which is enough
// to catch a regression that re-introduces Faint.Sprintf wrapping —
// the wrapped form contains additional non-numeric runs even after
// color stripping.
func TestPrintTablesVerbose_COLSPlainNumeric(t *testing.T) {
	tbls := []*metadata.Table{
		{
			Name:      "actor",
			TableType: "table",
			RowCount:  200,
			Columns: []*metadata.Column{
				{Name: "actor_id", BaseType: "INTEGER", PrimaryKey: true},
				{Name: "first_name", BaseType: "TEXT"},
			},
		},
		{
			// Column-less table: exercises the len(tbl.Columns) == 0
			// short-circuit that also pre-styled "0" before round-3.
			Name:      "empty_view",
			TableType: "view",
			RowCount:  0,
			Columns:   nil,
		},
	}

	var buf bytes.Buffer
	pr := output.NewPrinting()
	pr.EnableColor(false)
	w := NewMetadataWriter(&buf, pr).(*mdWriter)
	require.NoError(t, w.printTablesVerbose(tbls))

	out := buf.String()
	require.NotEmpty(t, out)

	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	// Locate each table's leading row — they're the ones that start
	// with the table name (column-continuation rows start with
	// whitespace because NAME / TYPE / ROWS / COLS are blanked out).
	var actorRow, emptyRow string
	for _, line := range lines {
		switch {
		case strings.HasPrefix(line, "actor "):
			actorRow = line
		case strings.HasPrefix(line, "empty_view"):
			emptyRow = line
		}
	}
	require.NotEmpty(t, actorRow, "actor row not found in output: %q", out)
	require.NotEmpty(t, emptyRow, "empty_view row not found in output: %q", out)

	// With color disabled the COLS cell should be a bare numeric
	// surrounded by table-padding whitespace. A regression that
	// pre-styled the cell would inject ANSI escape codes; even
	// disabled, the wrong code path leaves visible artefacts.
	require.Regexp(t, `\s2\s`, actorRow,
		"actor row COLS cell must render as a bare numeric (no double-style wrapping)")
	require.Regexp(t, `\s0\s`, emptyRow,
		"empty_view row COLS cell must render as a bare 0 (no double-style wrapping)")
	require.NotContains(t, actorRow, "\x1b[",
		"with color disabled the row must contain no ANSI escape sequences")
	require.NotContains(t, emptyRow, "\x1b[")
}
