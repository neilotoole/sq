// Package testrun contains helper functionality for executing CLI tests.
package testrun

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

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/cli"
	"github.com/neilotoole/sq/cli/config"
	"github.com/neilotoole/sq/cli/config/yamlstore"
	"github.com/neilotoole/sq/cli/run"
	"github.com/neilotoole/sq/libsq/core/ioz"
	"github.com/neilotoole/sq/libsq/core/lg"
	"github.com/neilotoole/sq/libsq/core/lg/lgt"
	"github.com/neilotoole/sq/libsq/core/options"
	"github.com/neilotoole/sq/libsq/source"
	"github.com/neilotoole/sq/testh/tu"
)

// TestRun is a helper for testing sq commands.
type TestRun struct {
	T       testing.TB
	Context context.Context
	mu      *sync.Mutex
	Run     *run.Run

	// Out contains the output written to stdout after TestRun.Exec is invoked.
	Out *bytes.Buffer

	// ErrOut contains the output written to stderr after TestRun.Exec is invoked.
	ErrOut *bytes.Buffer
	used   bool

	// When true, out and errOut are not logged.
	hushOutput bool
}

// New returns a new run instance for testing sq commands.
// If from is non-nil, its config is used. This allows sequential
// commands to use the same config.
//
// You can also use TestRun.Reset to reuse a TestRun instance.
func New(ctx context.Context, t testing.TB, from *TestRun) *TestRun {
	if ctx == nil {
		ctx = context.Background()
	}

	if !lg.InContext(ctx) {
		ctx = lg.NewContext(ctx, lgt.New(t))
	}

	tr := &TestRun{T: t, Context: ctx, mu: &sync.Mutex{}}

	var cfgStore config.Store
	if from != nil {
		cfgStore = from.Run.ConfigStore
		tr.hushOutput = from.hushOutput
	}

	tr.Run, tr.Out, tr.ErrOut = newRun(ctx, t, cfgStore)
	tr.Context = options.NewContext(ctx, tr.Run.Config.Options)
	return tr
}

// newRun returns a Run for testing, along
// with buffers for out and errOut (instead of the
// ru writing to stdout and stderr). The contents of
// these buffers can be written to t.Log() if desired.
//
// If cfgStore is nil, a new one is created in a temp dir.
func newRun(ctx context.Context, t testing.TB, cfgStore config.Store) (ru *run.Run, out, errOut *bytes.Buffer) {
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

	ru = &run.Run{
		Stdin:           os.Stdin,
		Out:             out,
		ErrOut:          errOut,
		Config:          cfg,
		ConfigStore:     cfgStore,
		OptionsRegistry: optsReg,
	}

	// The Files instance needs unique dirs for temp and cache because
	// the test runs may execute in parallel inside the same test binary
	// process, thus breaking the pid-based lockfile mechanism.
	ru.Files, err = source.NewFiles(
		ctx,
		ru.OptionsRegistry,
		tu.TempDir(t, false),
		tu.CacheDir(t, false),
		true,
	)
	require.NoError(t, err)

	require.NoError(t, cli.FinishRunInit(ctx, ru))
	return ru, out, errOut
}

// Reset resets tr to a clean slate. Note that a new TestRun instance
// is created behind the scenes, so any references to the previous
// TestRun's fields are now invalid.
//
// See also: testrun.New.
func (tr *TestRun) Reset() *TestRun {
	tr2 := New(tr.Context, tr.T, tr)
	*tr = *tr2
	return tr
}

// Add adds srcs to tr.Run.Config.Collection. If the collection
// does not already have an active source, the first element
// of srcs is used as the active source.
//
// REVISIT: Why not use *source.Source instead of the value?
func (tr *TestRun) Add(srcs ...source.Source) *TestRun {
	tr.mu.Lock()
	defer tr.mu.Unlock()

	if len(srcs) == 0 {
		return tr
	}

	coll := tr.Run.Config.Collection
	hasActive := tr.Run.Config.Collection.Active() != nil

	for _, src := range srcs {
		src := src
		require.NoError(tr.T, coll.Add(&src))
	}

	if !hasActive {
		_, err := coll.SetActive(srcs[0].Handle, false)
		require.NoError(tr.T, err)
	}

	err := tr.Run.ConfigStore.Save(tr.Context, tr.Run.Config)
	require.NoError(tr.T, err)

	return tr
}

// Exec executes the sq command specified by args. If the first
// element of args is not "sq", that value is prepended to the
// args for execution. This method may only be invoked once on
// this TestRun instance, unless TestRun.Reset is called.
// The backing Run will also be closed. If an error
// occurs on the client side during execution, that error is returned.
// Either tr.Out or tr.ErrOut will be filled, according to what the
// CLI outputs.
func (tr *TestRun) Exec(args ...string) error {
	tr.mu.Lock()
	defer tr.mu.Unlock()

	return tr.doExec(args)
}

func (tr *TestRun) doExec(args []string) error {
	defer func() { tr.used = true }()

	require.False(tr.T, tr.used, "TestRun instance must only be used once")

	ctx, cancelFn := context.WithCancel(tr.Context)
	tr.T.Cleanup(cancelFn)

	execErr := cli.ExecuteWith(ctx, tr.Run, args)

	if !tr.hushOutput {
		// We log the CLI's output now (before calling ru.Close) because
		// it reads better in testing's output that way.
		if tr.Out.Len() > 0 {
			tr.T.Log(strings.TrimSuffix(tr.Out.String(), "\n"))
		}
		if tr.ErrOut.Len() > 0 {
			tr.T.Log(strings.TrimSuffix(tr.ErrOut.String(), "\n"))
		}
	}

	closeErr := tr.Run.Close()
	if execErr != nil {
		// We return the ExecuteWith err first
		return execErr
	}

	// Return the closeErr (hopefully is nil)
	return closeErr
}

// Bind marshals tr.Out to v (as JSON), failing the test on any error.
func (tr *TestRun) Bind(v any) *TestRun {
	tr.mu.Lock()
	defer tr.mu.Unlock()

	err := json.Unmarshal(tr.Out.Bytes(), v)
	require.NoError(tr.T, err)
	return tr
}

// BindYAML marshals tr.Out to v (as YAML), failing the test on any error.
func (tr *TestRun) BindYAML(v any) *TestRun {
	tr.mu.Lock()
	defer tr.mu.Unlock()

	err := ioz.UnmarshallYAML(tr.Out.Bytes(), v)
	require.NoError(tr.T, err)
	return tr
}

// BindMap is a convenience method for binding tr.Out to a map
// (assuming tr.Out is JSON).
func (tr *TestRun) BindMap() map[string]any {
	m := map[string]any{}
	tr.Bind(&m)
	return m
}

// BindSliceMap is a convenience method for binding tr.Out
// to a slice of map (assuming tr.Out is JSON).
func (tr *TestRun) BindSliceMap() []map[string]any {
	var a []map[string]any
	tr.Bind(&a)
	return a
}

// BindCSV reads CSV from tr.Out and returns all records,
// failing the testing on any problem. Obviously the Exec call
// should have specified "--csv".
func (tr *TestRun) BindCSV() [][]string {
	tr.mu.Lock()
	defer tr.mu.Unlock()

	recs, err := csv.NewReader(tr.Out).ReadAll()
	require.NoError(tr.T, err)
	return recs
}

// OutString returns the contents of tr.Out as a string,
// with the final trailing newline removed.
func (tr *TestRun) OutString() string {
	return strings.TrimSuffix(tr.Out.String(), "\n")
}

// Hush suppresses the printing of output collected in out
// and errOut to t.Log. Set to true for tests
// that output excessive content, binary files, etc.
func (tr *TestRun) Hush() *TestRun {
	tr.hushOutput = true
	return tr
}
