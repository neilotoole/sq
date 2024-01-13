package cli

import (
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/neilotoole/sq/cli/flag"

	"github.com/neilotoole/sq/libsq/core/ioz/checksum"
	"github.com/neilotoole/sq/libsq/core/ioz/lockfile"

	"github.com/spf13/cobra"

	"github.com/neilotoole/sq/cli/config"
	"github.com/neilotoole/sq/cli/config/yamlstore"
	v0_34_0 "github.com/neilotoole/sq/cli/config/yamlstore/upgrades/v0.34.0" //nolint:revive
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
	"github.com/neilotoole/sq/libsq/core/progress"
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
// until preRun and FinishRunInit are executed on it.
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
	ru.Cleanup = cleanup.New()
	// FIXME: re-enable log closing
	ru.LogCloser = logCloser
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

	ctx := cmd.Context()
	if ru.Cleanup == nil {
		ru.Cleanup = cleanup.New()
	}

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
	ru.Writers, ru.Out, ru.ErrOut = newWriters(ru.Cmd, ru.Cleanup, cmdOpts, ru.Out, ru.ErrOut)

	if err = FinishRunInit(ctx, ru); err != nil {
		return err
	}

	if cmdRequiresConfigLock(cmd) {
		var unlock func()
		if unlock, err = lockReloadConfig(cmd); err != nil {
			return err
		}
		ru.Cleanup.Add(unlock)
	}
	return nil
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
	// The Files instance may already have been created. If not, create it.
	if ru.Files == nil {
		var cfgLock lockfile.Lockfile
		if cfgLock, err = ru.ConfigStore.Lockfile(); err != nil {
			return err
		}
		cfgLockFunc := newProgressLockFunc(
			cfgLock,
			"acquire config lock",
			config.OptConfigLockTimeout.Get(options.FromContext(ctx)),
		)

		// We use cache and temp dirs with paths based on a hash of the config's
		// location. This ensures that multiple sq instances using different
		// configs don't share the same cache/temp dir.
		sum := checksum.Sum([]byte(ru.ConfigStore.Location()))

		ru.Files, err = source.NewFiles(
			ctx,
			ru.OptionsRegistry,
			cfgLockFunc,
			filepath.Join(source.DefaultTempDir(), sum),
			filepath.Join(source.DefaultCacheDir(), sum),
		)
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

	ru.DriverRegistry = driver.NewRegistry(log)
	dr := ru.DriverRegistry

	ru.Grips = driver.NewGrips(dr, ru.Files, scratchSrcFunc)
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

	for i, udd := range cfg.Ext.UserDrivers {
		udd := udd

		errs := userdriver.ValidateDriverDef(udd)
		if len(errs) > 0 {
			err = errz.Combine(errs...)
			err = errz.Wrapf(err, "failed validation of user driver definition [%d] {%s} from config",
				i, udd.Name)
			return err
		}

		importFn, ok := userDriverImporters[udd.Genre]
		if !ok {
			return errz.Errorf("unsupported genre {%s} for user driver {%s} specified via config",
				udd.Genre, udd.Name)
		}

		// For each user driver definition, we register a
		// distinct userdriver.Provider instance.
		udp := &userdriver.Provider{
			Log:       log,
			DriverDef: udd,
			ImportFn:  importFn,
			Ingester:  ru.Grips,
			Files:     ru.Files,
		}

		ru.DriverRegistry.AddProvider(drivertype.Type(udd.Name), udp)
		ru.Files.AddDriverDetectors(udp.Detectors()...)
	}

	return nil
}

// markCmdRequiresConfigLock marks cmd as requiring a config lock.
// Thus, before the command's RunE is invoked, the config lock
// is acquired (in preRun), and released on cleanup.
func markCmdRequiresConfigLock(cmd *cobra.Command) {
	if cmd.Annotations == nil {
		cmd.Annotations = make(map[string]string)
	}
	cmd.Annotations["config.lock"] = "true"
}

// cmdRequiresConfigLock returns true if markCmdRequiresConfigLock was
// previously invoked on cmd.
func cmdRequiresConfigLock(cmd *cobra.Command) bool {
	return cmd.Annotations != nil && cmd.Annotations["config.lock"] == "true"
}

// lockReloadConfig acquires the lock for the config store, and updates the
// run (as found on cmd's context) with a fresh copy of the config, loaded
// after lock acquisition.
//
// The config lock should be acquired before making any changes to config.
// Timeout and progress options from ctx are honored.
// The caller is responsible for invoking the returned unlock func.
// Example usage:
//
//	if unlock, err := lockReloadConfig(cmd); err != nil {
//		return err
//	} else {
//		defer unlock()
//	}
//
// However, in practice, most commands will invoke markCmdRequiresConfigLock
// instead of explicitly invoking lockReloadConfig.
func lockReloadConfig(cmd *cobra.Command) (unlock func(), err error) {
	ctx := cmd.Context()
	ru := run.FromContext(ctx)
	if ru.ConfigStore == nil {
		return nil, errz.New("config store is nil")
	}

	lock, err := ru.ConfigStore.Lockfile()
	if err != nil {
		return nil, errz.Wrap(err, "failed to get config lock")
	}

	lockTimeout := config.OptConfigLockTimeout.Get(options.FromContext(ctx))
	bar := progress.FromContext(ctx).NewTimeoutWaiter(
		"Acquire config lock",
		time.Now().Add(lockTimeout),
	)

	err = lock.Lock(ctx, lockTimeout)
	bar.Stop()
	if err != nil {
		return nil, errz.Wrap(err, "acquire config lock")
	}

	var cfg *config.Config
	if cfg, err = ru.ConfigStore.Load(ctx); err != nil {
		// An error occurred reloading config; release the lock before returning.
		if unlockErr := lock.Unlock(); unlockErr != nil {
			lg.FromContext(ctx).Warn("Failed to release config lock",
				lga.Lock, lock, lga.Err, unlockErr)
		}
		return nil, err
	}

	// Update the run with the fresh config.
	ru.Config = cfg

	return func() {
		if unlockErr := lock.Unlock(); unlockErr != nil {
			lg.FromContext(ctx).Warn("Failed to release config lock",
				lga.Lock, lock, lga.Err, unlockErr)
		}
	}, nil
}

// newProgressLockFunc returns a new lockfile.LockFunc that that acquires lock,
// and displays a progress bar while doing so.
func newProgressLockFunc(lock lockfile.Lockfile, msg string, timeout time.Duration) lockfile.LockFunc {
	return func(ctx context.Context) (unlock func(), err error) {
		bar := progress.FromContext(ctx).NewTimeoutWaiter(
			msg,
			time.Now().Add(timeout),
		)
		err = lock.Lock(ctx, timeout)
		bar.Stop()
		if err != nil {
			return nil, errz.Wrap(err, msg)
		}
		return func() {
			if err = lock.Unlock(); err != nil {
				lg.FromContext(ctx).With(lga.Lock, lock, "for", msg).
					Warn("Failed to release lock", lga.Err, err)
			}
		}, nil
	}
}
