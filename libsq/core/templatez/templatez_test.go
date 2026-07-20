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

// TestFuncMap verifies that NewTemplate loads the full text/template
// sprig function set, including the non-hermetic functions such as
// "env" and "now".
func TestFuncMap(t *testing.T) {
	t.Setenv("TEMPLATEZ_TEST_SECRET", "revealed")

	// Plain sprig funcs are available.
	got, err := templatez.ExecuteTemplate(t.Name(), `{{"abc" | upper}}`, nil)
	require.NoError(t, err)
	require.Equal(t, "ABC", got)

	// The "env" func reads the process environment.
	got, err = templatez.ExecuteTemplate(t.Name(), `{{env "TEMPLATEZ_TEST_SECRET"}}`, nil)
	require.NoError(t, err)
	require.Equal(t, "revealed", got)

	// Other non-hermetic funcs parse and execute without error.
	for _, tpl := range []string{
		`{{now}}`,
		`{{randAlphaNum 8}}`,
		`{{uuidv4}}`,
	} {
		require.NoError(t, templatez.ValidTemplate(t.Name(), tpl),
			"expected %q to be a valid template", tpl)
		got, err = templatez.ExecuteTemplate(t.Name(), tpl, nil)
		require.NoError(t, err)
		require.NotEmpty(t, got)
	}
}
