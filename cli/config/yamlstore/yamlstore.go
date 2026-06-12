// Package yamlstore contains an implementation of config.Store that
// uses YAML files for persistence.
package yamlstore

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/neilotoole/sq/cli/buildinfo"
	"github.com/neilotoole/sq/cli/config"
	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/ioz"
	"github.com/neilotoole/sq/libsq/core/ioz/lockfile"
	"github.com/neilotoole/sq/libsq/core/lg"
	"github.com/neilotoole/sq/libsq/core/lg/lga"
	"github.com/neilotoole/sq/libsq/core/options"
	"github.com/neilotoole/sq/libsq/source"
)

// Origin of the config path.
// See Store.PathOrigin.

var _ config.Store = (*Store)(nil)

// Store provides persistence of config via YAML file.
// It implements config.Store.
type Store struct {
	// If HookLoad is non-nil, it is invoked by Load
	// on Path's bytes before the YAML is unmarshalled.
	// This allows expansion of variables etc.
	HookLoad func(data []byte) ([]byte, error)

	// UpgradeRegistry holds upgrade funcs for upgrading the config file.
	UpgradeRegistry UpgradeRegistry

	// OptionsRegistry holds options.
	OptionsRegistry *options.Registry
	// Path is the location of the config file
	Path string

	// PathOrigin is one of "flag", "env", or "default".
	PathOrigin config.Origin

	// newerCfgVers is set by Load when the config file's version is
	// newer than the build version (the config was written by a newer
	// sq version). When non-empty, Save writes a verbatim backup of
	// the config file before overwriting it: see backupNewerConfig.
	newerCfgVers string

	// ExtPaths holds locations of potential ext config, both dirs and files (with suffix ".sq.yml")
	ExtPaths []string
}

// Lockfile implements Store.Lockfile.
func (fs *Store) Lockfile() (lockfile.Lockfile, error) {
	fp := filepath.Join(filepath.Dir(fs.Path), "config.pid.lock")
	fp, err := filepath.Abs(fp)
	if err != nil {
		return "", errz.Wrap(err, "failed to get abs path for lockfile")
	}
	return lockfile.Lockfile(fp), nil
}

// String returns a log/debug-friendly representation.
func (fs *Store) String() string {
	return fmt.Sprintf("config via %s: %v", fs.PathOrigin, fs.Path)
}

// Location implements Store. It returns the location of the config dir.
func (fs *Store) Location() string {
	return filepath.Dir(fs.Path)
}

// Load reads config from disk. It implements Store.
func (fs *Store) Load(ctx context.Context) (*config.Config, error) {
	log := lg.FromContext(ctx)
	log.Debug("Loading config from file", lga.Path, fs.Path)

	if fs.UpgradeRegistry != nil {
		// The config schema version (highest registered upgrade version),
		// derived once and used for both the needsUpgrade decision and the
		// upgrade target, so the two can't drift.
		schemaVers := fs.UpgradeRegistry.highestVersion()

		mightNeedUpgrade, foundVers, checkErr := fs.checkNeedsUpgrade(ctx, schemaVers)
		if err := fs.applyVersionCheck(ctx, foundVers, checkErr); err != nil {
			return nil, err
		}

		if mightNeedUpgrade {
			// The config might need to be upgraded. But, there's an edge case
			// where another process might upgrade the config file before we
			// get a chance to do so. So, we acquire the config lock, and
			// then check again if it still needs upgrade.
			unlock, err := fs.acquireLock(ctx)
			if err != nil {
				return nil, err
			}
			defer unlock()

			// Lock is acquired; re-check, because another process may have
			// upgraded the config while we were waiting for the lock.
			mightNeedUpgrade, foundVers, checkErr = fs.checkNeedsUpgrade(ctx, schemaVers)
			if err = fs.applyVersionCheck(ctx, foundVers, checkErr); err != nil {
				return nil, err
			}

			if mightNeedUpgrade {
				// Upgrade to the config schema version, NOT the sq build version.
				log.Info("Upgrade config?", lga.From, foundVers, lga.To, schemaVers)
				// doUpgrade re-marshals the upgraded config from the struct
				// (canonical key order) and saves it, so no extra load-save
				// cycle is needed here; the doLoad below returns it.
				if _, err = fs.doUpgrade(ctx, foundVers, schemaVers); err != nil {
					return nil, err
				}
			}
		}
	}

	return fs.doLoad(ctx)
}

// applyVersionCheck records or clears the newer-than-build state from a
// checkNeedsUpgrade result and returns the fatal error (already wrapped),
// if any. A result that is not errConfigVersionNewerThanBuild clears
// fs.newerCfgVers, so a post-lock re-check (or a later Load on a reused
// Store) that no longer sees a newer-than-build config doesn't leave the
// flag set and trigger a spurious backup on the next Save.
func (fs *Store) applyVersionCheck(ctx context.Context, foundVers string, checkErr error) error {
	switch {
	case checkErr == nil:
		fs.newerCfgVers = ""
		return nil
	case errors.Is(checkErr, errConfigVersionNewerThanBuild):
		// The config was created by a newer sq version, so it may contain
		// fields this version doesn't recognize. Continue optimistically
		// (this lets users downgrade sq for testing/debugging); the next
		// Save writes a verbatim backup of the config first. See
		// backupNewerConfig.
		fs.newerCfgVers = foundVers
		lg.FromContext(ctx).Warn("Config version is newer than sq version; continuing anyway",
			lga.ConfigVersion, foundVers,
			lga.BuildVersion, buildinfo.Version)
		return nil
	default:
		// A fatal check error leaves the version state indeterminate.
		// Clear newerCfgVers anyway so a later Save on a reused Store
		// can't act on a stale newer-than-build result from an earlier
		// Load (the doc's "any non-newer result clears it" invariant).
		fs.newerCfgVers = ""
		return errz.Wrapf(checkErr, "config: %s", fs.Path)
	}
}

func (fs *Store) doLoad(ctx context.Context) (*config.Config, error) {
	data, err := os.ReadFile(fs.Path)
	if err != nil {
		return nil, errz.Wrapf(err, "config: failed to load file: %s", fs.Path)
	}

	loadHookFn := fs.HookLoad
	if loadHookFn != nil {
		data, err = loadHookFn(data)
		if err != nil {
			return nil, err
		}
	}

	cfg := config.New()
	if err = ioz.UnmarshallYAML(data, cfg); err != nil {
		return nil, errz.Wrapf(err, "config: %s: failed to unmarshal config YAML", fs.Path)
	}

	if cfg.Version == "" {
		cfg.Version = buildinfo.Version
	}

	if cfg.Options == nil {
		cfg.Options = options.Options{}
	}

	if cfg.Options, err = fs.OptionsRegistry.Process(cfg.Options); err != nil {
		return nil, errz.Wrapf(err, "config: %s", fs.Path)
	}

	if cfg.Collection == nil {
		cfg.Collection = &source.Collection{}
	}

	repaired, err := source.VerifyIntegrity(cfg.Collection)
	if err != nil {
		if repaired {
			// The config was repaired. Save the changes.
			err = errz.Combine(err, fs.Save(ctx, cfg))
		}
		return nil, errz.Wrapf(err, "config: %s", fs.Path)
	}

	if err = fs.loadExt(cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

// Save writes config to disk. It implements Store.
func (fs *Store) Save(ctx context.Context, cfg *config.Config) error {
	if fs == nil {
		return errz.New("config file store is nil")
	}

	if err := canonicalizeConfig(fs.OptionsRegistry, cfg); err != nil {
		return err
	}

	if err := fs.backupNewerConfig(ctx); err != nil {
		return err
	}

	data, err := ioz.MarshalYAML(cfg)
	if err != nil {
		return err
	}

	return fs.write(ctx, data)
}

// backupNewerConfig writes a verbatim backup of the config file before
// Save overwrites it, when Load found the config's version to be newer
// than the build version (fs.newerCfgVers is non-empty). The config
// was written by a newer sq version and may contain fields unknown to
// this build's config.Config struct; Save's re-marshal silently drops
// such fields, so the backup is the only surviving copy. The backup is
// written at most once: if the backup file already exists, it holds
// the newer config from an earlier run, and overwriting it with the
// current (possibly already-degraded) file would destroy that.
func (fs *Store) backupNewerConfig(ctx context.Context) error {
	if fs.newerCfgVers == "" {
		return nil
	}

	data, err := os.ReadFile(fs.Path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			// Nothing on disk to back up.
			fs.newerCfgVers = ""
			return nil
		}
		return errz.Wrapf(err, "failed to read config for backup before save: %s", fs.Path)
	}

	// Leave newerCfgVers set on error so a retried Save tries again.
	wrote, err := fs.writeConfigBackupOnce(ctx, fs.newerCfgVers, data)
	if err != nil {
		return err
	}
	if wrote {
		lg.FromContext(ctx).Warn(
			"Config version is newer than sq version; wrote verbatim backup of config before save",
			lga.ConfigVersion, fs.newerCfgVers,
			lga.BuildVersion, buildinfo.Version,
			lga.Path, backupFilePath(fs.Path, fs.newerCfgVers))
	}
	fs.newerCfgVers = ""
	return nil
}

// writeConfigBackupOnce writes data (the verbatim current config bytes) to
// the backup path for config version vers, unless a backup for that version
// already exists. The backup is never overwritten: an existing one may be
// the downgrade guard's pristine copy of a newer config, or an identical
// earlier copy, so rewriting loses nothing and risks clobbering it. The
// backup name deliberately does not end in ".sq.yml" (see backupFilePath).
//
// The backup is created with O_CREATE|O_EXCL, so the create is atomic
// against a concurrent writer (only one process can create the file) and an
// existing backup is never clobbered. This is portable, unlike a hard link
// (which fails on filesystems without hard-link support) or an atomic
// rename (which replaces the destination). O_EXCL also refuses to follow a
// symlink, and the ErrExist branch confirms via Lstat that any existing
// entry is a real regular file, not a symlink or directory. The bytes are
// fsync'd before close, since the backup's purpose is recoverability; the
// only window not covered is a crash between create and write, which would
// leave a partial backup (far rarer than the races this guards against).
//
// It returns wrote=true only when it created a new backup file. A failure is
// returned as an error; both callers (doUpgrade and backupNewerConfig) treat
// a backup failure as fatal, because the backup's purpose is guaranteed
// recoverability.
func (fs *Store) writeConfigBackupOnce(ctx context.Context, vers string, data []byte) (wrote bool, err error) {
	backupPath := backupFilePath(fs.Path, vers)

	f, err := os.OpenFile(backupPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, ioz.RWPerms)
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			// A backup already exists. Confirm with Lstat (don't follow a
			// symlink) that it's a real regular file before treating it as
			// the existing backup and letting the caller overwrite the
			// config.
			info, lstatErr := os.Lstat(backupPath)
			if lstatErr != nil {
				return false, errz.Wrapf(lstatErr, "failed to stat existing config backup: %s", backupPath)
			}
			if !info.Mode().IsRegular() {
				return false, errz.Errorf("config backup path exists but is not a regular file: %s", backupPath)
			}
			lg.FromContext(ctx).Info("Config backup already exists; not overwriting", lga.Path, backupPath)
			return false, nil
		}
		return false, errz.Wrapf(err, "failed to create config backup: %s", backupPath)
	}

	// Write, fsync, and close. On any failure remove the partial file so a
	// later run can't mistake it for a complete backup.
	if _, err = f.Write(data); err != nil {
		_ = f.Close()
		_ = os.Remove(backupPath)
		return false, errz.Wrapf(err, "failed to write config backup: %s", backupPath)
	}
	if err = f.Sync(); err != nil {
		_ = f.Close()
		_ = os.Remove(backupPath)
		return false, errz.Wrapf(err, "failed to sync config backup: %s", backupPath)
	}
	if err = f.Close(); err != nil {
		_ = os.Remove(backupPath)
		return false, errz.Wrapf(err, "failed to close config backup: %s", backupPath)
	}
	return true, nil
}

// Write writes the config bytes to disk.
func (fs *Store) write(ctx context.Context, data []byte) error {
	// It's possible that the parent dir of fs.Path doesn't exist.
	if err := ioz.RequireDir(filepath.Dir(fs.Path)); err != nil {
		return errz.Wrapf(err, "failed to make parent dir of config file: %s", filepath.Dir(fs.Path))
	}

	if err := ioz.WriteFileAtomic(fs.Path, data, ioz.RWPerms); err != nil {
		return errz.Wrap(err, "failed to save config file")
	}

	lg.FromContext(ctx).Info("Wrote config file", lga.Path, fs.Path)
	return nil
}

// Exists returns true if the backing file can be accessed, false if it doesn't
// exist or on any error.
func (fs *Store) Exists() bool {
	_, err := os.Stat(fs.Path)
	return err == nil
}

// acquireLock acquires the config lock, and returns an unlock func.
func (fs *Store) acquireLock(ctx context.Context) (unlock func(), err error) {
	lock, err := fs.Lockfile()
	if err != nil {
		return nil, errz.Wrap(err, "failed to get config lock")
	}

	// We use the default timeout because config isn't loaded yet,
	// so we don't know what the value is.
	lockTimeout := config.OptConfigLockTimeout.Default()
	if err = lock.Lock(ctx, lockTimeout); err != nil {
		return nil, errz.Wrap(err, "acquire config lock")
	}

	return func() {
		lg.WarnIfFuncError(lg.FromContext(ctx), "Release config lock", lock.Unlock)
	}, nil
}

// canonicalizeConfig checks cfg's validity, and patches cfg to the canonical
// form,cfg's validity. For example, an unknown or nil value in an
// options.Options is deleted.
func canonicalizeConfig(optsReg *options.Registry, cfg *config.Config) error {
	var err error
	if err = config.Valid(cfg); err != nil {
		return err
	}

	cfg.Options, err = optsReg.Process(cfg.Options)
	if err != nil {
		return errz.Wrapf(err, "processing 'config.options'")
	}

	cfg.Options = options.DeleteNil(cfg.Options)
	if cfg.Collection == nil {
		return nil
	}

	if err = cfg.Collection.Visit(func(src *source.Source) error {
		if src.Options, err = optsReg.Process(src.Options); err != nil {
			return errz.Wrapf(err, "processing source options for %s", src.Handle)
		}

		src.Options = options.DeleteNil(src.Options)
		return nil
	}); err != nil {
		return err
	}

	return config.Valid(cfg)
}
