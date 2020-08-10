package testh_test

import (
	"io/ioutil"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"

	"github.com/neilotoole/sq/drivers/csv"
	"github.com/neilotoole/sq/libsq/source"
	"github.com/neilotoole/sq/libsq/sqlz"
	"github.com/neilotoole/sq/libsq/stringz"
	"github.com/neilotoole/sq/testh"
	"github.com/neilotoole/sq/testh/proj"
	"github.com/neilotoole/sq/testh/sakila"
)

func TestVal(t *testing.T) {
	want := "hello"
	var got interface{}

	if testh.Val(nil) != nil {
		t.FailNow()
	}

	var v0 interface{}
	if testh.Val(v0) != nil {
		t.FailNow()
	}

	var v1 = want
	var v1a interface{} = want
	var v2 = &v1
	var v3 interface{} = &v1
	var v4 = &v2
	var v5 = &v4

	vals := []interface{}{v1, v1a, v2, v3, v4, v5}
	for _, val := range vals {
		got = testh.Val(val)

		if got != want {
			t.Errorf("expected %T(%v) but got %T(%v)", want, want, got, got)
		}
	}

	slice := []string{"a", "b"}
	require.Equal(t, slice, testh.Val(slice))
	require.Equal(t, slice, testh.Val(&slice))

	b := true
	require.Equal(t, b, testh.Val(b))
	require.Equal(t, b, testh.Val(&b))

	type structT struct {
		f string
	}

	st1 := structT{f: "hello"}
	require.Equal(t, st1, testh.Val(st1))
	require.Equal(t, st1, testh.Val(&st1))

	var c chan int
	require.Nil(t, testh.Val(c))
	c = make(chan int, 10)
	require.Equal(t, c, testh.Val(c))
	require.Equal(t, c, testh.Val(&c))
}

func TestCopyRecords(t *testing.T) {
	var v1, v2, v3, v4, v5, v6 = int64(1), float64(1.1), true, "hello", []byte("hello"), time.Unix(0, 0)

	testCases := map[string][]sqlz.Record{
		"nil":   nil,
		"empty": {},
		"vals": {
			{nil, &v1, &v2, &v3, &v4, &v5, &v6},
			// {nil, &v1, &v2, &v3, &v4, &v5, &v6},
		},
	}

	for name, recs := range testCases {
		name, recs := name, recs

		t.Run(name, func(t *testing.T) {
			recs2 := testh.CopyRecords(recs)
			require.True(t, len(recs) == len(recs2))

			if recs == nil {
				require.True(t, recs2 == nil)
				return
			}

			for i := range recs {
				require.True(t, len(recs[i]) == len(recs2[i]))
				for j := range recs[i] {
					if recs[i][j] == nil {
						require.True(t, recs2[i][j] == nil)
						continue
					}

					require.False(t, recs[i][j] == recs2[i][j],
						"pointer values should not be equal: %#v --> %#v", recs[i][j], recs2[i][j])

					val1, val2 := testh.Val(recs[i][j]), testh.Val(recs2[i][j])
					require.Equal(t, val1, val2,
						"dereferenced values should be equal: %#v --> %#v", val1, val2)
				}
			}
		})
	}
}

func TestRecordsFromTbl(t *testing.T) {
	recMeta1, recs1 := testh.RecordsFromTbl(t, sakila.SL3, sakila.TblActor)
	require.Equal(t, sakila.TblActorColKinds(), recMeta1.Kinds())

	recs1[0][0] = t.Name()

	recMeta2, recs2 := testh.RecordsFromTbl(t, sakila.SL3, sakila.TblActor)
	require.False(t, &recMeta1 == &recMeta2, "should be distinct copies")
	require.False(t, &recs1 == &recs2, "should be distinct copies")
	require.NotEqual(t, recs1[0][0], recs2[0][0], "recs2 should not have the mutated value from recs1")
}

func TestHelper_Files(t *testing.T) {
	fpath := "drivers/csv/testdata/person.csv"
	wantBytes := proj.ReadFile(fpath)

	src := &source.Source{
		Handle:   "@test_" + stringz.Uniq8(),
		Type:     csv.TypeCSV,
		Location: proj.Abs(fpath),
	}

	th := testh.New(t)
	fs := th.Files()

	typ, err := fs.Type(th.Context, src.Location)
	require.NoError(t, err)
	require.Equal(t, src.Type, typ)

	g, gctx := errgroup.WithContext(th.Context)

	for i := 0; i < 1000; i++ {
		g.Go(func() error {
			r, err := fs.NewReader(gctx, src)
			require.NoError(t, err)

			defer func() { require.NoError(t, r.Close()) }()

			b, err := ioutil.ReadAll(r)
			require.NoError(t, err)

			require.Equal(t, wantBytes, b)
			return nil
		})
	}

	err = g.Wait()
	require.NoError(t, err)
}

func TestTName(t *testing.T) {
	testCases := []struct {
		a    []interface{}
		want string
	}{
		{a: []interface{}{}, want: "empty"},
		{a: []interface{}{"test", 1}, want: "test_1"},
		{a: []interface{}{"/file/path/name"}, want: "_file_path_name"},
	}

	for _, tc := range testCases {
		got := testh.TName(tc.a...)
		require.Equal(t, tc.want, got)
	}

}
