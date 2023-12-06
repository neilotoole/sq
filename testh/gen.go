package testh

import (
	stdcsv "encoding/csv"
	"fmt"
	"math/rand"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
	"golang.org/x/text/language"
	"golang.org/x/text/message"

	"github.com/neilotoole/sq/libsq/core/stringz"
	"github.com/neilotoole/sq/libsq/core/timez"
)

// GenerateLargeCSV generates a large CSV file.
// At count = 5000000, the generated file is ~500MB.
//
//nolint:gosec
func GenerateLargeCSV(t *testing.T, fp string) {
	const count = 5000000 // Generates ~500MB file
	start := time.Now()
	header := []string{
		"payment_id",
		"customer_id",
		"name",
		"staff_id",
		"rental_id",
		"amount",
		"payment_date",
		"last_update",
	}

	f, err := os.OpenFile(
		fp,
		os.O_CREATE|os.O_WRONLY|os.O_TRUNC,
		0o600,
	)
	require.NoError(t, err)
	t.Cleanup(func() { _ = f.Close() })

	w := stdcsv.NewWriter(f)
	require.NoError(t, w.Write(header))

	rec := make([]string, len(header))
	amount := decimal.New(50000, -2)
	paymentUTC := time.Now().UTC()
	lastUpdateUTC := time.Now().UTC()
	p := message.NewPrinter(language.English)
	for i := 0; i < count; i++ {
		if i%100000 == 0 {
			// Flush occasionally
			w.Flush()
		}

		rec[0] = strconv.Itoa(i + 1)          // payment id, always unique
		rec[1] = strconv.Itoa(rand.Intn(100)) // customer_id, one of 100 customers
		rec[2] = "Alice " + rec[1]            // name
		rec[3] = strconv.Itoa(rand.Intn(10))  // staff_id
		rec[4] = strconv.Itoa(i + 3)          // rental_id, always unique
		f64 := amount.InexactFloat64()
		// rec[5] = p.Sprintf("%.2f", f64) // amount
		rec[5] = fmt.Sprintf("%.2f", f64) // amount
		amount = amount.Add(decimal.New(33, -2))
		rec[6] = timez.TimestampUTC(paymentUTC) // payment_date
		paymentUTC = paymentUTC.Add(time.Minute)
		rec[7] = timez.TimestampUTC(lastUpdateUTC) // last_update
		lastUpdateUTC = lastUpdateUTC.Add(time.Minute + time.Second)
		err = w.Write(rec)
		require.NoError(t, err)
	}

	w.Flush()
	require.NoError(t, w.Error())
	require.NoError(t, f.Close())

	fi, err := os.Stat(f.Name())
	require.NoError(t, err)

	t.Logf(
		"Wrote %s records in %s, total size %s, to: %s",
		p.Sprintf("%d", count),
		time.Since(start).Round(time.Millisecond),
		stringz.ByteSized(fi.Size(), 1, ""),
		f.Name(),
	)
}
