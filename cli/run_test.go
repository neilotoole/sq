package cli_test

import (
	"bytes"
	"context"
	"encoding/csv"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/neilotoole/lg"
	"github.com/neilotoole/lg/testlg"
	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/cli"
	"github.com/neilotoole/sq/cli/config"
	"github.com/neilotoole/sq/libsq/errz"
	"github.com/neilotoole/sq/libsq/source"
)

// newTestRunCtx returns a RunContext for testing, along
// with buffers for out and errOut (instead of the
// rc writing to stdout and stderr). The contents of
// these buffers can be written to t.Log() if desired.
// The srcs args are added to rc.Config.Set.
func newTestRunCtx(log lg.Log) (rc *cli.RunContext, out, errOut *bytes.Buffer) {
	out = &bytes.Buffer{}
	errOut = &bytes.Buffer{}

	rc = &cli.RunContext{
		Context:     context.Background(),
		Stdin:       os.Stdin,
		Out:         out,
		ErrOut:      errOut,
		Log:         log,
		Config:      config.New(),
		ConfigStore: config.DiscardStore{},
	}

	return rc, out, errOut
}

// run is a helper for testing sq commands.
type run struct {
	t      *testing.T
	mu     sync.Mutex
	rc     *cli.RunContext
	out    *bytes.Buffer
	errOut *bytes.Buffer
	used   bool

	// When true, out and errOut are not logged.
	hushOutput bool
}

// newRun returns a new run instance for testing sq commands.
func newRun(t *testing.T) *run {
	ru := &run{t: t}
	ru.rc, ru.out, ru.errOut = newTestRunCtx(testlg.New(t))
	return ru
}

// add adds srcs to ru.rc.Config.Set. If the source set
// does not already have an active source, the first element
// of srcs is used.
func (ru *run) add(srcs ...source.Source) *run {
	ru.mu.Lock()
	defer ru.mu.Unlock()

	if len(srcs) == 0 {
		return ru
	}

	ss := ru.rc.Config.Sources
	hasActive := ru.rc.Config.Sources.Active() != nil

	for _, src := range srcs {
		src := src
		require.NoError(ru.t, ss.Add(&src))
	}

	if !hasActive {
		_, err := ss.SetActive(srcs[0].Handle)
		require.NoError(ru.t, err)
	}

	return ru
}

// exec executes the sq command specified by args. If the first
// element of args is not "sq", that value is prepended to the
// args for execution. This method may only be invoked once.
// The backing RunContext will also be closed.
func (ru *run) exec(args ...string) error {
	ru.mu.Lock()
	defer ru.mu.Unlock()

	if ru.used {
		err := errz.New("run instance must only be used once")
		ru.t.Fatal(err)
		return err
	}

	if len(args) > 0 && args[0] != "sq" {
		args = append([]string{"sq"}, args...)
	}

	execErr := cli.ExecuteWith(ru.rc, args)

	if !ru.hushOutput {
		// We log sq's output now (before calling rc.Close) because
		// it reads better in testing's output that way.
		if ru.out.Len() > 0 {
			ru.t.Log(strings.TrimSuffix(ru.out.String(), "\n"))
		}
		if ru.errOut.Len() > 0 {
			ru.t.Log(strings.TrimSuffix(ru.errOut.String(), "\n"))
		}
	}

	closeErr := ru.rc.Close()
	if execErr != nil {
		// We return the ExecuteWith err first
		return execErr
	}

	// Return the closeErr (hopefully is nil)
	return closeErr
}

// mustReadCSV reads CSV from ru.out and returns all records,
// failing the testing on any problem.
func (ru *run) mustReadCSV() [][]string {
	ru.mu.Lock()
	defer ru.mu.Unlock()

	recs, err := csv.NewReader(ru.out).ReadAll()
	require.NoError(ru.t, err)
	return recs
}

// hush suppresses the printing of output collected in out
// and errOut to t.Log. Set to true for tests
// that output excessive content, binary files, etc.
func (ru *run) hush() *run {
	ru.hushOutput = true
	return ru
}
