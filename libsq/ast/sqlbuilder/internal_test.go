package sqlbuilder

import (
	"testing"

	"github.com/neilotoole/sq/testh/tutil"

	"github.com/stretchr/testify/require"
)

func TestQuoteTableOrColSelector(t *testing.T) {
	testCases := []struct {
		in      string
		want    string
		wantErr bool
	}{
		{in: "", wantErr: true},
		{in: "  ", wantErr: true},
		{in: "not_start_with_period", wantErr: true},
		{in: ".table", want: `"table"`},
		{in: ".table.col", want: `"table"."col"`},
		{in: ".table.col.other", wantErr: true},
	}

	const quote = `"`

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.in, func(t *testing.T) {
			got, gotErr := quoteTableOrColSelector(quote, tc.in)
			if tc.wantErr {
				require.Error(t, gotErr)
				return
			}

			require.NoError(t, gotErr)
			require.Equal(t, tc.want, got)
		})
	}
}

func TestEscapeLiteralString(t *testing.T) {
	testCases := []struct {
		in   string
		want string
	}{
		{in: ``, want: ``},
		{in: `  `, want: `  `},
		{in: `hello`, want: `hello`},
		{in: `"hello"`, want: `"hello"`},
		{in: `there's`, want: `there''s`},
		{in: `double''`, want: `double''''`},
	}

	for i, tc := range testCases {
		tc := tc

		t.Run(tutil.Name(i, tc.in), func(t *testing.T) {
			got := escapeLiteral(tc.in)
			require.Equal(t, tc.want, got)
		})
	}
}
