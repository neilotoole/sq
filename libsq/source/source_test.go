package source_test

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	yaml "gopkg.in/yaml.v3"

	"github.com/neilotoole/sq/libsq/core/lg/lga"
	"github.com/neilotoole/sq/libsq/core/lg/lgt"
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
	log := lgt.New(t)

	src := &source.Source{
		Handle:   "@sakila",
		Type:     drivertype.SQLite,
		Location: "/tmp/sakila.db",
		Options:  nil,
	}

	log.Debug("src with nil Options", lga.Src, src)

	src.Options = options.Options{"a": 1, "b": true, "c": "hello"}

	log.Debug("src with non-nil Options", lga.Src, src)
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
