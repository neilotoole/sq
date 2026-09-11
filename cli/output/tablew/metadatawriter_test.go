package tablew

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/cli/output"
	"github.com/neilotoole/sq/libsq/source/drivertype"
	"github.com/neilotoole/sq/libsq/source/metadata"
)

// TestSourceMetadata_nilSize verifies that a source whose driver doesn't
// report a size renders "-" in the SIZE column rather than "0.0B" (gh744).
func TestSourceMetadata_nilSize(t *testing.T) {
	src := &metadata.Source{
		Handle: "@test", Driver: drivertype.SQLite, Name: "testdb",
		FQName: "testdb", Location: "sqlite3:///tmp/testdb.db",
		// Size left nil to simulate a driver that doesn't report it.
	}

	buf := &bytes.Buffer{}
	pr := output.NewPrinting()
	pr.EnableColor(false)
	w := NewMetadataWriter(buf, pr)
	require.NoError(t, w.SourceMetadata(src, true))

	got := buf.String()
	require.NotContains(t, got, "0.0B")

	// With showSchema=true the source row is rendered by doSourceMetaFull
	// with 8 columns: SOURCE, DRIVER, NAME, FQ NAME, SIZE, TABLES, VIEWS,
	// LOCATION. SIZE is the 5th column (index 4).
	var dataRow string
	for line := range strings.SplitSeq(got, "\n") {
		if strings.Contains(line, "@test") {
			dataRow = line
			break
		}
	}
	require.NotEmpty(t, dataRow, "data row not found in tablew output")
	fields := strings.Fields(dataRow)
	require.Len(t, fields, 8, "data row should have 8 columns")
	require.Equal(t, "-", fields[4], "SIZE column should render as dash for nil size")
}

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

// TestIndexEntriesByColumn_SkipsExpressionSentinel verifies that an
// empty-string sentinel key (a functional/expression index position)
// is not attached to any column and creates no dead "" bucket.
func TestIndexEntriesByColumn_SkipsExpressionSentinel(t *testing.T) {
	tbl := &metadata.Table{
		Name: "t",
		Columns: []*metadata.Column{
			{Name: "a"}, {Name: "c"},
		},
		Indexes: []*metadata.Index{
			{Name: "ix_expr", Table: "t", Columns: []string{"a", "", "c"}},
		},
	}

	got := indexEntriesByColumn(tbl)

	require.Len(t, got["a"], 1)
	require.Equal(t, "ix_expr", got["a"][0].name)
	require.Len(t, got["c"], 1)
	require.Empty(t, got[""], "the sentinel key must not create a \"\" bucket")
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

// TestPrintTablesVerbose_AutoPopulationMarkers verifies that the AUTO column
// in printTablesVerbose renders the correct compact markers for identity,
// auto_increment, and generated columns, and that plain columns render no marker.
// It also confirms the output contains no triggers or check-constraint section
// (tiered-output decision: those are reserved for Markdown/HTML writers).
func TestPrintTablesVerbose_AutoPopulationMarkers(t *testing.T) {
	tbls := []*metadata.Table{
		{
			Name:      "accounts",
			TableType: "table",
			RowCount:  10,
			Columns: []*metadata.Column{
				{Name: "id", BaseType: "BIGINT", Identity: true},
				{Name: "code", BaseType: "TEXT", AutoIncrement: true},
				{Name: "hash", BaseType: "TEXT", Generated: true},
				{Name: "name", BaseType: "TEXT"},
			},
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

	// Helper to find the row for a given column name.
	findColRow := func(colName string) string {
		for _, line := range lines {
			if strings.Contains(line, colName) {
				return line
			}
		}
		return ""
	}

	idRow := findColRow("id")
	require.NotEmpty(t, idRow, "id column row not found in:\n%s", out)
	require.Contains(t, idRow, "identity", "identity column must render 'identity' marker")

	codeRow := findColRow("code")
	require.NotEmpty(t, codeRow, "code column row not found in:\n%s", out)
	require.Contains(t, codeRow, "auto_inc", "auto_increment column must render 'auto_inc' marker")

	hashRow := findColRow("hash")
	require.NotEmpty(t, hashRow, "hash column row not found in:\n%s", out)
	require.Contains(t, hashRow, "generated", "generated column must render 'generated' marker")

	nameRow := findColRow("name")
	require.NotEmpty(t, nameRow, "name column row not found in:\n%s", out)
	require.NotContains(t, nameRow, "identity", "plain column must not render identity marker")
	require.NotContains(t, nameRow, "auto_inc", "plain column must not render auto_inc marker")
	require.NotContains(t, nameRow, "generated", "plain column must not render generated marker")

	// Tiered output decision: text table must not contain triggers or
	// check-constraint sections (those are reserved for Markdown/HTML writers).
	require.NotContains(t, out, "TRIGGER", "text table must not contain trigger section")
	require.NotContains(t, out, "CHECK", "text table must not contain check-constraint section")
}

// TestPrintTablesVerbose_GoldenLayout is the end-to-end golden test
// for [mdWriter.printTablesVerbose]; helper-function coverage lives
// alongside [indexEntriesByColumn] / [formatIdxCell]. It pins:
//
//   - the header row order (NAME / TYPE / ROWS / COLS / NAME / TYPE /
//     PK / AUTO / FK / INDEXES / UNIQUE CONSTRAINTS) so a header reorder or
//     rename in the writer surfaces as a test failure rather than as
//     a silent CLI UX regression.
//   - the UC-backing index rendering (parens-wrap, surviving even
//     after the Subdued style is stripped by EnableColor(false)) so a
//     regression that drops the parens — leaving the index name
//     indistinguishable from a standalone unique index — is caught.
//   - the standalone-unique-index path (no parens) so the two cases
//     stay visibly distinct.
func TestPrintTablesVerbose_GoldenLayout(t *testing.T) {
	tbls := []*metadata.Table{
		{
			Name:      "demo",
			TableType: "table",
			RowCount:  42,
			Columns: []*metadata.Column{
				{Name: "id", BaseType: "INTEGER", PrimaryKey: true},
				{Name: "email", BaseType: "TEXT"},
				{Name: "nickname", BaseType: "TEXT"},
			},
			UniqueConstraints: []*metadata.UniqueConstraint{
				{Name: "demo_email_key", Table: "demo", Columns: []string{"email"}},
			},
			Indexes: []*metadata.Index{
				// PK-backing — filtered upstream by indexEntriesByColumn.
				{Name: "demo_pkey", Table: "demo", Columns: []string{"id"}, Unique: true, Primary: true},
				// UC-backing — must render parenthesized.
				{Name: "demo_email_key", Table: "demo", Columns: []string{"email"}, Unique: true},
				// Standalone unique — must render WITHOUT parens.
				{Name: "idx_email_solo", Table: "demo", Columns: []string{"email"}, Unique: true},
				// Plain secondary — bare name.
				{Name: "idx_demo_nick", Table: "demo", Columns: []string{"nickname"}},
			},
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
	require.GreaterOrEqual(t, len(lines), 4, "expected header + 3 column rows: got %q", out)

	// lines[0] is the header row with this writer. The columns are
	// whitespace-padded in declaration order, so an in-order substring
	// search is enough to pin the layout.
	header := lines[0]
	wantHeaders := []string{
		"NAME", "TYPE", "ROWS", "COLS",
		"NAME", "TYPE", "PK", "AUTO", "FK",
		"INDEXES", "UNIQUE CONSTRAINTS",
	}
	pos := 0
	for _, h := range wantHeaders {
		idx := strings.Index(header[pos:], h)
		require.NotEqual(t, -1, idx,
			"header %q not found in expected position in row %q", h, header)
		pos += idx + len(h)
	}

	// Locate the per-column rows. The "id" row is the table's leading
	// row (starts with "demo"); the "email" and "nickname" rows are
	// continuation rows (whitespace-padded NAME/TYPE/ROWS/COLS cells).
	var emailRow string
	for _, line := range lines {
		// A continuation row mentioning "email" must NOT also start
		// with the table name (that's the leading "demo " row whose
		// per-column NAME cell holds "id").
		if !strings.HasPrefix(line, "demo") && strings.Contains(line, "email") {
			emailRow = line
			break
		}
	}
	require.NotEmpty(t, emailRow, "email column row not found in:\n%s", out)

	// The email column participates in:
	//   - demo_email_key (UC-backing → parenthesized)
	//   - idx_email_solo (standalone unique → bare)
	// Both must appear; the UC-backing one must be parens-wrapped, the
	// solo one must NOT be wrapped. Count-based assertions catch a
	// regression that double-wraps the name (e.g. "((demo_email_key))")
	// — a substring-only Contains check would silently pass on that.
	require.Equal(t, 1, strings.Count(emailRow, "(demo_email_key)"),
		"UC-backing index name must render exactly once parens-wrapped to mark it as secondary to UNIQUE CONSTRAINTS")
	require.Contains(t, emailRow, "idx_email_solo",
		"standalone unique index must appear in INDEXES")
	require.NotContains(t, emailRow, "(idx_email_solo)",
		"standalone unique index must NOT be parens-wrapped — only UC-backing entries get that styling")
	// The UC name must appear at least twice: once parens-wrapped in
	// the INDEXES cell, and once bare in the UNIQUE CONSTRAINTS cell.
	require.GreaterOrEqual(t, strings.Count(emailRow, "demo_email_key"), 2,
		"the UC name must appear in both the INDEXES (wrapped) and UNIQUE CONSTRAINTS (bare) cells")
}
