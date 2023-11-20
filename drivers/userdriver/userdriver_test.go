package userdriver_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/cli/config"
	"github.com/neilotoole/sq/drivers/userdriver"
	"github.com/neilotoole/sq/libsq/core/ioz"
	"github.com/neilotoole/sq/libsq/core/stringz"
	"github.com/neilotoole/sq/testh"
	"github.com/neilotoole/sq/testh/testsrc"
)

func TestDriver(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		handle   string
		tbl      string
		wantRecs int
	}{
		{handle: testsrc.PplUD, tbl: "person", wantRecs: 3},
		{handle: testsrc.RSSNYTLocalUD, tbl: "item", wantRecs: 45},
	}

	for _, tc := range testCases {
		tc := tc

		t.Run(tc.handle, func(t *testing.T) {
			t.Parallel()

			th := testh.New(t, testh.OptLongOpen())
			src := th.Source(tc.handle)

			drvr := th.DriverFor(src)
			err := drvr.Ping(th.Context, src)
			require.NoError(t, err)

			pool, err := drvr.Open(th.Context, src)
			require.NoError(t, err)
			t.Cleanup(func() { assert.NoError(t, pool.Close()) })

			srcMeta, err := pool.SourceMetadata(th.Context, false)
			require.NoError(t, err)
			require.True(t, stringz.InSlice(srcMeta.TableNames(), tc.tbl))

			tblMeta, err := pool.TableMetadata(th.Context, tc.tbl)
			require.NoError(t, err)
			require.Equal(t, tc.tbl, tblMeta.Name)

			sink, err := th.QuerySQL(src, nil, "SELECT * FROM "+tc.tbl)
			require.NoError(t, err)
			require.Equal(t, tc.wantRecs, len(sink.Recs))
		})
	}
}

func TestValidateDriverDef_KnownGood(t *testing.T) {
	t.Parallel()

	testCases := []string{testsrc.PathDriverDefPpl, testsrc.PathDriverDefRSS}

	for _, defFile := range testCases {
		defFile := defFile

		t.Run(defFile, func(t *testing.T) {
			t.Parallel()

			defs := testh.DriverDefsFrom(t, defFile)
			for _, def := range defs {
				errs := userdriver.ValidateDriverDef(def)
				require.Empty(t, errs)
			}
		})
	}
}

func TestValidateDriverDef(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		title    string
		yml      string
		wantErrs int
	}{
		{
			title: "driver name is missing, should return with 1 error",
			yml: `user_drivers:
- driver:
  genre: xml
  title: People
  selector: /people`,
			wantErrs: 1,
		},
		{
			title: "missing genre, title, selector and tables",
			yml: `user_drivers:
- driver: ppl
  genre:
  title:
  selector:
  tables:`,
			wantErrs: 4,
		},
		{
			title: "table name, selector and cols are empty",
			yml: `user_drivers:
- driver: ppl
  genre: xml
  title: People
  selector: /people
  tables:
  - table:
    selector:
    cols:`,
			wantErrs: 3,
		},
		{
			title: "table selector is empty, col name is empty",
			yml: `user_drivers:
- driver: ppl
  genre: xml
  title: People
  selector: /people
  tables:
  - table: person
    selector:
    primary_key:
      - first_name
    cols:
    - col:
      kind: int
    - col: first_name
      selector: ./firstName
      kind: text`,
			wantErrs: 2,
		},
		{
			title: "primary key is invalid",
			yml: `user_drivers:
- driver: ppl
  genre: xml
  title: People
  selector: /people
  tables:
  - table: person
    selector: /people/person
    primary_key:
      - not_a_col_name
    cols:
    - col: person_id
      kind: int`,
			wantErrs: 1,
		},
	}

	for _, tc := range testCases {
		tc := tc

		t.Run(tc.title, func(t *testing.T) {
			t.Parallel()

			defs := defsFromString(t, tc.yml)
			require.Equal(t, 1, len(defs))
			def := defs[0]
			errs := userdriver.ValidateDriverDef(def)
			require.Equal(t, tc.wantErrs, len(errs),
				"wanted %d errs but got %d: %v", tc.wantErrs, len(errs), errs)
		})
	}
}

func defsFromString(t *testing.T, yml string) []*userdriver.DriverDef {
	ext := &config.Ext{}
	require.NoError(t, ioz.UnmarshallYAML([]byte(yml), ext))
	return ext.UserDrivers
}
