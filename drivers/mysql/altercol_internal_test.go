package mysql

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestExtractColumnDef(t *testing.T) {
	const showCreate = "CREATE TABLE `t` (\n" +
		"  `id` int(11) NOT NULL AUTO_INCREMENT,\n" +
		"  `first_name` varchar(45) NOT NULL DEFAULT 'x' COMMENT 'hi',\n" +
		"  `age` int(11) DEFAULT NULL,\n" +
		"  PRIMARY KEY (`id`)\n" +
		") ENGINE=InnoDB DEFAULT CHARSET=latin1"

	got, err := extractColumnDef(showCreate, "first_name")
	require.NoError(t, err)
	require.Equal(t, "varchar(45) NOT NULL DEFAULT 'x' COMMENT 'hi'", got)

	got, err = extractColumnDef(showCreate, "id")
	require.NoError(t, err)
	require.Equal(t, "int(11) NOT NULL AUTO_INCREMENT", got)

	_, err = extractColumnDef(showCreate, "missing")
	require.Error(t, err)

	// The last column in the table has no trailing comma (it's immediately
	// followed by the closing paren, not another column). This proves
	// TrimSuffix(def, ",") correctly no-ops instead of chopping a character
	// off the definition.
	const showCreateNoTrailingComma = "CREATE TABLE `t` (\n" +
		"  `id` int(11) NOT NULL AUTO_INCREMENT,\n" +
		"  `last_col` varchar(20) NOT NULL\n" +
		") ENGINE=InnoDB DEFAULT CHARSET=latin1"

	got, err = extractColumnDef(showCreateNoTrailingComma, "last_col")
	require.NoError(t, err)
	require.Equal(t, "varchar(20) NOT NULL", got)

	// `id` is a prefix of `id2`. Extracting `id` must return `id`'s
	// definition, not `id2`'s: the closing backtick baked into the match
	// prefix ("`id` ") prevents "`id`" from matching a line that starts with
	// "`id2`".
	const showCreatePrefixCollision = "CREATE TABLE `t` (\n" +
		"  `id` int(11) NOT NULL AUTO_INCREMENT,\n" +
		"  `id2` varchar(10) NOT NULL,\n" +
		"  PRIMARY KEY (`id`)\n" +
		") ENGINE=InnoDB DEFAULT CHARSET=latin1"

	got, err = extractColumnDef(showCreatePrefixCollision, "id")
	require.NoError(t, err)
	require.Equal(t, "int(11) NOT NULL AUTO_INCREMENT", got)

	got, err = extractColumnDef(showCreatePrefixCollision, "id2")
	require.NoError(t, err)
	require.Equal(t, "varchar(10) NOT NULL", got)
}
