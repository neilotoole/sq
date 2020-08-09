package cli_test

import (
	"encoding/csv"
	"fmt"
	"io/ioutil"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/libsq/source"
	"github.com/neilotoole/sq/libsq/stringz"
	"github.com/neilotoole/sq/testh"
	"github.com/neilotoole/sq/testh/sakila"
)

// TestCmdSLQ_Insert tests "sq slq QUERY --insert=dest.tbl".
func TestCmdSLQ_Insert(t *testing.T) {
	for _, origin := range sakila.SQLLatest {
		origin := origin

		t.Run("origin_"+origin, func(t *testing.T) {
			testh.SkipShort(t, origin == sakila.XLSX)

			for _, dest := range sakila.SQLLatest {
				dest := dest

				t.Run("dest_"+dest, func(t *testing.T) {
					t.Parallel()

					th := testh.New(t)
					originSrc, destSrc := th.Source(origin), th.Source(dest)
					srcTbl := sakila.TblActor
					if th.IsMonotable(originSrc) {
						srcTbl = source.MonotableName
					}

					// To avoid dirtying the destination table, we make a copy
					// of it (without data).
					actualDestTbl := th.CopyTable(false, destSrc, sakila.TblActor, "", false)
					t.Cleanup(func() { th.DropTable(destSrc, actualDestTbl) })

					ru := newRun(t).add(*originSrc)
					if destSrc.Handle != originSrc.Handle {
						ru.add(*destSrc)
					}

					insertTo := fmt.Sprintf("%s.%s", destSrc.Handle, actualDestTbl)
					cols := stringz.PrefixSlice(sakila.TblActorCols, ".")
					query := fmt.Sprintf("%s.%s | %s", originSrc.Handle, srcTbl, strings.Join(cols, ", "))

					err := ru.exec("slq", "--insert="+insertTo, query)
					require.NoError(t, err)

					sink, err := th.QuerySQL(destSrc, "select * from "+actualDestTbl)
					require.NoError(t, err)
					require.Equal(t, sakila.TblActorCount, len(sink.Recs))
				})
			}
		})
	}
}

func TestCmdSLQ_CSV(t *testing.T) {
	t.Parallel()

	src := testh.New(t).Source(sakila.CSVActor)
	ru := newRun(t).add(*src)
	err := ru.exec("slq", "--header=false", "--csv", fmt.Sprintf("%s.data", src.Handle))
	require.NoError(t, err)

	recs := ru.mustReadCSV()
	require.Equal(t, sakila.TblActorCount, len(recs))
}

// TestCmdSLQ_OutputFlag verifies that flag --output=<file> works.
func TestCmdSLQ_OutputFlag(t *testing.T) {
	t.Parallel()

	src := testh.New(t).Source(sakila.SL3)
	ru := newRun(t).add(*src)
	outputFile, err := ioutil.TempFile("", t.Name())
	require.NoError(t, err)

	t.Cleanup(func() {
		assert.NoError(t, outputFile.Close())
		assert.NoError(t, os.Remove(outputFile.Name()))
	})

	err = ru.exec("slq",
		"--header=false", "--csv", fmt.Sprintf("%s.%s", src.Handle, sakila.TblActor),
		"--output", outputFile.Name())
	require.NoError(t, err)

	recs, err := csv.NewReader(outputFile).ReadAll()
	require.NoError(t, err)
	require.Equal(t, sakila.TblActorCount, len(recs))
}

func TestCmdSLQ_Join(t *testing.T) {
	const queryTpl = `%s.customer, %s.address | join(.address_id) | .customer_id == %d | .[0] | .customer_id, .email, .city_id`
	handles := sakila.SQLAll

	// Attempt to join every SQL test source against every SQL test source.
	for _, h1 := range handles {
		h1 := h1

		t.Run("origin_"+h1, func(t *testing.T) {
			for _, h2 := range handles {
				h2 := h2

				t.Run("dest_"+h2, func(t *testing.T) {
					t.Parallel()

					th := testh.New(t)
					src1, src2 := th.Source(h1), th.Source(h2)

					ru := newRun(t).add(*src1)
					if src2.Handle != src1.Handle {
						ru.add(*src2)
					}

					query := fmt.Sprintf(queryTpl, src1.Handle, src2.Handle, sakila.MillerCustID)

					err := ru.exec("slq", "--header=false", "--csv", query)
					require.NoError(t, err)

					recs := ru.mustReadCSV()
					require.Equal(t, 1, len(recs), "should only be one matching record")
					require.Equal(t, 3, len(recs[0]), "should have three fields")
					require.Equal(t, strconv.Itoa(sakila.MillerCustID), recs[0][0])
					require.Equal(t, sakila.MillerEmail, recs[0][1])
					require.Equal(t, strconv.Itoa(sakila.MillerCityID), recs[0][2])
				})
			}
		})
	}
}

// TestCmdSLQ_ActiveSrcHandle verifies that source.ActiveHandle is
// interpreted as the active src in a SLQ query.
func TestCmdSLQ_ActiveSrcHandle(t *testing.T) {
	src := testh.New(t).Source(sakila.SL3)

	// 1. Verify that the query works as expected using the actual src handle
	ru := newRun(t).add(*src).hush()

	require.Equal(t, src.Handle, ru.rc.Config.Sources.Active().Handle)
	err := ru.exec("slq", "--header=false", "--csv", "@sakila_sl3.actor")
	require.NoError(t, err)
	recs := ru.mustReadCSV()
	require.Equal(t, sakila.TblActorCount, len(recs))

	// 2. Verify that it works using source.ActiveHandle as the src handle
	ru = newRun(t).add(*src).hush()
	require.Equal(t, src.Handle, ru.rc.Config.Sources.Active().Handle)
	err = ru.exec("slq", "--header=false", "--csv", source.ActiveHandle+".actor")
	require.NoError(t, err)
	recs = ru.mustReadCSV()
	require.Equal(t, sakila.TblActorCount, len(recs))
}
