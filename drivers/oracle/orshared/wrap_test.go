package orshared

import (
	"errors"
	"fmt"
	"testing"

	goora "github.com/sijms/go-ora/v2/network"
	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/libsq/driver"
)

// TestWrap covers the error-translation paths: nil passthrough, the two
// "object does not exist" codes that map to driver.NotExistError, and a
// generic Oracle/standard error that is annotated but not reclassified.
func TestWrap(t *testing.T) {
	t.Parallel()

	require.NoError(t, Wrap(nil), "nil must pass through")

	var notExist *driver.NotExistError

	tableNotFound := Wrap(goora.NewOracleError(ErrCodeTableNotFound))
	require.Error(t, tableNotFound)
	require.ErrorAs(t, tableNotFound, &notExist,
		"ORA-00942 must classify as NotExistError")

	notExist = nil
	invalidIdent := Wrap(goora.NewOracleError(ErrCodeInvalidIdentifier))
	require.Error(t, invalidIdent)
	require.ErrorAs(t, invalidIdent, &notExist,
		"ORA-00904 must classify as NotExistError")

	// A different Oracle code is annotated but not reclassified.
	notExist = nil
	other := Wrap(goora.NewOracleError(1))
	require.Error(t, other)
	require.False(t, errors.As(other, &notExist),
		"unrelated Oracle codes must not become NotExistError")

	// A plain standard error is wrapped without reclassification.
	notExist = nil
	std := Wrap(errors.New("boom"))
	require.Error(t, std)
	require.False(t, errors.As(std, &notExist))
}

// TestHasErrCode covers nil, the zero-code guard, a matching code, a
// non-matching code, and matching through an fmt.Errorf wrap.
func TestHasErrCode(t *testing.T) {
	t.Parallel()

	require.False(t, HasErrCode(nil, ErrCodeTableNotFound))
	require.False(t, HasErrCode(goora.NewOracleError(ErrCodeTableNotFound), 0),
		"code 0 is the no-Oracle-error sentinel and must never match")

	oraErr := goora.NewOracleError(ErrCodeTableNotFound)
	require.True(t, HasErrCode(oraErr, ErrCodeTableNotFound))
	require.False(t, HasErrCode(oraErr, ErrCodeInvalidIdentifier))
	require.True(t, HasErrCode(fmt.Errorf("exec: %w", oraErr), ErrCodeTableNotFound),
		"must match through fmt.Errorf wrapping")
	require.False(t, HasErrCode(errors.New("not an oracle error"), ErrCodeTableNotFound))
}

// TestIsErrTableNotExist pins ORA-00942 detection, which DropTable relies on
// to honor ifExists.
func TestIsErrTableNotExist(t *testing.T) {
	t.Parallel()

	require.False(t, IsErrTableNotExist(nil))
	require.True(t, IsErrTableNotExist(goora.NewOracleError(ErrCodeTableNotFound)))
	require.True(t, IsErrTableNotExist(fmt.Errorf("exec: %w",
		goora.NewOracleError(ErrCodeTableNotFound))))
	require.False(t, IsErrTableNotExist(goora.NewOracleError(ErrCodeInvalidIdentifier)),
		"ORA-00904 is a different error and must not match")
}
