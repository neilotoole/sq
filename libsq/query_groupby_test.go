package libsq_test

import (
	"testing"

	"github.com/neilotoole/sq/drivers/sqlite3"

	"github.com/neilotoole/sq/drivers/mysql"

	"github.com/neilotoole/sq/libsq/source"

	_ "github.com/mattn/go-sqlite3"
)

//nolint:exhaustive,lll
func TestQuery_groupby(t *testing.T) {
	testCases := []queryTestCase{
		{
			name:         "group_by/single-term",
			in:           `@sakila | .payment | .customer_id, sum(.amount) | group_by(.customer_id)`,
			wantSQL:      `SELECT "customer_id", sum("amount") AS "sum(.amount)" FROM "payment" GROUP BY "customer_id"`,
			override:     driverMap{mysql.Type: "SELECT `customer_id`, sum(`amount`) AS `sum(.amount)` FROM `payment` GROUP BY `customer_id`"},
			wantRecCount: 599,
		},
		{
			name:         "group_by/multiple_terms",
			in:           `@sakila | .payment | .customer_id, .staff_id, sum(.amount) | group_by(.customer_id, .staff_id)`,
			wantSQL:      `SELECT "customer_id", "staff_id", sum("amount") AS "sum(.amount)" FROM "payment" GROUP BY "customer_id", "staff_id"`,
			override:     driverMap{mysql.Type: "SELECT `customer_id`, `staff_id`, sum(`amount`) AS `sum(.amount)` FROM `payment` GROUP BY `customer_id`, `staff_id`"},
			wantRecCount: 1198,
		},
		{
			name:         "group_by/with_func/sqlite",
			in:           `@sakila | .payment | _date("month", .payment_date):month, count(.payment_id):count | group_by(_date("month", .payment_date))`,
			wantSQL:      `SELECT date('month', "payment_date") AS "month", count("payment_id") AS "count" FROM "payment" GROUP BY date('month', "payment_date")`,
			onlyFor:      []source.DriverType{sqlite3.Type},
			wantRecCount: 1,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			execQueryTestCase(t, tc)
		})
	}
}
