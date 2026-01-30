package source_test

import (
	"testing"

	"github.com/stretchr/testify/require"

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
		tc := tc
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
		tc := tc
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
