package source_test

import (
	"encoding/json"
	"errors"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/require"
	yaml "gopkg.in/yaml.v3"

	"github.com/neilotoole/sq/libsq/core/lg/lga"
	"github.com/neilotoole/sq/libsq/core/options"
	"github.com/neilotoole/sq/libsq/source"
	"github.com/neilotoole/sq/libsq/source/drivertype"
	"github.com/neilotoole/sq/libsq/source/location"
	"github.com/neilotoole/sq/testh/proj"
	"github.com/neilotoole/sq/testh/tu"
)

const (
	prodGroup    = "prod"
	devGroup     = "dev"
	devCustGroup = "dev/customer"
)

// newSource returns a new source with handle, pointing to
// the sqlite sakila.db.
func newSource(handle string) *source.Source {
	return &source.Source{
		Handle:   handle,
		Type:     drivertype.SQLite,
		Location: proj.Abs("drivers/sqlite3/testdata/sakila.db"),
	}
}

func TestCollection_Groups(t *testing.T) {
	srcs := []*source.Source{
		{Handle: "@db1", Location: "0"},
		{Handle: "@prod/db1", Location: "1"},
		{Handle: "@prod/sub1/db1", Location: "2"},
		{Handle: "@prod/sub1/db2", Location: "3"},
		{Handle: "@prod/sub1/sub2/sub3/db2", Location: "4"},
		{Handle: "@prod/sub1/sub2/sub4/sub5/db", Location: "5"},
		{Handle: "@staging/sub1/sub2/db", Location: "6"},
		{Handle: "@dev/db", Location: "7"},
	}

	require.Equal(t, srcs[0].Group(), "")
	require.Equal(t, srcs[1].Group(), "prod")
	require.Equal(t, srcs[2].Group(), "prod/sub1")
	require.Equal(t, srcs[5].Group(), "prod/sub1/sub2/sub4/sub5")
	require.Equal(t, srcs[7].Group(), "dev")

	wantGroups := []string{
		source.RootGroup,
		"dev",
		"prod",
		"prod/sub1",
		"prod/sub1/sub2",
		"prod/sub1/sub2/sub3",
		"prod/sub1/sub2/sub4",
		"prod/sub1/sub2/sub4/sub5",
		"staging",
		"staging/sub1",
		"staging/sub1/sub2",
	}

	coll := &source.Collection{}

	gotGroup := coll.ActiveGroup()
	require.Equal(t, source.RootGroup, gotGroup)

	for i := range srcs {
		require.NoError(t, coll.Add(srcs[i]))
	}

	for _, src := range srcs {
		require.True(t, coll.IsExistingSource(src.Handle))
		gotSrc, err := coll.Get(src.Handle)
		require.NoError(t, err)
		require.Equal(t, *src, *gotSrc)
	}

	gotGroups := coll.Groups()
	require.EqualValues(t, wantGroups, gotGroups)

	gotErr := coll.SetActiveGroup("not_a_group")
	require.Error(t, gotErr)

	groupTest := map[string]int{
		"":                         len(srcs),
		"prod":                     5,
		"prod/sub1":                4,
		"prod/sub1/sub2/sub4/sub5": 1,
		"dev":                      1,
		"prod/sub1/sub2":           2,
	}

	for g, wantCount := range groupTest {
		gotSrcs, err := coll.SourcesInGroup(g)
		require.NoError(t, err)
		require.Equal(t, wantCount, len(gotSrcs))
	}
}

func TestRedactedLocation(t *testing.T) {
	testCases := []struct {
		loc  string
		want string
	}{
		{
			loc:  "/path/to/sqlite.db",
			want: "/path/to/sqlite.db",
		},
		{
			loc:  "/path/to/data.xlsx",
			want: "/path/to/data.xlsx",
		},
		{
			loc:  "https://path/to/data.xlsx",
			want: "https://path/to/data.xlsx",
		},
		{
			loc:  "http://path/to/data.xlsx",
			want: "http://path/to/data.xlsx",
		},
		{
			loc:  "sqlserver://sq:p_ssW0rd@localhost?database=sqtest",
			want: "sqlserver://sq:xxxxx@localhost?database=sqtest",
		},
		{
			loc:  "postgres://sq:p_ssW0rd@localhost/sqtest?sslmode=disable",
			want: "postgres://sq:xxxxx@localhost/sqtest?sslmode=disable",
		},
		{
			loc:  "mysql://sq:p_ssW0rd@localhost:3306/sqtest",
			want: "mysql://sq:xxxxx@localhost:3306/sqtest",
		},
		{
			loc:  "sqlite3:///path/to/sqlite.db",
			want: "sqlite3:///path/to/sqlite.db",
		},
	}

	for _, tc := range testCases {
		t.Run(tu.Name(tc.loc), func(t *testing.T) {
			src := &source.Source{Location: tc.loc}
			got := src.RedactedLocation()
			t.Logf("%s  -->  %s", src.Location, got)
			require.Equal(t, tc.want, got)
		})
	}
}

func TestShortLocation(t *testing.T) {
	testCases := []struct {
		name string
		loc  string
		want string
	}{
		{
			name: "sqlite3_scheme",
			loc:  "sqlite3:///path/to/sqlite.db",
			want: "sqlite.db",
		},
		{
			name: "sqlite3",
			loc:  "/path/to/sqlite.db",
			want: "sqlite.db",
		},
		{
			name: "xlsx",
			loc:  "/path/to/data.xlsx",
			want: "data.xlsx",
		},
		{
			name: "https",
			loc:  "https://path/to/data.xlsx",
			want: "data.xlsx",
		},
		{
			name: "http",
			loc:  "http://path/to/data.xlsx",
			want: "data.xlsx",
		},
		{
			name: "sqlserver",
			loc:  "sqlserver://sq:p_ssw0rd@localhost?database=sqtest",
			want: "sq@localhost/sqtest",
		},
		{
			name: "sqlserver-no-params",
			loc:  "sqlserver://sq:p_ssw0rd@localhost",
			want: "sq@localhost",
		},
		{
			name: "sqlserver-with-param-no-database",
			loc:  "sqlserver://sq:p_ssw0rd@localhost?encrypt=false",
			want: "sq@localhost",
		},
		{
			name: "postgres",
			loc:  "postgres://sq:p_ssW0rd@localhost/sqtest?sslmode=disable",
			want: "sq@localhost/sqtest",
		},
		{
			name: "mysql",
			loc:  "mysql://sq:p_ssW0rd@localhost:3306/sqtest",
			want: "sq@localhost:3306/sqtest",
		},
		{
			name: "mysql",
			loc:  "mysql://sq:p_ssW0rd@localhost/sqtest",
			want: "sq@localhost/sqtest",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := location.Short(tc.loc)
			t.Logf("%s  -->  %s", tc.loc, got)
			require.Equal(t, tc.want, got)
		})
	}
}

func TestContains(t *testing.T) {
	src1 := &source.Source{Handle: "@src1"}
	src2 := &source.Source{Handle: "@src2"}

	var srcs []*source.Source
	require.False(t, source.Contains(nil, (*source.Source)(nil)))
	require.False(t, source.Contains(nil, ""))
	require.False(t, source.Contains(srcs, src1.Handle))
	srcs = make([]*source.Source, 0)
	require.False(t, source.Contains(srcs, src1.Handle))
	srcs = append(srcs, src1)
	require.True(t, source.Contains(srcs, src1))
	require.True(t, source.Contains(srcs, src1.Handle))
	// A different *Source with the same handle is not contained: the
	// typed path matches on pointer identity, not handle equality.
	require.False(t, source.Contains(srcs, &source.Source{Handle: "@src1"}))
	require.False(t, source.Contains(srcs, src2))
	require.False(t, source.Contains(srcs, src2.Handle))
	srcs = append(srcs, src2)
	require.True(t, source.Contains(srcs, src2))
	require.True(t, source.Contains(srcs, src2.Handle))
}

func TestCollection_Active(t *testing.T) {
	coll := &source.Collection{}

	activeSrc := coll.Active()
	require.Nil(t, activeSrc)
	require.Equal(t, source.RootGroup, coll.ActiveGroup())

	require.Error(t, coll.SetActiveGroup("non_exist"))

	sakilaSrc := newSource("@sakila")

	// Test that the active group and
	require.NoError(t, coll.Add(sakilaSrc))
	gotSrc, err := coll.Get(sakilaSrc.Handle)
	require.NoError(t, err)
	require.Equal(t, sakilaSrc, gotSrc)
	require.Equal(t, source.RootGroup, coll.ActiveGroup(),
		"active group should not have changed due to adding a source")
	require.Nil(t, coll.Active())

	// Test setting the active source
	gotSrc, err = coll.SetActive(sakilaSrc.Handle, false)
	require.NoError(t, err)
	require.Equal(t, sakilaSrc, gotSrc)
	require.Equal(t, gotSrc, coll.Active())

	// Test removing the active source
	require.NoError(t, coll.Remove(coll.ActiveHandle()))
	require.Nil(t, coll.Active())

	// Test group
	sakilaProdSrc := newSource("@prod/sakila")
	require.NoError(t, coll.Add(sakilaProdSrc))
	require.Equal(t, source.RootGroup, coll.ActiveGroup(),
		"adding a grouped src should not set the active group")

	gotSrc, err = coll.SetActive(sakilaProdSrc.Handle, false)
	require.NoError(t, err)
	require.Equal(t, sakilaProdSrc, gotSrc)
	require.Equal(t, source.RootGroup, coll.ActiveGroup(),
		"setting active src should not set active group")

	require.NoError(t, coll.SetActiveGroup(prodGroup))
	require.Equal(t, prodGroup, coll.ActiveGroup())
	gotSrcs, err := coll.RemoveGroup(prodGroup)
	require.NoError(t, err)
	require.Equal(t, sakilaProdSrc, gotSrcs[0])
	require.Equal(t, source.RootGroup, coll.ActiveGroup(),
		"active group should have been reset to root")
	require.False(t, coll.IsExistingGroup(prodGroup))
	require.Empty(t, coll.Sources())
}

func TestCollection_RenameGroup_toRoot(t *testing.T) {
	coll := &source.Collection{}

	gotSrcs, err := coll.RenameGroup(source.RootGroup, prodGroup)
	require.Error(t, err, "can't rename root group")
	require.Nil(t, gotSrcs)

	src := newSource("@prod/sakila")
	originalHandle := src.Handle
	require.NoError(t, coll.Add(src))

	gotSrcs, err = coll.SourcesInGroup(prodGroup)
	require.NoError(t, err)
	require.Len(t, gotSrcs, 1)
	require.Equal(t, src, gotSrcs[0])

	// Rename "prod" group to root effectively moves all prod sources
	// into root. The prod group will cease to exist.
	gotSrcs, err = coll.RenameGroup(prodGroup, source.RootGroup)
	require.NoError(t, err)
	require.Len(t, gotSrcs, 1)
	require.Equal(t, source.RootGroup, coll.ActiveGroup())
	require.Equal(t, "@sakila", src.Handle, "src should have new handle")

	require.False(t, coll.IsExistingGroup(prodGroup))
	gotSrc, err := coll.Get(originalHandle)
	require.Error(t, err, "original handle no longer exists")
	require.Nil(t, gotSrc)

	gotSrcs, err = coll.SourcesInGroup(prodGroup)
	require.Error(t, err, "group should not not exist")
	require.Empty(t, gotSrcs)

	gotSrc, err = coll.Get("@sakila")
	require.NoError(t, err, "should be available via new handle")
	require.Equal(t, src.Location, gotSrc.Location)

	gotSrcs, err = coll.SourcesInGroup(source.RootGroup)
	require.NoError(t, err)
	require.Len(t, gotSrcs, 1)
	require.Equal(t, src, gotSrcs[0])

	// Do the same as above, but rename "prod" group to "prod/customer".
}

func TestCollection_RenameGroup_toOther(t *testing.T) {
	coll := &source.Collection{}

	src := newSource("@prod/sakila")
	originalHandle := src.Handle
	require.NoError(t, coll.Add(src))

	// Rename "prod" group to "dev/customer" effectively moves all prod sources
	// into "dev/customer". The prod group will cease to exist.
	gotSrcs, err := coll.RenameGroup(prodGroup, devCustGroup)
	require.NoError(t, err)
	require.Len(t, gotSrcs, 1)
	require.Equal(t, source.RootGroup, coll.ActiveGroup())
	require.Equal(t, "@dev/customer/sakila", src.Handle,
		"src should have new handle")

	require.False(t, coll.IsExistingGroup(prodGroup))
	gotSrc, err := coll.Get(originalHandle)
	require.Error(t, err, "original handle no longer exists")
	require.Nil(t, gotSrc)

	gotSrcs, err = coll.SourcesInGroup(prodGroup)
	require.Error(t, err, "group should not not exist")
	require.Empty(t, gotSrcs)

	gotSrc, err = coll.Get("@dev/customer/sakila")
	require.NoError(t, err, "should be available via new handle")
	require.Equal(t, src.Location, gotSrc.Location)

	gotSrcs, err = coll.SourcesInGroup(devCustGroup)
	require.NoError(t, err)
	require.Len(t, gotSrcs, 1)
	require.Equal(t, src, gotSrcs[0])
}

func TestCollection_Add_conflictsWithGroup(t *testing.T) {
	coll := &source.Collection{}

	src1 := newSource("@prod/sakila")
	require.NoError(t, coll.Add(src1))
	require.True(t, coll.IsExistingGroup(prodGroup))

	src2 := newSource("@prod")
	require.Error(t, coll.Add(src2), "handle conflicts with existing group")
}

func TestCollection_Add_groupConflictsWithSource(t *testing.T) {
	coll := &source.Collection{}

	src1 := newSource("@sakila")
	require.NoError(t, coll.Add(src1))

	src2 := newSource("@sakila/sakiladb")
	require.Error(t, coll.Add(src2), "handle group (sakila) conflicts with source @sakila")
}

// TestCollection_Add_rejectsNestingUnderAncestorHandle verifies that a new
// handle cannot nest below an existing handle at any depth. The immediate
// case (@sakila, then @sakila/sakiladb) is covered by
// TestCollection_Add_groupConflictsWithSource; this exercises the deeper
// case that previously slipped through: only the new handle's immediate
// group was checked against existing handles, so @prod followed by
// @prod/db/x was accepted, while the reverse order was rejected. Handles
// and groups share path semantics (e.g. the cache dir layout), so the
// check must be symmetric and transitive in both directions.
func TestCollection_Add_rejectsNestingUnderAncestorHandle(t *testing.T) {
	coll := &source.Collection{}
	require.NoError(t, coll.Add(newSource("@prod")))

	err := coll.Add(newSource("@prod/db/x"))
	require.Error(t, err)
	require.Contains(t, err.Error(), "@prod")
}

// TestCollection_RenameSource_rejectsNestingUnderHandle verifies that
// renaming (sq mv @x @prod/x) cannot create a handle nested below an
// existing handle: RenameSource previously performed no such check at
// all, so it could create nesting that Add would reject.
func TestCollection_RenameSource_rejectsNestingUnderHandle(t *testing.T) {
	coll := &source.Collection{}
	require.NoError(t, coll.Add(newSource("@prod")))
	require.NoError(t, coll.Add(newSource("@x")))

	_, err := coll.RenameSource("@x", "@prod/x")
	require.Error(t, err, "depth-1 nesting under @prod must be rejected")

	_, err = coll.RenameSource("@x", "@prod/db/x")
	require.Error(t, err, "deep nesting under @prod must be rejected")
}

// TestCollection_MoveHandleToGroup_rejectsNestingUnderHandle: moving a
// source into a group whose path nests below an existing handle must be
// rejected, including when only an ancestor of the dest group collides.
func TestCollection_MoveHandleToGroup_rejectsNestingUnderHandle(t *testing.T) {
	coll := &source.Collection{}
	require.NoError(t, coll.Add(newSource("@prod")))
	require.NoError(t, coll.Add(newSource("@x")))

	_, err := coll.MoveHandleToGroup("@x", "prod/db")
	require.Error(t, err, "moving @x into group prod/db nests it under handle @prod")
}

// TestCollection_RenameGroup_rejectsNestingUnderHandle: renaming a group
// to a path nested below an existing handle must be rejected.
func TestCollection_RenameGroup_rejectsNestingUnderHandle(t *testing.T) {
	coll := &source.Collection{}
	require.NoError(t, coll.Add(newSource("@g/x")))
	require.NoError(t, coll.Add(newSource("@prod")))

	_, err := coll.RenameGroup("g", "prod/sub")
	require.Error(t, err, "renaming group g to prod/sub nests its sources under handle @prod")
}

func TestCollection_RenameGroup(t *testing.T) {
	coll := &source.Collection{}

	src1 := newSource("@prod/sakila")
	require.NoError(t, coll.Add(src1))

	gotSrcs, err := coll.RenameGroup(devGroup, prodGroup)
	require.Error(t, err, "group dev does not exist")
	require.Nil(t, gotSrcs)

	gotSrcs, err = coll.RenameGroup(prodGroup, devGroup)
	require.NoError(t, err)
	require.Equal(t, gotSrcs[0].Handle, "@dev/sakila")
}

func TestCollection_RenameGroup_conflictsWithSource(t *testing.T) {
	coll := &source.Collection{}

	src1 := newSource("@sakila")
	require.NoError(t, coll.Add(src1))

	src2 := newSource("@prod/db")
	require.NoError(t, coll.Add(src2))

	_, err := coll.RenameGroup("prod", "sakila")
	require.Error(t, err, "should be a conflict error")
}

func TestCollection_MoveHandleToGroup(t *testing.T) {
	coll := &source.Collection{}

	src1 := newSource("@sakila")
	require.NoError(t, coll.Add(src1))

	gotSrc, err := coll.MoveHandleToGroup(src1.Handle, "/")
	// This is effectively no-op
	require.NoError(t, err)
	require.Equal(t, src1, gotSrc)

	gotSrc, err = coll.MoveHandleToGroup(src1.Handle, prodGroup)
	require.NoError(t, err, "it is legal to move a handle to a non-existing group")
	require.Equal(t, "@prod/sakila", gotSrc.Handle)
	require.Equal(t, prodGroup, gotSrc.Group())
}

func TestCollection_MoveHandleToGroup_conflictsWithExistingSource(t *testing.T) {
	coll := &source.Collection{}

	src1 := newSource("@sakila")
	require.NoError(t, coll.Add(src1))

	src2 := newSource("@prod/db")
	require.NoError(t, coll.Add(src2))

	gotSrc, err := coll.MoveHandleToGroup(src1.Handle, "sakila")
	// This is effectively no-op
	require.Error(t, err, "group 'sakila' should conflict with handle @sakila")
	require.Nil(t, gotSrc)
}

func TestCollection_RenameSource(t *testing.T) {
	coll := &source.Collection{}

	src1 := newSource("@sakila")
	require.NoError(t, coll.Add(src1))

	gotSrc, err := coll.RenameSource(src1.Handle, "@sakila2")
	require.NoError(t, err)
	require.Equal(t, "@sakila2", gotSrc.Handle)
	require.Equal(t, src1, gotSrc)
}

func TestCollection_RenameSource_conflictsWithExistingHandle(t *testing.T) {
	coll := &source.Collection{}

	src1 := newSource("@prod/sakila")
	require.NoError(t, coll.Add(src1))

	src2 := newSource("@dev/sakila")
	require.NoError(t, coll.Add(src2))

	gotSrc, err := coll.RenameSource(src2.Handle, src1.Handle)
	require.Error(t, err)
	require.Nil(t, gotSrc)
}

func TestCollection_RenameSource_conflictsWithExistingGroup(t *testing.T) {
	coll := &source.Collection{}

	src1 := newSource("@prod/sakila")
	require.NoError(t, coll.Add(src1))

	src2 := newSource("@dev/sakila")
	require.NoError(t, coll.Add(src2))

	gotSrc, err := coll.RenameSource(src1.Handle, "/")
	require.Error(t, err)
	require.Nil(t, gotSrc)

	gotSrc, err = coll.RenameSource(src1.Handle, "@prod")
	require.Error(t, err)
	require.Nil(t, gotSrc)
}

func TestCollection_Tree(t *testing.T) {
	coll := &source.Collection{}

	handles := []string{
		"@sakila_csv",
		"@sakila_tsv",
		"@dev/db1",
		"@dev/pg/db1",
		"@dev/pg/db2",
		"@dev/pg/db3",
		"@staging/db1",
		"@prod/pg/db1",
		"@prod/pg/db2",
		"@prod/pg/backup/db1",
		"@prod/pg/backup/db2",
	}

	for _, handle := range handles {
		require.NoError(t, coll.Add(newSource(handle)))
	}

	gotSrcs := coll.Sources()
	require.Len(t, gotSrcs, 11)

	gotGroupNames := coll.Groups()
	require.Len(t, gotGroupNames, 7)

	gotTree, err := coll.Tree(source.RootGroup)
	require.NoError(t, err)

	directSrcCount, allSrcCount, directGroupCount, allGroupCount := gotTree.Counts()
	require.Equal(t, 2, directSrcCount)
	require.Equal(t, directSrcCount, len(gotTree.Sources))
	require.Equal(t, 11, allSrcCount)
	require.Equal(t, 3, directGroupCount)
	require.Equal(t, directGroupCount, len(gotTree.Groups))
	require.Equal(t, 6, allGroupCount)
	require.True(t, gotTree.Active, "root group is active")
	require.False(t, gotTree.Groups[0].Active)

	// Try with a subgroup
	gotTree, err = coll.Tree("dev")
	require.NoError(t, err)
	directSrcCount, allSrcCount, directGroupCount, allGroupCount = gotTree.Counts()
	require.Equal(t, 1, directSrcCount)
	require.Equal(t, directSrcCount, len(gotTree.Sources))
	require.Equal(t, 4, allSrcCount)
	require.Equal(t, 1, directGroupCount)
	require.Equal(t, directGroupCount, len(gotTree.Groups))
	require.Equal(t, 1, allGroupCount)
	require.False(t, gotTree.Active)
}

func TestSource_LogValue(t *testing.T) {
	// Nil receiver yields an empty (KindAny) value, not a panic.
	var nilSrc *source.Source
	require.Equal(t, slog.KindAny, nilSrc.LogValue().Kind())

	logAttrs := func(s *source.Source) map[string]string {
		attrs := s.LogValue().Group()
		m := make(map[string]string, len(attrs))
		for _, a := range attrs {
			m[a.Key] = a.Value.String()
		}
		return m
	}

	// Without Catalog/Schema, those attrs are omitted.
	base := logAttrs(&source.Source{
		Handle:   "@sakila",
		Type:     drivertype.SQLite,
		Location: "/tmp/sakila.db",
	})
	require.Equal(t, "@sakila", base[lga.Handle])
	require.NotContains(t, base, lga.Catalog)
	require.NotContains(t, base, lga.Schema)

	// With Catalog/Schema set, both attrs are emitted, and the location
	// is redacted.
	got := logAttrs(&source.Source{
		Handle:   "@sakila",
		Type:     drivertype.Pg,
		Location: "postgres://user:tibsqlpw@localhost/sakila",
		Catalog:  "cat",
		Schema:   "sch",
	})
	require.Equal(t, "cat", got[lga.Catalog])
	require.Equal(t, "sch", got[lga.Schema])
	require.NotContains(t, got[lga.Loc], "tibsqlpw",
		"location attr must be redacted")
}

func TestSource_Clone(t *testing.T) {
	t.Run("nil_source", func(t *testing.T) {
		var src *source.Source
		got := src.Clone()
		require.Nil(t, got)
	})

	t.Run("full_source", func(t *testing.T) {
		src := &source.Source{
			Handle:   "@sakila",
			Type:     drivertype.Pg,
			Location: "postgres://user:pass@localhost/sakila",
			Catalog:  "sakila",
			Schema:   "public",
			Options:  options.Options{"key1": "value1", "key2": 42},
		}

		got := src.Clone()
		require.NotNil(t, got)
		require.NotSame(t, src, got)
		require.Equal(t, src.Handle, got.Handle)
		require.Equal(t, src.Type, got.Type)
		require.Equal(t, src.Location, got.Location)
		require.Equal(t, src.Catalog, got.Catalog)
		require.Equal(t, src.Schema, got.Schema)

		// Verify Options is a deep copy
		require.Equal(t, src.Options, got.Options)
		got.Options["new_key"] = "new_value"
		require.NotEqual(t, src.Options, got.Options)
	})

	t.Run("nil_options", func(t *testing.T) {
		src := &source.Source{
			Handle:   "@test",
			Type:     drivertype.SQLite,
			Location: "/tmp/test.db",
		}

		got := src.Clone()
		require.NotNil(t, got)
		require.Nil(t, got.Options)
	})

	t.Run("empty_options", func(t *testing.T) {
		src := &source.Source{
			Handle:   "@test",
			Type:     drivertype.SQLite,
			Location: "/tmp/test.db",
			Options:  options.Options{},
		}

		got := src.Clone()
		require.NotNil(t, got)
		require.NotNil(t, got.Options)
		require.Empty(t, got.Options)
	})
}

func TestSource_String(t *testing.T) {
	src := &source.Source{
		Handle:   "@sakila",
		Type:     drivertype.Pg,
		Location: "postgres://user:p_ssW0rd@localhost/sakila",
	}

	got := src.String()
	require.NotEmpty(t, got)
	require.Contains(t, got, "@sakila")
	require.Contains(t, got, "postgres")
	// Password should be redacted
	require.NotContains(t, got, "p_ssW0rd")
	require.Contains(t, got, "xxxxx")
}

func TestSource_ShortLocation(t *testing.T) {
	t.Run("nil_source", func(t *testing.T) {
		var src *source.Source
		got := src.ShortLocation()
		require.Empty(t, got)
	})

	t.Run("postgres", func(t *testing.T) {
		src := &source.Source{
			Location: "postgres://user:pass@localhost/sakila",
		}
		got := src.ShortLocation()
		require.Equal(t, "user@localhost/sakila", got)
	})

	t.Run("file_path", func(t *testing.T) {
		src := &source.Source{
			Location: "/path/to/data.xlsx",
		}
		got := src.ShortLocation()
		require.Equal(t, "data.xlsx", got)
	})

	// Placeholder Locations must NOT be filepath-shortened: ${file:...}
	// looks like a path with trailing '}' to filepath.Base and produces
	// nonsense like "pg.dsn}". Bare-placeholder Locations are returned
	// verbatim — they're already as short as they meaningfully get.
	t.Run("placeholder_file_scheme", func(t *testing.T) {
		const loc = "${file:/Users/me/work/pg.dsn}"
		src := &source.Source{Location: loc}
		require.Equal(t, loc, src.ShortLocation(),
			"file: placeholder must not be sliced by filepath.Base")
	})
	t.Run("placeholder_keyring_scheme", func(t *testing.T) {
		const loc = "${keyring:j2k7m3pxtz}"
		src := &source.Source{Location: loc}
		require.Equal(t, loc, src.ShortLocation())
	})
	t.Run("placeholder_keyring_with_slash_in_body", func(t *testing.T) {
		// Hand-crafted path with embedded '/' (shared namespacing,
		// e.g. team/env/role). The '/' in the body must not trip
		// filepath.Base.
		const loc = "${keyring:acme/prod/db_pw}"
		src := &source.Source{Location: loc}
		require.Equal(t, loc, src.ShortLocation())
	})
	t.Run("placeholder_env_scheme", func(t *testing.T) {
		const loc = "${env:SAKILA_DSN}"
		src := &source.Source{Location: loc}
		require.Equal(t, loc, src.ShortLocation())
	})
}

func TestSource_Group(t *testing.T) {
	testCases := []struct {
		handle string
		want   string
	}{
		{handle: "@sakila", want: ""},
		{handle: "@prod/sakila", want: "prod"},
		{handle: "@prod/sub/sakila", want: "prod/sub"},
		{handle: "@a/b/c/d/sakila", want: "a/b/c/d"},
	}

	for _, tc := range testCases {
		t.Run(tc.handle, func(t *testing.T) {
			src := &source.Source{Handle: tc.handle}
			got := src.Group()
			require.Equal(t, tc.want, got)
		})
	}
}

func TestSource_RedactedLocation_nil(t *testing.T) {
	var src *source.Source
	got := src.RedactedLocation()
	require.Empty(t, got)
}

func TestSource_RedactedLocation_Placeholders(t *testing.T) {
	tests := []struct {
		name, loc, want string
	}{
		{
			// Placeholder in password position is masked like any password
			// would be. The placeholder text is lost in the redacted form
			// (use `sq config keyring ls` to enumerate references).
			name: "password placeholder masked",
			loc:  "postgres://alice:${keyring:my_db_pw}@db/sakila",
			want: "postgres://alice:xxxxx@db/sakila",
		},
		{
			// Placeholder in HOST: inline password must still be masked.
			// This was the regression Copilot caught — the old code
			// short-circuited on any "${" and leaked the inline password.
			name: "placeholder in host, inline password masked",
			loc:  "postgres://alice:hunter2@${env:DB_HOST}/sakila",
			want: "postgres://alice:xxxxx@${env:DB_HOST}/sakila",
		},
		{
			// Placeholder in USERNAME: inline password still masked.
			name: "placeholder in username, inline password masked",
			loc:  "postgres://${env:USR}:hunter2@db/sakila",
			want: "postgres://${env:USR}:xxxxx@db/sakila",
		},
		{
			// Placeholder in PORT — port must be all-digits per RFC 3986,
			// so the sentinel for that position must also be digit-only.
			name: "placeholder in port, inline password masked",
			loc:  "postgres://alice:hunter2@db:${env:PORT}/sakila",
			want: "postgres://alice:xxxxx@db:${env:PORT}/sakila",
		},
		{
			// Multiple placeholders in different positions — sentinel
			// ordering must align with ExtractRefs order.
			name: "multiple placeholders across positions",
			loc:  "postgres://${env:USR}:hunter2@${env:HOST}/${env:DB}",
			want: "postgres://${env:USR}:xxxxx@${env:HOST}/${env:DB}",
		},
		{
			name: "whole-dsn placeholder",
			loc:  "${keyring:@prod/dsn}",
			want: "${keyring:@prod/dsn}",
		},
		{
			name: "non-url location with placeholder",
			loc:  "${keyring:@my/file_path}",
			want: "${keyring:@my/file_path}",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			src := &source.Source{Handle: "@h", Type: "postgres", Location: tc.loc}
			require.Equal(t, tc.want, src.RedactedLocation())
		})
	}
}

// TestSource_RedactedLocation_BareDollarBraceFallsThrough verifies that a
// bare "${" substring without a well-formed placeholder does NOT short-
// circuit through the placeholder-aware path; it falls through to
// location.Redact via the standard branch.
func TestSource_RedactedLocation_BareDollarBraceFallsThrough(t *testing.T) {
	// Unclosed ${ — ExtractRefs returns an error; RedactedLocation must
	// fall through to location.Redact (not return the location verbatim).
	// We construct a URL that location.Redact CAN handle (so we get an
	// observable masking), with the malformed ${ in the query string:
	loc := "postgres://alice:hunter2@db/sakila?note=open${brace"
	src := &source.Source{Handle: "@h", Type: "postgres", Location: loc}
	got := src.RedactedLocation()
	require.NotContains(t, got, "hunter2",
		"inline password must be masked even when ${ substring is malformed")
	require.Contains(t, got, "${brace",
		"the malformed ${ should be passed through unchanged by location.Redact")
}

func TestTarget(t *testing.T) {
	t.Run("nil_source", func(t *testing.T) {
		got := source.Target(nil, "actor")
		require.Empty(t, got)
	})

	t.Run("with_source", func(t *testing.T) {
		src := &source.Source{Handle: "@sakila"}
		got := source.Target(src, "actor")
		require.Equal(t, "@sakila.actor", got)
	})
}

func TestRedactSources(t *testing.T) {
	t.Run("empty_slice", func(t *testing.T) {
		got := source.RedactSources()
		require.Empty(t, got)
	})

	t.Run("nil_elements", func(t *testing.T) {
		got := source.RedactSources(nil, nil)
		require.Len(t, got, 2)
		require.Nil(t, got[0])
		require.Nil(t, got[1])
	})

	t.Run("mixed", func(t *testing.T) {
		src1 := &source.Source{
			Handle:   "@db1",
			Location: "postgres://user:secret@localhost/db1",
		}
		src2 := &source.Source{
			Handle:   "@db2",
			Location: "/path/to/file.db",
		}

		got := source.RedactSources(src1, nil, src2)
		require.Len(t, got, 3)

		// Verify src1 is cloned and redacted
		require.NotSame(t, src1, got[0])
		require.Equal(t, src1.Handle, got[0].Handle)
		require.Contains(t, got[0].Location, "xxxxx")
		require.NotContains(t, got[0].Location, "secret")

		// Verify nil is preserved
		require.Nil(t, got[1])

		// Verify src2 is cloned (no password to redact)
		require.NotSame(t, src2, got[2])
		require.Equal(t, src2.Location, got[2].Location)

		// Verify original sources are not modified
		require.Contains(t, src1.Location, "secret")
	})
}

func TestReservedHandles(t *testing.T) {
	handles := source.ReservedHandles()
	require.NotEmpty(t, handles)

	// Verify known reserved handles are present
	require.Contains(t, handles, source.StdinHandle)
	require.Contains(t, handles, source.ActiveHandle)
	require.Contains(t, handles, source.ScratchHandle)
	require.Contains(t, handles, source.JoinHandle)
}

func TestCollection_Clone(t *testing.T) {
	t.Run("empty_collection", func(t *testing.T) {
		coll := &source.Collection{}
		clone := coll.Clone()
		require.NotNil(t, clone)
		require.Empty(t, clone.Sources())
	})

	t.Run("with_sources", func(t *testing.T) {
		coll := &source.Collection{}
		src1 := newSource("@src1")
		src2 := newSource("@prod/src2")
		require.NoError(t, coll.Add(src1))
		require.NoError(t, coll.Add(src2))

		_, err := coll.SetActive(src1.Handle, false)
		require.NoError(t, err)

		clone := coll.Clone()
		require.NotNil(t, clone)
		require.NotSame(t, coll, clone)

		// Verify sources are cloned
		cloneSrcs := clone.Sources()
		require.Len(t, cloneSrcs, 2)

		// Verify active handle is preserved
		require.Equal(t, coll.ActiveHandle(), clone.ActiveHandle())

		// Verify sources are deep copies
		cloneSrc1, err := clone.Get("@src1")
		require.NoError(t, err)
		require.NotSame(t, src1, cloneSrc1)
		require.Equal(t, src1.Handle, cloneSrc1.Handle)
		require.Equal(t, src1.Location, cloneSrc1.Location)

		// Verify modifying clone doesn't affect original
		require.NoError(t, clone.Remove("@src1"))
		_, err = coll.Get("@src1")
		require.NoError(t, err, "original should still have @src1")
	})
}

func TestCollection_Get(t *testing.T) {
	coll := &source.Collection{}
	src := newSource("@sakila")
	require.NoError(t, coll.Add(src))

	t.Run("existing", func(t *testing.T) {
		got, err := coll.Get("@sakila")
		require.NoError(t, err)
		require.Equal(t, src, got)
	})

	t.Run("not_found", func(t *testing.T) {
		got, err := coll.Get("@nonexistent")
		require.Error(t, err)
		require.Nil(t, got)
	})

	t.Run("invalid_handle", func(t *testing.T) {
		got, err := coll.Get("invalid")
		require.Error(t, err)
		require.Nil(t, got)
	})
}

func TestCollection_Scratch(t *testing.T) {
	coll := &source.Collection{}

	// Initially no scratch source
	require.Nil(t, coll.Scratch())

	// Add a source and set it as scratch
	src := newSource("@scratch_db")
	require.NoError(t, coll.Add(src))

	got, err := coll.SetScratch(src.Handle)
	require.NoError(t, err)
	require.Equal(t, src, got)
	require.Equal(t, src, coll.Scratch())

	// Setting non-existent source as scratch should fail
	_, err = coll.SetScratch("@nonexistent")
	require.Error(t, err)
}

func TestCollection_Remove(t *testing.T) {
	coll := &source.Collection{}
	src := newSource("@sakila")
	require.NoError(t, coll.Add(src))

	require.True(t, coll.IsExistingSource("@sakila"))
	require.NoError(t, coll.Remove("@sakila"))
	require.False(t, coll.IsExistingSource("@sakila"))

	// Removing non-existent source should error
	require.Error(t, coll.Remove("@nonexistent"))
}

func TestCollection_Handles(t *testing.T) {
	coll := &source.Collection{}

	// Empty collection
	require.Empty(t, coll.Handles())

	// Add sources
	require.NoError(t, coll.Add(newSource("@src1")))
	require.NoError(t, coll.Add(newSource("@src2")))
	require.NoError(t, coll.Add(newSource("@prod/src3")))

	handles := coll.Handles()
	require.Len(t, handles, 3)
	require.Contains(t, handles, "@src1")
	require.Contains(t, handles, "@src2")
	require.Contains(t, handles, "@prod/src3")
}

func TestCollection_HandlesInGroup(t *testing.T) {
	coll := &source.Collection{}
	require.NoError(t, coll.Add(newSource("@src1")))
	require.NoError(t, coll.Add(newSource("@prod/src2")))
	require.NoError(t, coll.Add(newSource("@prod/src3")))
	require.NoError(t, coll.Add(newSource("@prod/sub/src4")))

	t.Run("root_group", func(t *testing.T) {
		// Root group returns ALL handles
		handles, err := coll.HandlesInGroup(source.RootGroup)
		require.NoError(t, err)
		require.Len(t, handles, 4)
	})

	t.Run("prod_group", func(t *testing.T) {
		// prod group includes subgroups
		handles, err := coll.HandlesInGroup("prod")
		require.NoError(t, err)
		require.Len(t, handles, 3)
		require.Contains(t, handles, "@prod/src2")
		require.Contains(t, handles, "@prod/src3")
		require.Contains(t, handles, "@prod/sub/src4")
	})

	t.Run("prod_sub_group", func(t *testing.T) {
		handles, err := coll.HandlesInGroup("prod/sub")
		require.NoError(t, err)
		require.Len(t, handles, 1)
		require.Contains(t, handles, "@prod/sub/src4")
	})

	t.Run("nonexistent_group", func(t *testing.T) {
		handles, err := coll.HandlesInGroup("nonexistent")
		require.Error(t, err)
		require.Empty(t, handles)
	})
}

func TestCollection_Visit(t *testing.T) {
	coll := &source.Collection{}
	require.NoError(t, coll.Add(newSource("@src1")))
	require.NoError(t, coll.Add(newSource("@src2")))
	require.NoError(t, coll.Add(newSource("@src3")))

	var visited []string
	err := coll.Visit(func(src *source.Source) error {
		visited = append(visited, src.Handle)
		return nil
	})
	require.NoError(t, err)
	require.Len(t, visited, 3)

	// Test early termination on error
	count := 0
	err = coll.Visit(func(_ *source.Source) error {
		count++
		if count == 2 {
			return errors.New("stop")
		}
		return nil
	})
	require.Error(t, err)
	require.Equal(t, 2, count)
}

func TestCollection_Sources(t *testing.T) {
	coll := &source.Collection{}

	// Empty collection
	require.Empty(t, coll.Sources())

	// With sources
	src1 := newSource("@src1")
	src2 := newSource("@src2")
	require.NoError(t, coll.Add(src1))
	require.NoError(t, coll.Add(src2))

	srcs := coll.Sources()
	require.Len(t, srcs, 2)
}

func TestCollection_String(t *testing.T) {
	coll := &source.Collection{}
	require.NoError(t, coll.Add(newSource("@sakila")))

	s := coll.String()
	require.NotEmpty(t, s)
}

func TestCollection_MarshalJSON(t *testing.T) {
	coll := &source.Collection{}

	// Empty collection
	data, err := json.Marshal(coll)
	require.NoError(t, err)
	require.NotNil(t, data)

	// With sources
	src1 := newSource("@src1")
	src2 := newSource("@prod/src2")
	require.NoError(t, coll.Add(src1))
	require.NoError(t, coll.Add(src2))
	_, err = coll.SetActive(src1.Handle, false)
	require.NoError(t, err)
	_, err = coll.SetScratch("@src1")
	require.NoError(t, err)

	data, err = json.Marshal(coll)
	require.NoError(t, err)
	require.Contains(t, string(data), "@src1")
	require.Contains(t, string(data), "@prod/src2")
	require.Contains(t, string(data), `"active.source":"@src1"`)
	require.Contains(t, string(data), `"scratch":"@src1"`)
}

func TestCollection_UnmarshalJSON(t *testing.T) {
	// Create and marshal a collection
	original := &source.Collection{}
	src1 := newSource("@src1")
	src2 := newSource("@prod/src2")
	require.NoError(t, original.Add(src1))
	require.NoError(t, original.Add(src2))
	_, err := original.SetActive(src1.Handle, false)
	require.NoError(t, err)
	_, err = original.SetScratch("@src1")
	require.NoError(t, err)

	data, err := json.Marshal(original)
	require.NoError(t, err)

	// Unmarshal into new collection
	restored := &source.Collection{}
	err = json.Unmarshal(data, restored)
	require.NoError(t, err)

	// Verify restored collection matches original
	require.Equal(t, len(original.Sources()), len(restored.Sources()))
	activeSrc := restored.Active()
	require.NotNil(t, activeSrc)
	require.Equal(t, "@src1", activeSrc.Handle)
	scratch := restored.Scratch()
	require.NotNil(t, scratch)
	require.Equal(t, "@src1", scratch.Handle)

	// Verify sources were restored
	restoredSrc1, err := restored.Get("@src1")
	require.NoError(t, err)
	require.Equal(t, src1.Handle, restoredSrc1.Handle)
	require.Equal(t, src1.Type, restoredSrc1.Type)
	require.Equal(t, src1.Location, restoredSrc1.Location)
}

func TestCollection_MarshalYAML(t *testing.T) {
	coll := &source.Collection{}

	// Empty collection
	data, err := yaml.Marshal(coll)
	require.NoError(t, err)
	require.NotNil(t, data)

	// With sources
	src1 := newSource("@src1")
	src2 := newSource("@prod/src2")
	require.NoError(t, coll.Add(src1))
	require.NoError(t, coll.Add(src2))
	_, err = coll.SetActive(src1.Handle, false)
	require.NoError(t, err)

	data, err = yaml.Marshal(coll)
	require.NoError(t, err)
	require.Contains(t, string(data), "@src1")
	require.Contains(t, string(data), "@prod/src2")
	require.Contains(t, string(data), "active.source")
}

func TestCollection_UnmarshalYAML(t *testing.T) {
	// Create and marshal a collection
	original := &source.Collection{}
	src1 := newSource("@src1")
	src2 := newSource("@prod/src2")
	require.NoError(t, original.Add(src1))
	require.NoError(t, original.Add(src2))
	_, err := original.SetActive(src1.Handle, false)
	require.NoError(t, err)

	data, err := yaml.Marshal(original)
	require.NoError(t, err)

	// Unmarshal into new collection
	restored := &source.Collection{}
	err = yaml.Unmarshal(data, restored)
	require.NoError(t, err)

	// Verify restored collection matches original
	require.Equal(t, len(original.Sources()), len(restored.Sources()))
	activeSrc := restored.Active()
	require.NotNil(t, activeSrc)
	require.Equal(t, "@src1", activeSrc.Handle)

	// Verify sources were restored
	restoredSrc1, err := restored.Get("@src1")
	require.NoError(t, err)
	require.Equal(t, src1.Handle, restoredSrc1.Handle)
	require.Equal(t, src1.Type, restoredSrc1.Type)
}

func TestGroup_String(t *testing.T) {
	g := &source.Group{Name: "prod"}
	require.Equal(t, "prod", g.String())

	g = &source.Group{Name: source.RootGroup}
	require.Equal(t, source.RootGroup, g.String())
}

func TestGroup_Counts(t *testing.T) {
	// nil group
	var g *source.Group
	directSrc, totalSrc, directGroup, totalGroup := g.Counts()
	require.Equal(t, 0, directSrc)
	require.Equal(t, 0, totalSrc)
	require.Equal(t, 0, directGroup)
	require.Equal(t, 0, totalGroup)

	// Group with sources and subgroups
	g = &source.Group{
		Name:    "root",
		Sources: []*source.Source{newSource("@src1"), newSource("@src2")},
		Groups: []*source.Group{
			{
				Name:    "sub1",
				Sources: []*source.Source{newSource("@sub1/src1")},
				Groups: []*source.Group{
					{
						Name:    "sub1/nested",
						Sources: []*source.Source{newSource("@sub1/nested/src1")},
					},
				},
			},
			{
				Name:    "sub2",
				Sources: []*source.Source{newSource("@sub2/src1"), newSource("@sub2/src2")},
			},
		},
	}

	directSrc, totalSrc, directGroup, totalGroup = g.Counts()
	require.Equal(t, 2, directSrc)   // @src1, @src2
	require.Equal(t, 6, totalSrc)    // all sources
	require.Equal(t, 2, directGroup) // sub1, sub2
	require.Equal(t, 3, totalGroup)  // sub1, sub1/nested, sub2
}

func TestGroup_AllSources(t *testing.T) {
	// nil group
	var g *source.Group
	require.Empty(t, g.AllSources())

	// Group with nested sources
	g = &source.Group{
		Name:    "root",
		Sources: []*source.Source{newSource("@src1")},
		Groups: []*source.Group{
			{
				Name:    "sub1",
				Sources: []*source.Source{newSource("@sub1/src1")},
			},
		},
	}

	srcs := g.AllSources()
	require.Len(t, srcs, 2)
}

func TestGroup_AllGroups(t *testing.T) {
	// nil group
	var g *source.Group
	require.Empty(t, g.AllGroups())

	// Group with nested groups
	g = &source.Group{
		Name: "root",
		Groups: []*source.Group{
			{
				Name: "sub1",
				Groups: []*source.Group{
					{Name: "sub1/nested"},
				},
			},
			{Name: "sub2"},
		},
	}

	groups := g.AllGroups()
	require.Len(t, groups, 4) // root, sub1, sub1/nested, sub2
}

func TestRedactGroup(t *testing.T) {
	// nil group - should not panic
	source.RedactGroup(nil)

	// Group with sources containing passwords
	src := &source.Source{
		Handle:   "@db",
		Type:     drivertype.Pg,
		Location: "postgres://user:secret_password@localhost/db",
	}

	g := &source.Group{
		Name:    "prod",
		Sources: []*source.Source{src},
		Groups: []*source.Group{
			{
				Name: "prod/sub",
				Sources: []*source.Source{
					{
						Handle:   "@db2",
						Type:     drivertype.MySQL,
						Location: "mysql://user:another_secret@localhost/db",
					},
				},
			},
		},
	}

	// Verify original location has password
	require.Contains(t, g.Sources[0].Location, "secret_password")
	require.Contains(t, g.Groups[0].Sources[0].Location, "another_secret")

	source.RedactGroup(g)

	// Verify locations are redacted
	require.NotContains(t, g.Sources[0].Location, "secret_password")
	require.NotContains(t, g.Groups[0].Sources[0].Location, "another_secret")
}

func TestVerifyIntegrity(t *testing.T) {
	// nil collection
	repaired, err := source.VerifyIntegrity(nil)
	require.Error(t, err)
	require.False(t, repaired)

	// Valid collection
	coll := &source.Collection{}
	require.NoError(t, coll.Add(newSource("@src1")))
	repaired, err = source.VerifyIntegrity(coll)
	require.NoError(t, err)
	require.False(t, repaired)

	// Collection with duplicate handles would fail at Add time,
	// so we test via the integrity check through active source
	coll = &source.Collection{}
	require.NoError(t, coll.Add(newSource("@src1")))
	_, err = coll.SetActive("@src1", false)
	require.NoError(t, err)

	repaired, err = source.VerifyIntegrity(coll)
	require.NoError(t, err)
	require.False(t, repaired)
}

func TestSort(t *testing.T) {
	srcs := []*source.Source{
		newSource("@z_last"),
		newSource("@a_first"),
		newSource("@m_middle"),
	}

	source.Sort(srcs)
	require.Equal(t, "@a_first", srcs[0].Handle)
	require.Equal(t, "@m_middle", srcs[1].Handle)
	require.Equal(t, "@z_last", srcs[2].Handle)

	// With nils
	srcs = []*source.Source{
		newSource("@b"),
		nil,
		newSource("@a"),
	}
	source.Sort(srcs)
	require.Nil(t, srcs[0])
	require.Equal(t, "@a", srcs[1].Handle)
	require.Equal(t, "@b", srcs[2].Handle)
}

func TestSortGroups(t *testing.T) {
	groups := []*source.Group{
		{Name: "z_last"},
		{Name: "a_first"},
		{Name: "m_middle"},
	}

	source.SortGroups(groups)
	require.Equal(t, "a_first", groups[0].Name)
	require.Equal(t, "m_middle", groups[1].Name)
	require.Equal(t, "z_last", groups[2].Name)

	// With nils
	groups = []*source.Group{
		{Name: "b"},
		nil,
		{Name: "a"},
	}
	source.SortGroups(groups)
	require.Nil(t, groups[0])
	require.Equal(t, "a", groups[1].Name)
	require.Equal(t, "b", groups[2].Name)
}

// addSrc is a helper that adds a source with the given handle to coll,
// failing the test on error.
func addSrc(t *testing.T, coll *source.Collection, handle string) {
	t.Helper()
	require.NoError(t, coll.Add(newSource(handle)))
}

// TestCollection_Data exercises Collection.Data, including the nil
// receiver guard.
func TestCollection_Data(t *testing.T) {
	var nilColl *source.Collection
	require.Nil(t, nilColl.Data())

	coll := &source.Collection{}
	addSrc(t, coll, "@src1")
	require.NotNil(t, coll.Data())
}

// TestCollection_Add_errPaths covers the error branches of Add.
func TestCollection_Add_errPaths(t *testing.T) {
	coll := &source.Collection{}

	// Invalid handle.
	require.Error(t, coll.Add(&source.Source{Handle: "no-at-prefix"}))

	// Valid add.
	addSrc(t, coll, "@src1")

	// Duplicate handle conflict.
	err := coll.Add(newSource("@src1"))
	require.Error(t, err)
	require.Contains(t, err.Error(), "already exists")
}

// TestCollection_renameSource_errPaths covers error branches of
// renameSource via RenameSource.
func TestCollection_renameSource_errPaths(t *testing.T) {
	t.Run("invalid_new_handle", func(t *testing.T) {
		coll := &source.Collection{}
		addSrc(t, coll, "@src1")
		_, err := coll.RenameSource("@src1", "bad-handle")
		require.Error(t, err)
	})

	t.Run("old_handle_not_found", func(t *testing.T) {
		coll := &source.Collection{}
		addSrc(t, coll, "@src1")
		_, err := coll.RenameSource("@nope", "@src2")
		require.Error(t, err)
	})

	t.Run("noop_same_handle", func(t *testing.T) {
		coll := &source.Collection{}
		addSrc(t, coll, "@src1")
		got, err := coll.RenameSource("@src1", "@src1")
		require.NoError(t, err)
		require.Equal(t, "@src1", got.Handle)
	})

	t.Run("new_handle_exists", func(t *testing.T) {
		coll := &source.Collection{}
		addSrc(t, coll, "@src1")
		addSrc(t, coll, "@src2")
		_, err := coll.RenameSource("@src1", "@src2")
		require.Error(t, err)
		require.Contains(t, err.Error(), "already exists")
	})

	t.Run("new_handle_conflicts_with_group", func(t *testing.T) {
		coll := &source.Collection{}
		addSrc(t, coll, "@prod/db")
		addSrc(t, coll, "@src1")
		// @prod is an existing group; renaming @src1 to @prod must fail.
		_, err := coll.RenameSource("@src1", "@prod")
		require.Error(t, err)
		require.Contains(t, err.Error(), "existing group")
	})

	t.Run("new_handle_nests_under_existing_handle", func(t *testing.T) {
		coll := &source.Collection{}
		addSrc(t, coll, "@prod")
		addSrc(t, coll, "@src1")
		// @prod is an existing handle; @prod/db nests below it.
		_, err := coll.RenameSource("@src1", "@prod/db")
		require.Error(t, err)
		require.Contains(t, err.Error(), "existing handle")
	})

	t.Run("active_src_follows_rename", func(t *testing.T) {
		coll := &source.Collection{}
		addSrc(t, coll, "@src1")
		_, err := coll.SetActive("@src1", false)
		require.NoError(t, err)

		_, err = coll.RenameSource("@src1", "@src2")
		require.NoError(t, err)
		require.Equal(t, "@src2", coll.ActiveHandle())
	})

	t.Run("active_group_reassigned_when_emptied", func(t *testing.T) {
		coll := &source.Collection{}
		addSrc(t, coll, "@prod/db")
		require.NoError(t, coll.SetActiveGroup("prod"))

		// Move the only source out of prod; prod no longer exists, so
		// the active group must be reassigned to the source's new group.
		_, err := coll.RenameSource("@prod/db", "@staging/db")
		require.NoError(t, err)
		require.Equal(t, "staging", coll.ActiveGroup())
	})
}

// TestCollection_RenameGroup_errPaths covers RenameGroup error and edge
// branches.
func TestCollection_RenameGroup_errPaths(t *testing.T) {
	t.Run("cannot_rename_root", func(t *testing.T) {
		coll := &source.Collection{}
		_, err := coll.RenameGroup("/", "prod")
		require.Error(t, err)
		_, err = coll.RenameGroup("", "prod")
		require.Error(t, err)
	})

	t.Run("invalid_old_group", func(t *testing.T) {
		coll := &source.Collection{}
		_, err := coll.RenameGroup("@bad", "prod")
		require.Error(t, err)
	})

	t.Run("invalid_new_group", func(t *testing.T) {
		coll := &source.Collection{}
		addSrc(t, coll, "@prod/db")
		_, err := coll.RenameGroup("prod", "@bad")
		require.Error(t, err)
	})

	t.Run("old_group_not_exist", func(t *testing.T) {
		coll := &source.Collection{}
		addSrc(t, coll, "@src1")
		_, err := coll.RenameGroup("nope", "prod")
		require.Error(t, err)
	})

	t.Run("new_group_nests_under_handle", func(t *testing.T) {
		coll := &source.Collection{}
		addSrc(t, coll, "@prod")
		addSrc(t, coll, "@staging/db")
		_, err := coll.RenameGroup("staging", "prod/sub")
		require.Error(t, err)
		require.Contains(t, err.Error(), "existing handle")
	})

	t.Run("rename_collides_with_existing_target", func(t *testing.T) {
		// Renaming group "a" to "b" tries to rename @a/x -> @b/x, but
		// @b/x already exists, so renameSource fails inside the loop.
		coll := &source.Collection{}
		addSrc(t, coll, "@a/x")
		addSrc(t, coll, "@b/x")
		_, err := coll.RenameGroup("a", "b")
		require.Error(t, err)
	})

	t.Run("rename_to_root_via_slash", func(t *testing.T) {
		coll := &source.Collection{}
		addSrc(t, coll, "@prod/db")
		require.NoError(t, coll.SetActiveGroup("prod"))

		srcs, err := coll.RenameGroup("prod", "/")
		require.NoError(t, err)
		require.Len(t, srcs, 1)
		require.Equal(t, "@db", srcs[0].Handle)
		require.Equal(t, "/", coll.ActiveGroup())
	})

	t.Run("rename_to_root_preserves_nested_subgroups", func(t *testing.T) {
		// Renaming "prod" to root must drop only the "prod" segment, not
		// flatten nested subgroups: @prod/sub/db2 -> @sub/db2, not @db2.
		coll := &source.Collection{}
		addSrc(t, coll, "@prod/db")
		addSrc(t, coll, "@prod/sub/db2")

		srcs, err := coll.RenameGroup("prod", "/")
		require.NoError(t, err)
		got := make([]string, len(srcs))
		for i, s := range srcs {
			got[i] = s.Handle
		}
		require.ElementsMatch(t, []string{"@db", "@sub/db2"}, got)
	})

	t.Run("active_group_follows_rename", func(t *testing.T) {
		// Active group "prod" holds no direct source members, only a
		// subgroup. The inner renameSource never reassigns the active
		// group (no src's own group equals "prod"), so RenameGroup's
		// own active-group update handles it.
		coll := &source.Collection{}
		addSrc(t, coll, "@prod/sub/db")
		require.NoError(t, coll.SetActiveGroup("prod"))

		srcs, err := coll.RenameGroup("prod", "production")
		require.NoError(t, err)
		require.Len(t, srcs, 1)
		require.Equal(t, "production", coll.ActiveGroup())
	})
}

// TestCollection_MoveHandleToGroup_errPaths covers MoveHandleToGroup
// error and switch branches.
func TestCollection_MoveHandleToGroup_errPaths(t *testing.T) {
	t.Run("handle_not_found", func(t *testing.T) {
		coll := &source.Collection{}
		_, err := coll.MoveHandleToGroup("@nope", "prod")
		require.Error(t, err)
	})

	t.Run("invalid_group", func(t *testing.T) {
		coll := &source.Collection{}
		addSrc(t, coll, "@src1")
		_, err := coll.MoveHandleToGroup("@src1", "@bad")
		require.Error(t, err)
	})

	t.Run("group_nests_under_handle", func(t *testing.T) {
		coll := &source.Collection{}
		addSrc(t, coll, "@prod")
		addSrc(t, coll, "@src1")
		_, err := coll.MoveHandleToGroup("@src1", "prod/sub")
		require.Error(t, err)
	})

	t.Run("move_root_src_to_group", func(t *testing.T) {
		// oldGroup == "" branch.
		coll := &source.Collection{}
		addSrc(t, coll, "@db")
		got, err := coll.MoveHandleToGroup("@db", "prod")
		require.NoError(t, err)
		require.Equal(t, "@prod/db", got.Handle)
	})

	t.Run("move_grouped_src_to_root", func(t *testing.T) {
		// toGroup == "/" branch.
		coll := &source.Collection{}
		addSrc(t, coll, "@prod/db")
		got, err := coll.MoveHandleToGroup("@prod/db", "/")
		require.NoError(t, err)
		require.Equal(t, "@db", got.Handle)
	})

	t.Run("move_grouped_src_to_other_group", func(t *testing.T) {
		// default switch branch.
		coll := &source.Collection{}
		addSrc(t, coll, "@prod/db")
		got, err := coll.MoveHandleToGroup("@prod/db", "staging")
		require.NoError(t, err)
		require.Equal(t, "@staging/db", got.Handle)
	})
}

// TestCollection_ActiveHandle_empty covers the no-active-source branch.
func TestCollection_ActiveHandle_empty(t *testing.T) {
	coll := &source.Collection{}
	require.Empty(t, coll.ActiveHandle())

	addSrc(t, coll, "@src1")
	_, err := coll.SetActive("@src1", false)
	require.NoError(t, err)
	require.Equal(t, "@src1", coll.ActiveHandle())
}

// TestCollection_active_staleActiveSrc covers active() when ActiveSrc
// points at a non-existent handle.
func TestCollection_active_staleActiveSrc(t *testing.T) {
	coll := &source.Collection{}
	addSrc(t, coll, "@src1")
	// Force-set an active src that doesn't exist.
	_, err := coll.SetActive("@ghost", true)
	require.NoError(t, err)

	// active() should return nil because @ghost isn't in sources.
	require.Nil(t, coll.Active())
}

// TestCollection_Scratch_branches covers Scratch including the
// stale-handle branch.
func TestCollection_Scratch_branches(t *testing.T) {
	coll := &source.Collection{}
	// No scratch set.
	require.Nil(t, coll.Scratch())

	addSrc(t, coll, "@src1")
	_, err := coll.SetScratch("@src1")
	require.NoError(t, err)
	require.NotNil(t, coll.Scratch())

	// Remove the source; scratch handle becomes stale.
	require.NoError(t, coll.Remove("@src1"))
	require.Nil(t, coll.Scratch())

	// A scratch handle pointing at a non-existent source (e.g. from a
	// hand-edited or stale config) yields nil.
	stale := &source.Collection{}
	err = stale.UnmarshalJSON([]byte(`{"scratch":"@ghost","sources":[` +
		`{"handle":"@real","driver":"sqlite3","location":"/tmp/a.db"}]}`))
	require.NoError(t, err)
	require.Nil(t, stale.Scratch())
}

// TestCollection_get_branches covers get's special cases.
func TestCollection_get_branches(t *testing.T) {
	coll := &source.Collection{}
	addSrc(t, coll, "@src1")

	t.Run("empty_handle", func(t *testing.T) {
		_, err := coll.Get("   ")
		require.Error(t, err)
	})

	t.Run("missing_at_prefix_added", func(t *testing.T) {
		got, err := coll.Get("src1")
		require.NoError(t, err)
		require.Equal(t, "@src1", got.Handle)
	})

	t.Run("active_handle_no_active", func(t *testing.T) {
		_, err := coll.Get(source.ActiveHandle)
		require.Error(t, err)
	})

	t.Run("active_handle_with_active", func(t *testing.T) {
		_, err := coll.SetActive("@src1", false)
		require.NoError(t, err)
		got, err := coll.Get(source.ActiveHandle)
		require.NoError(t, err)
		require.Equal(t, "@src1", got.Handle)
	})

	t.Run("unknown_handle", func(t *testing.T) {
		_, err := coll.Get("@nope")
		require.Error(t, err)
	})
}

// TestCollection_setActive_branches covers setActive paths.
func TestCollection_setActive_branches(t *testing.T) {
	t.Run("unset", func(t *testing.T) {
		coll := &source.Collection{}
		addSrc(t, coll, "@src1")
		_, err := coll.SetActive("@src1", false)
		require.NoError(t, err)

		got, err := coll.SetActive("", false)
		require.NoError(t, err)
		require.Nil(t, got)
		require.Empty(t, coll.ActiveHandle())
	})

	t.Run("invalid_handle", func(t *testing.T) {
		coll := &source.Collection{}
		_, err := coll.SetActive("bad-handle", false)
		require.Error(t, err)
	})

	t.Run("force_nonexistent", func(t *testing.T) {
		coll := &source.Collection{}
		got, err := coll.SetActive("@ghost", true)
		require.NoError(t, err)
		require.Nil(t, got)
	})

	t.Run("force_existent", func(t *testing.T) {
		coll := &source.Collection{}
		addSrc(t, coll, "@src1")
		got, err := coll.SetActive("@src1", true)
		require.NoError(t, err)
		require.NotNil(t, got)
	})

	t.Run("not_force_nonexistent", func(t *testing.T) {
		coll := &source.Collection{}
		_, err := coll.SetActive("@ghost", false)
		require.Error(t, err)
	})
}

// TestCollection_SetScratch_branches covers SetScratch paths.
func TestCollection_SetScratch_branches(t *testing.T) {
	coll := &source.Collection{}
	addSrc(t, coll, "@src1")

	// Unset.
	got, err := coll.SetScratch("")
	require.NoError(t, err)
	require.Nil(t, got)

	// Unknown.
	_, err = coll.SetScratch("@ghost")
	require.Error(t, err)

	// Valid.
	got, err = coll.SetScratch("@src1")
	require.NoError(t, err)
	require.NotNil(t, got)
}

// TestCollection_RemoveGroup_branches covers RemoveGroup paths.
func TestCollection_RemoveGroup_branches(t *testing.T) {
	t.Run("unknown_group", func(t *testing.T) {
		coll := &source.Collection{}
		addSrc(t, coll, "@src1")
		_, err := coll.RemoveGroup("nope")
		require.Error(t, err)
	})

	t.Run("removes_and_resets_active_group", func(t *testing.T) {
		coll := &source.Collection{}
		addSrc(t, coll, "@prod/db1")
		addSrc(t, coll, "@prod/db2")
		require.NoError(t, coll.SetActiveGroup("prod"))

		srcs, err := coll.RemoveGroup("prod")
		require.NoError(t, err)
		require.Len(t, srcs, 2)
		require.Equal(t, "/", coll.ActiveGroup())
	})

	t.Run("removes_keeps_active_group", func(t *testing.T) {
		coll := &source.Collection{}
		addSrc(t, coll, "@prod/db1")
		addSrc(t, coll, "@staging/db2")
		require.NoError(t, coll.SetActiveGroup("staging"))

		srcs, err := coll.RemoveGroup("prod")
		require.NoError(t, err)
		require.Len(t, srcs, 1)
		require.Equal(t, "staging", coll.ActiveGroup())
	})
}

// TestCollection_remove_branches covers remove paths.
func TestCollection_remove_branches(t *testing.T) {
	t.Run("empty_collection", func(t *testing.T) {
		coll := &source.Collection{}
		require.Error(t, coll.Remove("@nope"))
	})

	t.Run("unknown_handle", func(t *testing.T) {
		coll := &source.Collection{}
		addSrc(t, coll, "@src1")
		require.Error(t, coll.Remove("@nope"))
	})

	t.Run("remove_only_source", func(t *testing.T) {
		coll := &source.Collection{}
		addSrc(t, coll, "@src1")
		require.NoError(t, coll.Remove("@src1"))
		require.Empty(t, coll.Handles())
	})

	t.Run("remove_active_source", func(t *testing.T) {
		coll := &source.Collection{}
		addSrc(t, coll, "@src1")
		addSrc(t, coll, "@src2")
		_, err := coll.SetActive("@src1", false)
		require.NoError(t, err)
		require.NoError(t, coll.Remove("@src1"))
		require.Empty(t, coll.ActiveHandle())
	})

	t.Run("remove_resets_active_group", func(t *testing.T) {
		coll := &source.Collection{}
		addSrc(t, coll, "@prod/db")
		addSrc(t, coll, "@other")
		require.NoError(t, coll.SetActiveGroup("prod"))
		// Removing the only source in prod removes the group; active
		// group must reset to root.
		require.NoError(t, coll.Remove("@prod/db"))
		require.Equal(t, "/", coll.ActiveGroup())
	})

	t.Run("remove_middle_source", func(t *testing.T) {
		coll := &source.Collection{}
		addSrc(t, coll, "@a")
		addSrc(t, coll, "@b")
		addSrc(t, coll, "@c")
		require.NoError(t, coll.Remove("@b"))
		require.ElementsMatch(t, []string{"@a", "@c"}, coll.Handles())
	})
}

// TestCollection_handlesInGroup_branches covers handlesInGroup.
func TestCollection_handlesInGroup_branches(t *testing.T) {
	coll := &source.Collection{}
	addSrc(t, coll, "@prod/db1")
	addSrc(t, coll, "@prod/db2")
	addSrc(t, coll, "@other")

	// Root group returns all handles.
	all, err := coll.HandlesInGroup("/")
	require.NoError(t, err)
	require.Len(t, all, 3)

	// Specific group.
	got, err := coll.HandlesInGroup("prod")
	require.NoError(t, err)
	require.ElementsMatch(t, []string{"@prod/db1", "@prod/db2"}, got)

	// Unknown group.
	_, err = coll.HandlesInGroup("nope")
	require.Error(t, err)
}

// TestCollection_Clone_nil covers Clone's nil-receiver guard.
func TestCollection_Clone_nil(t *testing.T) {
	var coll *source.Collection
	require.Nil(t, coll.Clone())
}

// TestCollection_Tree_branches covers Tree's nil guard and error paths.
func TestCollection_Tree_branches(t *testing.T) {
	t.Run("nil_receiver", func(t *testing.T) {
		var coll *source.Collection
		got, err := coll.Tree("")
		require.NoError(t, err)
		require.Nil(t, got)
	})

	t.Run("default_fromGroup", func(t *testing.T) {
		coll := &source.Collection{}
		addSrc(t, coll, "@prod/db")
		got, err := coll.Tree("")
		require.NoError(t, err)
		require.NotNil(t, got)
		require.Equal(t, source.RootGroup, got.Name)
	})

	t.Run("invalid_group", func(t *testing.T) {
		coll := &source.Collection{}
		_, err := coll.Tree("@bad")
		require.Error(t, err)
	})

	t.Run("nested_tree", func(t *testing.T) {
		coll := &source.Collection{}
		addSrc(t, coll, "@prod/sub/db")
		addSrc(t, coll, "@prod/db2")
		got, err := coll.Tree("prod")
		require.NoError(t, err)
		require.Equal(t, "prod", got.Name)
		require.NotEmpty(t, got.Groups)
	})
}

// TestVerifyIntegrity_branches covers the harder VerifyIntegrity paths.
func TestVerifyIntegrity_branches(t *testing.T) {
	t.Run("invalid_source_in_collection", func(t *testing.T) {
		coll := &source.Collection{}
		// Add a valid source, then corrupt it via marshaling roundtrip
		// is awkward; instead build via JSON to bypass Add validation.
		err := coll.UnmarshalJSON([]byte(`{"sources":[{"handle":"@bad","driver":"","location":""}]}`))
		require.NoError(t, err)
		repaired, verr := source.VerifyIntegrity(coll)
		require.Error(t, verr)
		require.False(t, repaired)
	})

	t.Run("null_entry_stripped_at_load", func(t *testing.T) {
		coll := &source.Collection{}
		err := coll.UnmarshalJSON([]byte(`{"sources":[null]}`))
		require.NoError(t, err)
		// The null entry is dropped at unmarshal, so integrity passes.
		repaired, verr := source.VerifyIntegrity(coll)
		require.NoError(t, verr)
		require.False(t, repaired)
		require.Empty(t, coll.Handles())
	})

	t.Run("duplicate_handle", func(t *testing.T) {
		coll := &source.Collection{}
		err := coll.UnmarshalJSON([]byte(`{"sources":[` +
			`{"handle":"@dup","driver":"sqlite3","location":"/tmp/a.db"},` +
			`{"handle":"@dup","driver":"sqlite3","location":"/tmp/b.db"}]}`))
		require.NoError(t, err)
		repaired, verr := source.VerifyIntegrity(coll)
		require.Error(t, verr)
		require.False(t, repaired)
	})

	t.Run("stale_active_source_repaired", func(t *testing.T) {
		coll := &source.Collection{}
		err := coll.UnmarshalJSON([]byte(`{"active.source":"@ghost","sources":[` +
			`{"handle":"@real","driver":"sqlite3","location":"/tmp/a.db"}]}`))
		require.NoError(t, err)
		repaired, verr := source.VerifyIntegrity(coll)
		require.Error(t, verr)
		require.True(t, repaired)
		require.Empty(t, coll.ActiveHandle())
	})
}

// TestValidSource_viaVerifyIntegrity exercises validSource error
// branches (empty location, unknown driver type) through the public
// VerifyIntegrity path.
func TestValidSource_viaVerifyIntegrity(t *testing.T) {
	t.Run("empty_location", func(t *testing.T) {
		coll := &source.Collection{}
		err := coll.UnmarshalJSON([]byte(`{"sources":[{"handle":"@x","driver":"sqlite3","location":""}]}`))
		require.NoError(t, err)
		_, verr := source.VerifyIntegrity(coll)
		require.Error(t, verr)
		require.Contains(t, verr.Error(), "location is empty")
	})

	t.Run("unknown_driver", func(t *testing.T) {
		coll := &source.Collection{}
		err := coll.UnmarshalJSON([]byte(`{"sources":[{"handle":"@x","driver":"","location":"/tmp/x.db"}]}`))
		require.NoError(t, err)
		_, verr := source.VerifyIntegrity(coll)
		require.Error(t, verr)
		require.Contains(t, verr.Error(), "driver type")
	})
}

// TestSort_bothNil covers the a==nil && b==nil branch of Sort and
// SortGroups (multiple nil elements must be compared against each other).
func TestSort_bothNil(t *testing.T) {
	srcs := []*source.Source{nil, newSource("@b"), nil, newSource("@a"), nil}
	source.Sort(srcs)
	require.Nil(t, srcs[0])
	require.Nil(t, srcs[1])
	require.Nil(t, srcs[2])
	require.Equal(t, "@a", srcs[3].Handle)
	require.Equal(t, "@b", srcs[4].Handle)

	groups := []*source.Group{nil, {Name: "b"}, nil, {Name: "a"}, nil}
	source.SortGroups(groups)
	require.Nil(t, groups[0])
	require.Nil(t, groups[1])
	require.Nil(t, groups[2])
	require.Equal(t, "a", groups[3].Name)
	require.Equal(t, "b", groups[4].Name)
}

// TestCollection_Add_nil verifies Add rejects a nil source instead of
// panicking.
func TestCollection_Add_nil(t *testing.T) {
	coll := &source.Collection{}
	require.Error(t, coll.Add(nil))
}

// TestCollection_nullEntryStrippedOnLoad verifies that a null entry in a
// config's sources list is dropped at unmarshal, so the collection never
// carries a nil *Source through the public load path. The per-reader
// nil guards (defense in depth, for a nil that bypasses this boundary)
// are covered by the internal TestCollection_nilEntryDefenseInDepth.
func TestCollection_nullEntryStrippedOnLoad(t *testing.T) {
	coll := &source.Collection{}
	err := coll.UnmarshalJSON([]byte(`{"sources":[null,` +
		`{"handle":"@prod/real","driver":"sqlite3","location":"/tmp/a.db"}]}`))
	require.NoError(t, err)

	require.Equal(t, []string{"@prod/real"}, coll.Handles())
	require.True(t, coll.IsExistingSource("@prod/real"))

	// The stripped collection passes integrity verification.
	repaired, verr := source.VerifyIntegrity(coll)
	require.NoError(t, verr)
	require.False(t, repaired)
}

// TestSource_String_Group_nil verifies the nil-receiver guards on
// Source.String and Source.Group.
func TestSource_String_Group_nil(t *testing.T) {
	var s *source.Source
	require.Equal(t, "<nil>", s.String())
	require.Empty(t, s.Group())

	// A non-nil source with an empty handle exercises the
	// groupFromHandle empty-string guard.
	require.Empty(t, (&source.Source{}).Group())
}

// TestCollection_Unmarshal_errPaths covers the error returns of
// UnmarshalJSON and UnmarshalYAML.
func TestCollection_Unmarshal_errPaths(t *testing.T) {
	coll := &source.Collection{}
	require.Error(t, coll.UnmarshalJSON([]byte("{invalid json")))
	require.Error(t, coll.UnmarshalYAML(func(any) error {
		return errors.New("boom")
	}))
}
