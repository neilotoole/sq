package cli_test

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/neilotoole/sq/cli/run"

	"github.com/neilotoole/sq/cli/config/yamlstore"
	"github.com/neilotoole/sq/libsq/core/options"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/cli"
	"github.com/neilotoole/sq/cli/config"
	"github.com/neilotoole/sq/libsq/source"
)

// newTestRunCtx returns a RunContext for testing, along
// with buffers for out and errOut (instead of the
// rc writing to stdout and stderr). The contents of
// these buffers can be written to t.Log() if desired.
// The srcs args are added to rc.Config.Collection.
//
// If cfgStore is nil, a new one is created in a temp dir.
func newTestRunCtx(ctx context.Context, t testing.TB, cfgStore config.Store,
) (rc *run.Run, out, errOut *bytes.Buffer) {
	out = &bytes.Buffer{}
	errOut = &bytes.Buffer{}

	optsReg := &options.Registry{}
	cli.RegisterDefaultOpts(optsReg)

	var cfg *config.Config
	var err error
	if cfgStore == nil {
		var cfgDir string
		cfgDir, err = os.MkdirTemp("", "sq_test")
		require.NoError(t, err)
		cfgStore = &yamlstore.Store{
			Path:            filepath.Join(cfgDir, "sq.yml"),
			OptionsRegistry: optsReg,
		}
		cfg = config.New()
		require.NoError(t, cfgStore.Save(ctx, cfg))
	} else {
		cfg, err = cfgStore.Load(ctx)
		require.NoError(t, err)
	}

	rc = &run.Run{
		Stdin:           os.Stdin,
		Out:             out,
		ErrOut:          errOut,
		Config:          cfg,
		ConfigStore:     cfgStore,
		OptionsRegistry: optsReg,
	}

	return rc, out, errOut
}

// run is a helper for testing sq commands.
type TestRun struct {
	t      *testing.T
	ctx    context.Context
	mu     sync.Mutex
	rc     *run.Run
	out    *bytes.Buffer
	errOut *bytes.Buffer
	used   bool

	// When true, out and errOut are not logged.
	hushOutput bool
}

// NewTestRun returns a new run instance for testing sq commands.
// If from is non-nil, its config is used. This allows sequential
// commands to use the same config.
func NewTestRun(ctx context.Context, t *testing.T, from *TestRun) *TestRun {
	ru := &TestRun{t: t, ctx: ctx}

	var cfgStore config.Store
	if from != nil {
		cfgStore = from.rc.ConfigStore
	}
	ru.rc, ru.out, ru.errOut = newTestRunCtx(ctx, t, cfgStore)
	return ru
}

// add adds srcs to ru.rc.Config.Collection. If the collection
// does not already have an active source, the first element
// of srcs is used as the active source.
func (ru *TestRun) add(srcs ...source.Source) *TestRun {
	ru.mu.Lock()
	defer ru.mu.Unlock()

	if len(srcs) == 0 {
		return ru
	}

	coll := ru.rc.Config.Collection
	hasActive := ru.rc.Config.Collection.Active() != nil

	for _, src := range srcs {
		src := src
		require.NoError(ru.t, coll.Add(&src))
	}

	if !hasActive {
		_, err := coll.SetActive(srcs[0].Handle, false)
		require.NoError(ru.t, err)
	}

	return ru
}

// Exec executes the sq command specified by args. If the first
// element of args is not "sq", that value is prepended to the
// args for execution. This method may only be invoked once.
// The backing RunContext will also be closed. If an error
// occurs on the client side during execution, that error is returned.
// Either ru.out or ru.errOut will be filled, according to what the
// CLI outputs.
func (ru *TestRun) Exec(args ...string) error {
	ru.mu.Lock()
	defer ru.mu.Unlock()

	return ru.doExec(args)
}

func (ru *TestRun) doExec(args []string) error {
	defer func() { ru.used = true }()

	require.False(ru.t, ru.used, "TestRun instance must only be used once")

	ctx, cancelFn := context.WithCancel(context.Background())
	ru.t.Cleanup(cancelFn)

	execErr := cli.ExecuteWith(ctx, ru.rc, args)

	if !ru.hushOutput {
		// We log the CLI's output now (before calling rc.Close) because
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

// Bind marshals ru.Out to v (as JSON), failing the test on any error.
func (ru *TestRun) Bind(v any) *TestRun {
	ru.mu.Lock()
	defer ru.mu.Unlock()

	err := json.Unmarshal(ru.out.Bytes(), &v)
	require.NoError(ru.t, err)
	return ru
}

// BindMap is a convenience method for binding ru.Out to a map.
func (ru *TestRun) BindMap() map[string]any {
	m := map[string]any{}
	ru.Bind(&m)
	return m
}

// mustReadCSV reads CSV from ru.out and returns all records,
// failing the testing on any problem. Obviously the Exec call
// should have specified "--csv".
func (ru *TestRun) mustReadCSV() [][]string {
	ru.mu.Lock()
	defer ru.mu.Unlock()

	recs, err := csv.NewReader(ru.out).ReadAll()
	require.NoError(ru.t, err)
	return recs
}

// hush suppresses the printing of output collected in out
// and errOut to t.Log. Collection to true for tests
// that output excessive content, binary files, etc.
func (ru *TestRun) hush() *TestRun {
	ru.hushOutput = true
	return ru
}
