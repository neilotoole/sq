package sqlserver

import (
	"errors"
	"testing"

	mssql "github.com/microsoft/go-mssqldb"
	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/libsq/driver"
)

func Test_placeholders(t *testing.T) {
	testCases := []struct {
		numCols int
		numRows int
		want    string
	}{
		{numCols: 0, numRows: 0, want: ""},
		{numCols: 1, numRows: 1, want: "(@p1)"},
		{numCols: 2, numRows: 1, want: "(@p1, @p2)"},
		{numCols: 1, numRows: 2, want: "(@p1), (@p2)"},
		{numCols: 2, numRows: 2, want: "(@p1, @p2), (@p3, @p4)"},
	}

	for _, tc := range testCases {
		got := placeholders(tc.numCols, tc.numRows)
		require.Equal(t, tc.want, got)
	}
}

func Test_hasErrCode(t *testing.T) {
	const wantCode = 100
	var err error

	require.False(t, hasErrCode(nil, wantCode))
	err = errors.New("huzzah")
	require.False(t, hasErrCode(err, wantCode))

	err = mssql.Error{
		Number: wantCode,
	}

	require.True(t, hasErrCode(err, wantCode))
}

// Test_isObjectVanishedErr pins the predicate that a source-wide metadata scan
// uses to tolerate objects dropped mid-scan by concurrent DDL: error 15009
// (sp_spaceused, object does not exist), error 208 (invalid object name, e.g.
// from the view row-count fallback), and error 4413 (view binding errors,
// when a view's underlying object vanished). In production the errors arrive
// wrapped via errw (which maps 208 to driver.NotExistError), so wrapped forms
// are covered too. See issue #1027.
func Test_isObjectVanishedErr(t *testing.T) {
	testCases := []struct {
		name string
		err  error
		want bool
	}{
		{name: "nil", err: nil, want: false},
		{name: "object_not_exist_15009", err: mssql.Error{Number: errCodeObjectNotExist}, want: true},
		{name: "object_not_exist_15009_wrapped", err: errw(mssql.Error{Number: errCodeObjectNotExist}), want: true},
		{name: "bad_object_208", err: mssql.Error{Number: errCodeBadObject}, want: true},
		{name: "bad_object_208_wrapped", err: errw(mssql.Error{Number: errCodeBadObject}), want: true},
		{name: "view_binding_4413", err: mssql.Error{Number: errCodeViewBindingErr}, want: true},
		{name: "view_binding_4413_wrapped", err: errw(mssql.Error{Number: errCodeViewBindingErr}), want: true},
		{
			name: "not_exist_error",
			err:  driver.NewNotExistError(errors.New("table {t} not found")),
			want: true,
		},
		{name: "identity_insert_544", err: mssql.Error{Number: errCodeIdentityInsert}, want: false},
		{name: "wrapped_other_code", err: errw(mssql.Error{Number: errCodeIdentityInsert}), want: false},
		{name: "non_mssql_error", err: errors.New("huzzah"), want: false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, isObjectVanishedErr(tc.err))
		})
	}
}

// Test_isErrDeadlockVictim pins the retry predicate for catalog metadata
// queries chosen as a deadlock victim (error 1205) by concurrent DDL. See
// issue #1031.
func Test_isErrDeadlockVictim(t *testing.T) {
	require.False(t, isErrDeadlockVictim(nil))
	require.False(t, isErrDeadlockVictim(errors.New("huzzah")))
	require.False(t, isErrDeadlockVictim(mssql.Error{Number: errCodeBadObject}))
	require.True(t, isErrDeadlockVictim(mssql.Error{Number: errCodeDeadlockVictim}))
	require.True(t, isErrDeadlockVictim(errw(mssql.Error{Number: errCodeDeadlockVictim})))
}

func TestParseSemver(t *testing.T) {
	testCases := []struct {
		raw     string
		want    string
		wantErr bool
	}{
		{raw: "16.0.4115.5", want: "v16.0.4115"}, // four-part; regex caps at three
		{raw: "15.0.2000.5", want: "v15.0.2000"},
		{raw: "not-a-version", wantErr: true},
		{raw: "", wantErr: true},
	}

	for _, tc := range testCases {
		t.Run(tc.raw, func(t *testing.T) {
			got, err := parseSemver(tc.raw)
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.want, got)
		})
	}
}
