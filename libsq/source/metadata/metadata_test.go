package metadata_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/libsq/core/kind"
	"github.com/neilotoole/sq/libsq/source/drivertype"
	"github.com/neilotoole/sq/libsq/source/metadata"
)

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
		src := &metadata.Source{
			Handle:     "@sakila",
			Location:   "postgres://localhost/sakila",
			Name:       "sakila",
			FQName:     "sakila.public",
			Schema:     "public",
			Catalog:    "sakila",
			Driver:     drivertype.Pg,
			DBDriver:   drivertype.Pg,
			DBProduct:  "PostgreSQL 14.0",
			DBVersion:  "14.0",
			User:       "postgres",
			Size:       1024,
			TableCount: 10,
			ViewCount:  5,
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
	})
}

func TestSource_TableNames(t *testing.T) {
	testCases := []struct {
		name string
		src  *metadata.Source
		want []string
	}{
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
		metadata.LinkForeignKeys(nil)
	})

	t.Run("empty_tables", func(t *testing.T) {
		src := &metadata.Source{}
		metadata.LinkForeignKeys(src)
		require.Nil(t, src.Tables)
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
					FKOutgoing: []*metadata.ForeignKey{fk},
				},
				{
					Name: "language",
					Columns: []*metadata.Column{
						{Name: "language_id", PrimaryKey: true},
					},
				},
			},
		}

		metadata.LinkForeignKeys(src)

		film := src.Table("film")
		require.NotNil(t, film)
		require.Len(t, film.FKOutgoing, 1)
		require.Same(t, fk, film.FKOutgoing[0])

		language := src.Table("language")
		require.Len(t, language.FKIncoming, 1)
		require.Same(t, fk, language.FKIncoming[0],
			"language.FKIncoming entry must share identity with the FK on film.FKOutgoing")
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
					FKOutgoing: []*metadata.ForeignKey{fk},
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

		metadata.LinkForeignKeys(src)

		lookup := src.Table("film_actor_lookup")
		require.Len(t, lookup.FKIncoming, 1,
			"composite FK should appear once on the referenced table, not once per column")
		require.Same(t, fk, lookup.FKIncoming[0])
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
					Name:       "audit",
					Columns:    []*metadata.Column{{Name: "user_id"}},
					FKOutgoing: []*metadata.ForeignKey{fk},
				},
				{
					Name:    "users", // local namesake — must not be matched
					Columns: []*metadata.Column{{Name: "id"}},
				},
			},
		}

		metadata.LinkForeignKeys(src)

		// Local "users" must not appear as an FKIncoming target.
		require.Nil(t, src.Table("users").FKIncoming,
			"local users.FKIncoming must be empty when FK points to a different schema")
		// The outgoing FK is still on the audit table's FKOutgoing slice.
		require.Len(t, src.Table("audit").FKOutgoing, 1)
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
					Name:       "child",
					Columns:    []*metadata.Column{{Name: "parent_id"}},
					FKOutgoing: []*metadata.ForeignKey{fk},
				},
				{
					Name:    "parent",
					Columns: []*metadata.Column{{Name: "id"}},
				},
			},
		}

		metadata.LinkForeignKeys(src)

		require.Equal(t, "", fk.RefCatalog, "same-catalog qualifier should be cleared")
		require.Equal(t, "", fk.RefSchema, "same-schema qualifier should be cleared")
		require.Len(t, src.Table("parent").FKIncoming, 1)
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
					FKOutgoing: []*metadata.ForeignKey{fk},
				},
			},
		}

		metadata.LinkForeignKeys(src)

		local := src.Table("local")
		// The outgoing FK is still on FKOutgoing even though the
		// referenced table is outside this Source.
		require.Len(t, local.FKOutgoing, 1)
		// No incoming back-ref appears anywhere within the Source.
		require.Nil(t, local.FKIncoming)
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
					Name:       "child",
					Columns:    []*metadata.Column{{Name: "parent_id"}},
					FKOutgoing: []*metadata.ForeignKey{fk},
				},
				{
					Name:    "parent",
					Columns: []*metadata.Column{{Name: "id"}},
				},
			},
		}

		metadata.LinkForeignKeys(src)
		metadata.LinkForeignKeys(src)

		require.Len(t, src.Table("parent").FKIncoming, 1)
		require.Same(t, fk, src.Table("parent").FKIncoming[0])
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
				FKOutgoing: []*metadata.ForeignKey{fk},
			},
			{
				Name:    "language",
				Columns: []*metadata.Column{{Name: "language_id"}},
			},
		},
	}
	metadata.LinkForeignKeys(src)

	got := src.Clone()
	gotFilm := got.Table("film")
	gotLanguage := got.Table("language")

	// The clone's incoming back-references must point at the clone's
	// own ForeignKey objects, not the originals.
	require.Len(t, gotFilm.FKOutgoing, 1)
	require.NotSame(t, fk, gotFilm.FKOutgoing[0])
	require.Len(t, gotLanguage.FKIncoming, 1)
	require.Same(t, gotFilm.FKOutgoing[0], gotLanguage.FKIncoming[0],
		"FKIncoming on the clone must share identity with the clone's own FKOutgoing entry")
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
