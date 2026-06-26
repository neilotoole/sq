package mysql_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/drivers/mysql"
	"github.com/neilotoole/sq/libsq/core/kind"
	"github.com/neilotoole/sq/libsq/core/lg"
	"github.com/neilotoole/sq/libsq/core/lg/lgt"
	"github.com/neilotoole/sq/libsq/core/stringz"
	"github.com/neilotoole/sq/libsq/core/tablefq"
	"github.com/neilotoole/sq/libsq/source/metadata"
	"github.com/neilotoole/sq/testh"
	"github.com/neilotoole/sq/testh/sakila"
	"github.com/neilotoole/sq/testh/tu"
)

func TestKindFromDBTypeName(t *testing.T) {
	t.Parallel()

	ctx := lg.NewContext(context.Background(), lgt.New(t))

	testCases := map[string]kind.Kind{
		"":                 kind.Unknown,
		"INTEGER":          kind.Int,
		"INT":              kind.Int,
		"SMALLINT":         kind.Int,
		"TINYINT":          kind.Int,
		"MEDIUMINT":        kind.Int,
		"BIGINT":           kind.Int,
		"BIT":              kind.Int,
		"DECIMAL":          kind.Decimal,
		"DECIMAL(5,2)":     kind.Decimal,
		"NUMERIC":          kind.Decimal,
		"FLOAT":            kind.Float,
		"FLOAT(8)":         kind.Float,
		"FLOAT(7,4)":       kind.Float,
		"REAL":             kind.Float,
		"DOUBLE":           kind.Float,
		"DOUBLE PRECISION": kind.Float,
		"DATE":             kind.Date,
		"DATETIME":         kind.Datetime,
		"TIMESTAMP":        kind.Datetime,
		"TIME":             kind.Time,
		"YEAR":             kind.Int,
		"CHAR":             kind.Text,
		"VARCHAR":          kind.Text,
		"VARCHAR(64)":      kind.Text,
		"TINYTEXT":         kind.Text,
		"TEXT":             kind.Text,
		"MEDIUMTEXT":       kind.Text,
		"LONGTEXT":         kind.Text,
		"BINARY":           kind.Bytes,
		"BINARY(4)":        kind.Bytes,
		"VARBINARY":        kind.Bytes,
		"BLOB":             kind.Bytes,
		"MEDIUMBLOB":       kind.Bytes,
		"LONGBLOB":         kind.Bytes,
		"ENUM":             kind.Text,
		"SET":              kind.Text,
		"BOOL":             kind.Bool,
		"BOOLEAN":          kind.Bool,
	}

	for dbTypeName, wantKind := range testCases {
		gotKind := mysql.KindFromDBTypeName(ctx, "col", dbTypeName)
		require.Equal(t, wantKind, gotKind, "{%s} should produce %s but got %s", dbTypeName, wantKind, gotKind)
	}
}

func TestDatabase_SourceMetadata_MySQL(t *testing.T) {
	t.Parallel()

	handles := sakila.MyAll()
	for _, handle := range handles {
		t.Run(handle, func(t *testing.T) {
			t.Parallel()

			th, _, _, grip, _ := testh.NewWith(t, handle)
			md, err := grip.SourceMetadata(th.Context, false)
			require.NoError(t, err)
			require.Equal(t, "sakila", md.Name)
			require.Equal(t, handle, md.Handle)

			tblActor := md.Tables[0]
			require.Equal(t, sakila.TblActor, tblActor.Name)
			require.Equal(t, int64(sakila.TblActorCount), tblActor.RowCount)
			require.Equal(t, len(sakila.TblActorCols()), len(tblActor.Columns))
		})
	}
}

func TestDatabase_TableMetadata(t *testing.T) {
	t.Parallel()

	for _, handle := range sakila.MyAll() {
		t.Run(handle, func(t *testing.T) {
			t.Parallel()

			th, _, _, grip, _ := testh.NewWith(t, handle)
			md, err := grip.TableMetadata(th.Context, sakila.TblActor)
			require.NoError(t, err)
			require.Equal(t, sakila.TblActor, md.Name)
		})
	}
}

func TestGetTableRowCounts(t *testing.T) {
	th, _, _, _, db := testh.NewWith(t, sakila.My)

	counts, err := mysql.GetTableRowCountsBatch(th.Context, db, []string{sakila.TblActor, sakila.TblFilm})
	require.NoError(t, err)
	require.Len(t, counts, 2)
	require.Equal(t, int64(sakila.TblActorCount), counts[sakila.TblActor])
	require.Equal(t, int64(sakila.TblFilmCount), counts[sakila.TblFilm])
}

// TestIndexes_ExpressionArity_MySQL verifies that MySQL 8 functional
// index keys (COLUMN_NAME NULL in INFORMATION_SCHEMA.STATISTICS) are
// preserved as empty-string sentinels, keeping composite arity/position.
func TestIndexes_ExpressionArity_MySQL(t *testing.T) {
	tu.SkipShort(t, true)
	t.Parallel()

	th := testh.New(t)
	src := th.Source(sakila.My) // sakila.My == MySQL 8 (functional indexes)
	db := th.OpenDB(src)

	tbl := stringz.UniqTableName("idx_arity")
	_, err := db.ExecContext(th.Context,
		"CREATE TABLE "+tbl+" (a INT, b VARCHAR(64), c INT)")
	require.NoError(t, err)
	t.Cleanup(func() { th.DropTable(src, tablefq.From(tbl)) })

	_, err = db.ExecContext(th.Context,
		"CREATE INDEX ix_mixed ON "+tbl+" (a, (LOWER(b)), c)")
	require.NoError(t, err)
	_, err = db.ExecContext(th.Context,
		"CREATE INDEX ix_allexpr ON "+tbl+" ((LOWER(b)))")
	require.NoError(t, err)

	md, err := th.Open(src).TableMetadata(th.Context, tbl)
	require.NoError(t, err)

	var mixed *metadata.Index
	for _, idx := range md.Indexes {
		require.NotEqual(t, "ix_allexpr", idx.Name,
			"an all-expression index must be omitted")
		if idx.Name == "ix_mixed" {
			mixed = idx
		}
	}
	require.NotNil(t, mixed, "ix_mixed should be reported")
	require.Equal(t, []string{"a", "", "c"}, mixed.Columns,
		"the (LOWER(b)) key position must be the empty-string sentinel")
}

// TestForeignKey_CompositeOrdering_MySQL verifies that a composite FK
// preserves the declared column pairing across (Columns, RefColumns).
// The parent PK uses (b, a) descending while the child FK uses
// (x, y) ascending so any loader bug that sorts either side
// independently — or pairs by name rather than by position — is
// caught (an alphabetic-on-both-sides fixture would let such a bug
// slip past). Looped across MyAll() because INFORMATION_SCHEMA
// column casing has historically differed between MySQL 5.6/5.7 and
// 8.0 — a real regression vector for this loader.
func TestForeignKey_CompositeOrdering_MySQL(t *testing.T) {
	tu.SkipShort(t, true)
	t.Parallel()

	for _, handle := range sakila.MyAll() {
		t.Run(handle, func(t *testing.T) {
			t.Parallel()

			th := testh.New(t)
			src := th.Source(handle)
			db := th.OpenDB(src)

			parent := stringz.UniqTableName("fk_comp_parent")
			child := stringz.UniqTableName("fk_comp_child")
			_, err := db.ExecContext(th.Context,
				"CREATE TABLE "+parent+" (a INT, b INT, PRIMARY KEY (b, a)) ENGINE=InnoDB")
			require.NoError(t, err)
			t.Cleanup(func() { th.DropTable(src, tablefq.From(parent)) })
			_, err = db.ExecContext(th.Context,
				"CREATE TABLE "+child+" (x INT, y INT, FOREIGN KEY (x, y) REFERENCES "+parent+
					" (b, a)) ENGINE=InnoDB")
			require.NoError(t, err)
			t.Cleanup(func() { th.DropTable(src, tablefq.From(child)) })

			md, err := th.Open(src).TableMetadata(th.Context, child)
			require.NoError(t, err)
			require.NotNil(t, md.FK)
			require.Len(t, md.FK.Outgoing, 1)
			fk := md.FK.Outgoing[0]
			require.Equal(t, parent, fk.RefTable)
			require.Equal(t, []string{"x", "y"}, fk.Columns)
			require.Equal(t, []string{"b", "a"}, fk.RefColumns)
		})
	}
}

// TestMySQL_ColumnFlags verifies that AUTO_INCREMENT, GENERATED, GeneratedExpr,
// and Collation are populated on MySQL column metadata, both through the
// per-table path (TableMetadata) and the source-wide path (SourceMetadata).
func TestMySQL_ColumnFlags(t *testing.T) {
	tu.SkipShort(t, true)
	t.Parallel()

	th := testh.New(t)
	src := th.Source(sakila.My)
	db := th.OpenDB(src)

	tbl := stringz.UniqTableName("col_flags")
	_, err := db.ExecContext(th.Context, `CREATE TABLE `+tbl+` (
  id         INT AUTO_INCREMENT PRIMARY KEY,
  name       VARCHAR(100) COLLATE utf8mb4_bin NOT NULL,
  name_upper VARCHAR(100) COLLATE utf8mb4_bin GENERATED ALWAYS AS (UPPER(name)) STORED,
  ts         TIMESTAMP DEFAULT CURRENT_TIMESTAMP
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`)
	require.NoError(t, err)
	t.Cleanup(func() { th.DropTable(src, tablefq.From(tbl)) })

	// Per-table path.
	md, err := th.Open(src).TableMetadata(th.Context, tbl)
	require.NoError(t, err)
	require.Len(t, md.Columns, 4)

	colID := md.Columns[0]
	require.True(t, colID.AutoIncrement, "id should be AUTO_INCREMENT")
	require.False(t, colID.Generated, "id should not be GENERATED")

	colName := md.Columns[1]
	require.False(t, colName.AutoIncrement)
	require.False(t, colName.Generated)
	require.Equal(t, "utf8mb4_bin", colName.Collation)

	colGen := md.Columns[2]
	require.False(t, colGen.AutoIncrement)
	require.True(t, colGen.Generated, "name_upper should be GENERATED")
	require.NotEmpty(t, colGen.GeneratedExpr, "GeneratedExpr should be set")
	require.Equal(t, "utf8mb4_bin", colGen.Collation)

	// Negative case: ts has DEFAULT CURRENT_TIMESTAMP, which MySQL reports as
	// EXTRA="DEFAULT_GENERATED..."; the substring "GENERATED" must NOT cause
	// Generated=true when GENERATION_EXPRESSION is empty.
	colTS := md.Columns[3]
	require.False(t, colTS.Generated, "ts has an expression default, not a generated column")
	require.Empty(t, colTS.GeneratedExpr, "GeneratedExpr should be empty for expression-default column")

	// Source-wide path: verify the same table's columns are also mapped.
	srcMd, err := th.Open(src).SourceMetadata(th.Context, false)
	require.NoError(t, err)
	var srcTbl *metadata.Table
	for _, t2 := range srcMd.Tables {
		if t2.Name == tbl {
			srcTbl = t2
			break
		}
	}
	require.NotNil(t, srcTbl, "table should appear in source metadata")
	require.Len(t, srcTbl.Columns, 4)
	require.True(t, srcTbl.Columns[0].AutoIncrement)
	require.Equal(t, "utf8mb4_bin", srcTbl.Columns[1].Collation)
	require.True(t, srcTbl.Columns[2].Generated)
	require.False(t, srcTbl.Columns[3].Generated, "ts: expression default must not be flagged as generated")
}

// TestMySQL_CheckConstraints verifies that named CHECK constraints are
// returned by the per-table and source-wide metadata paths on MySQL 8.
func TestMySQL_CheckConstraints(t *testing.T) {
	tu.SkipShort(t, true)
	t.Parallel()

	th := testh.New(t)
	src := th.Source(sakila.My)
	db := th.OpenDB(src)

	tbl := stringz.UniqTableName("chk_constr")
	_, err := db.ExecContext(th.Context, `CREATE TABLE `+tbl+` (
  id    INT PRIMARY KEY,
  score INT NOT NULL,
  CONSTRAINT chk_score_positive CHECK (score > 0)
) ENGINE=InnoDB`)
	require.NoError(t, err)
	t.Cleanup(func() { th.DropTable(src, tablefq.From(tbl)) })

	// Per-table path.
	md, err := th.Open(src).TableMetadata(th.Context, tbl)
	require.NoError(t, err)
	require.NotEmpty(t, md.CheckConstraints, "expected at least one CHECK constraint")

	var found *metadata.CheckConstraint
	for _, cc := range md.CheckConstraints {
		if cc.Name == "chk_score_positive" {
			found = cc
			break
		}
	}
	require.NotNil(t, found, "chk_score_positive should be present")
	require.Equal(t, tbl, found.Table)
	require.NotEmpty(t, found.Clause)

	// Source-wide path.
	srcMd, err := th.Open(src).SourceMetadata(th.Context, false)
	require.NoError(t, err)
	var srcTbl *metadata.Table
	for _, t2 := range srcMd.Tables {
		if t2.Name == tbl {
			srcTbl = t2
			break
		}
	}
	require.NotNil(t, srcTbl)
	var found2 *metadata.CheckConstraint
	for _, cc := range srcTbl.CheckConstraints {
		if cc.Name == "chk_score_positive" {
			found2 = cc
			break
		}
	}
	require.NotNil(t, found2, "chk_score_positive should appear in source-wide metadata")
}

// TestMySQL_Triggers verifies that trigger metadata (Timing, Events,
// Definition) is populated, and that Enabled is nil (MySQL has no
// per-trigger enabled/disabled state).
func TestMySQL_Triggers(t *testing.T) {
	tu.SkipShort(t, true)
	t.Parallel()

	th := testh.New(t)
	src := th.Source(sakila.My)
	db := th.OpenDB(src)

	tbl := stringz.UniqTableName("trig_tbl")
	_, err := db.ExecContext(th.Context, `CREATE TABLE `+tbl+` (
  id   INT AUTO_INCREMENT PRIMARY KEY,
  val  INT NOT NULL DEFAULT 0
) ENGINE=InnoDB`)
	require.NoError(t, err)
	t.Cleanup(func() { th.DropTable(src, tablefq.From(tbl)) })

	_, err = db.ExecContext(th.Context,
		`CREATE TRIGGER trg_`+tbl+` BEFORE INSERT ON `+tbl+` FOR EACH ROW SET NEW.val = ABS(NEW.val)`)
	require.NoError(t, err)

	// Per-table path.
	md, err := th.Open(src).TableMetadata(th.Context, tbl)
	require.NoError(t, err)
	require.NotEmpty(t, md.Triggers, "expected at least one trigger")

	trig := md.Triggers[0]
	require.Equal(t, "BEFORE", trig.Timing)
	require.Equal(t, []string{"INSERT"}, trig.Events)
	require.NotEmpty(t, trig.Definition)
	require.Nil(t, trig.Enabled, "MySQL has no per-trigger enabled state; Enabled must be nil")

	// Source-wide path.
	srcMd, err := th.Open(src).SourceMetadata(th.Context, false)
	require.NoError(t, err)
	var srcTbl *metadata.Table
	for _, t2 := range srcMd.Tables {
		if t2.Name == tbl {
			srcTbl = t2
			break
		}
	}
	require.NotNil(t, srcTbl)
	require.NotEmpty(t, srcTbl.Triggers)
	require.Nil(t, srcTbl.Triggers[0].Enabled)
}

// TestMySQL_ViewDefinition verifies that view definition SQL is populated
// for view-typed tables, via both the per-table and source-wide paths.
func TestMySQL_ViewDefinition(t *testing.T) {
	tu.SkipShort(t, true)
	t.Parallel()

	th := testh.New(t)
	src := th.Source(sakila.My)
	db := th.OpenDB(src)

	tbl := stringz.UniqTableName("vdef_base")
	view := stringz.UniqTableName("vdef_view")

	_, err := db.ExecContext(th.Context,
		`CREATE TABLE `+tbl+` (id INT PRIMARY KEY, label VARCHAR(50)) ENGINE=InnoDB`)
	require.NoError(t, err)
	t.Cleanup(func() { th.DropTable(src, tablefq.From(tbl)) })

	_, err = db.ExecContext(th.Context,
		`CREATE VIEW `+view+` AS SELECT id, label FROM `+tbl+` WHERE id > 0`)
	require.NoError(t, err)
	// Use DROP VIEW (not th.DropTable, which issues DROP TABLE and does
	// NOT drop a MySQL view). Registered after the base-table cleanup so
	// LIFO ordering drops the view first, leaving no dangling view that
	// would break a concurrent SourceMetadata's getTableRowCounts.
	t.Cleanup(func() {
		_, _ = db.ExecContext(th.Context, "DROP VIEW IF EXISTS "+view)
	})

	// Per-table path.
	md, err := th.Open(src).TableMetadata(th.Context, view)
	require.NoError(t, err)
	require.NotEmpty(t, md.ViewDefinition, "ViewDefinition must be set for a view")

	// Source-wide path.
	srcMd, err := th.Open(src).SourceMetadata(th.Context, false)
	require.NoError(t, err)
	var srcView *metadata.Table
	for _, t2 := range srcMd.Tables {
		if t2.Name == view {
			srcView = t2
			break
		}
	}
	require.NotNil(t, srcView)
	require.NotEmpty(t, srcView.ViewDefinition, "ViewDefinition must be set in source-wide metadata")
}

// TestForeignKey_OnDeleteOnUpdate_MySQL pins that the loader populates
// OnDelete / OnUpdate from INFORMATION_SCHEMA.REFERENTIAL_CONSTRAINTS
// with the explicit non-default actions ("CASCADE" / "SET NULL").
// Looped across MyAll() — MySQL only enforces FKs under InnoDB (the
// DDL pins ENGINE=InnoDB explicitly) and the REFERENTIAL_CONSTRAINTS
// view's column casing has shifted between 5.6/5.7 and 8.0, so a
// per-version regression in the loader's column-name unmarshaling
// would surface here.
func TestForeignKey_OnDeleteOnUpdate_MySQL(t *testing.T) {
	tu.SkipShort(t, true)
	t.Parallel()

	for _, handle := range sakila.MyAll() {
		t.Run(handle, func(t *testing.T) {
			t.Parallel()

			th := testh.New(t)
			src := th.Source(handle)
			db := th.OpenDB(src)

			parent := stringz.UniqTableName("fk_act_parent")
			child := stringz.UniqTableName("fk_act_child")
			_, err := db.ExecContext(th.Context,
				"CREATE TABLE "+parent+" (id INT PRIMARY KEY) ENGINE=InnoDB")
			require.NoError(t, err)
			t.Cleanup(func() { th.DropTable(src, tablefq.From(parent)) })
			_, err = db.ExecContext(th.Context,
				"CREATE TABLE "+child+" (parent_id INT, FOREIGN KEY (parent_id) REFERENCES "+parent+
					" (id) ON DELETE CASCADE ON UPDATE SET NULL) ENGINE=InnoDB")
			require.NoError(t, err)
			t.Cleanup(func() { th.DropTable(src, tablefq.From(child)) })

			md, err := th.Open(src).TableMetadata(th.Context, child)
			require.NoError(t, err)
			require.Len(t, md.FK.Outgoing, 1)
			fk := md.FK.Outgoing[0]
			require.Equal(t, "CASCADE", fk.OnDelete)
			require.Equal(t, "SET NULL", fk.OnUpdate)
		})
	}
}
