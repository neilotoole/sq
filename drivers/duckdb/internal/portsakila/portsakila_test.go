package portsakila

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestStripTriggers(t *testing.T) {
	in := `BEGIN TRANSACTION;
CREATE TABLE actor (id INTEGER PRIMARY KEY);

CREATE TRIGGER actor_trigger_ai AFTER INSERT ON actor
 BEGIN
  UPDATE actor SET last_update = DATETIME('NOW') WHERE rowid = new.rowid;
 END
;

CREATE TABLE film (id INTEGER PRIMARY KEY);
COMMIT;
`

	out, err := Port(in)
	require.NoError(t, err)
	require.NotContains(t, out, "CREATE TRIGGER")
	require.NotContains(t, out, "DATETIME('NOW')")
	require.Contains(t, out, "CREATE TABLE actor")
	require.Contains(t, out, "CREATE TABLE film")
	require.True(t, strings.Contains(out, "BEGIN TRANSACTION"))
	require.True(t, strings.Contains(out, "COMMIT"))
}

func TestStripAutoincrement(t *testing.T) {
	in := `CREATE TABLE x (id INTEGER PRIMARY KEY AUTOINCREMENT NOT NULL);`
	out, err := Port(in)
	require.NoError(t, err)
	require.Equal(t, `CREATE TABLE x (id INTEGER PRIMARY KEY NOT NULL);`, out)
}

func TestStripFKConstraintLines(t *testing.T) {
	// Test that CONSTRAINT ... FOREIGN KEY lines are removed while
	// preserving column definitions and maintaining valid SQL structure.
	in := `CREATE TABLE rental (
  rental_id INTEGER NOT NULL PRIMARY KEY,
  rental_date TIMESTAMP NOT NULL,
  inventory_id INT NOT NULL,
  customer_id SMALLINT NOT NULL,
  return_date TIMESTAMP,
  staff_id SMALLINT NOT NULL,
  last_update TIMESTAMP NOT NULL,
  CONSTRAINT fk_rental_customer FOREIGN KEY (customer_id) REFERENCES customer (customer_id) ON DELETE CASCADE,
  CONSTRAINT fk_rental_inventory FOREIGN KEY (inventory_id) REFERENCES inventory (inventory_id) ON DELETE CASCADE,
  CONSTRAINT fk_rental_staff FOREIGN KEY (staff_id) REFERENCES staff (staff_id) ON DELETE CASCADE
);`

	out, err := Port(in)
	require.NoError(t, err)

	// FK constraint lines must be removed.
	require.NotContains(t, out, "CONSTRAINT fk_rental_customer")
	require.NotContains(t, out, "CONSTRAINT fk_rental_inventory")
	require.NotContains(t, out, "CONSTRAINT fk_rental_staff")
	require.NotContains(t, out, "FOREIGN KEY")
	require.NotContains(t, out, "REFERENCES")

	// Column definitions must be preserved.
	require.Contains(t, out, "rental_id INTEGER NOT NULL PRIMARY KEY")
	require.Contains(t, out, "customer_id SMALLINT NOT NULL")
	require.Contains(t, out, "staff_id SMALLINT NOT NULL")

	// SQL must be valid: no orphan trailing commas before closing paren.
	require.NotRegexp(t, `,\s*\n\s*\)`, out)

	// The closing paren and semicolon must be intact.
	require.Contains(t, out, ");")
}

func TestStripFKConstraintLines_SingleFK(t *testing.T) {
	// Test with a single FK constraint (simplest case).
	in := `CREATE TABLE film_actor (
  actor_id SMALLINT NOT NULL,
  film_id SMALLINT NOT NULL,
  last_update TIMESTAMP NOT NULL,
  CONSTRAINT fk_film_actor_actor FOREIGN KEY (actor_id) REFERENCES actor (actor_id)
);`

	out, err := Port(in)
	require.NoError(t, err)

	require.NotContains(t, out, "FOREIGN KEY")
	require.Contains(t, out, "actor_id SMALLINT NOT NULL")
	require.Contains(t, out, "film_id SMALLINT NOT NULL")
	require.NotRegexp(t, `,\s*\n\s*\)`, out)
}

func TestStripFKConstraintLines_NoFK(t *testing.T) {
	// Test that tables without FK constraints are unchanged.
	in := `CREATE TABLE language (
  language_id TINYINT NOT NULL PRIMARY KEY,
  name CHAR(20) NOT NULL,
  last_update TIMESTAMP NOT NULL
);`

	out, err := Port(in)
	require.NoError(t, err)

	// Input and output should be identical.
	require.Equal(t, in, out)
}

func TestReplaceBlobSubTypeText(t *testing.T) {
	// Test that BLOB SUB_TYPE TEXT is replaced with TEXT.
	in := `CREATE TABLE film (
  film_id SMALLINT NOT NULL PRIMARY KEY,
  title VARCHAR(128) NOT NULL,
  description BLOB SUB_TYPE TEXT DEFAULT NULL,
  release_year YEAR,
  last_update TIMESTAMP NOT NULL
);`

	out, err := Port(in)
	require.NoError(t, err)

	require.NotContains(t, out, "BLOB SUB_TYPE TEXT")
	require.Contains(t, out, "description TEXT DEFAULT NULL")
}

func TestReplaceBlobSubTypeText_MultipleColumns(t *testing.T) {
	// Test with multiple BLOB SUB_TYPE TEXT columns.
	in := `CREATE TABLE test_table (
  id INTEGER PRIMARY KEY,
  col1 BLOB SUB_TYPE TEXT NOT NULL,
  col2 VARCHAR(50),
  col3 BLOB SUB_TYPE TEXT
);`

	out, err := Port(in)
	require.NoError(t, err)

	require.NotContains(t, out, "BLOB SUB_TYPE TEXT")
	require.Contains(t, out, "col1 TEXT NOT NULL")
	require.Contains(t, out, "col3 TEXT")
}

func TestPort_RealSakilaSchema(t *testing.T) {
	in, err := os.ReadFile("../../../sqlite3/testdata/sqlite-sakila-schema.sql")
	require.NoError(t, err)

	out, err := Port(string(in))
	require.NoError(t, err)

	require.NotContains(t, out, "CREATE TRIGGER")
	require.NotContains(t, out, "AUTOINCREMENT")
	require.NotContains(t, out, "DATETIME('NOW')")
	require.NotContains(t, out, "FOREIGN KEY")
	require.NotContains(t, out, "BLOB SUB_TYPE TEXT")

	// Preservation checks: every table/index/view from the input must remain.
	for _, kw := range []string{"CREATE TABLE", "CREATE  INDEX", "CREATE INDEX", "CREATE VIEW"} {
		inCount := strings.Count(string(in), kw)
		outCount := strings.Count(out, kw)
		require.Equal(t, inCount, outCount, "%s count mismatch", kw)
	}
}
