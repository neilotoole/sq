package duckdb_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/libsq/core/kind"
	"github.com/neilotoole/sq/libsq/core/record"
	"github.com/neilotoole/sq/libsq/source"
	"github.com/neilotoole/sq/libsq/source/drivertype"
	"github.com/neilotoole/sq/libsq/source/metadata"
	"github.com/neilotoole/sq/testh"
	"github.com/neilotoole/sq/testh/sakila"
)

// TestSourceMetadata_Sakila verifies that SourceMetadata returns valid metadata
// for the sakila DuckDB fixture.
func TestSourceMetadata_Sakila(t *testing.T) {
	th := testh.New(t)
	src := th.Source(sakila.Duck)

	grip := th.Open(src)

	md, err := grip.SourceMetadata(context.Background(), false)
	require.NoError(t, err)
	require.Contains(t, md.DBProduct, "DuckDB")
	require.NotEmpty(t, md.DBVersion)
	require.NotEmpty(t, md.Tables)

	tableNames := make([]string, len(md.Tables))
	for i, tbl := range md.Tables {
		tableNames[i] = tbl.Name
	}
	require.Contains(t, tableNames, "actor")
	require.Contains(t, tableNames, "film")
}

// TestTableMetadata_Actor verifies that TableMetadata returns correct column
// metadata for the sakila "actor" table.
func TestTableMetadata_Actor(t *testing.T) {
	th := testh.New(t)
	src := th.Source(sakila.Duck)

	grip := th.Open(src)

	tblMeta, err := grip.TableMetadata(context.Background(), "actor")
	require.NoError(t, err)
	require.Equal(t, "actor", tblMeta.Name)
	require.NotEmpty(t, tblMeta.Columns)

	colNames := make([]string, len(tblMeta.Columns))
	for i, c := range tblMeta.Columns {
		colNames[i] = c.Name
	}
	require.Contains(t, colNames, "actor_id")
	require.Contains(t, colNames, "first_name")
	require.Contains(t, colNames, "last_name")
	require.Contains(t, colNames, "last_update")
}

// TestSakilaFixture_ForeignKeyCount pins the bundled sakila.duckdb
// fixture to 21 FK constraints (22 in the source minus the
// fk_store_staff cycle-breaker). Detects regressions in the portsakila
// tool that produce a fixture with the wrong FK count — those wouldn't
// surface in unit tests since the fixture is regenerated via a separate
// `go run ./.../portsakila` invocation.
func TestSakilaFixture_ForeignKeyCount(t *testing.T) {
	th := testh.New(t)
	src := th.Source(sakila.Duck)
	grip := th.Open(src)
	db, err := grip.DB(context.Background())
	require.NoError(t, err)

	var got int
	require.NoError(t, db.QueryRowContext(
		context.Background(),
		`SELECT count(*) FROM duckdb_constraints() WHERE constraint_type = 'FOREIGN KEY'`,
	).Scan(&got))
	require.Equal(t, 21, got,
		"sakila.duckdb must have 21 of 22 FKs preserved (only fk_store_staff is stripped)")
}

// TestTableMetadata_PrimaryKey asserts the contract of stmtPrimaryKeys (in
// metadata.go): it uses UNNEST so each PK column name is returned as its
// own row. A string-split implementation would pass simple identifiers
// but mis-split column names containing a comma or space.
//
// The single_pk + composite_pk subtests cover the basic shape (simple
// identifiers, single + multi-column). The whitespace_identifier subtest
// builds an in-memory DB with a PK column name containing a space and a
// comma, which is what UNNEST is actually load-bearing for.
func TestTableMetadata_PrimaryKey(t *testing.T) {
	th := testh.New(t)
	src := th.Source(sakila.Duck)
	grip := th.Open(src)
	ctx := context.Background()

	t.Run("single_pk", func(t *testing.T) {
		md, err := grip.TableMetadata(ctx, "actor")
		require.NoError(t, err)
		pkCols := pkColumnNames(md.Columns)
		require.Equal(t, []string{"actor_id"}, pkCols)
	})

	t.Run("composite_pk", func(t *testing.T) {
		md, err := grip.TableMetadata(ctx, "film_actor")
		require.NoError(t, err)
		pkCols := pkColumnNames(md.Columns)
		require.ElementsMatch(t, []string{"actor_id", "film_id"}, pkCols)
	})

	t.Run("whitespace_identifier", func(t *testing.T) {
		// Open a separate :memory: source so we don't pollute the shared
		// sakila fixture with an ad-hoc table.
		memSrc := &source.Source{
			Handle:   "@pk_whitespace",
			Type:     drivertype.DuckDB,
			Location: "duckdb://:memory:",
		}
		th.Add(memSrc)
		memGrip := th.Open(memSrc)
		memDB, err := memGrip.DB(ctx)
		require.NoError(t, err)

		// "first, last" contains both a comma and a space — exactly what a
		// string-split implementation would mishandle.
		_, err = memDB.ExecContext(ctx,
			`CREATE TABLE t ("first, last" VARCHAR, "age" INTEGER, PRIMARY KEY ("first, last"))`)
		require.NoError(t, err)

		md, err := memGrip.TableMetadata(ctx, "t")
		require.NoError(t, err)
		pkCols := pkColumnNames(md.Columns)
		require.Equal(t, []string{"first, last"}, pkCols)
	})
}

// TestSourceMetadata_Misc verifies multi-schema enumeration against the
// testdata/misc.duckdb fixture (foo, bar schemas).
func TestSourceMetadata_Misc(t *testing.T) {
	th := testh.New(t)
	src := th.Source("@miscdb_duck")
	grip := th.Open(src)
	drvr := grip.SQLDriver()

	ctx := context.Background()
	db, err := grip.DB(ctx)
	require.NoError(t, err)

	schemas, err := drvr.ListSchemas(ctx, db)
	require.NoError(t, err)
	require.Subset(t, schemas, []string{"foo", "bar"})

	fooTables, err := drvr.ListTableNames(ctx, db, "foo", true, true)
	require.NoError(t, err)
	require.Equal(t, []string{"t1"}, fooTables)

	barTables, err := drvr.ListTableNames(ctx, db, "bar", true, true)
	require.NoError(t, err)
	require.Equal(t, []string{"t2"}, barTables)
}

// TestSourceMetadata_Empty verifies that a DuckDB source with no user
// tables surfaces sensible metadata: catalog/schema set, table/view
// counts zero.
func TestSourceMetadata_Empty(t *testing.T) {
	th := testh.New(t)
	src := th.Source("@emptydb_duck")
	grip := th.Open(src)

	md, err := grip.SourceMetadata(context.Background(), false)
	require.NoError(t, err)
	require.NotEmpty(t, md.Name)
	require.NotEmpty(t, md.Schema)
	require.Zero(t, md.TableCount)
	require.Zero(t, md.ViewCount)
	require.Empty(t, md.Tables)
}

// TestRecordMeta_BlobScan verifies BLOB and NULL-BLOB rows scan cleanly
// through the type munge against the testdata/blob.duckdb fixture.
func TestRecordMeta_BlobScan(t *testing.T) {
	th := testh.New(t)
	src := th.Source("@blobdb_duck")
	grip := th.Open(src)

	ctx := context.Background()
	db, err := grip.DB(ctx)
	require.NoError(t, err)

	rows, err := db.QueryContext(ctx, `SELECT id, data FROM blobs ORDER BY id`)
	require.NoError(t, err)
	defer rows.Close()

	colTypes, err := rows.ColumnTypes()
	require.NoError(t, err)
	recMeta, newRecFn, err := grip.SQLDriver().RecordMeta(ctx, colTypes, nil)
	require.NoError(t, err)
	require.Equal(t, kind.Bytes, recMeta[1].Kind())

	var recs []record.Record
	for rows.Next() {
		scanRow := recMeta.NewScanRow()
		require.NoError(t, rows.Scan(scanRow...))
		rec, err := newRecFn(scanRow)
		require.NoError(t, err)
		recs = append(recs, rec)
	}
	require.NoError(t, rows.Err())
	require.Len(t, recs, 2)

	// Row 1: BLOB \x00\x01\x02\x03
	_, ok := recs[0][1].([]byte)
	require.True(t, ok, "row 1 BLOB should scan as []byte, got %T", recs[0][1])
	require.Equal(t, []byte{0x00, 0x01, 0x02, 0x03}, recs[0][1])

	// Row 2: NULL BLOB.
	require.Nil(t, recs[1][1])
}

// TestInspect_GeneratedColumn verifies that a column declared GENERATED ALWAYS
// AS is flagged as generated and that a regular DEFAULT column is not.
func TestInspect_GeneratedColumn(t *testing.T) {
	th := testh.New(t)
	src := &source.Source{
		Handle:   "@gen_col_duck",
		Type:     drivertype.DuckDB,
		Location: "duckdb://:memory:",
	}
	th.Add(src)
	grip := th.Open(src)
	ctx := context.Background()
	db, err := grip.DB(ctx)
	require.NoError(t, err)

	_, err = db.ExecContext(ctx, `CREATE TABLE t (
		id      INTEGER,
		price   DECIMAL(10,2) DEFAULT 9.99,
		tax     DECIMAL(10,2) GENERATED ALWAYS AS (price * 0.1),
		note    VARCHAR DEFAULT 'GENERATED ALWAYS AS legacy'
	)`)
	require.NoError(t, err)

	// A second plain table ensures the schema-wide map build in
	// getSchemaGeneratedColumns iterates over more than one table.
	_, err = db.ExecContext(ctx, `CREATE TABLE plain (id INTEGER, name VARCHAR)`)
	require.NoError(t, err)

	t.Run("per_table", func(t *testing.T) {
		md, err := grip.TableMetadata(ctx, "t")
		require.NoError(t, err)

		colByName := make(map[string]*metadata.Column, len(md.Columns))
		for _, col := range md.Columns {
			colByName[col.Name] = col
		}

		idCol := colByName["id"]
		require.NotNil(t, idCol)
		require.False(t, idCol.Generated, "plain column should not be marked generated")
		require.Empty(t, idCol.DefaultValue)

		priceCol := colByName["price"]
		require.NotNil(t, priceCol)
		require.False(t, priceCol.Generated, "DEFAULT column should not be marked generated")
		require.NotEmpty(t, priceCol.DefaultValue, "regular default should be preserved")

		taxCol := colByName["tax"]
		require.NotNil(t, taxCol)
		require.True(t, taxCol.Generated, "GENERATED ALWAYS AS column must be marked generated")
		require.NotEmpty(t, taxCol.GeneratedExpr, "GeneratedExpr must be populated")
		require.Empty(t, taxCol.DefaultValue, "generated expr must not appear as DefaultValue")

		noteCol := colByName["note"]
		require.NotNil(t, noteCol)
		require.False(t, noteCol.Generated,
			"column with GENERATED-keyword inside a string literal DEFAULT must NOT be marked generated")
		require.NotEmpty(t, noteCol.DefaultValue,
			"DEFAULT value must be preserved when literal contains GENERATED keyword")
		require.Contains(t, noteCol.DefaultValue, "legacy")
	})

	t.Run("source_level", func(t *testing.T) {
		srcMd, err := grip.SourceMetadata(ctx, false)
		require.NoError(t, err)

		var tbl *metadata.Table
		for _, tb := range srcMd.Tables {
			if tb.Name == "t" {
				tbl = tb
				break
			}
		}
		require.NotNil(t, tbl, "generated-column table must appear in source metadata")

		colByName := make(map[string]*metadata.Column, len(tbl.Columns))
		for _, col := range tbl.Columns {
			colByName[col.Name] = col
		}

		idCol := colByName["id"]
		require.NotNil(t, idCol)
		require.False(t, idCol.Generated, "plain column should not be marked generated")

		taxCol := colByName["tax"]
		require.NotNil(t, taxCol)
		require.True(t, taxCol.Generated,
			"GENERATED ALWAYS AS column must be marked generated in source-wide path")
		require.NotEmpty(t, taxCol.GeneratedExpr, "GeneratedExpr must be populated in source-wide path")
		require.Empty(t, taxCol.DefaultValue, "generated expr must not appear as DefaultValue")

		noteCol := colByName["note"]
		require.NotNil(t, noteCol)
		require.False(t, noteCol.Generated,
			"column with GENERATED-keyword inside a string literal DEFAULT must NOT be marked generated (source-wide path)")
		require.NotEmpty(t, noteCol.DefaultValue,
			"DEFAULT value must be preserved when literal contains GENERATED keyword (source-wide path)")
		require.Contains(t, noteCol.DefaultValue, "legacy")
	})
}

// TestInspect_CheckConstraint verifies that CHECK constraints are returned for
// both per-table and source-level inspect.
func TestInspect_CheckConstraint(t *testing.T) {
	th := testh.New(t)
	src := &source.Source{
		Handle:   "@check_duck",
		Type:     drivertype.DuckDB,
		Location: "duckdb://:memory:",
	}
	th.Add(src)
	grip := th.Open(src)
	ctx := context.Background()
	db, err := grip.DB(ctx)
	require.NoError(t, err)

	_, err = db.ExecContext(ctx, `CREATE TABLE t (
		id    INTEGER,
		price DECIMAL(10,2),
		CONSTRAINT chk_price CHECK (price > 0)
	)`)
	require.NoError(t, err)

	t.Run("per_table", func(t *testing.T) {
		md, err := grip.TableMetadata(ctx, "t")
		require.NoError(t, err)
		require.Len(t, md.CheckConstraints, 1)
		cc := md.CheckConstraints[0]
		// DuckDB auto-generates constraint names (the CONSTRAINT name clause
		// is not preserved in this version), so check only that a name is present.
		require.NotEmpty(t, cc.Name)
		require.Equal(t, "t", cc.Table)
		require.NotEmpty(t, cc.Clause)
	})

	t.Run("source_level", func(t *testing.T) {
		srcMd, err := grip.SourceMetadata(ctx, false)
		require.NoError(t, err)
		var tbl *metadata.Table
		for _, tb := range srcMd.Tables {
			if tb.Name == "t" {
				tbl = tb
				break
			}
		}
		require.NotNil(t, tbl)
		require.Len(t, tbl.CheckConstraints, 1)
		require.NotEmpty(t, tbl.CheckConstraints[0].Name)
	})
}

// TestInspect_ViewDefinition verifies that ViewDefinition is populated for
// both per-table and source-level inspect.
func TestInspect_ViewDefinition(t *testing.T) {
	th := testh.New(t)
	src := &source.Source{
		Handle:   "@view_def_duck",
		Type:     drivertype.DuckDB,
		Location: "duckdb://:memory:",
	}
	th.Add(src)
	grip := th.Open(src)
	ctx := context.Background()
	db, err := grip.DB(ctx)
	require.NoError(t, err)

	_, err = db.ExecContext(ctx, `CREATE TABLE t (id INT, name VARCHAR)`)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `CREATE VIEW v AS SELECT id, name FROM t`)
	require.NoError(t, err)

	t.Run("per_table", func(t *testing.T) {
		md, err := grip.TableMetadata(ctx, "v")
		require.NoError(t, err)
		require.NotEmpty(t, md.ViewDefinition)
		require.Contains(t, md.ViewDefinition, "SELECT")
	})

	t.Run("source_level", func(t *testing.T) {
		srcMd, err := grip.SourceMetadata(ctx, false)
		require.NoError(t, err)
		var viewTbl *metadata.Table
		for _, tbl := range srcMd.Tables {
			if tbl.Name == "v" {
				viewTbl = tbl
				break
			}
		}
		require.NotNil(t, viewTbl)
		require.NotEmpty(t, viewTbl.ViewDefinition)
		require.Contains(t, viewTbl.ViewDefinition, "SELECT")
	})
}

// pkColumnNames returns the names of columns where PrimaryKey is true.
func pkColumnNames(cols []*metadata.Column) []string {
	var names []string
	for _, c := range cols {
		if c.PrimaryKey {
			names = append(names, c.Name)
		}
	}
	return names
}

// TestSourceMetadata_ForeignKeys verifies that source-level inspect against
// the sakila DuckDB fixture populates Table.FK.Outgoing on the referencing
// tables and Table.FK.Incoming on the referenced tables, including for
// composite-PK link tables (film_actor, film_category).
func TestSourceMetadata_ForeignKeys(t *testing.T) {
	th := testh.New(t)
	src := th.Source(sakila.Duck)
	grip := th.Open(src)

	md, err := grip.SourceMetadata(context.Background(), false)
	require.NoError(t, err)

	tblByName := make(map[string]*metadata.Table, len(md.Tables))
	for _, tbl := range md.Tables {
		tblByName[tbl.Name] = tbl
	}

	// film_actor links actor + film with single-column FKs.
	filmActor := tblByName["film_actor"]
	require.NotNil(t, filmActor)
	require.NotNil(t, filmActor.FK)
	outgoingRefs := make([]string, 0, len(filmActor.FK.Outgoing))
	for _, fk := range filmActor.FK.Outgoing {
		outgoingRefs = append(outgoingRefs, fk.RefTable)
	}
	require.ElementsMatch(t, []string{"actor", "film"}, outgoingRefs)

	// actor.FK.Incoming should include the FK from film_actor.
	actor := tblByName["actor"]
	require.NotNil(t, actor)
	require.NotNil(t, actor.FK)
	incomingFromActor := make([]string, 0, len(actor.FK.Incoming))
	for _, fk := range actor.FK.Incoming {
		incomingFromActor = append(incomingFromActor, fk.Table)
	}
	require.Contains(t, incomingFromActor, "film_actor")

	// The sakila fixture's portsakila tool preserves 21 of 22 FKs
	// (fk_store_staff is the cycle-breaker). Verify the source-wide
	// outgoing count matches.
	totalOutgoing := 0
	for _, tbl := range md.Tables {
		if tbl.FK != nil {
			totalOutgoing += len(tbl.FK.Outgoing)
		}
	}
	require.Equal(t, 21, totalOutgoing,
		"sakila DuckDB fixture should report 21 outgoing FK constraints")
}

// TestTableMetadata_ForeignKeys verifies per-table inspect populates
// outgoing and incoming FKs.
func TestTableMetadata_ForeignKeys(t *testing.T) {
	th := testh.New(t)
	src := th.Source(sakila.Duck)
	grip := th.Open(src)
	ctx := context.Background()

	t.Run("outgoing", func(t *testing.T) {
		md, err := grip.TableMetadata(ctx, "film")
		require.NoError(t, err)
		require.NotNil(t, md.FK)
		// film has FKs to language (language_id, original_language_id).
		refs := make([]string, 0, len(md.FK.Outgoing))
		for _, fk := range md.FK.Outgoing {
			refs = append(refs, fk.RefTable)
		}
		require.Contains(t, refs, "language")
	})

	t.Run("incoming", func(t *testing.T) {
		md, err := grip.TableMetadata(ctx, "actor")
		require.NoError(t, err)
		require.NotNil(t, md.FK)
		owners := make([]string, 0, len(md.FK.Incoming))
		for _, fk := range md.FK.Incoming {
			owners = append(owners, fk.Table)
		}
		require.Contains(t, owners, "film_actor")
	})
}

// TestTableMetadata_CompositeForeignKey verifies that DuckDB composite FKs
// (multi-column REFERENCES) are populated with paired Columns / RefColumns.
func TestTableMetadata_CompositeForeignKey(t *testing.T) {
	th := testh.New(t)
	src := &source.Source{
		Handle:   "@composite_fk_duck",
		Type:     drivertype.DuckDB,
		Location: "duckdb://:memory:",
	}
	th.Add(src)
	grip := th.Open(src)
	ctx := context.Background()
	db, err := grip.DB(ctx)
	require.NoError(t, err)

	_, err = db.ExecContext(ctx,
		`CREATE TABLE parent (a INT, b INT, PRIMARY KEY (a, b))`)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `CREATE TABLE child (
        x INT, y INT,
        FOREIGN KEY (x, y) REFERENCES parent (a, b)
    )`)
	require.NoError(t, err)

	md, err := grip.TableMetadata(ctx, "child")
	require.NoError(t, err)
	require.NotNil(t, md.FK)
	require.Len(t, md.FK.Outgoing, 1)
	fk := md.FK.Outgoing[0]
	require.Equal(t, "parent", fk.RefTable)
	require.Equal(t, []string{"x", "y"}, fk.Columns)
	require.Equal(t, []string{"a", "b"}, fk.RefColumns)
}

// TestTableMetadata_ReservedWordForeignKeyColumn verifies that a
// DuckDB FK declared on a quoted reserved-word column ("from")
// round-trips through the loader with the identifier unwrapped in
// FK.Columns. A loader that stripped or double-quoted the identifier
// would surface the wrong name.
func TestTableMetadata_ReservedWordForeignKeyColumn(t *testing.T) {
	th := testh.New(t)
	src := &source.Source{
		Handle:   "@reserved_fk_duck",
		Type:     drivertype.DuckDB,
		Location: "duckdb://:memory:",
	}
	th.Add(src)
	grip := th.Open(src)
	ctx := context.Background()
	db, err := grip.DB(ctx)
	require.NoError(t, err)

	_, err = db.ExecContext(ctx, `CREATE TABLE parent (id INT PRIMARY KEY)`)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx,
		`CREATE TABLE child ("from" INT, FOREIGN KEY ("from") REFERENCES parent (id))`)
	require.NoError(t, err)

	md, err := grip.TableMetadata(ctx, "child")
	require.NoError(t, err)
	require.NotNil(t, md.FK)
	require.Len(t, md.FK.Outgoing, 1)
	fk := md.FK.Outgoing[0]
	require.Equal(t, "parent", fk.RefTable)
	require.Equal(t, []string{"from"}, fk.Columns,
		`quoted reserved-word column "from" must round-trip unquoted in Columns`)
	require.Equal(t, []string{"id"}, fk.RefColumns)
}

// TestTableMetadata_UniqueConstraints verifies UNIQUE-constraint
// introspection on inline and composite forms.
func TestTableMetadata_UniqueConstraints(t *testing.T) {
	th := testh.New(t)
	src := &source.Source{
		Handle:   "@unique_duck",
		Type:     drivertype.DuckDB,
		Location: "duckdb://:memory:",
	}
	th.Add(src)
	grip := th.Open(src)
	ctx := context.Background()
	db, err := grip.DB(ctx)
	require.NoError(t, err)

	_, err = db.ExecContext(ctx, `CREATE TABLE t (
        id INT PRIMARY KEY,
        email VARCHAR UNIQUE,
        first VARCHAR,
        last VARCHAR,
        UNIQUE (first, last)
    )`)
	require.NoError(t, err)

	md, err := grip.TableMetadata(ctx, "t")
	require.NoError(t, err)
	require.Len(t, md.UniqueConstraints, 2)

	colSets := make([][]string, 0, 2)
	for _, uc := range md.UniqueConstraints {
		require.Equal(t, "t", uc.Table)
		colSets = append(colSets, uc.Columns)
	}
	require.Contains(t, colSets, []string{"email"})
	require.Contains(t, colSets, []string{"first", "last"})
}

// TestTableMetadata_Indexes verifies that user-declared indexes are
// populated, that unique flags are honored, that composite indexes
// preserve column order, that reserved-word columns (which DuckDB
// re-quotes in duckdb_indexes().expressions) are unwrapped, and that
// functional-index keys become empty-string sentinels in Columns.
func TestTableMetadata_Indexes(t *testing.T) {
	th := testh.New(t)
	src := &source.Source{
		Handle:   "@indexes_duck",
		Type:     drivertype.DuckDB,
		Location: "duckdb://:memory:",
	}
	th.Add(src)
	grip := th.Open(src)
	ctx := context.Background()
	db, err := grip.DB(ctx)
	require.NoError(t, err)

	_, err = db.ExecContext(ctx, `CREATE TABLE t (id INT, name VARCHAR, email VARCHAR, "a""b" INTEGER)`)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `CREATE INDEX ix_name ON t(name)`)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `CREATE UNIQUE INDEX ix_email ON t(email)`)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `CREATE INDEX ix_lower_email ON t(LOWER(email))`)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `CREATE INDEX ix_comp ON t(name, email)`)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `CREATE INDEX ix_mixed ON t(name, LOWER(email))`)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `CREATE INDEX ix_quotecol ON t("a""b")`)
	require.NoError(t, err)

	md, err := grip.TableMetadata(ctx, "t")
	require.NoError(t, err)

	idxByName := make(map[string]*metadata.Index, len(md.Indexes))
	for _, idx := range md.Indexes {
		idxByName[idx.Name] = idx
	}

	// DuckDB re-quotes "name" because it collides with a reserved word —
	// the parser must unwrap the single-quoted-double-quoted form.
	require.Contains(t, idxByName, "ix_name")
	require.Equal(t, []string{"name"}, idxByName["ix_name"].Columns)
	require.False(t, idxByName["ix_name"].Unique)

	require.Contains(t, idxByName, "ix_email")
	require.Equal(t, []string{"email"}, idxByName["ix_email"].Columns)
	require.True(t, idxByName["ix_email"].Unique)

	// Composite indexes must preserve declaration order across the
	// reserved-word / bare-identifier mix.
	require.Contains(t, idxByName, "ix_comp")
	require.Equal(t, []string{"name", "email"}, idxByName["ix_comp"].Columns)

	// Functional-only indexes are dropped because every key is an
	// expression (Columns is all sentinels), so the index is omitted.
	require.NotContains(t, idxByName, "ix_lower_email")

	// A mixed plain/expression index preserves arity: the LOWER(email)
	// key position becomes the empty-string sentinel.
	require.Contains(t, idxByName, "ix_mixed")
	require.Equal(t, []string{"name", ""}, idxByName["ix_mixed"].Columns)

	// A column whose name contains a double-quote (DuckDB emits it as
	// `'"a""b"'` in the expressions list) must round-trip to its real
	// name, not be misclassified as an expression sentinel.
	require.Contains(t, idxByName, "ix_quotecol")
	require.Equal(t, []string{`a"b`}, idxByName["ix_quotecol"].Columns)
}

// TestSourceMetadata_SkipsFKMetadataOnViews ensures the FK / index /
// unique introspection path doesn't error against a schema that has
// views in addition to tables.
func TestSourceMetadata_SkipsFKMetadataOnViews(t *testing.T) {
	th := testh.New(t)
	src := &source.Source{
		Handle:   "@views_duck",
		Type:     drivertype.DuckDB,
		Location: "duckdb://:memory:",
	}
	th.Add(src)
	grip := th.Open(src)
	ctx := context.Background()
	db, err := grip.DB(ctx)
	require.NoError(t, err)

	_, err = db.ExecContext(ctx, `CREATE TABLE t (id INT PRIMARY KEY, name VARCHAR)`)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `CREATE VIEW v AS SELECT id FROM t`)
	require.NoError(t, err)

	md, err := grip.SourceMetadata(ctx, false)
	require.NoError(t, err)
	require.Len(t, md.Tables, 2)
}

// TestRecordMeta_BasicQuery verifies that RecordMeta correctly maps a
// simple query's column types to record.Meta with the right kinds.
func TestRecordMeta_BasicQuery(t *testing.T) {
	th := testh.New(t)
	src := th.Source(sakila.Duck)
	grip := th.Open(src)

	db, err := grip.DB(context.Background())
	require.NoError(t, err)

	rows, err := db.QueryContext(context.Background(),
		`SELECT actor_id, first_name, last_name, last_update FROM actor LIMIT 1`)
	require.NoError(t, err)
	defer rows.Close()

	colTypes, err := rows.ColumnTypes()
	require.NoError(t, err)

	recMeta, newRecFn, err := grip.SQLDriver().RecordMeta(context.Background(), colTypes, nil)
	require.NoError(t, err)
	require.NotNil(t, newRecFn)
	require.Len(t, recMeta, 4)

	require.Equal(t, "actor_id", recMeta[0].Name())
	require.Equal(t, kind.Int, recMeta[0].Kind())
	require.Equal(t, kind.Text, recMeta[1].Kind())     // first_name VARCHAR
	require.Equal(t, kind.Text, recMeta[2].Kind())     // last_name VARCHAR
	require.Equal(t, kind.Datetime, recMeta[3].Kind()) // last_update TIMESTAMP

	// Verify the munge function produces a valid record.
	require.True(t, rows.Next())
	scanRow := recMeta.NewScanRow()
	require.NoError(t, rows.Scan(scanRow...))
	rec, err := newRecFn(scanRow)
	require.NoError(t, err)
	require.Len(t, rec, 4)
	// actor_id should be a non-nil int64.
	require.NotNil(t, rec[0])
	_, ok := rec[0].(int64)
	require.True(t, ok, "actor_id should be int64, got %T", rec[0])
}
