package cli_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/cli/explore"
	"github.com/neilotoole/sq/cli/testrun"
	"github.com/neilotoole/sq/testh"
	"github.com/neilotoole/sq/testh/sakila"
)

// TestExplore_SQLite_LoadsAndQuits opens the explorer against the
// sakila SQLite fixture, runs initial fetches, then cancels the
// program. The TUI rendering itself is not asserted — model-level
// tests cover that — but a misconfigured fetcher or msg-routing
// regression would surface as a hang or error.
func TestExplore_SQLite_LoadsAndQuits(t *testing.T) {
	th := testh.New(t)
	src := th.Source(sakila.SL3)
	tr := testrun.New(th.Context, t, nil).Add(*src)

	ctx, cancel := context.WithTimeout(th.Context, 10*time.Second)
	defer cancel()

	cfg := explore.Config{
		Sources:    tr.Run.Config.Collection.Sources(),
		FocusedSrc: src,
		NoColor:    true,
	}

	done := make(chan error, 1)
	go func() {
		_, err := explore.RunWithIO(ctx, tr.Run, cfg, &nopWriter{}, &nopWriter{})
		done <- err
	}()
	// Let the initial fetch complete, then cancel the program.
	time.Sleep(500 * time.Millisecond)
	cancel()

	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(3 * time.Second):
		t.Fatal("explore.RunWithIO did not return after ctx cancel")
	}
}

// nopWriter discards everything written.
type nopWriter struct{}

func (nopWriter) Write(p []byte) (int, error) { return len(p), nil }
