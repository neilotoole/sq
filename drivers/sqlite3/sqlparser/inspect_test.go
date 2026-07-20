package sqlparser_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/drivers/sqlite3/sqlparser"
)

func TestExtractCheckConstraints(t *testing.T) {
	testCases := []struct {
		name string
		stmt string
		want []sqlparser.CheckClause
	}{
		{
			name: "none",
			stmt: `CREATE TABLE t (id INTEGER PRIMARY KEY, name TEXT)`,
			want: nil,
		},
		{
			name: "column_level_unnamed",
			stmt: `CREATE TABLE t (id INTEGER PRIMARY KEY, price INTEGER CHECK (price > 0))`,
			want: []sqlparser.CheckClause{{Name: "", Clause: "price > 0"}},
		},
		{
			name: "table_level_unnamed",
			stmt: `CREATE TABLE t (price INTEGER, discount INTEGER, CHECK (price > discount))`,
			want: []sqlparser.CheckClause{{Name: "", Clause: "price > discount"}},
		},
		{
			name: "named_table_level",
			stmt: `CREATE TABLE t (price INTEGER, CONSTRAINT chk_pos CHECK (price > 0))`,
			want: []sqlparser.CheckClause{{Name: "chk_pos", Clause: "price > 0"}},
		},
		{
			name: "multiple",
			stmt: `CREATE TABLE t (a INTEGER CHECK (a > 0), b INTEGER, CHECK (b < 100))`,
			want: []sqlparser.CheckClause{
				{Name: "", Clause: "a > 0"},
				{Name: "", Clause: "b < 100"},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := sqlparser.ExtractCheckConstraints(tc.stmt)
			require.NoError(t, err)
			require.Equal(t, tc.want, got)
		})
	}
}

func TestExtractCheckConstraints_Malformed(t *testing.T) {
	_, err := sqlparser.ExtractCheckConstraints(`CREATE TABLE this is not valid (((`)
	require.Error(t, err)
}

func TestExtractTriggerTimingEvents(t *testing.T) {
	testCases := []struct {
		name       string
		stmt       string
		wantTiming string
		wantEvents []string
	}{
		{
			name:       "after_insert",
			stmt:       `CREATE TRIGGER trg AFTER INSERT ON t BEGIN UPDATE t SET n = n + 1; END`,
			wantTiming: "AFTER",
			wantEvents: []string{"INSERT"},
		},
		{
			name:       "before_update",
			stmt:       `CREATE TRIGGER trg BEFORE UPDATE ON t BEGIN SELECT 1; END`,
			wantTiming: "BEFORE",
			wantEvents: []string{"UPDATE"},
		},
		{
			name:       "before_delete",
			stmt:       `CREATE TRIGGER trg BEFORE DELETE ON t BEGIN SELECT 1; END`,
			wantTiming: "BEFORE",
			wantEvents: []string{"DELETE"},
		},
		{
			name:       "instead_of_insert_on_view",
			stmt:       `CREATE TRIGGER trg INSTEAD OF INSERT ON v BEGIN SELECT 1; END`,
			wantTiming: "INSTEAD OF",
			wantEvents: []string{"INSERT"},
		},
		{
			name:       "update_of_columns",
			stmt:       `CREATE TRIGGER trg AFTER UPDATE OF a, b ON t BEGIN SELECT 1; END`,
			wantTiming: "AFTER",
			wantEvents: []string{"UPDATE"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			timing, events, err := sqlparser.ExtractTriggerTimingEvents(tc.stmt)
			require.NoError(t, err)
			require.Equal(t, tc.wantTiming, timing)
			require.Equal(t, tc.wantEvents, events)
		})
	}
}

func TestExtractTriggerTimingEvents_Malformed(t *testing.T) {
	_, _, err := sqlparser.ExtractTriggerTimingEvents(`CREATE TRIGGER ((( not valid`)
	require.Error(t, err)
}

func TestExtractColumnDDLInfo(t *testing.T) {
	t.Run("autoincrement", func(t *testing.T) {
		got, err := sqlparser.ExtractColumnDDLInfo(
			`CREATE TABLE t (id INTEGER PRIMARY KEY AUTOINCREMENT, name TEXT)`,
		)
		require.NoError(t, err)
		require.True(t, got["id"].AutoIncrement)
		require.Empty(t, got["id"].GeneratedExpr)
		_, hasName := got["name"]
		require.False(t, hasName)
	})

	t.Run("generated_stored", func(t *testing.T) {
		got, err := sqlparser.ExtractColumnDDLInfo(
			`CREATE TABLE t (a INTEGER, b INTEGER GENERATED ALWAYS AS (a * 2) STORED)`,
		)
		require.NoError(t, err)
		require.Equal(t, "a * 2", got["b"].GeneratedExpr)
		require.False(t, got["b"].AutoIncrement)
	})

	t.Run("generated_virtual_short_form", func(t *testing.T) {
		got, err := sqlparser.ExtractColumnDDLInfo(
			`CREATE TABLE t (a INTEGER, b INTEGER AS (a + 1) VIRTUAL)`,
		)
		require.NoError(t, err)
		require.Equal(t, "a + 1", got["b"].GeneratedExpr)
	})

	t.Run("pk_no_autoincrement", func(t *testing.T) {
		// A bare INTEGER PRIMARY KEY is a rowid alias, NOT an explicit
		// AUTOINCREMENT column; it must not be flagged AutoIncrement.
		got, err := sqlparser.ExtractColumnDDLInfo(
			`CREATE TABLE t (id INTEGER PRIMARY KEY, name TEXT)`,
		)
		require.NoError(t, err)
		require.False(t, got["id"].AutoIncrement, "bare INTEGER PRIMARY KEY is not AUTOINCREMENT")
		_, hasID := got["id"]
		require.False(t, hasID, "a column with no DDL-only attributes is omitted from the map")
	})

	t.Run("none", func(t *testing.T) {
		got, err := sqlparser.ExtractColumnDDLInfo(`CREATE TABLE t (a INTEGER, b TEXT)`)
		require.NoError(t, err)
		require.Empty(t, got)
	})
}

func TestExtractColumnDDLInfo_Malformed(t *testing.T) {
	_, err := sqlparser.ExtractColumnDDLInfo(`NOT A CREATE TABLE`)
	require.Error(t, err)
}
