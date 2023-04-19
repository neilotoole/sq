package cli

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"sync"

	"github.com/neilotoole/sq/cli/config"
	"github.com/neilotoole/sq/cli/flag"
	"github.com/neilotoole/sq/drivers/csv"
	"github.com/neilotoole/sq/drivers/json"
	"github.com/neilotoole/sq/drivers/mysql"
	"github.com/neilotoole/sq/drivers/postgres"
	"github.com/neilotoole/sq/drivers/sqlite3"
	"github.com/neilotoole/sq/drivers/sqlserver"
	"github.com/neilotoole/sq/drivers/userdriver"
	"github.com/neilotoole/sq/drivers/userdriver/xmlud"
	"github.com/neilotoole/sq/drivers/xlsx"
	"github.com/neilotoole/sq/libsq/core/cleanup"
	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/lg"
	"github.com/neilotoole/sq/libsq/core/lg/lga"
	"github.com/neilotoole/sq/libsq/driver"
	"github.com/neilotoole/sq/libsq/source"
	"github.com/spf13/cobra"
	"golang.org/x/exp/slog"
)

type runContextKey struct{}

// WithRunContext returns ctx with rc added as a value.
func WithRunContext(ctx context.Context, rc *RunContext) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}

	return context.WithValue(ctx, runContextKey{}, rc)
}

// RunContextFrom extracts the RunContext added to ctx via WithRunContext.
func RunContextFrom(ctx context.Context) *RunContext {
	return ctx.Value(runContextKey{}).(*RunContext)
}

// getRunContext is a convenience function for getting RunContext
// from the cmd.Context().
func getRunContext(cmd *cobra.Command) *RunContext {
	rc := RunContextFrom(cmd.Context())
	if rc.Cmd == nil {
		// rc.Cmd is usually set by the cmd.PreRunE that is added
		// by addCmd. But some commands (I'm looking at you __complete) don't
		// interact with that mechanism. So, we set the field here for those
		// odd cases.
		rc.Cmd = cmd
	}
	return rc
}

// RunContext is a container for injectable resources passed
// to all execX funcs. The Close method should be invoked when
// the RunContext is no longer needed.
type RunContext struct {
	// Stdin typically is os.Stdin, but can be changed for testing.
	Stdin *os.File

	// Out is the output destination.
	// If nil, default to stdout.
	Out io.Writer

	// ErrOut is the error output destination.
	// If nil, default to stderr.
	ErrOut io.Writer

	// Cmd is the command instance provided by cobra for
	// the currently executing command. This field will
	// be set before the command's runFunc is invoked.
	Cmd *cobra.Command

	// Log is the run's logger.
	Log *slog.Logger

	// Args is the arg slice supplied by cobra for
	// the currently executing command. This field will
	// be set before the command's runFunc is invoked.
	Args []string

	// Config is the run's config.
	Config *config.Config

	// ConfigStore is run's config store.
	ConfigStore config.Store

	initOnce sync.Once
	initErr  error

	// writers holds the various writer types that
	// the CLI uses to print output.
	writers *writers

	registry  *driver.Registry
	files     *source.Files
	databases *driver.Databases
	clnup     *cleanup.Cleanup
}

// newDefaultRunContext returns a RunContext configured
// with standard values for logging, config, etc. This
// effectively is the bootstrap mechanism for sq.
//
// Note: This func always returns a RunContext, even if
// an error occurs during bootstrap of the RunContext (for
// example if there's a config error). We do this to provide
// enough framework so that such an error can be logged or
// printed per the normal mechanisms if at all possible.
func newDefaultRunContext(_ context.Context, stdin *os.File,
	stdout, stderr io.Writer, args []string,
) (*RunContext, error) {
	rc := &RunContext{
		Stdin:  stdin,
		Out:    stdout,
		ErrOut: stderr,
	}

	cfg, cfgStore, configErr := config.DefaultLoad(args)
	rc.ConfigStore = cfgStore
	rc.Config = cfg

	log, clnup, loggingErr := defaultLogging()
	rc.Log = log
	rc.clnup = clnup

	switch {
	case rc.clnup == nil:
		rc.clnup = cleanup.New()
	case rc.Config == nil:
		rc.Config = config.New()
	}

	if configErr != nil {
		// configErr is more important, return that first
		return rc, configErr
	}

	if loggingErr != nil {
		return rc, loggingErr
	}

	return rc, nil
}

// init is invoked by cobra prior to the command RunE being
// invoked. It sets up the registry, databases, writers and related
// fundamental components. Subsequent invocations of this method
// are no-op.
func (rc *RunContext) init() error {
	if rc == nil {
		return errz.New("fatal: RunContext is nil")
	}

	rc.initOnce.Do(func() {
		rc.initErr = rc.doInit()
	})

	return rc.initErr
}

// doInit performs the actual work of initializing rc.
// It must only be invoked once.
func (rc *RunContext) doInit() error {
	rc.clnup = cleanup.New()
	cfg, log := rc.Config, rc.Log

	// If the --output=/some/file flag is set, then we need to
	// override rc.Out (which is typically stdout) to point it at
	// the output destination file.
	if cmdFlagChanged(rc.Cmd, flag.Output) {
		fpath, _ := rc.Cmd.Flags().GetString(flag.Output)
		fpath, err := filepath.Abs(fpath)
		if err != nil {
			return errz.Wrapf(err, "failed to get absolute path for --%s", flag.Output)
		}

		// Ensure the parent dir exists
		err = os.MkdirAll(filepath.Dir(fpath), os.ModePerm)
		if err != nil {
			return errz.Wrapf(err, "failed to make parent dir for --%s", flag.Output)
		}

		f, err := os.Create(fpath)
		if err != nil {
			return errz.Wrapf(err, "failed to open file specified by flag --%s", flag.Output)
		}

		rc.clnup.AddC(f) // Make sure the file gets closed eventually
		rc.Out = f
	}

	rc.writers, rc.Out, rc.ErrOut = newWriters(rc.Cmd, rc.Config.Options, rc.Out, rc.ErrOut)

	var scratchSrcFunc driver.ScratchSrcFunc

	// scratchSrc could be nil, and that's ok
	scratchSrc := cfg.Collection.Scratch()
	if scratchSrc == nil {
		scratchSrcFunc = sqlite3.NewScratchSource
	} else {
		scratchSrcFunc = func(log *slog.Logger, name string) (src *source.Source, clnup func() error, err error) {
			return scratchSrc, nil, nil
		}
	}

	var err error
	rc.files, err = source.NewFiles(rc.Log)
	if err != nil {
		lg.WarnIfFuncError(rc.Log, lga.Cleanup, rc.clnup.Run)
		return err
	}

	// Note: it's important that files.Close is invoked
	// after databases.Close (hence added to clnup first),
	// because databases could depend upon the existence of
	// files (such as a sqlite db file).
	rc.clnup.AddE(rc.files.Close)
	rc.files.AddTypeDetectors(source.DetectMagicNumber)

	rc.registry = driver.NewRegistry(log)
	rc.databases = driver.NewDatabases(log, rc.registry, scratchSrcFunc)
	rc.clnup.AddC(rc.databases)

	// TODO: this should come from user config.
	sqlCfg := driver.Tuning.SQLConfig

	rc.registry.AddProvider(sqlite3.Type, &sqlite3.Provider{Log: log})
	rc.registry.AddProvider(postgres.Type, &postgres.Provider{Log: log, SQLConfig: sqlCfg})
	rc.registry.AddProvider(sqlserver.Type, &sqlserver.Provider{Log: log, SQLConfig: sqlCfg})
	rc.registry.AddProvider(mysql.Type, &mysql.Provider{Log: log, SQLConfig: sqlCfg})
	csvp := &csv.Provider{Log: log, Scratcher: rc.databases, Files: rc.files}
	rc.registry.AddProvider(csv.TypeCSV, csvp)
	rc.registry.AddProvider(csv.TypeTSV, csvp)
	rc.files.AddTypeDetectors(csv.DetectCSV, csv.DetectTSV)

	jsonp := &json.Provider{Log: log, Scratcher: rc.databases, Files: rc.files}
	rc.registry.AddProvider(json.TypeJSON, jsonp)
	rc.registry.AddProvider(json.TypeJSONA, jsonp)
	rc.registry.AddProvider(json.TypeJSONL, jsonp)
	rc.files.AddTypeDetectors(json.DetectJSON, json.DetectJSONA, json.DetectJSONL)

	rc.registry.AddProvider(xlsx.Type, &xlsx.Provider{Log: log, Scratcher: rc.databases, Files: rc.files})
	rc.files.AddTypeDetectors(xlsx.DetectXLSX)
	// One day we may have more supported user driver genres.
	userDriverImporters := map[string]userdriver.ImportFunc{
		xmlud.Genre: xmlud.Import,
	}

	for i, userDriverDef := range cfg.Ext.UserDrivers {
		userDriverDef := userDriverDef

		errs := userdriver.ValidateDriverDef(userDriverDef)
		if len(errs) > 0 {
			err := errz.Combine(errs...)
			err = errz.Wrapf(err, "failed validation of user driver definition [%d] {%s} from config",
				i, userDriverDef.Name)
			return err
		}

		importFn, ok := userDriverImporters[userDriverDef.Genre]
		if !ok {
			return errz.Errorf("unsupported genre {%s} for user driver {%s} specified via config",
				userDriverDef.Genre, userDriverDef.Name)
		}

		// For each user driver definition, we register a
		// distinct userdriver.Provider instance.
		udp := &userdriver.Provider{
			Log:       log,
			DriverDef: userDriverDef,
			ImportFn:  importFn,
			Scratcher: rc.databases,
			Files:     rc.files,
		}

		rc.registry.AddProvider(source.Type(userDriverDef.Name), udp)
		rc.files.AddTypeDetectors(udp.TypeDetectors()...)
	}

	return nil
}

// Close should be invoked to dispose of any open resources
// held by rc. If an error occurs during Close and rc.Log
// is not nil, that error is logged at WARN level before
// being returned.
func (rc *RunContext) Close() error {
	if rc == nil {
		return nil
	}

	err := rc.clnup.Run()
	if err != nil && rc.Log != nil {
		rc.Log.Warn("Failed to close RunContext", lga.Err, err)
	}

	return err
}
