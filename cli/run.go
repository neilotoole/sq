package cli

import (
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/neilotoole/sq/cli/config"
	"github.com/neilotoole/sq/cli/config/yamlstore"
	v0_34_0 "github.com/neilotoole/sq/cli/config/yamlstore/upgrades/v0.34.0"
	"github.com/neilotoole/sq/cli/flag"
	"github.com/neilotoole/sq/cli/run"
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
	"github.com/neilotoole/sq/libsq/core/lg/slogbuf"
	"github.com/neilotoole/sq/libsq/core/options"
	"github.com/neilotoole/sq/libsq/driver"
	"github.com/neilotoole/sq/libsq/source"
	"github.com/neilotoole/sq/libsq/source/drivertype"
)

// getRun is a convenience function for getting Run
// from the cmd.Context().
func getRun(cmd *cobra.Command) *run.Run {
	ru := run.FromContext(cmd.Context())
	if ru.Cmd == nil {
		// ru.Cmd is usually set by the cmd.preRun that is added
		// by addCmd. But some commands (I'm looking at you __complete) don't
		// interact with that mechanism. So, we set the field here for those
		// odd cases.
		ru.Cmd = cmd
	}
	return ru
}

// newRun returns a run.Run configured with standard values for logging,
// config, etc. This effectively is the bootstrap mechanism for sq.
// Note that the run.Run is not fully configured for use by a command
// until preRun is executed on it.
//
// Note: This func always returns a Run, even if an error occurs during
// bootstrap of the Run (for example if there's a config error). We do this
// to provide enough framework so that such an error can be logged or
// printed per the normal mechanisms, if at all possible.
func newRun(ctx context.Context, stdin *os.File, stdout, stderr io.Writer, args []string,
) (*run.Run, *slog.Logger, error) {
	// logbuf holds log records until defaultLogging is completed.
	log, logbuf := slogbuf.New()
	log = log.With(lga.Pid, os.Getpid())

	ru := &run.Run{
		Stdin:           stdin,
		Out:             stdout,
		ErrOut:          stderr,
		OptionsRegistry: &options.Registry{},
	}

	RegisterDefaultOpts(ru.OptionsRegistry)

	upgrades := yamlstore.UpgradeRegistry{
		v0_34_0.Version: v0_34_0.Upgrade,
	}

	ctx = lg.NewContext(ctx, log)

	var configErr error
	ru.Config, ru.ConfigStore, configErr = yamlstore.Load(ctx,
		args, ru.OptionsRegistry, upgrades)

	log, logHandler, logCloser, logErr := defaultLogging(ctx, args, ru.Config)
	ru.Cleanup = cleanup.New().AddE(logCloser)
	if logErr != nil {
		stderrLog, h := stderrLogger()
		_ = logbuf.Flush(ctx, h)
		return ru, stderrLog, logErr
	}

	if logHandler != nil {
		if err := logbuf.Flush(ctx, logHandler); err != nil {
			return ru, log, err
		}
	}

	if log == nil {
		log = lg.Discard()
	}

	log = log.With(lga.Pid, os.Getpid())

	if ru.Config == nil {
		ru.Config = config.New()
	}

	if configErr != nil {
		// configErr is more important, return that first
		return ru, log, configErr
	}

	return ru, log, nil
}

// FinishRunInit finishes setting up ru.
//
// TODO: This run.Run initialization mechanism is a bit of a mess.
// There's logic in newRun, preRun, FinishRunInit, as well as testh.Helper.init.
// Surely the init logic can be consolidated.
func FinishRunInit(ctx context.Context, ru *run.Run) error {
	if ru.Cleanup == nil {
		ru.Cleanup = cleanup.New()
	}

	cfg, log := ru.Config, lg.FromContext(ctx)

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

	var err error
	if ru.Files == nil {
		ru.Files, err = source.NewFiles(ctx, source.DefaultTempDir(), source.DefaultCacheDir())
		if err != nil {
			lg.WarnIfFuncError(log, lga.Cleanup, ru.Cleanup.Run)
			return err
		}
	}

	// Note: it's important that files.Close is invoked
	// after databases.Close (hence added to clnup first),
	// because databases could depend upon the existence of
	// files (such as a sqlite db file).
	ru.Cleanup.AddE(ru.Files.Close)
	ru.Files.AddDriverDetectors(source.DetectMagicNumber)

	ru.DriverRegistry = driver.NewRegistry(log)
	dr := ru.DriverRegistry

	ru.Grips = driver.NewGrips(log, dr, ru.Files, scratchSrcFunc)
	ru.Cleanup.AddC(ru.Grips)

	dr.AddProvider(sqlite3.Type, &sqlite3.Provider{Log: log})
	dr.AddProvider(postgres.Type, &postgres.Provider{Log: log})
	dr.AddProvider(sqlserver.Type, &sqlserver.Provider{Log: log})
	dr.AddProvider(mysql.Type, &mysql.Provider{Log: log})
	csvp := &csv.Provider{Log: log, Ingester: ru.Grips, Files: ru.Files}
	dr.AddProvider(csv.TypeCSV, csvp)
	dr.AddProvider(csv.TypeTSV, csvp)
	ru.Files.AddDriverDetectors(csv.DetectCSV, csv.DetectTSV)

	jsonp := &json.Provider{Log: log, Ingester: ru.Grips, Files: ru.Files}
	dr.AddProvider(json.TypeJSON, jsonp)
	dr.AddProvider(json.TypeJSONA, jsonp)
	dr.AddProvider(json.TypeJSONL, jsonp)
	sampleSize := driver.OptIngestSampleSize.Get(cfg.Options)
	ru.Files.AddDriverDetectors(
		json.DetectJSON(sampleSize),
		json.DetectJSONA(sampleSize),
		json.DetectJSONL(sampleSize),
	)

	dr.AddProvider(xlsx.Type, &xlsx.Provider{Log: log, Ingester: ru.Grips, Files: ru.Files})
	ru.Files.AddDriverDetectors(xlsx.DetectXLSX)
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
			Ingester:  ru.Grips,
			Files:     ru.Files,
		}

		ru.DriverRegistry.AddProvider(drivertype.Type(userDriverDef.Name), udp)
		ru.Files.AddDriverDetectors(udp.Detectors()...)
	}

	return nil
}

// preRun is invoked by cobra prior to the command's RunE being
// invoked. It sets up the driver registry, databases, writers and related
// fundamental components. Subsequent invocations of this method
// are no-op.
func preRun(cmd *cobra.Command, ru *run.Run) error {
	if ru == nil {
		return errz.New("Run is nil")
	}

	if ru.Writers != nil {
		// If ru.Writers is already set, then this function has already been
		// called on ru. That's ok, just return.
		return nil
	}

	if ru.Cleanup == nil {
		ru.Cleanup = cleanup.New()
	}

	ctx := cmd.Context()
	// If the --output=/some/file flag is set, then we need to
	// override ru.Out (which is typically stdout) to point it at
	// the output destination file.
	if cmdFlagChanged(ru.Cmd, flag.Output) {
		fpath, _ := ru.Cmd.Flags().GetString(flag.Output)
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

		ru.Cleanup.AddC(f) // Make sure the file gets closed eventually
		ru.Out = f
	}

	cmdOpts, err := getOptionsFromCmd(ru.Cmd)
	if err != nil {
		return err
	}
	ru.Writers, ru.Out, ru.ErrOut = newWriters(ru.Cmd, cmdOpts, ru.Out, ru.ErrOut)

	return FinishRunInit(ctx, ru)
}
