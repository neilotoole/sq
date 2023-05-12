package cli_test

import (
	"context"
	"encoding/csv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/libsq/source"
	"github.com/neilotoole/sq/testh"
	"github.com/neilotoole/sq/testh/sakila"
)

func TestCmdPing(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	err := NewTestRun(ctx, t, nil).Exec("ping")
	require.Error(t, err, "no active data source")

	err = NewTestRun(ctx, t, nil).Exec("ping", "invalid_handle")
	require.Error(t, err)

	err = NewTestRun(ctx, t, nil).Exec("ping", "@not_a_handle")
	require.Error(t, err)

	var ru *TestRun

	th := testh.New(t)
	src1, src2 := th.Source(sakila.CSVActor), th.Source(sakila.CSVActorNoHeader)

	ru = NewTestRun(ctx, t, nil).add(*src1)
	err = ru.Exec("ping", "--csv", src1.Handle)
	require.NoError(t, err)
	checkPingOutputCSV(t, ru, *src1)

	ru = NewTestRun(ctx, t, nil).add(*src2)
	err = ru.Exec("ping", "--csv", src2.Handle)
	require.NoError(t, err)
	checkPingOutputCSV(t, ru, *src2)

	ru = NewTestRun(ctx, t, nil).add(*src1, *src2)
	err = ru.Exec("ping", "--csv", src1.Handle, src2.Handle)
	require.NoError(t, err)
	checkPingOutputCSV(t, ru, *src1, *src2)
}

// checkPintOutputCSV reads CSV records from h.out, and verifies
// that there's an appropriate record for each of srcs.
func checkPingOutputCSV(t *testing.T, h *TestRun, srcs ...source.Source) {
	recs, err := csv.NewReader(h.out).ReadAll()
	require.NoError(t, err)
	require.Equal(t, len(srcs), len(recs))

	if len(srcs) > 0 {
		require.Equal(t, 3, len(recs[0]), "each ping record should have 3 fields, but got %d fields", len(recs[0]))
	}

	handles := make(map[string]bool)
	for _, src := range srcs {
		handles[src.Handle] = true
	}

	for i := 0; i < len(recs); i++ {
		recHandle := recs[i][0]
		require.True(t, handles[recHandle], "should have handle %s in map", recHandle)

		_, err = time.ParseDuration(recs[i][1])
		require.NoError(t, err, "should be a valid duration value")

		require.Equal(t, "pong", recs[i][2], "error field should be empty")
	}
}
