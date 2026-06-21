package metadata_test

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/libsq/core/kind"
	"github.com/neilotoole/sq/libsq/source/drivertype"
	"github.com/neilotoole/sq/libsq/source/metadata"
)

// newJSONLog returns a *slog.Logger that records each entry into buf
// as a single JSON line, and a parseEntries helper that decodes the
// buffer into one map per emitted entry. The JSON handler is more
// robust for assertions than the text handler because attribute keys
// are preserved structurally — `require.Contains(buf, "ghost")` would
// match the value of any attribute, but parseEntries lets tests pin
// the *attribute name* the value lives under.
func newJSONLog(buf *bytes.Buffer, level slog.Level) *slog.Logger {
	return slog.New(slog.NewJSONHandler(buf, &slog.HandlerOptions{Level: level}))
}

func parseEntries(t *testing.T, buf *bytes.Buffer) []map[string]any {
	t.Helper()
	var entries []map[string]any
	for line := range strings.SplitSeq(strings.TrimRight(buf.String(), "\n"), "\n") {
		if line == "" {
			continue
		}
		var entry map[string]any
		require.NoError(t, json.Unmarshal([]byte(line), &entry), "log line: %s", line)
		entries = append(entries, entry)
	}
	return entries
}

func TestSource_Table(t *testing.T) {
	testCases := []struct {
		name    string
		src     *metadata.Source
		tblName string
		want    *metadata.Table
	}{
		{
			name:    "nil_source",
			src:     nil,
			tblName: "actor",
			want:    nil,
		},
		{
			name: "table_found",
			src: &metadata.Source{
				Tables: []*metadata.Table{
					{Name: "actor"},
					{Name: "film"},
				},
			},
			tblName: "actor",
			want:    &metadata.Table{Name: "actor"},
		},
		{
			name: "table_not_found",
			src: &metadata.Source{
				Tables: []*metadata.Table{
					{Name: "actor"},
					{Name: "film"},
				},
			},
			tblName: "nonexistent",
			want:    nil,
		},
		{
			name:    "empty_tables",
			src:     &metadata.Source{},
			tblName: "actor",
			want:    nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.src.Table(tc.tblName)
			if tc.want == nil {
				require.Nil(t, got)
			} else {
				require.NotNil(t, got)
				require.Equal(t, tc.want.Name, got.Name)
			}
		})
	}
}

func TestSource_Clone(t *testing.T) {
	t.Run("nil_source", func(t *testing.T) {
		var src *metadata.Source
		got := src.Clone()
		require.Nil(t, got)
	})

	t.Run("full_source", func(t *testing.T) {
		var size int64 = 1024
		src := &metadata.Source{
			Handle:          "@sakila",
			Location:        "postgres://localhost/sakila",
			Name:            "sakila",
			FQName:          "sakila.public",
			Schema:          "public",
			Catalog:         "sakila",
			Driver:          drivertype.Pg,
			DBDriver:        drivertype.Pg,
			DBProduct:       "PostgreSQL 14.0",
			DBVersion:       "14.0",
			User:            "postgres",
			Size:            &size,
			TableCount:      10,
			ViewCount:       5,
			SecretsResolved: true,
			DBProperties: map[string]any{
				"max_connections": 100,
				"version":         "14.0",
			},
			Tables: []*metadata.Table{
				{
					Name:    "actor",
					FQName:  "sakila.public.actor",
					Columns: []*metadata.Column{{Name: "actor_id", Kind: kind.Int}},
				},
			},
		}

		got := src.Clone()
		require.NotNil(t, got)
		require.NotSame(t, src, got)
		require.Equal(t, src.Handle, got.Handle)
		require.Equal(t, src.Location, got.Location)
		require.Equal(t, src.Name, got.Name)
		require.Equal(t, src.FQName, got.FQName)
		require.Equal(t, src.Schema, got.Schema)
		require.Equal(t, src.Catalog, got.Catalog)
		require.Equal(t, src.Driver, got.Driver)
		require.Equal(t, src.DBDriver, got.DBDriver)
		require.Equal(t, src.DBProduct, got.DBProduct)
		require.Equal(t, src.DBVersion, got.DBVersion)
		require.Equal(t, src.User, got.User)
		require.Equal(t, src.Size, got.Size)
		require.Equal(t, src.TableCount, got.TableCount)
		require.Equal(t, src.ViewCount, got.ViewCount)
		require.Equal(t, src.SecretsResolved, got.SecretsResolved)

		// Verify DBProperties is a deep copy
		require.Equal(t, src.DBProperties, got.DBProperties)
		// Modify clone's DBProperties to verify independence
		got.DBProperties["new_key"] = "new_value"
		require.NotEqual(t, src.DBProperties, got.DBProperties)

		// Verify Tables is a separate slice
		require.Len(t, got.Tables, 1)
		require.NotSame(t, src.Tables[0], got.Tables[0])
		require.Equal(t, src.Tables[0].Name, got.Tables[0].Name)
	})

	t.Run("nil_properties_and_tables", func(t *testing.T) {
		src := &metadata.Source{
			Handle: "@test",
			Name:   "test",
		}

		got := src.Clone()
		require.NotNil(t, got)
		require.Nil(t, got.DBProperties)
		require.Nil(t, got.Tables)
		require.Nil(t, got.Size, "nil-Size source must clone with nil Size")
	})
}

func TestSource_TableNames(t *testing.T) {
	testCases := []struct {
		name string
		src  *metadata.Source
		want []string
	}{
		{
			name: "nil_source",
			src:  nil,
			want: nil,
		},
		{
			name: "multiple_tables",
			src: &metadata.Source{
				Tables: []*metadata.Table{
					{Name: "actor"},
					{Name: "film"},
					{Name: "category"},
				},
			},
			want: []string{"actor", "film", "category"},
		},
		{
			name: "empty_tables",
			src:  &metadata.Source{},
			want: []string{},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.src.TableNames()
			require.Equal(t, tc.want, got)
		})
	}
}

func TestSource_String(t *testing.T) {
	src := &metadata.Source{
		Handle: "@sakila",
		Name:   "sakila",
		Driver: drivertype.Pg,
	}

	got := src.String()
	require.NotEmpty(t, got)
	require.Contains(t, got, "@sakila")
	require.Contains(t, got, "sakila")
}

func TestTable_Column(t *testing.T) {
	testCases := []struct {
		name    string
		tbl     *metadata.Table
		colName string
		want    *metadata.Column
	}{
		{
			name:    "nil_table",
			tbl:     nil,
			colName: "actor_id",
			want:    nil,
		},
		{
			name: "column_found",
			tbl: &metadata.Table{
				Columns: []*metadata.Column{
					{Name: "actor_id"},
					{Name: "first_name"},
				},
			},
			colName: "actor_id",
			want:    &metadata.Column{Name: "actor_id"},
		},
		{
			name: "column_not_found",
			tbl: &metadata.Table{
				Columns: []*metadata.Column{
					{Name: "actor_id"},
				},
			},
			colName: "nonexistent",
			want:    nil,
		},
		{
			name:    "empty_columns",
			tbl:     &metadata.Table{},
			colName: "actor_id",
			want:    nil,
		},
		{
			name: "nil_column_entry_skipped",
			tbl: &metadata.Table{
				Columns: []*metadata.Column{
					nil,
					{Name: "actor_id"},
				},
			},
			colName: "actor_id",
			want:    &metadata.Column{Name: "actor_id"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.tbl.Column(tc.colName)
			if tc.want == nil {
				require.Nil(t, got)
			} else {
				require.NotNil(t, got)
				require.Equal(t, tc.want.Name, got.Name)
			}
		})
	}
}

func TestTable_PKCols(t *testing.T) {
	testCases := []struct {
		name string
		tbl  *metadata.Table
		want []string
	}{
		{
			name: "nil_table",
			tbl:  nil,
			want: nil,
		},
		{
			name: "single_pk",
			tbl: &metadata.Table{
				Columns: []*metadata.Column{
					{Name: "actor_id", PrimaryKey: true},
					{Name: "first_name", PrimaryKey: false},
				},
			},
			want: []string{"actor_id"},
		},
		{
			name: "composite_pk",
			tbl: &metadata.Table{
				Columns: []*metadata.Column{
					{Name: "film_id", PrimaryKey: true},
					{Name: "actor_id", PrimaryKey: true},
					{Name: "last_update", PrimaryKey: false},
				},
			},
			want: []string{"film_id", "actor_id"},
		},
		{
			name: "no_pk",
			tbl: &metadata.Table{
				Columns: []*metadata.Column{
					{Name: "col1", PrimaryKey: false},
					{Name: "col2", PrimaryKey: false},
				},
			},
			want: nil,
		},
		{
			name: "empty_columns",
			tbl:  &metadata.Table{},
			want: nil,
		},
		{
			name: "nil_column_entry_skipped",
			tbl: &metadata.Table{
				Columns: []*metadata.Column{
					nil,
					{Name: "actor_id", PrimaryKey: true},
				},
			},
			want: []string{"actor_id"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.tbl.PKCols()
			if tc.want == nil {
				require.Nil(t, got)
			} else {
				require.Len(t, got, len(tc.want))
				for i, col := range got {
					require.Equal(t, tc.want[i], col.Name)
				}
			}
		})
	}
}

func TestTable_Clone(t *testing.T) {
	t.Run("nil_table", func(t *testing.T) {
		var tbl *metadata.Table
		got := tbl.Clone()
		require.Nil(t, got)
	})

	t.Run("full_table", func(t *testing.T) {
		size := int64(1024)
		tbl := &metadata.Table{
			Name:        "actor",
			FQName:      "sakila.public.actor",
			TableType:   "table",
			DBTableType: "BASE TABLE",
			RowCount:    200,
			Size:        &size,
			Comment:     "Actor table",
			Columns: []*metadata.Column{
				{Name: "actor_id", Kind: kind.Int, PrimaryKey: true},
				{Name: "first_name", Kind: kind.Text},
			},
		}

		got := tbl.Clone()
		require.NotNil(t, got)
		require.NotSame(t, tbl, got)
		require.Equal(t, tbl.Name, got.Name)
		require.Equal(t, tbl.FQName, got.FQName)
		require.Equal(t, tbl.TableType, got.TableType)
		require.Equal(t, tbl.DBTableType, got.DBTableType)
		require.Equal(t, tbl.RowCount, got.RowCount)
		require.Equal(t, tbl.Size, got.Size)
		require.Equal(t, tbl.Comment, got.Comment)

		// Verify Columns is a separate slice
		require.Len(t, got.Columns, 2)
		require.NotSame(t, tbl.Columns[0], got.Columns[0])
		require.Equal(t, tbl.Columns[0].Name, got.Columns[0].Name)
	})

	t.Run("nil_columns", func(t *testing.T) {
		tbl := &metadata.Table{Name: "test"}
		got := tbl.Clone()
		require.NotNil(t, got)
		require.Nil(t, got.Columns)
	})

	t.Run("fk_uc_indexes_deep_copied", func(t *testing.T) {
		fk := &metadata.ForeignKey{
			Name:       "fk_film_language",
			Table:      "film",
			Columns:    []string{"language_id"},
			RefTable:   "language",
			RefColumns: []string{"language_id"},
		}
		uc := &metadata.UniqueConstraint{
			Name:    "uq_film_title",
			Table:   "film",
			Columns: []string{"title"},
		}
		idx := &metadata.Index{
			Name:    "idx_film_title",
			Table:   "film",
			Columns: []string{"title"},
		}
		tbl := &metadata.Table{
			Name:              "film",
			FK:                metadata.NewFKGroup([]*metadata.ForeignKey{fk}, nil),
			UniqueConstraints: []*metadata.UniqueConstraint{uc},
			Indexes:           []*metadata.Index{idx},
		}

		got := tbl.Clone()
		require.NotNil(t, got)

		require.NotNil(t, got.FK)
		require.Len(t, got.FK.Outgoing, 1)
		require.NotSame(t, fk, got.FK.Outgoing[0], "FK must be deep-copied")
		require.Equal(t, fk, got.FK.Outgoing[0])

		require.Len(t, got.UniqueConstraints, 1)
		require.NotSame(t, uc, got.UniqueConstraints[0], "unique constraint must be deep-copied")
		require.Equal(t, uc, got.UniqueConstraints[0])

		require.Len(t, got.Indexes, 1)
		require.NotSame(t, idx, got.Indexes[0], "index must be deep-copied")
		require.Equal(t, idx, got.Indexes[0])
	})
}

func TestTable_String(t *testing.T) {
	tbl := &metadata.Table{
		Name:     "actor",
		RowCount: 200,
	}

	got := tbl.String()
	require.NotEmpty(t, got)
	require.Contains(t, got, "actor")
}

func TestColumn_Clone(t *testing.T) {
	t.Run("nil_column", func(t *testing.T) {
		var col *metadata.Column
		got := col.Clone()
		require.Nil(t, got)
	})

	t.Run("full_column", func(t *testing.T) {
		col := &metadata.Column{
			Name:         "actor_id",
			Position:     1,
			PrimaryKey:   true,
			BaseType:     "integer",
			ColumnType:   "int4",
			Kind:         kind.Int,
			Nullable:     false,
			DefaultValue: "nextval('actor_actor_id_seq')",
			Comment:      "Primary key",
		}

		got := col.Clone()
		require.NotNil(t, got)
		require.NotSame(t, col, got)
		require.Equal(t, col.Name, got.Name)
		require.Equal(t, col.Position, got.Position)
		require.Equal(t, col.PrimaryKey, got.PrimaryKey)
		require.Equal(t, col.BaseType, got.BaseType)
		require.Equal(t, col.ColumnType, got.ColumnType)
		require.Equal(t, col.Kind, got.Kind)
		require.Equal(t, col.Nullable, got.Nullable)
		require.Equal(t, col.DefaultValue, got.DefaultValue)
		require.Equal(t, col.Comment, got.Comment)
	})
}

func TestColumn_String(t *testing.T) {
	col := &metadata.Column{
		Name: "actor_id",
		Kind: kind.Int,
	}

	got := col.String()
	require.NotEmpty(t, got)
	require.Contains(t, got, "actor_id")
}

func TestForeignKey_Clone(t *testing.T) {
	t.Run("nil_fk", func(t *testing.T) {
		var fk *metadata.ForeignKey
		require.Nil(t, fk.Clone())
	})

	t.Run("composite_fk", func(t *testing.T) {
		fk := &metadata.ForeignKey{
			Name:       "fk_film_actor",
			Table:      "film_actor",
			Columns:    []string{"film_id", "actor_id"},
			RefSchema:  "public",
			RefTable:   "film_actor_lookup",
			RefColumns: []string{"film_id", "actor_id"},
			OnDelete:   "CASCADE",
			OnUpdate:   "NO ACTION",
		}

		got := fk.Clone()
		require.NotSame(t, fk, got)
		require.Equal(t, fk, got)

		// Mutating clone slices must not affect the original.
		got.Columns[0] = "mutated"
		require.NotEqual(t, fk.Columns, got.Columns)
	})
}

func TestLinkForeignKeys(t *testing.T) {
	t.Run("nil_source", func(_ *testing.T) {
		metadata.LinkForeignKeys(nil, nil)
	})

	t.Run("empty_tables", func(t *testing.T) {
		src := &metadata.Source{}
		metadata.LinkForeignKeys(nil, src)
		require.Nil(t, src.Tables)
	})

	t.Run("nil_table_entries_skipped", func(t *testing.T) {
		// A source whose Tables slice contains nil entries (defensive:
		// shouldn't happen in practice, but must not panic) must still
		// link the valid FKs and leave the nil holes untouched.
		fk := &metadata.ForeignKey{
			Name:       "fk_film_language",
			Table:      "film",
			Columns:    []string{"language_id"},
			RefTable:   "language",
			RefColumns: []string{"language_id"},
		}
		src := &metadata.Source{
			Tables: []*metadata.Table{
				nil,
				{
					Name: "film",
					FK:   metadata.NewFKGroup([]*metadata.ForeignKey{fk}, nil),
				},
				nil,
				{Name: "language"},
				nil,
			},
		}

		require.NotPanics(t, func() { metadata.LinkForeignKeys(nil, src) })

		require.Len(t, src.Table("language").FK.Incoming, 1)
		require.Same(t, fk, src.Table("language").FK.Incoming[0])
	})

	t.Run("nil_fk_in_outgoing_skipped", func(t *testing.T) {
		// A nil entry in FK.Outgoing must be skipped without panicking,
		// and a sibling valid FK in the same slice must still link.
		good := &metadata.ForeignKey{
			Name:       "fk_film_language",
			Table:      "film",
			Columns:    []string{"language_id"},
			RefTable:   "language",
			RefColumns: []string{"language_id"},
		}
		src := &metadata.Source{
			Tables: []*metadata.Table{
				{
					Name: "film",
					FK:   &metadata.FKGroup{Outgoing: []*metadata.ForeignKey{nil, good, nil}},
				},
				{Name: "language"},
			},
		}

		require.NotPanics(t, func() { metadata.LinkForeignKeys(nil, src) })
		require.Len(t, src.Table("language").FK.Incoming, 1)
		require.Same(t, good, src.Table("language").FK.Incoming[0])
	})

	t.Run("empty_fk_wrapper_dropped", func(t *testing.T) {
		// A table that arrives with a non-nil but empty FKGroup (both
		// Outgoing and Incoming empty) must have its FK set back to nil
		// so JSON / YAML omit the empty `fk: {}` wrapper.
		src := &metadata.Source{
			Tables: []*metadata.Table{
				{Name: "standalone", FK: &metadata.FKGroup{}},
			},
		}

		metadata.LinkForeignKeys(nil, src)
		require.Nil(t, src.Table("standalone").FK,
			"an empty FKGroup must be dropped to nil after linking")
	})

	t.Run("simple_fk", func(t *testing.T) {
		fk := &metadata.ForeignKey{
			Name:       "fk_film_language",
			Table:      "film",
			Columns:    []string{"language_id"},
			RefTable:   "language",
			RefColumns: []string{"language_id"},
		}

		src := &metadata.Source{
			Tables: []*metadata.Table{
				{
					Name: "film",
					Columns: []*metadata.Column{
						{Name: "film_id"},
						{Name: "language_id"},
					},
					FK: metadata.NewFKGroup([]*metadata.ForeignKey{fk}, nil),
				},
				{
					Name: "language",
					Columns: []*metadata.Column{
						{Name: "language_id", PrimaryKey: true},
					},
				},
			},
		}

		metadata.LinkForeignKeys(nil, src)

		film := src.Table("film")
		require.NotNil(t, film)
		require.Len(t, film.FK.Outgoing, 1)
		require.Same(t, fk, film.FK.Outgoing[0])

		language := src.Table("language")
		require.Len(t, language.FK.Incoming, 1)
		require.Same(t, fk, language.FK.Incoming[0],
			"language.FK.Incoming entry must share identity with the FK on film.FK.Outgoing")
	})

	t.Run("composite_fk_incoming_shares_pointer", func(t *testing.T) {
		fk := &metadata.ForeignKey{
			Name:       "fk_film_actor_lookup",
			Table:      "film_actor",
			Columns:    []string{"film_id", "actor_id"},
			RefTable:   "film_actor_lookup",
			RefColumns: []string{"film_id", "actor_id"},
		}

		src := &metadata.Source{
			Tables: []*metadata.Table{
				{
					Name: "film_actor",
					Columns: []*metadata.Column{
						{Name: "film_id"},
						{Name: "actor_id"},
					},
					FK: metadata.NewFKGroup([]*metadata.ForeignKey{fk}, nil),
				},
				{
					Name: "film_actor_lookup",
					Columns: []*metadata.Column{
						{Name: "film_id"},
						{Name: "actor_id"},
					},
				},
			},
		}

		metadata.LinkForeignKeys(nil, src)

		lookup := src.Table("film_actor_lookup")
		require.Len(t, lookup.FK.Incoming, 1,
			"composite FK should appear once on the referenced table, not once per column")
		require.Same(t, fk, lookup.FK.Incoming[0])
	})

	t.Run("cross_schema_does_not_link_local_namesake", func(t *testing.T) {
		// A FK that points at other_schema.users must NOT be linked to
		// a local "users" table just because the name matches — the
		// reference is in a different schema.
		fk := &metadata.ForeignKey{
			Name:       "fk_to_other_schema_users",
			Table:      "audit",
			Columns:    []string{"user_id"},
			RefSchema:  "other_schema",
			RefTable:   "users",
			RefColumns: []string{"id"},
		}

		src := &metadata.Source{
			Schema:  "public",
			Catalog: "app",
			Tables: []*metadata.Table{
				{
					Name:    "audit",
					Columns: []*metadata.Column{{Name: "user_id"}},
					FK:      metadata.NewFKGroup([]*metadata.ForeignKey{fk}, nil),
				},
				{
					Name:    "users", // local namesake — must not be matched
					Columns: []*metadata.Column{{Name: "id"}},
				},
			},
		}

		metadata.LinkForeignKeys(nil, src)

		// Local "users" must not appear as an incoming-FK target.
		// LinkForeignKeys drops the empty FK wrapper entirely, so FK
		// is nil here rather than being a non-nil group with empty
		// slices.
		require.Nil(t, src.Table("users").FK,
			"local users.FK must be nil when no FKs point at it")
		// The outgoing FK is still on the audit table's FK.Outgoing slice.
		require.Len(t, src.Table("audit").FK.Outgoing, 1)
	})

	t.Run("same_schema_qualifiers_normalized", func(t *testing.T) {
		// Drivers report RefSchema / RefCatalog populated even for
		// same-schema refs (information_schema returns non-empty
		// values). LinkForeignKeys should normalize them away so the
		// JSON output omits the redundant qualifiers and the
		// "ref-is-elsewhere" check above stays reliable.
		fk := &metadata.ForeignKey{
			Table:      "child",
			Columns:    []string{"parent_id"},
			RefCatalog: "app",
			RefSchema:  "public",
			RefTable:   "parent",
			RefColumns: []string{"id"},
		}
		src := &metadata.Source{
			Schema:  "public",
			Catalog: "app",
			Tables: []*metadata.Table{
				{
					Name:    "child",
					Columns: []*metadata.Column{{Name: "parent_id"}},
					FK:      metadata.NewFKGroup([]*metadata.ForeignKey{fk}, nil),
				},
				{
					Name:    "parent",
					Columns: []*metadata.Column{{Name: "id"}},
				},
			},
		}

		metadata.LinkForeignKeys(nil, src)

		require.Equal(t, "", fk.RefCatalog, "same-catalog qualifier should be cleared")
		require.Equal(t, "", fk.RefSchema, "same-schema qualifier should be cleared")
		require.Len(t, src.Table("parent").FK.Incoming, 1)
	})

	t.Run("unresolved_ref_is_skipped", func(t *testing.T) {
		fk := &metadata.ForeignKey{
			Name:       "fk_to_external",
			Table:      "local",
			Columns:    []string{"external_id"},
			RefSchema:  "other_schema",
			RefTable:   "external_tbl",
			RefColumns: []string{"id"},
		}

		src := &metadata.Source{
			Tables: []*metadata.Table{
				{
					Name: "local",
					Columns: []*metadata.Column{
						{Name: "external_id"},
					},
					FK: metadata.NewFKGroup([]*metadata.ForeignKey{fk}, nil),
				},
			},
		}

		metadata.LinkForeignKeys(nil, src)

		local := src.Table("local")
		// The outgoing FK is still on FK.Outgoing even though the
		// referenced table is outside this Source.
		require.Len(t, local.FK.Outgoing, 1)
		// No incoming back-ref appears anywhere within the Source.
		require.Nil(t, local.FK.Incoming)
	})

	t.Run("idempotent", func(t *testing.T) {
		fk := &metadata.ForeignKey{
			Table:      "child",
			Columns:    []string{"parent_id"},
			RefTable:   "parent",
			RefColumns: []string{"id"},
		}

		src := &metadata.Source{
			Tables: []*metadata.Table{
				{
					Name:    "child",
					Columns: []*metadata.Column{{Name: "parent_id"}},
					FK:      metadata.NewFKGroup([]*metadata.ForeignKey{fk}, nil),
				},
				{
					Name:    "parent",
					Columns: []*metadata.Column{{Name: "id"}},
				},
			},
		}

		metadata.LinkForeignKeys(nil, src)
		metadata.LinkForeignKeys(nil, src)

		require.Len(t, src.Table("parent").FK.Incoming, 1)
		require.Same(t, fk, src.Table("parent").FK.Incoming[0])
	})

	t.Run("empty_catalog_and_schema", func(t *testing.T) {
		// SQLite (and DuckDB in some configurations) leave Source.Schema
		// empty or treat it semantically as "no schema qualifier". A
		// driver that doesn't populate FK.RefCatalog / FK.RefSchema for
		// same-source references should still get its FKs linked.
		sameSrc := &metadata.ForeignKey{
			Table:      "child",
			Columns:    []string{"parent_id"},
			RefTable:   "parent",
			RefColumns: []string{"id"},
		}
		// And an FK whose RefSchema is non-empty (external reference)
		// must still be skipped — non-empty RefSchema means "outside
		// this Source" regardless of whether Source.Schema is empty.
		external := &metadata.ForeignKey{
			Table:      "child",
			Columns:    []string{"audit_id"},
			RefTable:   "audit_log",
			RefColumns: []string{"id"},
			RefSchema:  "other_schema",
		}

		src := &metadata.Source{
			Catalog: "",
			Schema:  "",
			Tables: []*metadata.Table{
				{
					Name:    "child",
					Columns: []*metadata.Column{{Name: "parent_id"}, {Name: "audit_id"}},
					FK:      metadata.NewFKGroup([]*metadata.ForeignKey{sameSrc, external}, nil),
				},
				{
					Name:    "parent",
					Columns: []*metadata.Column{{Name: "id"}},
				},
				{
					// A locally-named table that shouldn't be linked to
					// because the FK's RefSchema marks it as external.
					Name:    "audit_log",
					Columns: []*metadata.Column{{Name: "id"}},
				},
			},
		}

		metadata.LinkForeignKeys(nil, src)

		parent := src.Table("parent")
		require.NotNil(t, parent.FK)
		require.Equal(t, []*metadata.ForeignKey{sameSrc}, parent.FK.Incoming,
			"same-source FK must link even when Source.Catalog/Schema are empty")

		audit := src.Table("audit_log")
		require.Nil(t, audit.FK,
			"FK with non-empty RefSchema must not link to a same-named local table")
		require.Equal(t, "other_schema", external.RefSchema,
			"RefSchema must remain unchanged when it doesn't match Source.Schema")
	})

	t.Run("missing_ref_table_warns_when_logger_present", func(t *testing.T) {
		// An in-source FK (no RefCatalog / RefSchema) whose RefTable
		// doesn't match any table in s should surface a warn entry so
		// the operator can spot driver-side bugs or transiently dropped
		// tables.
		var buf bytes.Buffer
		log := newJSONLog(&buf, slog.LevelWarn)
		bad := &metadata.ForeignKey{
			Name:       "fk_film_typo",
			Table:      "film",
			Columns:    []string{"language_id"},
			RefTable:   "lanugage", // typo — no such table in the source.
			RefColumns: []string{"language_id"},
		}
		src := &metadata.Source{
			Schema:  "public",
			Catalog: "app",
			Tables: []*metadata.Table{
				{Name: "film", FK: metadata.NewFKGroup([]*metadata.ForeignKey{bad}, nil)},
				{Name: "language"},
			},
		}
		metadata.LinkForeignKeys(log, src)
		entries := parseEntries(t, &buf)
		require.Len(t, entries, 1)
		require.Equal(t, "WARN", entries[0]["level"])
		require.Equal(t, "fk_film_typo", entries[0]["constraint"])
		require.Equal(t, "film", entries[0]["table"])
		require.Equal(t, "lanugage", entries[0]["ref_table"])
	})

	t.Run("mixed_resolved_and_unknown_refs", func(t *testing.T) {
		// A real driver run will mix valid FKs and the occasional
		// unknown-ref edge case in the same call. The unknown one
		// must warn AND the valid ones must still be linked.
		var buf bytes.Buffer
		log := newJSONLog(&buf, slog.LevelWarn)
		good := &metadata.ForeignKey{
			Name: "fk_film_language", Table: "film",
			Columns: []string{"language_id"}, RefTable: "language", RefColumns: []string{"language_id"},
		}
		bad := &metadata.ForeignKey{
			Name: "fk_film_typo", Table: "film",
			Columns: []string{"category_id"}, RefTable: "categroy", RefColumns: []string{"id"},
		}
		src := &metadata.Source{
			Tables: []*metadata.Table{
				{Name: "film", FK: metadata.NewFKGroup([]*metadata.ForeignKey{good, bad}, nil)},
				{Name: "language"},
				{Name: "category"},
			},
		}
		metadata.LinkForeignKeys(log, src)
		entries := parseEntries(t, &buf)
		require.Len(t, entries, 1, "exactly one warn for the unknown ref; the valid FK must not produce noise")
		require.Equal(t, "categroy", entries[0]["ref_table"])

		require.Len(t, src.Table("language").FK.Incoming, 1,
			"the valid FK must still link Incoming despite the sibling typo")
		require.Same(t, good, src.Table("language").FK.Incoming[0])
		require.Nil(t, src.Table("category").FK,
			"category got no incoming link because the typo'd FK didn't reach it")
	})

	t.Run("self_referential_fk_pointer_identity", func(t *testing.T) {
		// A table whose outgoing FK references itself must appear in both
		// FK.Outgoing and FK.Incoming with identical pointer identity —
		// LinkForeignKeys treats the referenced table as just another
		// table in the source, and "the same table" is not a special
		// case. Asserting Same (not just Equal) catches a future
		// regression where LinkForeignKeys clones the FK before
		// inserting into Incoming, breaking back-ref-to-outgoing
		// pointer equality.
		fk := &metadata.ForeignKey{
			Name:       "fk_employee_manager",
			Table:      "employee",
			Columns:    []string{"manager_id"},
			RefTable:   "employee",
			RefColumns: []string{"id"},
		}

		src := &metadata.Source{
			Tables: []*metadata.Table{
				{
					Name: "employee",
					Columns: []*metadata.Column{
						{Name: "id", PrimaryKey: true},
						{Name: "manager_id"},
					},
					FK: metadata.NewFKGroup([]*metadata.ForeignKey{fk}, nil),
				},
			},
		}

		metadata.LinkForeignKeys(nil, src)

		employee := src.Table("employee")
		require.NotNil(t, employee.FK)
		require.Len(t, employee.FK.Outgoing, 1)
		require.Len(t, employee.FK.Incoming, 1,
			"self-referential FK must surface on FK.Incoming on the same table")
		require.Same(t, fk, employee.FK.Outgoing[0])
		require.Same(t, employee.FK.Outgoing[0], employee.FK.Incoming[0],
			"self-ref Outgoing and Incoming must share pointer identity")
	})

	t.Run("idempotence_clears_bogus_pointer", func(t *testing.T) {
		// The existing "idempotent" subcase calls LinkForeignKeys twice
		// and verifies the legitimate Incoming entry isn't duplicated.
		// This subcase goes further: it injects a bogus FK pointer
		// directly into an Incoming slice between the two calls and
		// verifies the second LinkForeignKeys pass purges it. That
		// pins the pre-clear step in [metadata.LinkForeignKeys]
		// (FK.Incoming is set to nil on every FK-bearing table before
		// re-deriving) — a regression that only appended would leave
		// the bogus pointer in place forever.
		fk := &metadata.ForeignKey{
			Table:      "child",
			Columns:    []string{"parent_id"},
			RefTable:   "parent",
			RefColumns: []string{"id"},
		}
		bogus := &metadata.ForeignKey{
			Name:     "fk_bogus_injected",
			Table:    "ghost",
			RefTable: "parent",
		}

		src := &metadata.Source{
			Tables: []*metadata.Table{
				{
					Name:    "child",
					Columns: []*metadata.Column{{Name: "parent_id"}},
					FK:      metadata.NewFKGroup([]*metadata.ForeignKey{fk}, nil),
				},
				{
					Name:    "parent",
					Columns: []*metadata.Column{{Name: "id"}},
				},
			},
		}

		metadata.LinkForeignKeys(nil, src)
		parent := src.Table("parent")
		require.Len(t, parent.FK.Incoming, 1)

		// Inject a bogus pointer into the Incoming slice as a regression
		// would: e.g. a re-entrant caller appending without clearing.
		parent.FK.Incoming = append(parent.FK.Incoming, bogus)
		require.Len(t, parent.FK.Incoming, 2)

		metadata.LinkForeignKeys(nil, src)
		require.Len(t, parent.FK.Incoming, 1,
			"the second LinkForeignKeys must drop the injected bogus pointer")
		require.Same(t, fk, parent.FK.Incoming[0],
			"the legitimate Incoming entry must survive the clear-and-rederive cycle")
	})
}

func TestSource_Clone_RelinksForeignKeys(t *testing.T) {
	fk := &metadata.ForeignKey{
		Name:       "fk_film_language",
		Table:      "film",
		Columns:    []string{"language_id"},
		RefTable:   "language",
		RefColumns: []string{"language_id"},
	}

	src := &metadata.Source{
		Tables: []*metadata.Table{
			{
				Name: "film",
				Columns: []*metadata.Column{
					{Name: "language_id"},
				},
				FK: metadata.NewFKGroup([]*metadata.ForeignKey{fk}, nil),
			},
			{
				Name:    "language",
				Columns: []*metadata.Column{{Name: "language_id"}},
			},
		},
	}
	metadata.LinkForeignKeys(nil, src)

	got := src.Clone()
	gotFilm := got.Table("film")
	gotLanguage := got.Table("language")

	// The clone's incoming back-references must point at the clone's
	// own ForeignKey objects, not the originals.
	require.Len(t, gotFilm.FK.Outgoing, 1)
	require.NotSame(t, fk, gotFilm.FK.Outgoing[0])
	require.Len(t, gotLanguage.FK.Incoming, 1)
	require.Same(t, gotFilm.FK.Outgoing[0], gotLanguage.FK.Incoming[0],
		"FK.Incoming on the clone must share identity with the clone's own FK.Outgoing entry")

	// And conversely: the original source's incoming back-reference
	// must still point at the original FK pointer, not at any cloned
	// value. Catches pointer-aliasing regressions where Clone mutates
	// the source it cloned from.
	origLanguage := src.Table("language")
	require.Len(t, origLanguage.FK.Incoming, 1)
	require.Same(t, fk, origLanguage.FK.Incoming[0],
		"original Source.FK.Incoming must be unaffected by Clone()")
	require.NotSame(t, gotFilm.FK.Outgoing[0], origLanguage.FK.Incoming[0],
		"clone and original FK.Incoming must not share pointer identity")
}

// TestSource_Clone_FKMutationIsolated pins the end-to-end deep-copy
// guarantee: mutating an FK's Columns slice on the clone must not
// affect the original Source, and the original's FK.Incoming back-ref
// must still point at the original's Outgoing entry (no aliasing
// across the Clone boundary).
func TestSource_Clone_FKMutationIsolated(t *testing.T) {
	fk := &metadata.ForeignKey{
		Name:       "fk_film_language",
		Table:      "film",
		Columns:    []string{"language_id"},
		RefTable:   "language",
		RefColumns: []string{"language_id"},
	}

	src := &metadata.Source{
		Tables: []*metadata.Table{
			{
				Name:    "film",
				Columns: []*metadata.Column{{Name: "language_id"}},
				FK:      metadata.NewFKGroup([]*metadata.ForeignKey{fk}, nil),
			},
			{
				Name:    "language",
				Columns: []*metadata.Column{{Name: "language_id"}},
			},
		},
	}
	metadata.LinkForeignKeys(nil, src)

	got := src.Clone()
	gotFK := got.Table("film").FK.Outgoing[0]

	// Mutate every reference-typed field on the cloned FK and verify
	// the original is untouched. Use index-assignment (in-place
	// mutation) on both Columns and RefColumns — a shallow-copy bug
	// where the cloned slice shares the original's backing array would
	// surface here. Append on its own typically reallocates because
	// slice literals have cap == len, so it doesn't reliably exercise
	// the shared-backing-array hazard; the index-assign does.
	gotFK.Columns[0] = "mutated_col"
	gotFK.RefColumns[0] = "mutated_ref"
	gotFK.OnDelete = "CASCADE"

	require.Equal(t, []string{"language_id"}, fk.Columns,
		"original FK.Columns must be unaffected by in-place mutation on the clone")
	require.Equal(t, []string{"language_id"}, fk.RefColumns,
		"original FK.RefColumns must be unaffected by in-place mutation on the clone")
	require.Empty(t, fk.OnDelete,
		"original FK.OnDelete must be unaffected by clone mutation")

	// The original Source's FK.Incoming back-ref must still point at
	// the original FK, not at the (now-mutated) clone.
	origLanguage := src.Table("language")
	require.Len(t, origLanguage.FK.Incoming, 1)
	require.Same(t, fk, origLanguage.FK.Incoming[0],
		"original Source.FK.Incoming must still point at the original FK after clone mutation")
	require.Equal(t, []string{"language_id"}, origLanguage.FK.Incoming[0].Columns,
		"original Incoming view of FK.Columns must remain unchanged")
}

func TestUniqueConstraint_Clone(t *testing.T) {
	t.Run("nil", func(t *testing.T) {
		var uc *metadata.UniqueConstraint
		require.Nil(t, uc.Clone())
	})

	t.Run("composite", func(t *testing.T) {
		uc := &metadata.UniqueConstraint{
			Name:    "uq_film_actor",
			Table:   "film_actor",
			Columns: []string{"film_id", "actor_id"},
		}
		got := uc.Clone()
		require.NotSame(t, uc, got)
		require.Equal(t, uc, got)

		got.Columns[0] = "mutated"
		require.NotEqual(t, uc.Columns, got.Columns)
	})
}

func TestIndex_Clone(t *testing.T) {
	t.Run("nil", func(t *testing.T) {
		var idx *metadata.Index
		require.Nil(t, idx.Clone())
	})

	t.Run("full", func(t *testing.T) {
		idx := &metadata.Index{
			Name:    "idx_film_language",
			Table:   "film",
			Columns: []string{"language_id"},
			Unique:  false,
			Primary: false,
			Type:    "BTREE",
		}
		got := idx.Clone()
		require.NotSame(t, idx, got)
		require.Equal(t, idx, got)

		got.Columns[0] = "mutated"
		require.NotEqual(t, idx.Columns, got.Columns)
	})
}

func TestSchema_Clone(t *testing.T) {
	t.Run("nil_schema", func(t *testing.T) {
		var s *metadata.Schema
		got := s.Clone()
		require.Nil(t, got)
	})

	t.Run("full_schema", func(t *testing.T) {
		s := &metadata.Schema{
			Name:    "public",
			Catalog: "sakila",
			Owner:   "postgres",
		}

		got := s.Clone()
		require.NotNil(t, got)
		require.NotSame(t, s, got)
		require.Equal(t, s.Name, got.Name)
		require.Equal(t, s.Catalog, got.Catalog)
		require.Equal(t, s.Owner, got.Owner)
	})
}

func TestSchema_LogValue(t *testing.T) {
	t.Run("nil_schema", func(_ *testing.T) {
		var s *metadata.Schema
		// Just verify it doesn't panic.
		_ = s.LogValue()
	})

	t.Run("with_owner", func(t *testing.T) {
		s := &metadata.Schema{
			Name:    "public",
			Catalog: "sakila",
			Owner:   "postgres",
		}

		got := s.LogValue()
		require.NotEmpty(t, got.String())
	})

	t.Run("without_owner", func(t *testing.T) {
		s := &metadata.Schema{
			Name:    "public",
			Catalog: "sakila",
		}

		got := s.LogValue()
		require.NotEmpty(t, got.String())
	})
}

func TestAssignForeignKeys(t *testing.T) {
	t.Run("empty_fks_noop", func(t *testing.T) {
		tables := []*metadata.Table{{Name: "actor"}}
		metadata.AssignForeignKeys(nil, tables, nil)
		require.Nil(t, tables[0].FK)
	})

	t.Run("basic_assignment", func(t *testing.T) {
		fkLang := &metadata.ForeignKey{Table: "film", RefTable: "language"}
		tables := []*metadata.Table{
			{Name: "actor"},
			{Name: "film"},
			{Name: "language"},
		}
		metadata.AssignForeignKeys(nil, tables, []*metadata.ForeignKey{fkLang})
		require.Nil(t, tables[0].FK, "actor has no FKs; should be untouched")
		require.NotNil(t, tables[1].FK)
		require.Equal(t, []*metadata.ForeignKey{fkLang}, tables[1].FK.Outgoing)
		require.Nil(t, tables[2].FK, "language has no outgoing FKs and no Assign* should touch it")
	})

	t.Run("nil_fk_entry_skipped", func(t *testing.T) {
		fkOK := &metadata.ForeignKey{Table: "film", RefTable: "language"}
		tables := []*metadata.Table{{Name: "film"}}
		metadata.AssignForeignKeys(nil, tables, []*metadata.ForeignKey{nil, fkOK, nil})
		require.NotNil(t, tables[0].FK)
		require.Equal(t, []*metadata.ForeignKey{fkOK}, tables[0].FK.Outgoing)
	})

	t.Run("nil_table_entry_skipped", func(t *testing.T) {
		fkOK := &metadata.ForeignKey{Table: "film", RefTable: "language"}
		tables := []*metadata.Table{nil, {Name: "film"}, nil}
		metadata.AssignForeignKeys(nil, tables, []*metadata.ForeignKey{fkOK})
		require.NotNil(t, tables[1].FK)
		require.Equal(t, []*metadata.ForeignKey{fkOK}, tables[1].FK.Outgoing)
	})

	t.Run("replaces_existing_outgoing", func(t *testing.T) {
		old := &metadata.ForeignKey{Table: "film", RefTable: "old_ref"}
		new1 := &metadata.ForeignKey{Table: "film", RefTable: "language"}
		new2 := &metadata.ForeignKey{Table: "film", RefTable: "category"}
		tables := []*metadata.Table{
			{Name: "film", FK: &metadata.FKGroup{Outgoing: []*metadata.ForeignKey{old}}},
		}
		metadata.AssignForeignKeys(nil, tables, []*metadata.ForeignKey{new1, new2})
		require.Equal(t, []*metadata.ForeignKey{new1, new2}, tables[0].FK.Outgoing,
			"AssignForeignKeys should fully replace Outgoing, not merge")
	})

	t.Run("preserves_existing_incoming", func(t *testing.T) {
		incoming := &metadata.ForeignKey{Table: "film_actor", RefTable: "actor"}
		fkOut := &metadata.ForeignKey{Table: "actor", RefTable: "external"}
		tables := []*metadata.Table{
			{Name: "actor", FK: &metadata.FKGroup{Incoming: []*metadata.ForeignKey{incoming}}},
		}
		metadata.AssignForeignKeys(nil, tables, []*metadata.ForeignKey{fkOut})
		require.Equal(t, []*metadata.ForeignKey{fkOut}, tables[0].FK.Outgoing)
		require.Equal(t, []*metadata.ForeignKey{incoming}, tables[0].FK.Incoming,
			"AssignForeignKeys must not clobber pre-existing Incoming")
	})

	t.Run("unmatched_fk_dropped", func(t *testing.T) {
		fkOrphan := &metadata.ForeignKey{Table: "missing_table", RefTable: "language"}
		tables := []*metadata.Table{{Name: "film"}}
		metadata.AssignForeignKeys(nil, tables, []*metadata.ForeignKey{fkOrphan})
		require.Nil(t, tables[0].FK, "FK referencing a table not in the slice should not land anywhere")
	})

	t.Run("unmatched_fk_warns_when_logger_present", func(t *testing.T) {
		var buf bytes.Buffer
		log := newJSONLog(&buf, slog.LevelWarn)
		fkOrphan := &metadata.ForeignKey{Name: "fk_orphan", Table: "ghost", RefTable: "language"}
		tables := []*metadata.Table{{Name: "film"}}
		metadata.AssignForeignKeys(log, tables, []*metadata.ForeignKey{fkOrphan})
		entries := parseEntries(t, &buf)
		require.Len(t, entries, 1)
		require.Equal(t, "WARN", entries[0]["level"])
		require.Equal(t, "ghost", entries[0]["table"],
			"warn payload must name the unknown owning table under the `table` attribute")
		require.Equal(t, "foreign key", entries[0]["kind"])
		require.EqualValues(t, 1, entries[0]["dropped"])
	})

	t.Run("multi_orphan_warns_in_sorted_order", func(t *testing.T) {
		var buf bytes.Buffer
		log := newJSONLog(&buf, slog.LevelWarn)
		// Insertion order is deliberately scrambled so any non-sorted
		// implementation would surface a different log sequence.
		fks := []*metadata.ForeignKey{
			{Name: "fk_z", Table: "zeta"},
			{Name: "fk_a", Table: "alpha"},
			{Name: "fk_m", Table: "mike"},
		}
		tables := []*metadata.Table{{Name: "film"}}
		metadata.AssignForeignKeys(log, tables, fks)
		entries := parseEntries(t, &buf)
		require.Len(t, entries, 3)
		var got []string
		for _, e := range entries {
			got = append(got, e["table"].(string))
		}
		require.Equal(t, []string{"alpha", "mike", "zeta"}, got,
			"warnOrphans must emit warns in lexicographic order so log output is reproducible")
	})
}

func TestAssignUniqueConstraints(t *testing.T) {
	t.Run("empty_ucs_noop", func(t *testing.T) {
		tables := []*metadata.Table{{Name: "actor"}}
		metadata.AssignUniqueConstraints(nil, tables, nil)
		require.Nil(t, tables[0].UniqueConstraints)
	})

	t.Run("basic_assignment", func(t *testing.T) {
		uc := &metadata.UniqueConstraint{Name: "u_email", Table: "customer", Columns: []string{"email"}}
		tables := []*metadata.Table{{Name: "customer"}, {Name: "actor"}}
		metadata.AssignUniqueConstraints(nil, tables, []*metadata.UniqueConstraint{uc})
		require.Equal(t, []*metadata.UniqueConstraint{uc}, tables[0].UniqueConstraints)
		require.Nil(t, tables[1].UniqueConstraints)
	})

	t.Run("nil_uc_entry_skipped", func(t *testing.T) {
		uc := &metadata.UniqueConstraint{Name: "u", Table: "t"}
		tables := []*metadata.Table{{Name: "t"}}
		metadata.AssignUniqueConstraints(nil, tables, []*metadata.UniqueConstraint{nil, uc, nil})
		require.Equal(t, []*metadata.UniqueConstraint{uc}, tables[0].UniqueConstraints)
	})

	t.Run("nil_table_entry_skipped", func(t *testing.T) {
		uc := &metadata.UniqueConstraint{Name: "u", Table: "t"}
		tables := []*metadata.Table{nil, {Name: "t"}, nil}
		metadata.AssignUniqueConstraints(nil, tables, []*metadata.UniqueConstraint{uc})
		require.Equal(t, []*metadata.UniqueConstraint{uc}, tables[1].UniqueConstraints)
	})

	t.Run("replaces_existing", func(t *testing.T) {
		old := &metadata.UniqueConstraint{Name: "u_old", Table: "t"}
		new1 := &metadata.UniqueConstraint{Name: "u_new", Table: "t"}
		tables := []*metadata.Table{
			{Name: "t", UniqueConstraints: []*metadata.UniqueConstraint{old}},
		}
		metadata.AssignUniqueConstraints(nil, tables, []*metadata.UniqueConstraint{new1})
		require.Equal(t, []*metadata.UniqueConstraint{new1}, tables[0].UniqueConstraints)
	})

	t.Run("unmatched_uc_warns_when_logger_present", func(t *testing.T) {
		var buf bytes.Buffer
		log := newJSONLog(&buf, slog.LevelWarn)
		uc := &metadata.UniqueConstraint{Name: "u_ghost", Table: "ghost", Columns: []string{"id"}}
		tables := []*metadata.Table{{Name: "film"}}
		metadata.AssignUniqueConstraints(log, tables, []*metadata.UniqueConstraint{uc})
		entries := parseEntries(t, &buf)
		require.Len(t, entries, 1)
		require.Equal(t, "WARN", entries[0]["level"])
		require.Equal(t, "ghost", entries[0]["table"])
		require.Equal(t, "unique constraint", entries[0]["kind"],
			"the warn must label the kind so operators can grep for unique-constraint drops specifically")
	})
}

func TestAssignIndexes(t *testing.T) {
	t.Run("empty_idxs_noop", func(t *testing.T) {
		tables := []*metadata.Table{{Name: "actor"}}
		metadata.AssignIndexes(nil, tables, nil)
		require.Nil(t, tables[0].Indexes)
	})

	t.Run("basic_assignment", func(t *testing.T) {
		idx := &metadata.Index{Name: "idx_last_name", Table: "actor", Columns: []string{"last_name"}}
		tables := []*metadata.Table{{Name: "actor"}, {Name: "film"}}
		metadata.AssignIndexes(nil, tables, []*metadata.Index{idx})
		require.Equal(t, []*metadata.Index{idx}, tables[0].Indexes)
		require.Nil(t, tables[1].Indexes)
	})

	t.Run("nil_idx_entry_skipped", func(t *testing.T) {
		idx := &metadata.Index{Name: "i", Table: "t"}
		tables := []*metadata.Table{{Name: "t"}}
		metadata.AssignIndexes(nil, tables, []*metadata.Index{nil, idx, nil})
		require.Equal(t, []*metadata.Index{idx}, tables[0].Indexes)
	})

	t.Run("nil_table_entry_skipped", func(t *testing.T) {
		idx := &metadata.Index{Name: "i", Table: "t"}
		tables := []*metadata.Table{nil, {Name: "t"}, nil}
		metadata.AssignIndexes(nil, tables, []*metadata.Index{idx})
		require.Equal(t, []*metadata.Index{idx}, tables[1].Indexes)
	})

	t.Run("replaces_existing", func(t *testing.T) {
		old := &metadata.Index{Name: "i_old", Table: "t"}
		new1 := &metadata.Index{Name: "i_new", Table: "t"}
		tables := []*metadata.Table{
			{Name: "t", Indexes: []*metadata.Index{old}},
		}
		metadata.AssignIndexes(nil, tables, []*metadata.Index{new1})
		require.Equal(t, []*metadata.Index{new1}, tables[0].Indexes)
	})

	t.Run("unmatched_idx_warns_when_logger_present", func(t *testing.T) {
		var buf bytes.Buffer
		log := newJSONLog(&buf, slog.LevelWarn)
		idx := &metadata.Index{Name: "i_ghost", Table: "ghost", Columns: []string{"id"}}
		tables := []*metadata.Table{{Name: "film"}}
		metadata.AssignIndexes(log, tables, []*metadata.Index{idx})
		entries := parseEntries(t, &buf)
		require.Len(t, entries, 1)
		require.Equal(t, "WARN", entries[0]["level"])
		require.Equal(t, "ghost", entries[0]["table"])
		require.Equal(t, "index", entries[0]["kind"])
	})
}

// TestFKGroup_Clone_StandaloneDropsIncoming locks the contract
// promised in [FKGroup.Clone]'s godoc — a standalone clone (not via
// [Source.Clone]) has Incoming == nil because LinkForeignKeys needs
// the whole source to re-derive back-references.
func TestFKGroup_Clone_StandaloneDropsIncoming(t *testing.T) {
	t.Run("nil_receiver", func(t *testing.T) {
		var g *metadata.FKGroup
		require.Nil(t, g.Clone())
	})

	t.Run("incoming_dropped_outgoing_deep_copied", func(t *testing.T) {
		outFK := &metadata.ForeignKey{Name: "fk_out", Table: "child", RefTable: "parent"}
		incFK := &metadata.ForeignKey{Name: "fk_in", Table: "other", RefTable: "child"}
		g := &metadata.FKGroup{
			Outgoing: []*metadata.ForeignKey{outFK},
			Incoming: []*metadata.ForeignKey{incFK},
		}
		clone := g.Clone()
		require.NotNil(t, clone)
		require.Nil(t, clone.Incoming,
			"standalone FKGroup.Clone must leave Incoming nil; use Source.Clone to re-link")
		require.Len(t, clone.Outgoing, 1)
		require.NotSame(t, outFK, clone.Outgoing[0],
			"Outgoing entries must be deep-copied, not aliased to the original")
		require.Equal(t, outFK.Name, clone.Outgoing[0].Name)
	})
}

func TestForeignKey_String(t *testing.T) {
	fk := &metadata.ForeignKey{
		Name:     "fk_film_language",
		Table:    "film",
		Columns:  []string{"language_id"},
		RefTable: "language",
	}
	got := fk.String()
	require.NotEmpty(t, got)
	require.Contains(t, got, "fk_film_language")

	// A nil receiver must not panic; json.Marshal of a nil pointer
	// yields "null".
	var nilFK *metadata.ForeignKey
	require.Equal(t, "null", nilFK.String())
}

func TestUniqueConstraint_String(t *testing.T) {
	uc := &metadata.UniqueConstraint{
		Name:    "uq_film_title",
		Table:   "film",
		Columns: []string{"title"},
	}
	got := uc.String()
	require.NotEmpty(t, got)
	require.Contains(t, got, "uq_film_title")

	var nilUC *metadata.UniqueConstraint
	require.Equal(t, "null", nilUC.String())
}

func TestIndex_String(t *testing.T) {
	idx := &metadata.Index{
		Name:    "idx_film_title",
		Table:   "film",
		Columns: []string{"title"},
	}
	got := idx.String()
	require.NotEmpty(t, got)
	require.Contains(t, got, "idx_film_title")

	var nilIdx *metadata.Index
	require.Equal(t, "null", nilIdx.String())
}

func TestNewFKGroup(t *testing.T) {
	t.Run("both_empty_returns_nil", func(t *testing.T) {
		require.Nil(t, metadata.NewFKGroup(nil, nil))
		require.Nil(t, metadata.NewFKGroup([]*metadata.ForeignKey{}, []*metadata.ForeignKey{}))
	})

	t.Run("outgoing_only", func(t *testing.T) {
		fk := &metadata.ForeignKey{Table: "film", RefTable: "language"}
		g := metadata.NewFKGroup([]*metadata.ForeignKey{fk}, nil)
		require.NotNil(t, g)
		require.Equal(t, []*metadata.ForeignKey{fk}, g.Outgoing)
		require.Nil(t, g.Incoming)
	})

	t.Run("incoming_only", func(t *testing.T) {
		fk := &metadata.ForeignKey{Table: "other", RefTable: "film"}
		g := metadata.NewFKGroup(nil, []*metadata.ForeignKey{fk})
		require.NotNil(t, g)
		require.Nil(t, g.Outgoing)
		require.Equal(t, []*metadata.ForeignKey{fk}, g.Incoming)
	})
}

func TestAllExpressionKeys(t *testing.T) {
	testCases := []struct {
		name string
		cols []string
		want bool
	}{
		{name: "nil", cols: nil, want: false},
		{name: "empty", cols: []string{}, want: false},
		{name: "single_expr", cols: []string{""}, want: true},
		{name: "all_expr", cols: []string{"", ""}, want: true},
		{name: "single_col", cols: []string{"a"}, want: false},
		{name: "mixed_leading_col", cols: []string{"a", ""}, want: false},
		{name: "mixed_trailing_col", cols: []string{"", "c"}, want: false},
		{name: "all_cols", cols: []string{"a", "b"}, want: false},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, metadata.AllExpressionKeys(tc.cols))
		})
	}
}

// TestSource_Clone_NilLoggerSilencesWarnings locks the contract from
// [Source.Clone]'s godoc — Clone passes nil to LinkForeignKeys, so a
// programmatically-constructed source with an unresolved FK clones
// cleanly with zero log output. Callers wanting those warnings must
// call LinkForeignKeys with a real logger themselves.
func TestSource_Clone_NilLoggerSilencesWarnings(t *testing.T) {
	prev := slog.Default()
	var buf bytes.Buffer
	slog.SetDefault(newJSONLog(&buf, slog.LevelDebug))
	t.Cleanup(func() { slog.SetDefault(prev) })

	bad := &metadata.ForeignKey{
		Name: "fk_typo", Table: "film",
		Columns: []string{"language_id"}, RefTable: "lanugage", RefColumns: []string{"id"},
	}
	src := &metadata.Source{
		Tables: []*metadata.Table{
			{Name: "film", FK: metadata.NewFKGroup([]*metadata.ForeignKey{bad}, nil)},
			{Name: "language"},
		},
	}

	clone := src.Clone()
	require.NotNil(t, clone)
	require.Empty(t, buf.String(),
		"Source.Clone must not emit log entries — programmatic callers wanting "+
			"validation should call LinkForeignKeys with a real logger")
}
