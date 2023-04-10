package source_test

import (
	"testing"

	"github.com/neilotoole/sq/testh/tutil"
	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/libsq/source"
)

func TestGroups(t *testing.T) {
	srcs := []*source.Source{
		{Handle: "@handle1", Location: "0"},
		{Handle: "@group1/handle1", Location: "1"},
		{Handle: "@group1/sub1/handle1", Location: "2"},
		{Handle: "@group1/sub1/handle2", Location: "3"},
		{Handle: "@group1/sub1/sub2/sub3/handle2", Location: "4"},
		{Handle: "@group1/sub1/sub2/sub4/sub5/handle", Location: "5"},
		{Handle: "@group2/sub1/sub2/handle", Location: "6"},
		{Handle: "@g/handle", Location: "7"},
	}

	require.Equal(t, srcs[0].Group(), "")
	require.Equal(t, srcs[1].Group(), "group1")
	require.Equal(t, srcs[2].Group(), "group1/sub1")
	require.Equal(t, srcs[5].Group(), "group1/sub1/sub2/sub4/sub5")
	require.Equal(t, srcs[7].Group(), "g")

	wantGroups := []string{
		"g",
		"group1",
		"group1/sub1",
		"group1/sub1/sub2",
		"group1/sub1/sub2/sub3",
		"group1/sub1/sub2/sub4",
		"group1/sub1/sub2/sub4/sub5",
		"group2",
		"group2/sub1",
		"group2/sub1/sub2",
	}

	set := &source.Set{}

	gotGroup := set.Group()
	require.Equal(t, "", gotGroup)

	for i := range srcs {
		require.NoError(t, set.Add(srcs[i]))
	}

	for _, src := range srcs {
		require.True(t, set.Exists(src.Handle))
		gotSrc, err := set.Get(src.Handle)
		require.NoError(t, err)
		require.Equal(t, *src, *gotSrc)
	}

	gotGroups := set.Groups()
	require.EqualValues(t, wantGroups, gotGroups)

	gotErr := set.SetGroup("not_a_group")
	require.Error(t, gotErr)

	groupTest := map[string]int{
		"":                           len(srcs),
		"group1":                     5,
		"group1/sub1":                4,
		"group1/sub1/sub2/sub4/sub5": 1,
		"g":                          1,
		"group1/sub1/sub2":           2,
	}

	for g, wantCount := range groupTest {
		gotSrcs, err := set.GroupItems(g)
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
		tname string
		loc   string
		want  string
	}{
		{tname: "sqlite3_scheme", loc: "sqlite3:///path/to/sqlite.db", want: "sqlite.db"},
		{tname: "sqlite3", loc: "/path/to/sqlite.db", want: "sqlite.db"},
		{tname: "xlsx", loc: "/path/to/data.xlsx", want: "data.xlsx"},
		{tname: "https", loc: "https://path/to/data.xlsx", want: "data.xlsx"},
		{tname: "http", loc: "http://path/to/data.xlsx", want: "data.xlsx"},
		{tname: "sqlserver", loc: "sqlserver://sq:p_ssw0rd@localhost?database=sqtest", want: "sq@localhost/sqtest"},
		{
			tname: "postgres", loc: "postgres://sq:p_ssW0rd@localhost/sqtest?sslmode=disable",
			want: "sq@localhost/sqtest",
		},
		{tname: "mysql", loc: "mysql://sq:p_ssW0rd@localhost:3306/sqtest", want: "sq@localhost:3306/sqtest"},
		{tname: "mysql", loc: "mysql://sq:p_ssW0rd@localhost/sqtest", want: "sq@localhost/sqtest"},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.tname, func(t *testing.T) {
			got := source.ShortLocation(tc.loc)
			t.Logf("%s  -->  %s", tc.loc, got)
			require.Equal(t, tc.want, got)
		})
	}
}
