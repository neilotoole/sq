package drivers_test

import (
	"strconv"
	"strings"
	"testing"

	"github.com/neilotoole/sq/drivers/sqlserver"

	"github.com/neilotoole/sq/drivers/postgres"

	"github.com/neilotoole/sq/drivers/sqlite3"

	"golang.org/x/exp/slices"

	"github.com/neilotoole/sq/drivers/mysql"

	"github.com/neilotoole/sq/libsq/source"

	"github.com/neilotoole/sq/libsq"
	"github.com/stretchr/testify/require"

	_ "github.com/mattn/go-sqlite3"
	"github.com/neilotoole/sq/testh"
	"github.com/neilotoole/sq/testh/sakila"
)

//nolint:exhaustive,lll
func TestSLQ2SQL(t *testing.T) {
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

		// override allows an alternative "wantSQL" for a specific driver type.
		// For example, MySQL uses backtick as the quote char, so it needs
		// a separate wantSQL string.
		override map[source.Type]string

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
			name:     "select/cols",
			in:       `@sakila | .actor | .first_name, .last_name`,
			wantSQL:  `SELECT "first_name", "last_name" FROM "actor"`,
			override: map[source.Type]string{mysql.Type: "SELECT `first_name`, `last_name` FROM `actor`"},
			wantRecs: sakila.TblActorCount,
		},
		{
			name:     "select/cols-whitespace-single-col",
			in:       `@sakila | .actor | ."first name"`,
			wantSQL:  `SELECT "first name" FROM "actor"`,
			override: map[source.Type]string{mysql.Type: "SELECT `first name` FROM `actor`"},
			wantRecs: sakila.TblActorCount,
			skipExec: true,
		},
		{
			name:     "select/cols-whitespace-multiple-cols",
			in:       `@sakila | .actor | .actor_id, ."first name", ."last name"`,
			wantSQL:  `SELECT "actor_id", "first name", "last name" FROM "actor"`,
			override: map[source.Type]string{mysql.Type: "SELECT `actor_id`, `first name`, `last name` FROM `actor`"},
			wantRecs: sakila.TblActorCount,
			skipExec: true,
		},
		{
			name:     "count/whitespace-col",
			in:       `@sakila | .actor | count(."first name")`,
			wantSQL:  `SELECT count("first name") FROM "actor"`,
			override: map[source.Type]string{mysql.Type: "SELECT count(`first name`) FROM `actor`"},
			skipExec: true,
		},
		{
			name:     "select/table-whitespace",
			in:       `@sakila | ."film actor"`,
			wantSQL:  `SELECT * FROM "film actor"`,
			override: map[source.Type]string{mysql.Type: "SELECT * FROM `film actor`"},
			skipExec: true,
		},
		{
			name:     "select/cols-aliases",
			in:       `@sakila | .actor | .first_name:given_name, .last_name:family_name`,
			wantSQL:  `SELECT "first_name" AS "given_name", "last_name" AS "family_name" FROM "actor"`,
			override: map[source.Type]string{mysql.Type: "SELECT `first_name` AS `given_name`, `last_name` AS `family_name` FROM `actor`"},
			wantRecs: sakila.TblActorCount,
		},
		{
			name:     "count/no-parens-no-args",
			in:       `@sakila | .actor | count`,
			wantSQL:  `SELECT count(*) AS "count" FROM "actor"`,
			override: map[source.Type]string{mysql.Type: "SELECT count(*) AS `count` FROM `actor`"},
			wantRecs: 1,
		},
		{
			name: "count/no-parens-no-args-with-alias-count",
			// Test that the count:ALIAS form can handle the alias
			// being a reserved word (count).
			in:       `@sakila | .actor | count:count`,
			wantSQL:  `SELECT count(*) AS "count" FROM "actor"`,
			override: map[source.Type]string{mysql.Type: "SELECT count(*) AS `count` FROM `actor`"},
			wantRecs: 1,
		},
		{
			name: "count/no-parens-no-args-with-alias-unique",
			// Test that the count:ALIAS form can handle the alias
			// being a reserved word (unique).
			in:       `@sakila | .actor | count:unique`,
			wantSQL:  `SELECT count(*) AS "unique" FROM "actor"`,
			override: map[source.Type]string{mysql.Type: "SELECT count(*) AS `unique` FROM `actor`"},
			wantRecs: 1,
		},
		{
			name:     "count/no-parens-no-args-with-alias-arbitrary",
			in:       `@sakila | .actor | count:something_123`,
			wantSQL:  `SELECT count(*) AS "something_123" FROM "actor"`,
			override: map[source.Type]string{mysql.Type: "SELECT count(*) AS `something_123` FROM `actor`"},
			wantRecs: 1,
		},
		{
			name:     "count/count-parens-no-args",
			in:       `@sakila | .actor | count()`,
			wantSQL:  `SELECT count(*) AS "count" FROM "actor"`,
			override: map[source.Type]string{mysql.Type: "SELECT count(*) AS `count` FROM `actor`"},
			wantRecs: 1,
		},
		{
			name:    "count/error-star",
			in:      `@sakila | .actor | count(*)`,
			wantErr: true, // Star version is not supported
		},
		{
			name:     "count/single-selector",
			in:       `@sakila | .actor | count(.first_name)`,
			wantSQL:  `SELECT count("first_name") FROM "actor"`,
			override: map[source.Type]string{mysql.Type: "SELECT count(`first_name`) FROM `actor`"},
			wantRecs: 1,
		},
		{
			name:    "count/error-multiple-selector",
			in:      `@sakila | .actor | count(.first_name, .last_name)`,
			wantErr: true, // Only a single selector is permitted
		},
		{
			name:     "unique/single-col",
			in:       `@sakila | .actor | .first_name | unique`,
			wantSQL:  `SELECT DISTINCT "first_name" FROM "actor"`,
			override: map[source.Type]string{mysql.Type: "SELECT DISTINCT `first_name` FROM `actor`"},
			wantRecs: 128,
		},
		{
			name:     "unique/no-col",
			in:       `@sakila | .actor | unique`,
			wantSQL:  `SELECT DISTINCT * FROM "actor"`,
			override: map[source.Type]string{mysql.Type: "SELECT DISTINCT * FROM `actor`"},
			wantRecs: sakila.TblActorCount,
		},
		{
			name:     "unique/no-col",
			in:       `@sakila | .actor | unique`,
			wantSQL:  `SELECT DISTINCT * FROM "actor"`,
			override: map[source.Type]string{mysql.Type: "SELECT DISTINCT * FROM `actor`"},
			wantRecs: sakila.TblActorCount,
		},
		{
			name:     "select/handle-table/cols",
			in:       `@sakila.actor | .first_name, .last_name`,
			wantSQL:  `SELECT "first_name", "last_name" FROM "actor"`,
			override: map[source.Type]string{mysql.Type: "SELECT `first_name`, `last_name` FROM `actor`"},
			wantRecs: sakila.TblActorCount,
		},
		{
			name:     "select/handle-table/count",
			in:       `@sakila.actor | count`,
			wantSQL:  `SELECT count(*) AS "count" FROM "actor"`,
			override: map[source.Type]string{mysql.Type: "SELECT count(*) AS `count` FROM `actor`"},
			wantRecs: 1,
		},
		{
			name:     "select/handle-table/count-col",
			in:       `@sakila.actor | count(."first name")`,
			wantSQL:  `SELECT count("first name") FROM "actor"`,
			override: map[source.Type]string{mysql.Type: "SELECT count(`first name`) FROM `actor`"},
			skipExec: true,
		},
		{
			name:     "count/alias",
			in:       `@sakila | .actor | count:quantity`,
			wantSQL:  `SELECT count(*) AS "quantity" FROM "actor"`,
			override: map[source.Type]string{mysql.Type: "SELECT count(*) AS `quantity` FROM `actor`"},
			wantRecs: 1,
		},
		{
			name:     "select/count-same-alias",
			in:       `@sakila | .actor | count:count`,
			wantSQL:  `SELECT count(*) AS "count" FROM "actor"`,
			override: map[source.Type]string{mysql.Type: "SELECT count(*) AS `count` FROM `actor`"},
			wantRecs: 1,
		},
		{
			name:     "filter/equal",
			in:       `@sakila | .actor | .actor_id == 1`,
			wantSQL:  `SELECT * FROM "actor" WHERE "actor_id" = 1`,
			override: map[source.Type]string{mysql.Type: "SELECT * FROM `actor` WHERE `actor_id` = 1"},
			wantRecs: 1,
		},
		{
			name:     "join/single-selector",
			in:       `@sakila | .actor, .film_actor | join(.actor_id)`,
			wantSQL:  `SELECT * FROM "actor" INNER JOIN "film_actor" ON "actor"."actor_id" = "film_actor"."actor_id"`,
			override: map[source.Type]string{mysql.Type: "SELECT * FROM `actor` INNER JOIN `film_actor` ON `actor`.`actor_id` = `film_actor`.`actor_id`"},
			wantRecs: sakila.TblFilmActorCount,
		},
		{
			name:     "join/fq-table-cols-equal",
			in:       `@sakila | .actor, .film_actor | join(.film_actor.actor_id == .actor.actor_id)`,
			wantSQL:  `SELECT * FROM "actor" INNER JOIN "film_actor" ON "film_actor"."actor_id" = "actor"."actor_id"`,
			override: map[source.Type]string{mysql.Type: "SELECT * FROM `actor` INNER JOIN `film_actor` ON `film_actor`.`actor_id` = `actor`.`actor_id`"},
			wantRecs: sakila.TblFilmActorCount,
		},
		{
			name:     "join/fq-table-cols-equal-whitespace",
			in:       `@sakila | .actor, ."film actor" | join(."film actor".actor_id == .actor.actor_id)`,
			wantSQL:  `SELECT * FROM "actor" INNER JOIN "film actor" ON "film actor"."actor_id" = "actor"."actor_id"`,
			override: map[source.Type]string{mysql.Type: "SELECT * FROM `actor` INNER JOIN `film actor` ON `film actor`.`actor_id` = `actor`.`actor_id`"},
			skipExec: true,
		},
		{
			name:     "order_by/single-element",
			in:       `@sakila | .actor | order_by(.first_name)`,
			wantSQL:  `SELECT * FROM "actor" ORDER BY "first_name"`,
			override: map[source.Type]string{mysql.Type: "SELECT * FROM `actor` ORDER BY `first_name`"},
			wantRecs: sakila.TblActorCount,
		},
		{
			name:     "order_by/single-element-table-selector",
			in:       `@sakila | .actor | order_by(.actor.first_name)`,
			wantSQL:  `SELECT * FROM "actor" ORDER BY "actor"."first_name"`,
			override: map[source.Type]string{mysql.Type: "SELECT * FROM `actor` ORDER BY `actor`.`first_name`"},
			wantRecs: sakila.TblActorCount,
		},
		{
			name:     "order_by/single-element-asc",
			in:       `@sakila | .actor | order_by(.first_name+)`,
			wantSQL:  `SELECT * FROM "actor" ORDER BY "first_name" ASC`,
			override: map[source.Type]string{mysql.Type: "SELECT * FROM `actor` ORDER BY `first_name` ASC"},
			wantRecs: sakila.TblActorCount,
		},
		{
			name:     "order_by/single-element-desc",
			in:       `@sakila | .actor | order_by(.first_name-)`,
			wantSQL:  `SELECT * FROM "actor" ORDER BY "first_name" DESC`,
			override: map[source.Type]string{mysql.Type: "SELECT * FROM `actor` ORDER BY `first_name` DESC"},
			wantRecs: sakila.TblActorCount,
		},
		{
			name:     "order_by/multiple-elements",
			in:       `@sakila | .actor | order_by(.first_name+, .last_name-)`,
			wantSQL:  `SELECT * FROM "actor" ORDER BY "first_name" ASC, "last_name" DESC`,
			override: map[source.Type]string{mysql.Type: "SELECT * FROM `actor` ORDER BY `first_name` ASC, `last_name` DESC"},
			wantRecs: sakila.TblActorCount,
		},
		{
			name:     "order_by/synonym-sort-by",
			in:       `@sakila | .actor | sort_by(.first_name)`,
			wantSQL:  `SELECT * FROM "actor" ORDER BY "first_name"`,
			override: map[source.Type]string{mysql.Type: "SELECT * FROM `actor` ORDER BY `first_name`"},
			wantRecs: sakila.TblActorCount,
		},
		{
			name:    "order_by/error-no-selector",
			in:      `@sakila | .actor | order_by()`,
			wantErr: true,
		},
		{
			name:     "group_by/single-term",
			in:       `@sakila | .payment | .customer_id, sum(.amount) | group_by(.customer_id)`,
			wantSQL:  `SELECT "customer_id", sum("amount") AS "sum(.amount)" FROM "payment" GROUP BY "customer_id"`,
			override: map[source.Type]string{mysql.Type: "SELECT `customer_id`, sum(`amount`) AS `sum(.amount)` FROM `payment` GROUP BY `customer_id`"},
			wantRecs: 599,
		},
		{
			name:     "group_by/multiple_terms",
			in:       `@sakila | .payment | .customer_id, .staff_id, sum(.amount) | group_by(.customer_id, .staff_id)`,
			wantSQL:  `SELECT "customer_id", "staff_id", sum("amount") AS "sum(.amount)" FROM "payment" GROUP BY "customer_id", "staff_id"`,
			override: map[source.Type]string{mysql.Type: "SELECT `customer_id`, `staff_id`, sum(`amount`) AS `sum(.amount)` FROM `payment` GROUP BY `customer_id`, `staff_id`"},
			wantRecs: 1198,
		},
		{
			name:     "group_by/with_func/sqlite",
			in:       `@sakila | .payment | date("month", .payment_date):month, count(.payment_id):count | group_by(date("month", .payment_date))`,
			wantSQL:  `SELECT date('month', "payment_date") AS "month", count("payment_id") AS "count" FROM "payment" GROUP BY date('month', "payment_date")`,
			onlyFor:  []source.Type{sqlite3.Type},
			wantRecs: 1,
		},
		{
			name:     "datetime/strftime/sqlite",
			in:       `@sakila | .payment | strftime("%m", .payment_date)`,
			wantSQL:  `SELECT strftime('%m', "payment_date") AS "strftime(""%m"",.payment_date)" FROM "payment"`,
			onlyFor:  []source.Type{sqlite3.Type},
			wantRecs: sakila.TblPaymentCount,
		},
		{
			name:     "datetime/date_trunc/postgres",
			in:       `@sakila | .payment | date_trunc("month", .payment_date)`,
			wantSQL:  `SELECT date_trunc('month', "payment_date") AS "date_trunc(""month"",.payment_date)" FROM "payment"`,
			onlyFor:  []source.Type{postgres.Type},
			wantRecs: sakila.TblPaymentCount,
		},
		{
			name:     "datetime/month/sqlserver",
			in:       `@sakila | .payment | month(.payment_date)`,
			wantSQL:  `SELECT month("payment_date") AS "month(.payment_date)" FROM "payment"`,
			onlyFor:  []source.Type{sqlserver.Type},
			wantRecs: sakila.TblPaymentCount,
		},
		{
			name:     "datetime/date_format/mysql",
			in:       `@sakila | .payment | date_format(.payment_date, "%m")`,
			wantSQL:  "SELECT date_format(`payment_date`, '%m') AS `date_format(.payment_date,\"%m\")` FROM `payment`",
			onlyFor:  []source.Type{mysql.Type},
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
			// srcs := testh.New(t).NewSourceSet(sakila.SL3) // FIXME: remove when done debugging
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
					if overrideWant, ok := tc.override[src.Type]; ok {
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

					sink, err := th.QuerySLQ(in)
					require.NoError(t, err)
					require.Equal(t, tc.wantRecs, len(sink.Recs))
				})
			}
		})
	}
}

func TestQuoting(t *testing.T) {
	w := strconv.Quote("")
	t.Log(w)

	s := strconv.Quote(`hello "friend"`)
	t.Log(s)
}
