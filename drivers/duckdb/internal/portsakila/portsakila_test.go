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

func TestStripCycleBreakerFK_DropsOnlyStoreStaff(t *testing.T) {
	// Test that ONLY fk_store_staff is stripped — every other FK is
	// preserved. This is the cycle-breaker that lets us preserve 17
	// of 18 sakila FKs.
	in := `CREATE TABLE store (
  store_id INTEGER NOT NULL PRIMARY KEY,
  manager_staff_id SMALLINT NOT NULL,
  address_id INT NOT NULL,
  CONSTRAINT fk_store_staff FOREIGN KEY (manager_staff_id) REFERENCES staff (staff_id) ,
  CONSTRAINT fk_store_address FOREIGN KEY (address_id) REFERENCES address (address_id)
);
CREATE TABLE rental (
  rental_id INTEGER NOT NULL PRIMARY KEY,
  inventory_id INT NOT NULL,
  customer_id SMALLINT NOT NULL,
  staff_id SMALLINT NOT NULL,
  CONSTRAINT fk_rental_customer FOREIGN KEY (customer_id) REFERENCES customer (customer_id) ON DELETE CASCADE,
  CONSTRAINT fk_rental_inventory FOREIGN KEY (inventory_id) REFERENCES inventory (inventory_id) ON DELETE CASCADE,
  CONSTRAINT fk_rental_staff FOREIGN KEY (staff_id) REFERENCES staff (staff_id) ON DELETE CASCADE
);`

	out, err := Port(in)
	require.NoError(t, err)

	// Cycle-breaker is gone.
	require.NotContains(t, out, "fk_store_staff")

	// Every other FK is preserved.
	require.Contains(t, out, "CONSTRAINT fk_store_address")
	require.Contains(t, out, "CONSTRAINT fk_rental_customer")
	require.Contains(t, out, "CONSTRAINT fk_rental_inventory")
	require.Contains(t, out, "CONSTRAINT fk_rental_staff")

	// SQL is still valid: no orphan trailing comma before the closing paren
	// of the store block (we stripped the last FK constraint there).
	require.NotRegexp(t, `,\s*\n\s*\)`, out)
}

func TestStripCycleBreakerFK_NoCycleBreaker(t *testing.T) {
	// A table with no fk_store_staff constraint should be unchanged.
	in := `CREATE TABLE language (
  language_id TINYINT NOT NULL PRIMARY KEY,
  name CHAR(20) NOT NULL,
  last_update TIMESTAMP NOT NULL
);`

	out, err := Port(in)
	require.NoError(t, err)
	require.Equal(t, in, out)
}

func TestReplaceBlobSubTypeText(t *testing.T) {
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

func TestPortSchema_RealSakilaSchema(t *testing.T) {
	in, err := os.ReadFile("../../../sqlite3/testdata/sqlite-sakila-schema.sql")
	require.NoError(t, err)

	out, err := PortSchema(string(in))
	require.NoError(t, err)

	// Generic transforms applied.
	require.NotContains(t, out, "CREATE TRIGGER")
	require.NotContains(t, out, "AUTOINCREMENT")
	require.NotContains(t, out, "DATETIME('NOW')")
	require.NotContains(t, out, "BLOB SUB_TYPE TEXT")

	// Cycle-breaker stripped, every other FK preserved.
	require.NotContains(t, out, "fk_store_staff")
	inFKs := strings.Count(string(in), "FOREIGN KEY")
	outFKs := strings.Count(out, "FOREIGN KEY")
	require.Equal(t, inFKs-1, outFKs,
		"want 17 of 18 FKs preserved (input had %d, output has %d)", inFKs, outFKs)

	// Preservation checks: every table/index/view from the input remains.
	for _, kw := range []string{"CREATE TABLE", "CREATE  INDEX", "CREATE INDEX", "CREATE VIEW"} {
		inCount := strings.Count(string(in), kw)
		outCount := strings.Count(out, kw)
		require.Equal(t, inCount, outCount, "%s count mismatch", kw)
	}

	// Topological order: every CREATE TABLE references only tables that
	// appear earlier in the output.
	requireTopologicalCreateOrder(t, out)
}

// requireTopologicalCreateOrder verifies that within the CREATE TABLE
// region of the script, every FK references a table that has already
// been declared.
func requireTopologicalCreateOrder(t *testing.T, out string) {
	t.Helper()

	// Walk CREATE TABLE statements in order, building a set of declared
	// tables. For each table's FK lines, check that REFERENCES targets
	// are already in the set.
	declared := make(map[string]bool)
	lines := strings.Split(out, "\n")
	var curTable string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "CREATE TABLE ") {
			// Extract table name.
			rest := strings.TrimPrefix(trimmed, "CREATE TABLE ")
			rest = strings.TrimSpace(rest)
			parenIdx := strings.IndexByte(rest, '(')
			if parenIdx >= 0 {
				rest = rest[:parenIdx]
			}
			curTable = strings.TrimSpace(rest)
			declared[curTable] = true
			continue
		}
		if strings.HasPrefix(trimmed, "CREATE VIEW ") || strings.HasPrefix(trimmed, "CREATE  VIEW ") {
			// End of CREATE TABLE region.
			return
		}
		// Check for FK lines referencing other tables.
		if !strings.Contains(trimmed, "FOREIGN KEY") {
			continue
		}
		refIdx := strings.Index(trimmed, "REFERENCES ")
		if refIdx < 0 {
			continue
		}
		rest := trimmed[refIdx+len("REFERENCES "):]
		spaceIdx := strings.IndexAny(rest, " (")
		if spaceIdx < 0 {
			continue
		}
		referenced := strings.TrimSpace(rest[:spaceIdx])
		require.True(t, declared[referenced],
			"table %q references %q which is not declared yet", curTable, referenced)
	}
}

func TestPortInsertData_RealSakilaInsertData(t *testing.T) {
	in, err := os.ReadFile("../../../sqlite3/testdata/sqlite-sakila-insert-data.sql")
	require.NoError(t, err)

	out, err := PortInsertData(string(in))
	require.NoError(t, err)

	// Same number of INSERT statements as the input.
	require.Equal(t, strings.Count(string(in), "Insert into"),
		strings.Count(out, "Insert into"))

	// Topological order: store INSERTs come before staff INSERTs (the
	// cycle resolution point) — this is the only swap that matters for
	// FK enforcement.
	storeIdx := strings.Index(out, "Insert into store")
	staffIdx := strings.Index(out, "Insert into staff")
	require.GreaterOrEqual(t, storeIdx, 0, "store INSERTs must be present")
	require.GreaterOrEqual(t, staffIdx, 0, "staff INSERTs must be present")
	require.Less(t, storeIdx, staffIdx,
		"store INSERTs must precede staff INSERTs after cycle-breaker fix")
}

func TestPortDropObjects_RealSakilaDrop(t *testing.T) {
	in, err := os.ReadFile("../../../sqlite3/testdata/sqlite-sakila-drop-objects.sql")
	require.NoError(t, err)

	out, err := PortDropObjects(string(in))
	require.NoError(t, err)

	// Same number of DROP TABLE statements.
	require.Equal(t, strings.Count(string(in), "DROP TABLE"),
		strings.Count(out, "DROP TABLE"))

	// Topological order: staff DROP must come before store DROP (with
	// staff→store FK kept, dropping store first would violate FK).
	staffIdx := strings.Index(out, "DROP TABLE staff")
	storeIdx := strings.Index(out, "DROP TABLE store")
	require.GreaterOrEqual(t, staffIdx, 0)
	require.GreaterOrEqual(t, storeIdx, 0)
	require.Less(t, staffIdx, storeIdx,
		"staff must be dropped before store (staff has FK to store)")
}
