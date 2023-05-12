package cli

import (
	"context"
	"io"
	"os"
	"path/filepath"

	"github.com/neilotoole/sq/cli/run"

	"github.com/neilotoole/sq/drivers"

	"github.com/neilotoole/sq/cli/config/yamlstore"
	v0_34_0 "github.com/neilotoole/sq/cli/config/yamlstore/upgrades/v0.34.0"
	"github.com/neilotoole/sq/libsq/core/lg/slogbuf"
	"github.com/neilotoole/sq/libsq/core/options"

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

// getRunContext is a convenience function for getting Run
// from the cmd.Context().
func getRunContext(cmd *cobra.Command) *run.Run {
	rc := run.FromContext(cmd.Context())
	if rc.Cmd == nil {
		// rc.Cmd is usually set by the cmd.PreRun that is added
		// by addCmd. But some commands (I'm looking at you __complete) don't
		// interact with that mechanism. So, we set the field here for those
		// odd cases.
		rc.Cmd = cmd
	}
	return rc
}

// newRunContext returns a Run configured
// with standard values for logging, config, etc. This
// effectively is the bootstrap mechanism for sq.
//
// Note: This func always returns a Run, even if
// an error occurs during bootstrap of the Run (for
// example if there's a config error). We do this to provide
// enough framework so that such an error can be logged or
// printed per the normal mechanisms if at all possible.
func newRunContext(ctx context.Context,
	stdin *os.File, stdout, stderr io.Writer, args []string,
) (*run.Run, *slog.Logger, error) {
	// logbuf holds log records until defaultLogging is completed.
	log, logbuf := slogbuf.New()
	log = log.With(lga.Pid, os.Getpid())

	rc := &run.Run{
		Stdin:           stdin,
		Out:             stdout,
		ErrOut:          stderr,
		OptionsRegistry: &options.Registry{},
	}

	RegisterDefaultOpts(rc.OptionsRegistry)

	upgrades := yamlstore.UpgradeRegistry{
		v0_34_0.Version: v0_34_0.Upgrade,
	}

	ctx = lg.NewContext(ctx, log)

	var configErr error
	rc.Config, rc.ConfigStore, configErr = yamlstore.Load(ctx,
		args, rc.OptionsRegistry, upgrades)

	log, logHandler, logCloser, logErr := defaultLogging(ctx, args, rc.Config)
	rc.Cleanup = cleanup.New().AddE(logCloser)
	if logErr != nil {
		stderrLog, h := stderrLogger()
		_ = logbuf.Flush(ctx, h)
		return rc, stderrLog, logErr
	}

	if logHandler != nil {
		if err := logbuf.Flush(ctx, logHandler); err != nil {
			return rc, log, err
		}
	}

	if log == nil {
		log = lg.Discard()
	}

	log = log.With(lga.Pid, os.Getpid())

	if rc.Config == nil {
		rc.Config = config.New()
	}

	if configErr != nil {
		// configErr is more important, return that first
		return rc, log, configErr
	}

	return rc, log, nil
}

// PreRun is invoked by cobra prior to the command RunE being
// invoked. It sets up the driverReg, databases, Writers and related
// fundamental components. Subsequent invocations of this method
// are no-op.
func PreRun(ctx context.Context, rc *run.Run) error {
	if rc == nil {
		return errz.New("Run is nil")
	}

	if rc.Cleanup != nil {
		lg.FromContext(ctx).Error("Run already initialized")
		return errz.New("Run already initialized")
	}

	rc.Cleanup = cleanup.New()
	cfg, log := rc.Config, lg.FromContext(ctx)

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

		rc.Cleanup.AddC(f) // Make sure the file gets closed eventually
		rc.Out = f
	}

	cmdOpts, err := getOptionsFromCmd(rc.Cmd)
	if err != nil {
		return err
	}
	rc.Writers, rc.Out, rc.ErrOut = newWriters(rc.Cmd, cmdOpts, rc.Out, rc.ErrOut)

	var scratchSrcFunc driver.ScratchSrcFunc

	// scratchSrc could be nil, and that's ok
	scratchSrc := cfg.Collection.Scratch()
	if scratchSrc == nil {
		scratchSrcFunc = sqlite3.NewScratchSource
	} else {
		scratchSrcFunc = func(_ context.Context, name string) (src *source.Source, clnup func() error, err error) {
			return scratchSrc, nil, nil
		}
	}

	rc.Files, err = source.NewFiles(ctx)
	if err != nil {
		lg.WarnIfFuncError(log, lga.Cleanup, rc.Cleanup.Run)
		return err
	}

	// Note: it's important that files.Close is invoked
	// after databases.Close (hence added to clnup first),
	// because databases could depend upon the existence of
	// files (such as a sqlite db file).
	rc.Cleanup.AddE(rc.Files.Close)
	rc.Files.AddDriverDetectors(source.DetectMagicNumber)

	rc.DriverRegistry = driver.NewRegistry(log)
	rc.Databases = driver.NewDatabases(log, rc.DriverRegistry, scratchSrcFunc)
	rc.Cleanup.AddC(rc.Databases)

	rc.DriverRegistry.AddProvider(sqlite3.Type, &sqlite3.Provider{Log: log})
	rc.DriverRegistry.AddProvider(postgres.Type, &postgres.Provider{Log: log})
	rc.DriverRegistry.AddProvider(sqlserver.Type, &sqlserver.Provider{Log: log})
	rc.DriverRegistry.AddProvider(mysql.Type, &mysql.Provider{Log: log})
	csvp := &csv.Provider{Log: log, Scratcher: rc.Databases, Files: rc.Files}
	rc.DriverRegistry.AddProvider(csv.TypeCSV, csvp)
	rc.DriverRegistry.AddProvider(csv.TypeTSV, csvp)
	rc.Files.AddDriverDetectors(csv.DetectCSV, csv.DetectTSV)

	jsonp := &json.Provider{Log: log, Scratcher: rc.Databases, Files: rc.Files}
	rc.DriverRegistry.AddProvider(json.TypeJSON, jsonp)
	rc.DriverRegistry.AddProvider(json.TypeJSONA, jsonp)
	rc.DriverRegistry.AddProvider(json.TypeJSONL, jsonp)
	sampleSize := drivers.OptIngestSampleSize.Get(cfg.Options)
	rc.Files.AddDriverDetectors(
		json.DetectJSON(sampleSize),
		json.DetectJSONA(sampleSize),
		json.DetectJSONL(sampleSize),
	)

	rc.DriverRegistry.AddProvider(xlsx.Type, &xlsx.Provider{Log: log, Scratcher: rc.Databases, Files: rc.Files})
	rc.Files.AddDriverDetectors(xlsx.DetectXLSX)
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
			Scratcher: rc.Databases,
			Files:     rc.Files,
		}

		rc.DriverRegistry.AddProvider(source.DriverType(userDriverDef.Name), udp)
		rc.Files.AddDriverDetectors(udp.Detectors()...)
	}

	return nil
}
