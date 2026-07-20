package mysql

import (
	"testing"

	"github.com/go-sql-driver/mysql"
	"github.com/stretchr/testify/require"
)

// TestIsMissingInfoSchemaTable verifies that both the MySQL 8.0+ code
// (ER_NO_SUCH_TABLE, 1146) and the MySQL 5.6 code (ER_UNKNOWN_TABLE, 1109)
// are recognized as "information_schema table absent", so the CHECK_CONSTRAINTS
// inspect path degrades gracefully on older servers instead of erroring.
func TestIsMissingInfoSchemaTable(t *testing.T) {
	require.True(t, isMissingInfoSchemaTable(&mysql.MySQLError{Number: errNumTableNotExist}),
		"1146 (ER_NO_SUCH_TABLE, MySQL 8.0+) should be recognized")
	require.True(t, isMissingInfoSchemaTable(&mysql.MySQLError{Number: errNumUnknownTable}),
		"1109 (ER_UNKNOWN_TABLE, MySQL 5.6) should be recognized")
	require.False(t, isMissingInfoSchemaTable(&mysql.MySQLError{Number: errNumUnknownColumn}),
		"1054 (unknown column) is a different failure mode")
	require.False(t, isMissingInfoSchemaTable(nil), "nil is not a missing-table error")
}
