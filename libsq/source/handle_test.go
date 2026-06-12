package source_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/libsq/source"
	"github.com/neilotoole/sq/libsq/source/drivertype"
	"github.com/neilotoole/sq/testh/tu"
)

func TestIsValidGroup(t *testing.T) {
	testCases := []struct {
		in    string
		valid bool
	}{
		{"", true},
		{" ", false},
		{"/", true},
		{"//", false},
		{"prod", true},
		{"/prod", false},
		{"prod/", false},
		{"prod/user", true},
		{"prod/user/", false},
		{"prod/user/pg", true},
		{"pr_od", true},
	}

	for i, tc := range testCases {
		t.Run(tu.Name(i, tc.in), func(t *testing.T) {
			gotValid := source.IsValidGroup(tc.in)
			require.Equal(t, tc.valid, gotValid)
		})
	}
}

func TestValidHandle(t *testing.T) {
	testCases := []struct {
		in      string
		wantErr bool
	}{
		{in: "", wantErr: true},
		{in: "  ", wantErr: true},
		{in: "handle", wantErr: true},
		{in: "@", wantErr: true},
		{in: "1handle", wantErr: true},
		{in: "@ handle", wantErr: true},
		{in: "@handle ", wantErr: true},
		{in: "@handle#", wantErr: true},
		{in: "@1handle", wantErr: true},
		{in: "@1", wantErr: true},
		{in: "@?handle", wantErr: true},
		{in: "@?handle#", wantErr: true},
		{in: "@ha\nndle", wantErr: true},
		{in: "@group/handle"},
		{in: "@group/sub/sub2/handle"},
		{in: "@group/handle"},
		{in: "@group/", wantErr: true},
		{in: "@group/wub/", wantErr: true},
		{in: "@handle"},
		{in: "@handle1"},
		{in: "@h1"},
		{in: "@h_"},
		{in: "@h__1"},
		{in: "@h__1__a___"},
	}

	for i, tc := range testCases {
		t.Run(tu.Name(i, tc.in), func(t *testing.T) {
			gotErr := source.ValidHandle(tc.in)
			if tc.wantErr {
				require.Error(t, gotErr)
			} else {
				require.NoError(t, gotErr)
			}
		})
	}
}

// TestValidHandle_ErrorMessages pins the specific diagnostic text
// for each invalid-handle category. The error needs to name the
// offending part so users can self-correct; "invalid handle: <s>"
// alone is not enough.
func TestValidHandle_ErrorMessages(t *testing.T) {
	testCases := []struct {
		in   string
		want string // substring that must appear in the error
	}{
		{in: "", want: "empty"},
		{in: "  ", want: "whitespace"},
		{in: "handle", want: "must start with '@'"},
		{in: "@", want: "no name after '@'"},
		{in: "@1handle", want: `must start with a letter`},
		{in: "@_handle", want: `must start with a letter`},
		{in: "@sakila/1bad", want: `segment "1bad" must start with a letter`},
		{in: "@sakila/local/pg-keyring1", want: `illegal character "-"`},
		{in: "@sakila/local/pg-keyring1", want: `letters, digits, and underscore`},
		{in: "@sakila/", want: "empty segment"},
		{in: "@sakila//x", want: "empty segment"},
		{in: "@sakila/x.y", want: `illegal character "."`},
	}
	for _, tc := range testCases {
		t.Run(tc.in+" => "+tc.want, func(t *testing.T) {
			err := source.ValidHandle(tc.in)
			require.Error(t, err)
			require.Contains(t, err.Error(), tc.want,
				"diagnostic should explain why %q is invalid", tc.in)
		})
	}
}

// TestValidGroup_ErrorMessages mirrors TestValidHandle_ErrorMessages
// for groups: the leading '@' is rejected, illegal chars and empty
// segments still surface their specifics.
func TestValidGroup_ErrorMessages(t *testing.T) {
	testCases := []struct {
		in   string
		want string
	}{
		{in: "@prod", want: "leading '@' is reserved for handles"},
		{in: "prod-x", want: `illegal character "-"`},
		{in: "prod//x", want: "empty segment"},
		{in: "1prod", want: "must start with a letter"},
	}
	for _, tc := range testCases {
		t.Run(tc.in+" => "+tc.want, func(t *testing.T) {
			err := source.ValidGroup(tc.in)
			require.Error(t, err)
			require.Contains(t, err.Error(), tc.want)
		})
	}
}

// TestSuggestHandle_FromPlaceholder verifies the per-scheme handle
// derivation for bare-placeholder Locations. The goal: a non-ugly
// default for `sq add '${env:FOO}'` and friends, without consulting
// the resolver (which would make the suggestion machine-dependent).
func TestSuggestHandle_FromPlaceholder(t *testing.T) {
	testCases := []struct {
		loc  string
		want string // expected handle (with no active group set)
	}{
		// env: name verbatim, lowercased. No suffix-stripping —
		// the user's discriminators (_PROD, _DEV) must survive.
		{loc: "${env:PBDSN}", want: "@pbdsn"},
		{loc: "${env:SAKILA_DSN}", want: "@sakila_dsn"},
		{loc: "${env:SAKILA_DSN_PROD}", want: "@sakila_dsn_prod"},
		{loc: "${env:DB}", want: "@db"},
		{loc: "${env:already_lower}", want: "@already_lower"},

		// file: basename without extension. Drops the trailing
		// extension structurally (matches existing ./actor.csv
		// behavior), but does not interpret the user's name.
		{loc: "${file:./pg.dsn}", want: "@pg"},
		{loc: "${file:/etc/sq/sakila.dsn}", want: "@sakila"},
		{loc: "${file:~/secrets/db}", want: "@db"},
		{loc: "${file:/abs/path/Sakila.DSN}", want: "@sakila"},

		// op://<vault>/<item>/[<section>/]<field> — the item slot
		// is the identity-bearing segment.
		{loc: "${op://Private/sakila/dsn}", want: "@sakila"},
		{loc: "${op://Private/sakila/Custom/password}", want: "@sakila"},

		// vault path: last segment, fragment stripped.
		{loc: "${vault:secret/data/sakila}", want: "@sakila"},
		{loc: "${vault:secret/data/sakila#password}", want: "@sakila"},

		// keyring legacy handle-encoded form — extract handle name.
		{loc: "${keyring:@sakila/conn_str}", want: "@sakila"},
		{loc: "${keyring:@prod_db/password}", want: "@prod_db"},

		// Names containing runes illegal in handles (hyphens, dots,
		// spaces) must be sanitized to underscore, matching the URL
		// branch of SuggestHandle. Without this, the suggested handle
		// fails ValidHandle and the add aborts.
		{loc: "${file:/secrets/pg-prod.dsn}", want: "@pg_prod"},
		{loc: "${file:/secrets/my db.dsn}", want: "@my_db"},
		{loc: "${env:MY-DSN}", want: "@my_dsn"},
		{loc: "${op://Private/sakila-pg/dsn}", want: "@sakila_pg"},
		{loc: "${vault:secret/data/pg.prod}", want: "@pg_prod"},

		// keyring opaque (Crockford) — falls through to the generic
		// path, which produces the existing "h"-prefixed ugly form.
		// We don't try to make this pretty; rare hand-crafted case.
		{loc: "${keyring:j2k7m3pxtz}", want: "@h__keyring_j2k7m3pxtz_"},

		// Note: the cloud-SM schemes sketched in the design doc
		// (AWS Secrets Manager, GCP Secret Manager, Azure Key Vault)
		// are not exercised here. The placeholder grammar currently
		// requires [a-z][a-z0-9]* scheme names — no hyphens — so a
		// hyphenated scheme like "aws-sm" would be rejected by the
		// parser anyway. When those schemes land they'll either pick
		// hyphen-free names or the grammar will be widened; handle
		// suggestions will be added then.
		//
		// Composition Locations (placeholders embedded inside a
		// literal URL, e.g. postgres://...:${env:PW}@host/db) are
		// also not covered here: location.Parse rejects them with
		// "invalid userinfo" — a pre-existing limitation independent
		// of this branch. Users with composition Locations need to
		// pass --handle today.
	}

	for _, tc := range testCases {
		t.Run(tc.loc, func(t *testing.T) {
			coll := &source.Collection{}
			got, err := source.SuggestHandle(coll, drivertype.None, tc.loc)
			require.NoError(t, err)
			require.Equal(t, tc.want, got,
				"placeholder-derived handle for %q", tc.loc)
		})
	}
}

func TestSuggestHandle(t *testing.T) {
	testCases := []struct {
		typ   drivertype.Type
		loc   string
		want  string
		taken []string
	}{
		{
			typ:  drivertype.CSV,
			loc:  "/path/to/actor.csv",
			want: "@actor",
		},
		{
			typ:  drivertype.None,
			loc:  "/path/to/actor.csv",
			want: "@actor",
		},
		{
			typ:  drivertype.XLSX,
			loc:  "/path/to/sakila.xlsx",
			want: "@sakila",
		},
		{
			typ:  drivertype.XLSX,
			loc:  "/path/to/123_sakila.xlsx",
			want: "@h123_sakila",
		},
		{
			typ:  drivertype.XLSX,
			loc:  "/path/to/__sakila.xlsx",
			want: "@h__sakila",
		},
		{
			typ:  drivertype.XLSX,
			loc:  "/path/to/sakila.something.xlsx",
			want: "@sakila_something",
		},
		{
			typ:  drivertype.XLSX,
			loc:  "/path/to/😀abc123😀",
			want: "@h_abc123_",
		},
		{
			typ:  drivertype.None,
			loc:  "/path/to/sakila.xlsx",
			want: "@sakila",
		},
		{
			typ:   drivertype.XLSX,
			loc:   "/path/to/sakila.xlsx",
			want:  "@sakila2",
			taken: []string{"@sakila", "@sakila1"},
		},
		{
			typ:  drivertype.SQLite,
			loc:  "sqlite3:///path/to/sakila.db",
			want: "@sakila",
		},
		{
			typ:  drivertype.None,
			loc:  "sqlite3:///path/to/sakila.db",
			want: "@sakila",
		},
		{
			typ:  drivertype.SQLite,
			loc:  "/path/to/sakila.db",
			want: "@sakila",
		},
		{
			typ:  drivertype.MSSQL,
			loc:  "sqlserver://sakila_p_ssW0rd@localhost?database=sakila",
			want: "@sakila",
		},
		{
			typ:  drivertype.None,
			loc:  "sqlserver://sakila_p_ssW0rd@localhost?database=sakila",
			want: "@sakila",
		},
		{
			typ:   drivertype.None,
			loc:   "sqlserver://sakila_p_ssW0rd@localhost?database=sakila",
			want:  "@sakila2",
			taken: []string{"@sakila"},
		},
		{
			typ:  drivertype.Pg,
			loc:  "postgres://sakila_p_ssW0rd@localhost/sakila",
			want: "@sakila",
		},
		{
			typ:  drivertype.None,
			loc:  "postgres://sakila_p_ssW0rd@localhost/sakila",
			want: "@sakila",
		},
		{
			typ:  drivertype.Pg,
			loc:  "postgres://sakila_p_ssW0rd@localhost/sakila",
			want: "@sakila",
		},
		{
			typ:  drivertype.MySQL,
			loc:  "mysql://sakila_p_ssW0rd@localhost:3306/sakila",
			want: "@sakila",
		},
		{
			typ:  drivertype.None,
			loc:  "mysql://sakila_p_ssW0rd@localhost:3306/sakila",
			want: "@sakila",
		},
	}

	for i, tc := range testCases {
		t.Run(tu.Name(i, tc.typ, tc.loc), func(t *testing.T) {
			set := &source.Collection{}
			for i := range tc.taken {
				err := set.Add(&source.Source{
					Handle:   tc.taken[i],
					Type:     drivertype.SQLite,
					Location: "/tmp/taken.db",
				})
				require.NoError(t, err)
			}

			got, err := source.SuggestHandle(set, tc.typ, tc.loc)
			require.NoError(t, err)
			require.Equal(t, tc.want, got)
		})
	}
}

func TestIsValidHandle(t *testing.T) {
	testCases := []struct {
		in    string
		valid bool
	}{
		{"", false},
		{"@handle", true},
		{"@h1", true},
		{"@1handle", false},
		{"handle", false},
		{"@group/handle", true},
		{"@", false},
	}

	for i, tc := range testCases {
		tc := tc
		t.Run(tu.Name(i, tc.in), func(t *testing.T) {
			got := source.IsValidHandle(tc.in)
			require.Equal(t, tc.valid, got)
		})
	}
}

func TestValidGroup(t *testing.T) {
	testCases := []struct {
		in      string
		wantErr bool
	}{
		{"", false},         // root group is valid
		{"/", false},        // root group is valid
		{"prod", false},     // simple group
		{"prod/sub", false}, // nested group
		{" ", true},         // whitespace invalid
		{"//", true},        // double slash invalid
		{"/prod", true},     // leading slash invalid
		{"prod/", true},     // trailing slash invalid
		{"prod//sub", true}, // double slash invalid
		{"1prod", true},     // starts with digit
		{"@prod", true},     // @ not allowed
		{"prod sub", true},  // space not allowed
	}

	for i, tc := range testCases {
		tc := tc
		t.Run(tu.Name(i, tc.in), func(t *testing.T) {
			err := source.ValidGroup(tc.in)
			if tc.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestTable_String(t *testing.T) {
	testCases := []struct {
		tbl  source.Table
		want string
	}{
		{source.Table{Handle: "@sakila", Name: "actor"}, "@sakila.actor"},
		{source.Table{Handle: "@prod/db", Name: "users"}, "@prod/db.users"},
		{source.Table{Handle: "@src", Name: ""}, "@src."},
		{source.Table{Handle: "", Name: "tbl"}, ".tbl"},
	}

	for _, tc := range testCases {
		t.Run(tc.want, func(t *testing.T) {
			got := tc.tbl.String()
			require.Equal(t, tc.want, got)
		})
	}
}

func TestHandle2SafePath(t *testing.T) {
	testCases := []struct {
		handle string
		want   string
	}{
		{"@sakila", "sakila"},
		{"@prod/sakila", "prod__sakila"},
		{"@a/b/c/d", "a__b__c__d"},
		{"@handle_with_underscore", "handle_with_underscore"},
		{"", ""},
	}

	for i, tc := range testCases {
		tc := tc
		t.Run(tu.Name(i, tc.handle), func(t *testing.T) {
			got := source.Handle2SafePath(tc.handle)
			require.Equal(t, tc.want, got)
		})
	}
}

func TestParseTableHandle(t *testing.T) {
	testCases := []struct {
		input  string
		valid  bool
		handle string
		table  string
	}{
		{"@handle1", true, "@handle1", ""},
		{"  @handle1 ", true, "@handle1", ""},
		{"@handle1.tbl1", true, "@handle1", "tbl1"},
		{"  @handle1.tbl1  ", true, "@handle1", "tbl1"},
		{"@handle1 .tbl1", false, "", ""},
		{"@handle1. tbl1", false, "", ""},
		{"@handle1 . tbl1", false, "", ""},
		{".tbl1", true, "", "tbl1"},
		{" .tbl1 ", true, "", "tbl1"},
		{" ._tbl1 ", true, "", "_tbl1"},
		{"invalidhandle", false, "", ""},
		{"invalidhandle.tbl1", false, "", ""},
		{"invalidhandle.@tbl1", false, "", ""},
		{".invalid table", false, "", ""},
		{"", false, "", ""},
		{"  ", false, "", ""},
	}

	for i, tc := range testCases {
		t.Run(fmt.Sprintf("[%d]__%s", i, tc.input), func(t *testing.T) {
			handle, table, err := source.ParseTableHandle(tc.input)
			if tc.valid {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
			}
			assert.Equal(t, tc.handle, handle)
			assert.Equal(t, tc.table, table)
		})
	}
}
