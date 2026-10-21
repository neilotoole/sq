package render

import (
	"log/slog"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/libsq/ast"
	"github.com/neilotoole/sq/libsq/driver/dialect"
)

func TestDoFromTable_NotIndexed(t *testing.T) {
	dlct := dialect.Dialect{Enquote: strconv.Quote}

	testCases := []struct {
		slq   string
		hints map[string]*TableHints
		want  string
	}{
		{slq: `.t`, want: `FROM "t"`},
		{slq: `.t`, hints: map[string]*TableHints{"t": {}}, want: `FROM "t"`},
		{slq: `.t`, hints: map[string]*TableHints{"t": {NotIndexed: true}}, want: `FROM "t" NOT INDEXED`},
		{slq: `.t`, hints: map[string]*TableHints{"u": {NotIndexed: true}}, want: `FROM "t"`},
		{
			slq: `.t:a`, hints: map[string]*TableHints{"t": {NotIndexed: true}},
			want: `FROM "t" AS "a" NOT INDEXED`,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.slq+"_"+tc.want, func(t *testing.T) {
			a, err := ast.Parse(slog.Default(), tc.slq)
			require.NoError(t, err)
			sel := ast.NewInspector(a).FindFirstTableSelector()
			require.NotNil(t, sel)

			rc := &Context{Dialect: dlct, TableHints: tc.hints}
			got, err := doFromTable(rc, sel)
			require.NoError(t, err)
			require.Equal(t, tc.want, got)
		})
	}
}
