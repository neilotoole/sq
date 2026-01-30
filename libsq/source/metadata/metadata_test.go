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
