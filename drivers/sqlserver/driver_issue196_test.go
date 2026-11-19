package sqlserver_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/libsq/core/stringz"
	"github.com/neilotoole/sq/libsq/core/tablefq"
	"github.com/neilotoole/sq/testh"
	"github.com/neilotoole/sq/testh/sakila"
)

// Test_MSSQLDB_DriverIssue196 illustrates a bug in the go-mssqldb driver.
// When attempting to insert a NULL value into BINARY or VARBINARY columns,
// the go-mssqldb driver treats a Go nil value incorrectly.
// A workaround is to explicitly insert "([]byte)(nil)" instead
// of "nil" which the driver then handles correctly.
// See https://github.com/denisenkom/go-mssqldb/issues/196
func Test_MSSQLDB_DriverIssue196(t *testing.T) {
	t.Parallel()

	// Note that although this test only checks BINARY, the same
	// behavior is expected of VARBINARY.
	const (
		canonicalTblName = "type_test_issue_196"
		insertTpl        = "INSERT INTO %s (col_binary_n) VALUES %s"
		createStmtTpl    = `CREATE TABLE type_test_issue_196 (col_binary_n BINARY(255))`
	)

	actualTblName := stringz.UniqTableName(canonicalTblName)
	createStmt := strings.Replace(createStmtTpl, canonicalTblName, actualTblName, 1)

	// Create the demonstration table
	th := testh.New(t)
	src := th.Source(sakila.MS)
	db := th.OpenDB(src)
	_, err := db.ExecContext(th.Context, createStmt)
	require.NoError(t, err)

	t.Logf("Created table %s.%s", src.Handle, actualTblName)

	// Drop the newly-created table on cleanup
	t.Cleanup(func() {
		th.DropTable(src, tablefq.From(actualTblName))
	})

	// Build the INSERT statement
	placeholders := th.SQLDriverFor(src).Dialect().Placeholders(1, 1)
	insertStmt := fmt.Sprintf(insertTpl, actualTblName, placeholders)

	// Insert empty byte slice, should work
	_, err = db.ExecContext(th.Context, insertStmt, []byte{})
	require.NoError(t, err, "empty byte slice insert is expected to work with this go-mssqldb driver version")

	// Insert non-empty byte slice, should work
	_, err = db.ExecContext(th.Context, insertStmt, []byte("hello"))
	require.NoError(t, err, "non-empty byte slice insert is expected to work with this go-mssqldb driver version")

	// Insert ([]byte)(nil), should work
	_, err = db.ExecContext(th.Context, insertStmt, []byte(nil))
	require.NoError(t, err, "([]byte)(nil) insert is expected to work with this go-mssqldb driver version")

	// Insert nil, expected to fail with this driver version
	_, err = db.ExecContext(th.Context, insertStmt, nil)
	require.Error(t, err, "nil insert is, alas, expected to fail with this go-mssqldb driver version")
}
