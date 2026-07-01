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
}
