package drivers_test

import (
	"strings"
	"testing"

	"github.com/neilotoole/sq/drivers/sqlite3"

	"golang.org/x/exp/slices"

	"github.com/neilotoole/sq/cli/output"

	"github.com/neilotoole/sq/drivers/mysql"

	"github.com/neilotoole/sq/libsq/source"

	"github.com/neilotoole/sq/libsq"
	"github.com/stretchr/testify/require"

	_ "github.com/mattn/go-sqlite3"
	"github.com/neilotoole/sq/testh"
	"github.com/neilotoole/sq/testh/sakila"
)

//nolint:exhaustive,lll
func TestSLQ2SQLNew(t *testing.T) {
	testCases := []struct {
		// name is the test name
		name string

		// in is the SLQ input. The "@sakila" handle is replaced
		// with the source's actual handle before an individual
		// test cases is executed.
		in string

		// wantErr indicates that an error is expected
		wantErr bool

		// wantSQL is the wanted SQL
		wantSQL string

		// onlyFor indicates that this test should only run on sources of
		// the specified types. When empty, the test is executed on all types.
		onlyFor []source.Type

		// overrideWantSQL allows an alternative "wantSQL" for a specific driver type.
		// For example, MySQL uses backtick as the quote char, so it needs
		// a separate wantSQL string.
		overrideWantSQL map[source.Type]string

		// skip indicates the test should be skipped. Useful for test cases
		// that we wantSQL to implement in the future.
		skip bool

		// skipExec indicates that the resulting query should not be executed.
		// Some SLQ inputs we wantSQL to test don't actually have corresponding
		// data in the Sakila datasets.
		skipExec bool

		// wantRecs is the number of expected records from executing the query.
		wantRecs int
	}{
		{
			name:            "select/cols",
			in:              `@sakila | .actor | .first_name, .last_name`,
			wantSQL:         `SELECT "first_name", "last_name" FROM "actor"`,
			overrideWantSQL: map[source.Type]string{mysql.Type: "SELECT `first_name`, `last_name` FROM `actor`"},
			wantRecs:        sakila.TblActorCount,
		},
		{
			name:            "select/cols-whitespace-single-col",
			in:              `@sakila | .actor | ."first name"`,
			wantSQL:         `SELECT "first name" FROM "actor"`,
			overrideWantSQL: map[source.Type]string{mysql.Type: "SELECT `first name` FROM `actor`"},
			wantRecs:        sakila.TblActorCount,
			skipExec:        true,
		},
		{
			name:            "select/cols-whitespace-multiple-cols",
			in:              `@sakila | .actor | .actor_id, ."first name", ."last name"`,
			wantSQL:         `SELECT "actor_id", "first name", "last name" FROM "actor"`,
			overrideWantSQL: map[source.Type]string{mysql.Type: "SELECT `actor_id`, `first name`, `last name` FROM `actor`"},
			wantRecs:        sakila.TblActorCount,
			skipExec:        true,
		},
		{
			name:            "select/count-whitespace-col",
			in:              `@sakila | .actor | count(."first name")`,
			wantSQL:         `SELECT count("first name") FROM "actor"`,
			overrideWantSQL: map[source.Type]string{mysql.Type: "SELECT count(`first name`) FROM `actor`"},
			skipExec:        true,
		},
		{
			name:            "select/table-whitespace",
			in:              `@sakila | ."film actor"`,
			wantSQL:         `SELECT * FROM "film actor"`,
			overrideWantSQL: map[source.Type]string{mysql.Type: "SELECT * FROM `film actor`"},
			skipExec:        true,
		},
		{
			name:            "select/cols-aliases",
			in:              `@sakila | .actor | .first_name:given_name, .last_name:family_name`,
			wantSQL:         `SELECT "first_name" AS "given_name", "last_name" AS "family_name" FROM "actor"`,
			overrideWantSQL: map[source.Type]string{mysql.Type: "SELECT `first_name` AS `given_name`, `last_name` AS `family_name` FROM `actor`"},
			wantRecs:        sakila.TblActorCount,
		},
		{
			name:            "select/count-star",
			in:              `@sakila | .actor | count(*)`,
			wantSQL:         `SELECT count(*) FROM "actor"`,
			overrideWantSQL: map[source.Type]string{mysql.Type: "SELECT count(*) FROM `actor`"},
			wantRecs:        1,
		},
		{
			name:            "select/count",
			in:              `@sakila | .actor | count()`,
			wantSQL:         `SELECT count(*) FROM "actor"`,
			overrideWantSQL: map[source.Type]string{mysql.Type: "SELECT count(*) FROM `actor`"},
			wantRecs:        1,
		},
		{
			name:            "select/handle-table/cols",
			in:              `@sakila.actor | .first_name, .last_name`,
			wantSQL:         `SELECT "first_name", "last_name" FROM "actor"`,
			overrideWantSQL: map[source.Type]string{mysql.Type: "SELECT `first_name`, `last_name` FROM `actor`"},
			wantRecs:        sakila.TblActorCount,
		},
		{
			name:            "select/handle-table/count-star",
			in:              `@sakila.actor | count(*)`,
			wantSQL:         `SELECT count(*) FROM "actor"`,
			overrideWantSQL: map[source.Type]string{mysql.Type: "SELECT count(*) FROM `actor`"},
			wantRecs:        1,
		},
		{
			name:            "select/handle-table/count-col",
			in:              `@sakila.actor | count(."first name")`,
			wantSQL:         `SELECT count("first name") FROM "actor"`,
			overrideWantSQL: map[source.Type]string{mysql.Type: "SELECT count(`first name`) FROM `actor`"},
			skipExec:        true,
		},
		{
			name:            "select/count-alias",
			in:              `@sakila | .actor | count(*):quantity`,
			wantSQL:         `SELECT count(*) AS "quantity" FROM "actor"`,
			overrideWantSQL: map[source.Type]string{mysql.Type: "SELECT count(*) AS `quantity` FROM `actor`"},
			wantRecs:        1,
		},
		{
			name:            "filter/equal",
			in:              `@sakila | .actor | .actor_id == 1`,
			wantSQL:         `SELECT * FROM "actor" WHERE "actor_id" = 1`,
			overrideWantSQL: map[source.Type]string{mysql.Type: "SELECT * FROM `actor` WHERE `actor_id` = 1"},
			wantRecs:        1,
		},
		{
			name:            "join/single-selector",
			in:              `@sakila | .actor, .film_actor | join(.actor_id)`,
			wantSQL:         `SELECT * FROM "actor" INNER JOIN "film_actor" ON "actor"."actor_id" = "film_actor"."actor_id"`,
			overrideWantSQL: map[source.Type]string{mysql.Type: "SELECT * FROM `actor` INNER JOIN `film_actor` ON `actor`.`actor_id` = `film_actor`.`actor_id`"},
			wantRecs:        sakila.TblFilmActorCount,
		},
		{
			name:            "join/fq-table-cols-equal",
			in:              `@sakila | .actor, .film_actor | join(.film_actor.actor_id == .actor.actor_id)`,
			wantSQL:         `SELECT * FROM "actor" INNER JOIN "film_actor" ON "film_actor"."actor_id" = "actor"."actor_id"`,
			overrideWantSQL: map[source.Type]string{mysql.Type: "SELECT * FROM `actor` INNER JOIN `film_actor` ON `film_actor`.`actor_id` = `actor`.`actor_id`"},
			wantRecs:        sakila.TblFilmActorCount,
		},
		{
			name:            "join/fq-table-cols-equal-whitespace",
			in:              `@sakila | .actor, ."film actor" | join(."film actor".actor_id == .actor.actor_id)`,
			wantSQL:         `SELECT * FROM "actor" INNER JOIN "film actor" ON "film actor"."actor_id" = "actor"."actor_id"`,
			overrideWantSQL: map[source.Type]string{mysql.Type: "SELECT * FROM `actor` INNER JOIN `film actor` ON `film actor`.`actor_id` = `actor`.`actor_id`"},
			skipExec:        true,
		},
		{
			name:            "orderby/single-element",
			in:              `@sakila | .actor | orderby(.first_name)`,
			wantSQL:         `SELECT * FROM "actor" ORDER BY "first_name"`,
			overrideWantSQL: map[source.Type]string{mysql.Type: "SELECT * FROM `actor` ORDER BY `first_name`"},
			wantRecs:        sakila.TblActorCount,
		},
		{
			name:            "orderby/single-element-table-selector",
			in:              `@sakila | .actor | orderby(.actor.first_name)`,
			wantSQL:         `SELECT * FROM "actor" ORDER BY "actor"."first_name"`,
			overrideWantSQL: map[source.Type]string{mysql.Type: "SELECT * FROM `actor` ORDER BY `actor`.`first_name`"},
			wantRecs:        sakila.TblActorCount,
		},
		{
			name:            "orderby/single-element-asc",
			in:              `@sakila | .actor | orderby(.first_name+)`,
			wantSQL:         `SELECT * FROM "actor" ORDER BY "first_name" ASC`,
			overrideWantSQL: map[source.Type]string{mysql.Type: "SELECT * FROM `actor` ORDER BY `first_name` ASC"},
			wantRecs:        sakila.TblActorCount,
		},
		{
			name:            "orderby/single-element-desc",
			in:              `@sakila | .actor | orderby(.first_name-)`,
			wantSQL:         `SELECT * FROM "actor" ORDER BY "first_name" DESC`,
			overrideWantSQL: map[source.Type]string{mysql.Type: "SELECT * FROM `actor` ORDER BY `first_name` DESC"},
			wantRecs:        sakila.TblActorCount,
		},
		{
			name:            "orderby/multiple-elements",
			in:              `@sakila | .actor | orderby(.first_name+, .last_name-)`,
			wantSQL:         `SELECT * FROM "actor" ORDER BY "first_name" ASC, "last_name" DESC`,
			overrideWantSQL: map[source.Type]string{mysql.Type: "SELECT * FROM `actor` ORDER BY `first_name` ASC, `last_name` DESC"},
			wantRecs:        sakila.TblActorCount,
		},
		{
			name:            "orderby/synonym-sort-by",
			in:              `@sakila | .actor | sort_by(.first_name)`,
			wantSQL:         `SELECT * FROM "actor" ORDER BY "first_name"`,
			overrideWantSQL: map[source.Type]string{mysql.Type: "SELECT * FROM `actor` ORDER BY `first_name`"},
			wantRecs:        sakila.TblActorCount,
		},
		{
			name:    "orderby/error-no-selector",
			in:      `@sakila | .actor | orderby()`,
			wantErr: true,
		},
		{
			name:            "groupby/single-term",
			in:              `@sakila | .payment | .customer_id, sum(.amount) | groupby(.customer_id)`,
			wantSQL:         `SELECT "customer_id", sum("amount") FROM "payment" GROUP BY "customer_id"`,
			overrideWantSQL: map[source.Type]string{mysql.Type: "SELECT `customer_id`, sum(`amount`) FROM `payment` GROUP BY `customer_id`"},
			wantRecs:        599,
		},
		{
			name:            "groupby/synonym-group_by",
			in:              `@sakila | .payment | .customer_id, sum(.amount) | group_by(.customer_id)`,
			wantSQL:         `SELECT "customer_id", sum("amount") FROM "payment" GROUP BY "customer_id"`,
			overrideWantSQL: map[source.Type]string{mysql.Type: "SELECT `customer_id`, sum(`amount`) FROM `payment` GROUP BY `customer_id`"},
			wantRecs:        599,
		},
		{
			name:            "groupby/multiple_terms",
			in:              `@sakila | .payment | .customer_id, .staff_id, sum(.amount) | groupby(.customer_id, .staff_id)`,
			wantSQL:         `SELECT "customer_id", "staff_id", sum("amount") FROM "payment" GROUP BY "customer_id", "staff_id"`,
			overrideWantSQL: map[source.Type]string{mysql.Type: "SELECT `customer_id`, `staff_id`, sum(`amount`) FROM `payment` GROUP BY `customer_id`, `staff_id`"},
			wantRecs:        1198,
		},
		{
			name:     "groupby/with_func/sqlite",
			in:       `@sakila | .payment | date("month", .payment_date):month, count(.payment_id):count | groupby(date("month", .payment_date))`,
			wantSQL:  `SELECT date('month', "payment_date") AS "month", count("payment_id") AS "count" FROM "payment" GROUP BY date('month', "payment_date")`,
			onlyFor:  []source.Type{sqlite3.Type},
			wantRecs: 1,
		},
		{
			name:     "datetime/sqlite/date",
			in:       `@sakila | .payment | date("month", .payment_date)`,
			wantSQL:  `SELECT date('month', "payment_date") FROM "payment"`,
			onlyFor:  []source.Type{sqlite3.Type},
			wantRecs: sakila.TblPaymentCount,
		},
	}

	for _, tc := range testCases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			if tc.skip {
				t.Skip()
			}
			srcs := testh.New(t).NewSourceSet(sakila.SQLLatest()...)
			// srcs := testh.New(t).NewSourceSet(sakila.SL3)

			for _, src := range srcs.Items() {
				src := src

				t.Run(string(src.Type), func(t *testing.T) {
					if len(tc.onlyFor) > 0 {
						if !slices.Contains(tc.onlyFor, src.Type) {
							t.Skip()
						}
					}

					in := strings.Replace(tc.in, "@sakila", src.Handle, 1)
					t.Logf(in)
					want := tc.wantSQL
					if overrideWant, ok := tc.overrideWantSQL[src.Type]; ok {
						want = overrideWant
					}

					_, err := srcs.SetActive(src.Handle)
					require.NoError(t, err)

					th := testh.New(t)
					dbases := th.Databases()

					gotSQL, gotErr := libsq.SLQ2SQL(th.Context, th.Log, dbases, dbases, srcs, in)
					if tc.wantErr {
						require.Error(t, gotErr)
						return
					}

					require.NoError(t, gotErr)
					require.Equal(t, want, gotSQL)
					t.Log(gotSQL)

					if tc.skipExec {
						return
					}

					sink := &testh.RecordSink{}
					recw := output.NewRecordWriterAdapter(sink)

					gotErr = libsq.ExecuteSLQ(th.Context, th.Log, dbases, dbases, srcs, in, recw)
					require.NoError(t, gotErr)

					written, err := recw.Wait()
					require.NoError(t, err)
					require.Equal(t, tc.wantRecs, int(written))
					require.Equal(t, tc.wantRecs, len(sink.Recs))
				})
			}
		})
	}
}

func TestSQLiteDateTrunc(t *testing.T) {
	/*
		{
		name:    "datetime/sqlite/date_trunc",
			in:      `@sakila | .payment | date_trunc("month", .payment_date)`,
			wantSQL: `SELECT DATE_TRUNC('month', "payment_date") FROM "payment"`,
			onlyFor: []source.Type{sqlite3.Type},
		},

	*/

	const query1 = `SELECT DATE('now') FROM "payment";`
	const query2 = `SELECT DATE(?) FROM "payment";`
	_ = query2

	th := testh.New(t)
	ctx := th.Context
	src := th.Source(sakila.SL3)
	dbases, err := th.Databases().Open(ctx, src)
	require.NoError(t, err)
	db := dbases.DB()
	_ = db

	sink, err := th.QuerySQL(src, query1)
	require.NoError(t, err)
	require.Equal(t, sakila.TblPaymentCount, len(sink.Recs))
}

func TestSL3Date(t *testing.T) {
	th := testh.New(t)

	const query = `@sakila_sl3 | .payment | date("month", .payment_date)`
	// const query = `@sakila_sl3 | .actor`
	sink, err := th.QuerySLQ(query)
	require.NoError(t, err)
	require.Equal(t, sakila.TblPaymentCount, len(sink.Recs))
}
