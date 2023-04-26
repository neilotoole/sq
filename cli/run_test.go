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

	"github.com/neilotoole/slogt"
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
) (rc *cli.RunContext, out, errOut *bytes.Buffer) {
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
		cfgStore = &yamlstore.Store{Path: filepath.Join(cfgDir, "sq.yml")}
		cfg = config.New()
		require.NoError(t, cfgStore.Save(ctx, cfg))
	} else {
		cfg, err = cfgStore.Load(ctx, optsReg)
		require.NoError(t, err)
	}

	rc = &cli.RunContext{
		Stdin:           os.Stdin,
		Out:             out,
		ErrOut:          errOut,
		Log:             slogt.New(t),
		Config:          cfg,
		ConfigStore:     cfgStore,
		OptionsRegistry: optsReg,
	}

	return rc, out, errOut
}

// run is a helper for testing sq commands.
type Run struct {
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
// If from is non-nil, its config is used. This allows sequential
// commands to use the same config.
func newRun(ctx context.Context, t *testing.T, from *Run) *Run {
	ru := &Run{t: t}
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
func (ru *Run) add(srcs ...source.Source) *Run {
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
func (ru *Run) Exec(args ...string) error {
	ru.mu.Lock()
	defer ru.mu.Unlock()

	return ru.doExec(args)
}

func (ru *Run) doExec(args []string) error {
	defer func() { ru.used = true }()

	require.False(ru.t, ru.used, "Run instance must only be used once")

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

// Bind marshals Run.Out to v (as JSON), failing the test on any error.
func (ru *Run) Bind(v any) *Run {
	ru.mu.Lock()
	defer ru.mu.Unlock()

	err := json.Unmarshal(ru.out.Bytes(), &v)
	require.NoError(ru.t, err)
	return ru
}

// BindMap is a convenience method for binding ru.Out to a map.
func (ru *Run) BindMap() map[string]any {
	m := map[string]any{}
	ru.Bind(&m)
	return m
}

// mustReadCSV reads CSV from ru.out and returns all records,
// failing the testing on any problem. Obviously the Exec call
// should have specified "--csv".
func (ru *Run) mustReadCSV() [][]string {
	ru.mu.Lock()
	defer ru.mu.Unlock()

	recs, err := csv.NewReader(ru.out).ReadAll()
	require.NoError(ru.t, err)
	return recs
}

// hush suppresses the printing of output collected in out
// and errOut to t.Log. Collection to true for tests
// that output excessive content, binary files, etc.
func (ru *Run) hush() *Run {
	ru.hushOutput = true
	return ru
}
