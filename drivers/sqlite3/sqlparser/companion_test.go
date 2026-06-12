package sqlparser_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/drivers/sqlite3/sqlparser"
)

func TestRewriteCreateIndexStmt(t *testing.T) {
	testCases := []struct {
		name    string
		in      string
		newIdx  string
		newTbl  string
		want    string
		wantErr bool
	}{
		{
			name:   "basic",
			in:     `CREATE INDEX idx_actor_name ON actor (first_name)`,
			newIdx: `"idx_actor_name_actor2"`,
			newTbl: `"actor2"`,
			want:   `CREATE INDEX "idx_actor_name_actor2" ON "actor2" (first_name)`,
		},
		{
			name:   "unique_quoted",
			in:     `CREATE UNIQUE INDEX "idx_actor_name" ON "actor" ("first_name", "last_name")`,
			newIdx: `"idx2"`,
			newTbl: `"actor2"`,
			want:   `CREATE UNIQUE INDEX "idx2" ON "actor2" ("first_name", "last_name")`,
		},
		{
			name:   "schema_qualified_index_name",
			in:     `CREATE INDEX main.idx_actor_name ON actor (first_name)`,
			newIdx: `"idx2"`,
			newTbl: `"actor2"`,
			want:   `CREATE INDEX "idx2" ON "actor2" (first_name)`,
		},
		{
			name:   "dest_schema_qualifier",
			in:     `CREATE INDEX idx_actor_name ON actor (first_name)`,
			newIdx: `"aux"."idx2"`,
			newTbl: `"actor2"`,
			want:   `CREATE INDEX "aux"."idx2" ON "actor2" (first_name)`,
		},
		{
			name:   "partial_index_where_untouched",
			in:     `CREATE INDEX idx_p ON actor (first_name) WHERE first_name IS NOT NULL`,
			newIdx: `"idx_p2"`,
			newTbl: `"actor2"`,
			want:   `CREATE INDEX "idx_p2" ON "actor2" (first_name) WHERE first_name IS NOT NULL`,
		},
		{
			name:   "expression_index",
			in:     "CREATE INDEX idx_e ON actor (lower(first_name))",
			newIdx: `"idx_e2"`,
			newTbl: `"actor2"`,
			want:   `CREATE INDEX "idx_e2" ON "actor2" (lower(first_name))`,
		},
		{
			name:   "if_not_exists",
			in:     "CREATE INDEX IF NOT EXISTS idx_n ON actor (first_name)",
			newIdx: `"idx_n2"`,
			newTbl: `"actor2"`,
			want:   `CREATE INDEX IF NOT EXISTS "idx_n2" ON "actor2" (first_name)`,
		},
		{
			name:   "bracket_quoted",
			in:     "CREATE INDEX [idx one] ON [my table] ([first name])",
			newIdx: `"idx two"`,
			newTbl: `"other table"`,
			want:   `CREATE INDEX "idx two" ON "other table" ([first name])`,
		},
		{
			name:    "not_an_index_stmt",
			in:      `CREATE TABLE actor (id INTEGER)`,
			newIdx:  `"x"`,
			newTbl:  `"y"`,
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := sqlparser.RewriteCreateIndexStmt(tc.in, tc.newIdx, tc.newTbl)
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.want, got)
		})
	}
}

func TestRewriteCreateTriggerStmt(t *testing.T) {
	testCases := []struct {
		name    string
		in      string
		newTrg  string
		newTbl  string
		want    string
		wantErr bool
	}{
		{
			name: "before_insert_cross_table_body",
			in: `CREATE TRIGGER trg_bi BEFORE INSERT ON actor BEGIN ` +
				`INSERT INTO actor_log (msg) VALUES (NEW.first_name); END`,
			newTrg: `"trg_bi_actor2"`,
			newTbl: `"actor2"`,
			// The body's actor_log reference is a different table: untouched.
			want: `CREATE TRIGGER "trg_bi_actor2" BEFORE INSERT ON "actor2" BEGIN ` +
				`INSERT INTO actor_log (msg) VALUES (NEW.first_name); END`,
		},
		{
			name: "body_self_reference_rewritten",
			in: `CREATE TRIGGER trg_ai AFTER INSERT ON actor BEGIN ` +
				`UPDATE actor SET first_name = upper(NEW.first_name) WHERE id = NEW.id; END`,
			newTrg: `"trg_ai_actor2"`,
			newTbl: `"actor2"`,
			want: `CREATE TRIGGER "trg_ai_actor2" AFTER INSERT ON "actor2" BEGIN ` +
				`UPDATE "actor2" SET first_name = upper(NEW.first_name) WHERE id = NEW.id; END`,
		},
		{
			name: "body_self_reference_case_insensitive",
			in: `CREATE TRIGGER trg AFTER DELETE ON "Actor" BEGIN ` +
				`DELETE FROM ACTOR WHERE id = OLD.id; END`,
			newTrg: `"trg2"`,
			newTbl: `"actor2"`,
			want: `CREATE TRIGGER "trg2" AFTER DELETE ON "actor2" BEGIN ` +
				`DELETE FROM "actor2" WHERE id = OLD.id; END`,
		},
		{
			name: "qualified_column_ref_in_body_rewritten",
			in: `CREATE TRIGGER trg AFTER INSERT ON actor BEGIN ` +
				`INSERT INTO log (n) SELECT count(*) FROM actor WHERE actor.id > 0; END`,
			newTrg: `"trg2"`,
			newTbl: `"actor2"`,
			want: `CREATE TRIGGER "trg2" AFTER INSERT ON "actor2" BEGIN ` +
				`INSERT INTO log (n) SELECT count(*) FROM "actor2" WHERE "actor2".id > 0; END`,
		},
		{
			name: "update_of_columns",
			in: `CREATE TRIGGER trg AFTER UPDATE OF first_name, last_name ON actor BEGIN ` +
				`INSERT INTO log (msg) VALUES ('x'); END`,
			newTrg: `"trg2"`,
			newTbl: `"actor2"`,
			want: `CREATE TRIGGER "trg2" AFTER UPDATE OF first_name, last_name ON "actor2" BEGIN ` +
				`INSERT INTO log (msg) VALUES ('x'); END`,
		},
		{
			name: "when_clause_and_for_each_row",
			in: `CREATE TRIGGER trg BEFORE INSERT ON actor FOR EACH ROW WHEN NEW.id > 0 BEGIN ` +
				`SELECT RAISE(ABORT, 'no'); END`,
			newTrg: `"trg2"`,
			newTbl: `"actor2"`,
			want: `CREATE TRIGGER "trg2" BEFORE INSERT ON "actor2" FOR EACH ROW WHEN NEW.id > 0 BEGIN ` +
				`SELECT RAISE(ABORT, 'no'); END`,
		},
		{
			name: "schema_qualified_trigger_name",
			in: `CREATE TRIGGER main.trg BEFORE INSERT ON actor BEGIN ` +
				`INSERT INTO log (msg) VALUES ('x'); END`,
			newTrg: `"trg2"`,
			newTbl: `"actor2"`,
			want: `CREATE TRIGGER "trg2" BEFORE INSERT ON "actor2" BEGIN ` +
				`INSERT INTO log (msg) VALUES ('x'); END`,
		},
		{
			name: "trigger_name_same_as_table_untouched",
			in: `CREATE TRIGGER actor BEFORE INSERT ON actor BEGIN ` +
				`INSERT INTO log (msg) VALUES ('x'); END`,
			newTrg: `"actor_trg2"`,
			newTbl: `"actor2"`,
			want: `CREATE TRIGGER "actor_trg2" BEFORE INSERT ON "actor2" BEGIN ` +
				`INSERT INTO log (msg) VALUES ('x'); END`,
		},
		{
			name:    "not_a_trigger_stmt",
			in:      `CREATE INDEX idx ON actor (first_name)`,
			newTrg:  `"x"`,
			newTbl:  `"y"`,
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := sqlparser.RewriteCreateTriggerStmt(tc.in, tc.newTrg, tc.newTbl)
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.want, got)
		})
	}
}
