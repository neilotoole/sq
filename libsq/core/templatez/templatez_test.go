package templatez_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/libsq/core/templatez"
	"github.com/neilotoole/sq/testh/tu"
)

func TestTemplate(t *testing.T) {
	data := map[string]string{"Name": "wubble"}

	testCases := []struct {
		tpl  string
		data any
		want string
		// wantParseErr is true if the template is invalid and should
		// fail at parse time (NewTemplate/ValidTemplate/ExecuteTemplate).
		wantParseErr bool
		// wantExecErr is true if the template parses cleanly but fails
		// when executed against data (ExecuteTemplate only).
		wantExecErr bool
	}{
		// "upper" is a sprig func. Verify that it loads.
		{tpl: "{{.Name | upper}}", data: data, want: "WUBBLE"},
		// "trim" is a sprig func; verify the sprig func map is wired up.
		{tpl: "{{.Name | trim}}", data: data, want: "wubble"},
		// Plain template, no funcs.
		{tpl: "hello {{.Name}}", data: data, want: "hello wubble"},
		// Parse error: unknown function.
		{tpl: "{{not_a_func .Name}}_", data: data, wantParseErr: true},
		// Parse error: malformed action.
		{tpl: "{{.Name", data: data, wantParseErr: true},
		// Execution error: parses fine, but evaluating a field on a
		// non-struct/non-map value fails at Execute time. This exercises
		// ExecuteTemplate's t.Execute error branch, which the parse-only
		// NewTemplate/ValidTemplate can't reach.
		{tpl: "{{.Missing}}", data: 42, wantExecErr: true},
	}

	for i, tc := range testCases {
		t.Run(tu.Name(i, tc.tpl), func(t *testing.T) {
			got, gotErr := templatez.ExecuteTemplate(t.Name(), tc.tpl, tc.data)
			t.Logf("\nTPL:   %s\nGOT:   %s\nERR:   %v", tc.tpl, got, gotErr)

			validErr := templatez.ValidTemplate(t.Name(), tc.tpl)
			_, newErr := templatez.NewTemplate(t.Name(), tc.tpl)

			switch {
			case tc.wantParseErr:
				// All three entry points surface the parse error.
				require.Error(t, gotErr)
				require.Error(t, validErr)
				require.Error(t, newErr)
				require.Empty(t, got)
			case tc.wantExecErr:
				// Only ExecuteTemplate sees the error; parsing succeeds.
				require.Error(t, gotErr)
				require.NoError(t, validErr)
				require.NoError(t, newErr)
				require.Empty(t, got)
			default:
				require.NoError(t, gotErr)
				require.NoError(t, validErr)
				require.NoError(t, newErr)
				require.Equal(t, tc.want, got)
			}
		})
	}
}

// TestHermeticFuncMap verifies that NewTemplate loads only sprig's
// hermetic (repeatable) functions. Non-hermetic functions such as
// "env" and "now" read process environment or global state, and must
// not be available to user-supplied templates.
func TestHermeticFuncMap(t *testing.T) {
	t.Setenv("TEMPLATEZ_TEST_SECRET", "leaked")

	// Hermetic sprig funcs remain available.
	got, err := templatez.ExecuteTemplate(t.Name(), `{{"abc" | upper}}`, nil)
	require.NoError(t, err)
	require.Equal(t, "ABC", got)

	// Non-hermetic funcs are excluded, so referencing them is a parse error.
	for _, tpl := range []string{
		`{{env "TEMPLATEZ_TEST_SECRET"}}`,
		`{{now}}`,
		`{{randAlphaNum 8}}`,
		`{{uuidv4}}`,
	} {
		require.Error(t, templatez.ValidTemplate(t.Name(), tpl),
			"expected %q to be rejected as a non-hermetic function", tpl)
	}
}
