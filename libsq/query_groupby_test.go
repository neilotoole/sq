package libsq_test

import (
	"testing"

	_ "github.com/mattn/go-sqlite3"

	"github.com/neilotoole/sq/libsq/source/drivertype"
)

//nolint:exhaustive,lll
func TestQuery_groupby(t *testing.T) {
	testCases := []queryTestCase{
		{
			name:         "group_by/single-term",
			in:           `@sakila | .payment | .customer_id, sum(.amount) | group_by(.customer_id)`,
			wantSQL:      `SELECT "customer_id", sum("amount") AS "sum(.amount)" FROM "payment" GROUP BY "customer_id"`,
			override:     driverMap{drivertype.MySQL: "SELECT `customer_id`, sum(`amount`) AS `sum(.amount)` FROM `payment` GROUP BY `customer_id`"},
			wantRecCount: 599,
		},
		{
			name:         "alias-gb",
			in:           `@sakila | .payment | .customer_id, sum(.amount) | gb(.customer_id)`,
			wantSQL:      `SELECT "customer_id", sum("amount") AS "sum(.amount)" FROM "payment" GROUP BY "customer_id"`,
			override:     driverMap{drivertype.MySQL: "SELECT `customer_id`, sum(`amount`) AS `sum(.amount)` FROM `payment` GROUP BY `customer_id`"},
			wantRecCount: 599,
		},
		{
			name:         "group_by/multiple_terms",
			in:           `@sakila | .payment | .customer_id, .staff_id, sum(.amount) | group_by(.customer_id, .staff_id)`,
			wantSQL:      `SELECT "customer_id", "staff_id", sum("amount") AS "sum(.amount)" FROM "payment" GROUP BY "customer_id", "staff_id"`,
			override:     driverMap{drivertype.MySQL: "SELECT `customer_id`, `staff_id`, sum(`amount`) AS `sum(.amount)` FROM `payment` GROUP BY `customer_id`, `staff_id`"},
			wantRecCount: 1198,
		},
		{
			name:         "group_by/with_func/sqlite",
			in:           `@sakila | .payment | _date("month", .payment_date):month, count(.payment_id):count | group_by(_date("month", .payment_date))`,
			wantSQL:      `SELECT date('month', "payment_date") AS "month", count("payment_id") AS "count" FROM "payment" GROUP BY date('month', "payment_date")`,
			onlyFor:      []drivertype.Type{drivertype.SQLite},
			wantRecCount: 1,
		},
		{
			name:         "group_by/having",
			in:           `@sakila | .payment | .customer_id, sum(.amount) | group_by(.customer_id) | having(sum(.amount) > 100)`,
			wantSQL:      `SELECT "customer_id", sum("amount") AS "sum(.amount)" FROM "payment" GROUP BY "customer_id" HAVING sum("amount") > 100`,
			override:     driverMap{drivertype.MySQL: "SELECT `customer_id`, sum(`amount`) AS `sum(.amount)` FROM `payment` GROUP BY `customer_id` HAVING sum(`amount`) > 100"},
			wantRecCount: 395,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			execQueryTestCase(t, tc)
		})
	}
}
