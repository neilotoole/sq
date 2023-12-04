package testh_test

import (
	"io"
	"testing"
	"time"

	_ "github.com/ryboe/q" // keep the q lib around
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"

	"github.com/neilotoole/sq/drivers/csv"
	"github.com/neilotoole/sq/libsq/core/record"
	"github.com/neilotoole/sq/libsq/core/stringz"
	"github.com/neilotoole/sq/libsq/source"
	"github.com/neilotoole/sq/testh"
	"github.com/neilotoole/sq/testh/proj"
	"github.com/neilotoole/sq/testh/sakila"
	"github.com/neilotoole/sq/testh/tu"
)

func TestVal(t *testing.T) {
	want := "hello"
	var got any

	if stringz.Val(nil) != nil {
		t.FailNow()
	}

	var v0 any
	if stringz.Val(v0) != nil {
		t.FailNow()
	}

	v1 := want
	var v1a any = want
	v2 := &v1
	var v3 any = &v1
	v4 := &v2
	v5 := &v4

	vals := []any{v1, v1a, v2, v3, v4, v5}
	for _, val := range vals {
		got = stringz.Val(val)

		if got != want {
			t.Errorf("expected %T(%v) but got %T(%v)", want, want, got, got)
		}
	}

	slice := []string{"a", "b"}
	require.Equal(t, slice, stringz.Val(slice))
	require.Equal(t, slice, stringz.Val(&slice))

	b := true
	require.Equal(t, b, stringz.Val(b))
	require.Equal(t, b, stringz.Val(&b))

	type structT struct {
		f string
	}

	st1 := structT{f: "hello"}
	require.Equal(t, st1, stringz.Val(st1))
	require.Equal(t, st1, stringz.Val(&st1))

	var c chan int
	require.Nil(t, stringz.Val(c))
	c = make(chan int, 10)
	require.Equal(t, c, stringz.Val(c))
	require.Equal(t, c, stringz.Val(&c))
}

func TestCopyRecords(t *testing.T) {
	v1, v2, v3, v4, v5, v6 := int64(1), float64(1.1), true, "hello", []byte("hello"), time.Unix(0, 0)

	testCases := map[string][]record.Record{
		"nil":   nil,
		"empty": {},
		"vals": {
			{nil, v1, v2, v3, v4, v5, v6},
			// {nil, &v1, &v2, &v3, &v4, &v5, &v6},
		},
	}

	for name, recs := range testCases {
		name, recs := name, recs

		t.Run(name, func(t *testing.T) {
			recs2 := record.CloneSlice(recs)
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

					val1, val2 := stringz.Val(recs[i][j]), stringz.Val(recs2[i][j])
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

	typ, err := fs.DriverType(th.Context, src.Location)
	require.NoError(t, err)
	require.Equal(t, src.Type, typ)

	g, _ := errgroup.WithContext(th.Context)

	for i := 0; i < 1000; i++ {
		g.Go(func() error {
			r, fErr := fs.Open(th.Context, src)
			require.NoError(t, fErr)

			defer func() { require.NoError(t, r.Close()) }()

			b, fErr := io.ReadAll(r)
			require.NoError(t, fErr)

			require.Equal(t, wantBytes, b)
			return nil
		})
	}

	err = g.Wait()
	require.NoError(t, err)
}

func TestTName(t *testing.T) {
	testCases := []struct {
		a    []any
		want string
	}{
		{a: []any{}, want: "empty"},
		{a: []any{"test", 1}, want: "test_1"},
		{a: []any{"/file/path/name"}, want: "_file_path_name"},
	}

	for _, tc := range testCases {
		got := tu.Name(tc.a...)
		require.Equal(t, tc.want, got)
	}
}
