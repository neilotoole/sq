package source_test

import (
	"testing"

	"github.com/neilotoole/sq/drivers/sqlite3"
	"github.com/neilotoole/sq/testh/proj"

	"github.com/neilotoole/sq/testh/tutil"
	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/libsq/source"
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
		Type:     sqlite3.Type,
		Location: proj.Abs("drivers/sqlite3/testdata/sakila.db"),
	}
}

func TestSet_Groups(t *testing.T) {
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

	set := &source.Set{}

	gotGroup := set.ActiveGroup()
	require.Equal(t, source.RootGroup, gotGroup)

	for i := range srcs {
		require.NoError(t, set.Add(srcs[i]))
	}

	for _, src := range srcs {
		require.True(t, set.IsExistingSource(src.Handle))
		gotSrc, err := set.Get(src.Handle)
		require.NoError(t, err)
		require.Equal(t, *src, *gotSrc)
	}

	gotGroups := set.Groups()
	require.EqualValues(t, wantGroups, gotGroups)

	gotErr := set.SetActiveGroup("not_a_group")
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
		gotSrcs, err := set.SourcesInGroup(g)
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
		t.Run(tutil.Name(tc.loc), func(t *testing.T) {
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
			got := source.ShortLocation(tc.loc)
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

func TestSet_Active(t *testing.T) {
	ss := &source.Set{}

	activeSrc := ss.Active()
	require.Nil(t, activeSrc)
	require.Equal(t, source.RootGroup, ss.ActiveGroup())

	require.Error(t, ss.SetActiveGroup("non_exist"))

	sakilaSrc := newSource("@sakila")

	// Test that the active group and
	require.NoError(t, ss.Add(sakilaSrc))
	gotSrc, err := ss.Get(sakilaSrc.Handle)
	require.NoError(t, err)
	require.Equal(t, sakilaSrc, gotSrc)
	require.Equal(t, source.RootGroup, ss.ActiveGroup(),
		"active group should not have changed due to adding a source")
	require.Nil(t, ss.Active())

	// Test setting the active source
	gotSrc, err = ss.SetActive(sakilaSrc.Handle, false)
	require.NoError(t, err)
	require.Equal(t, sakilaSrc, gotSrc)
	require.Equal(t, gotSrc, ss.Active())

	// Test removing the active source
	require.NoError(t, ss.Remove(ss.ActiveHandle()))
	require.Nil(t, ss.Active())

	// Test group
	sakilaProdSrc := newSource("@prod/sakila")
	require.NoError(t, ss.Add(sakilaProdSrc))
	require.Equal(t, source.RootGroup, ss.ActiveGroup(),
		"adding a grouped src should not set the active group")

	gotSrc, err = ss.SetActive(sakilaProdSrc.Handle, false)
	require.NoError(t, err)
	require.Equal(t, sakilaProdSrc, gotSrc)
	require.Equal(t, source.RootGroup, ss.ActiveGroup(),
		"setting active src should not set active group")

	require.NoError(t, ss.SetActiveGroup(prodGroup))
	require.Equal(t, prodGroup, ss.ActiveGroup())
	gotSrcs, err := ss.RemoveGroup(prodGroup)
	require.NoError(t, err)
	require.Equal(t, sakilaProdSrc, gotSrcs[0])
	require.Equal(t, source.RootGroup, ss.ActiveGroup(),
		"active group should have been reset to root")
	require.False(t, ss.IsExistingGroup(prodGroup))
	require.Empty(t, ss.Sources())
}

func TestSet_RenameGroup_toRoot(t *testing.T) {
	ss := &source.Set{}

	gotSrcs, err := ss.RenameGroup(source.RootGroup, prodGroup)
	require.Error(t, err, "can't rename root group")
	require.Nil(t, gotSrcs)

	src := newSource("@prod/sakila")
	originalHandle := src.Handle
	require.NoError(t, ss.Add(src))

	gotSrcs, err = ss.SourcesInGroup(prodGroup)
	require.NoError(t, err)
	require.Len(t, gotSrcs, 1)
	require.Equal(t, src, gotSrcs[0])

	// Rename "prod" group to root effectively moves all prod sources
	// into root. The prod group will cease to exist.
	gotSrcs, err = ss.RenameGroup(prodGroup, source.RootGroup)
	require.NoError(t, err)
	require.Len(t, gotSrcs, 1)
	require.Equal(t, source.RootGroup, ss.ActiveGroup())
	require.Equal(t, "@sakila", src.Handle, "src should have new handle")

	require.False(t, ss.IsExistingGroup(prodGroup))
	gotSrc, err := ss.Get(originalHandle)
	require.Error(t, err, "original handle no longer exists")
	require.Nil(t, gotSrc)

	gotSrcs, err = ss.SourcesInGroup(prodGroup)
	require.Error(t, err, "group should not not exist")
	require.Empty(t, gotSrcs)

	gotSrc, err = ss.Get("@sakila")
	require.NoError(t, err, "should be available via new handle")
	require.Equal(t, src.Location, gotSrc.Location)

	gotSrcs, err = ss.SourcesInGroup(source.RootGroup)
	require.NoError(t, err)
	require.Len(t, gotSrcs, 1)
	require.Equal(t, src, gotSrcs[0])

	// Do the same as above, but rename "prod" group to "prod/customer".
}

func TestSet_RenameGroup_toOther(t *testing.T) {
	ss := &source.Set{}

	src := newSource("@prod/sakila")
	originalHandle := src.Handle
	require.NoError(t, ss.Add(src))

	// Rename "prod" group to "dev/customer" effectively moves all prod sources
	// into "dev/customer". The prod group will cease to exist.
	gotSrcs, err := ss.RenameGroup(prodGroup, devCustGroup)
	require.NoError(t, err)
	require.Len(t, gotSrcs, 1)
	require.Equal(t, source.RootGroup, ss.ActiveGroup())
	require.Equal(t, "@dev/customer/sakila", src.Handle,
		"src should have new handle")

	require.False(t, ss.IsExistingGroup(prodGroup))
	gotSrc, err := ss.Get(originalHandle)
	require.Error(t, err, "original handle no longer exists")
	require.Nil(t, gotSrc)

	gotSrcs, err = ss.SourcesInGroup(prodGroup)
	require.Error(t, err, "group should not not exist")
	require.Empty(t, gotSrcs)

	gotSrc, err = ss.Get("@dev/customer/sakila")
	require.NoError(t, err, "should be available via new handle")
	require.Equal(t, src.Location, gotSrc.Location)

	gotSrcs, err = ss.SourcesInGroup(devCustGroup)
	require.NoError(t, err)
	require.Len(t, gotSrcs, 1)
	require.Equal(t, src, gotSrcs[0])
}

func TestSet_Add_conflictsWithGroup(t *testing.T) {
	ss := &source.Set{}

	src1 := newSource("@prod/sakila")
	require.NoError(t, ss.Add(src1))
	require.True(t, ss.IsExistingGroup(prodGroup))

	src2 := newSource("@prod")
	require.Error(t, ss.Add(src2), "handle conflicts with existing group")
}

func TestSet_Add_groupConflictsWithSource(t *testing.T) {
	ss := &source.Set{}

	src1 := newSource("@sakila")
	require.NoError(t, ss.Add(src1))

	src2 := newSource("@sakila/sakiladb")
	require.Error(t, ss.Add(src2), "handle group (sakila) conflicts with source @sakila")
}

func TestSet_RenameGroup(t *testing.T) {
	ss := &source.Set{}

	src1 := newSource("@prod/sakila")
	require.NoError(t, ss.Add(src1))

	gotSrcs, err := ss.RenameGroup(devGroup, prodGroup)
	require.Error(t, err, "group dev does not exist")
	require.Nil(t, gotSrcs)

	gotSrcs, err = ss.RenameGroup(prodGroup, devGroup)
	require.NoError(t, err)
	require.Equal(t, gotSrcs[0].Handle, "@dev/sakila")
}

func TestSet_RenameGroup_conflictsWithSource(t *testing.T) {
	ss := &source.Set{}

	src1 := newSource("@sakila")
	require.NoError(t, ss.Add(src1))

	src2 := newSource("@prod/db")
	require.NoError(t, ss.Add(src2))

	_, err := ss.RenameGroup("prod", "sakila")
	require.Error(t, err, "should be a conflict error")
}

func TestSet_MoveHandleToGroup(t *testing.T) {
	ss := &source.Set{}

	src1 := newSource("@sakila")
	require.NoError(t, ss.Add(src1))

	gotSrc, err := ss.MoveHandleToGroup(src1.Handle, "/")
	// This is effectively no-op
	require.NoError(t, err)
	require.Equal(t, src1, gotSrc)

	gotSrc, err = ss.MoveHandleToGroup(src1.Handle, prodGroup)
	require.NoError(t, err, "it is legal to move a handle to a non-existing group")
	require.Equal(t, "@prod/sakila", gotSrc.Handle)
	require.Equal(t, prodGroup, gotSrc.Group())
}

func TestSet_MoveHandleToGroup_conflictsWithExistingSource(t *testing.T) {
	ss := &source.Set{}

	src1 := newSource("@sakila")
	require.NoError(t, ss.Add(src1))

	src2 := newSource("@prod/db")
	require.NoError(t, ss.Add(src2))

	gotSrc, err := ss.MoveHandleToGroup(src1.Handle, "sakila")
	// This is effectively no-op
	require.Error(t, err, "group 'sakila' should conflict with handle @sakila")
	require.Nil(t, gotSrc)
}

func TestSet_RenameSource(t *testing.T) {
	ss := &source.Set{}

	src1 := newSource("@sakila")
	require.NoError(t, ss.Add(src1))

	gotSrc, err := ss.RenameSource(src1.Handle, "@sakila2")
	require.NoError(t, err)
	require.Equal(t, "@sakila2", gotSrc.Handle)
	require.Equal(t, src1, gotSrc)
}

func TestSet_RenameSource_conflictsWithExistingHandle(t *testing.T) {
	ss := &source.Set{}

	src1 := newSource("@prod/sakila")
	require.NoError(t, ss.Add(src1))

	src2 := newSource("@dev/sakila")
	require.NoError(t, ss.Add(src2))

	gotSrc, err := ss.RenameSource(src2.Handle, src1.Handle)
	require.Error(t, err)
	require.Nil(t, gotSrc)
}

func TestSet_RenameSource_conflictsWithExistingGroup(t *testing.T) {
	ss := &source.Set{}

	src1 := newSource("@prod/sakila")
	require.NoError(t, ss.Add(src1))

	src2 := newSource("@dev/sakila")
	require.NoError(t, ss.Add(src2))

	gotSrc, err := ss.RenameSource(src1.Handle, "/")
	require.Error(t, err)
	require.Nil(t, gotSrc)

	gotSrc, err = ss.RenameSource(src1.Handle, "@prod")
	require.Error(t, err)
	require.Nil(t, gotSrc)
}

func TestSet_Tree(t *testing.T) {
	ss := &source.Set{}

	require.NoError(t, ss.Add(newSource("@sakila_csv")))
	require.NoError(t, ss.Add(newSource("@sakila_tsv")))
	require.NoError(t, ss.Add(newSource("@dev/db1")))
	require.NoError(t, ss.Add(newSource("@dev/pg/db1")))
	require.NoError(t, ss.Add(newSource("@dev/pg/db2")))
	require.NoError(t, ss.Add(newSource("@dev/pg/db3")))
	require.NoError(t, ss.Add(newSource("@staging/db1")))
	require.NoError(t, ss.Add(newSource("@prod/pg/db1")))
	require.NoError(t, ss.Add(newSource("@prod/pg/db2")))
	require.NoError(t, ss.Add(newSource("@prod/pg/backup/db1")))
	require.NoError(t, ss.Add(newSource("@prod/pg/backup/db2")))

	gotSrcs := ss.Sources()
	require.Len(t, gotSrcs, 11)

	gotGroupNames := ss.Groups()
	require.Len(t, gotGroupNames, 7)

	gotTree, err := ss.Tree(source.RootGroup)
	require.NoError(t, err)

	directSrcCount, allSrcCount, directGroupCount, allGroupCount := gotTree.Count()
	require.Equal(t, 2, directSrcCount)
	require.Equal(t, directSrcCount, len(gotTree.Sources))
	require.Equal(t, 11, allSrcCount)
	require.Equal(t, 3, directGroupCount)
	require.Equal(t, directGroupCount, len(gotTree.Groups))
	require.Equal(t, 6, allGroupCount)
	require.True(t, gotTree.Active, "root group is active")
	require.False(t, gotTree.Groups[0].Active)

	// Try with a subgroup
	gotTree, err = ss.Tree("dev")
	require.NoError(t, err)
	directSrcCount, allSrcCount, directGroupCount, allGroupCount = gotTree.Count()
	require.Equal(t, 1, directSrcCount)
	require.Equal(t, directSrcCount, len(gotTree.Sources))
	require.Equal(t, 4, allSrcCount)
	require.Equal(t, 1, directGroupCount)
	require.Equal(t, directGroupCount, len(gotTree.Groups))
	require.Equal(t, 1, allGroupCount)
	require.False(t, gotTree.Active)
}
