package yamlstore

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/mod/semver"

	"github.com/neilotoole/sq/cli/buildinfo"
	"github.com/neilotoole/sq/cli/config"
	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/ioz"
	"github.com/neilotoole/sq/libsq/core/lg"
	"github.com/neilotoole/sq/libsq/core/lg/lga"
)

// MinConfigVersion is the minimum semver value of Config.Version.
// This is basically how far back in time the config upgrade process
// can support. If the config dates from prior to this (unlikely),
// then the user needs to start with a new config.
const MinConfigVersion = "v0.0.0-dev"

// UpgradeFunc performs a (single) upgrade of the config file. Typically
// a func will read the config data from disk, perform some transformation
// on it, and write it back out to disk. Note that the func should not
// bind the config file YAML to the Config object, as they may differ
// significantly. Instead, the func should bind the YAML to a map, and
// manipulate that map directly.
type UpgradeFunc func(ctx context.Context, before []byte) (after []byte, err error)

// UpgradeRegistry is a map of config_version to upgrade funcs.
type UpgradeRegistry map[string]UpgradeFunc

// doUpgrade runs all the registered upgrade funcs between startVersion
// and targetVersion, then stamps config.version with targetVersion.
// Load passes the highest version in the UpgradeRegistry (the config
// schema version) as targetVersion: config.version advances only when
// a registered upgrade func transforms the config, never to the sq
// binary version. If no upgrade func falls in the version range,
// doUpgrade leaves the config file untouched: the load-save cycle
// below is not byte-preserving (unknown keys, YAML comments, and
// formatting are lost on re-marshal), so it must not run for releases
// without schema changes.
func (fs *Store) doUpgrade(ctx context.Context, startVersion, targetVersion string) (*config.Config, error) {
	log := lg.FromContext(ctx)
	log.Debug("Starting config upgrade", lga.From, startVersion, lga.To, targetVersion)

	if !semver.IsValid(targetVersion) {
		return nil, errz.Errorf("invalid semver for config version {%s}", targetVersion)
	}

	upgradeFns := fs.UpgradeRegistry.getUpgradeFuncs(startVersion, targetVersion)
	if len(upgradeFns) == 0 {
		log.Debug("No config upgrade funcs to run; config file untouched")
		return fs.doLoad(ctx)
	}

	data, err := os.ReadFile(fs.Path)
	if err != nil {
		return nil, errz.Err(err)
	}

	// Write a verbatim backup before the upgrade funcs transform
	// the config.
	backupPath := backupFilePath(fs.Path, startVersion)
	if err = ioz.WriteFileAtomic(backupPath, data, ioz.RWPerms); err != nil {
		// Abort rather than continue without a backup: the point of
		// the backup is guaranteed recoverability, and if a sibling
		// file can't be written, rewriting the config itself is
		// unlikely to go better.
		return nil, errz.Wrapf(err, "failed to write config backup before upgrade: %s", backupPath)
	}
	log.Info("Wrote verbatim backup of config before upgrade", lga.Path, backupPath)

	for _, fn := range upgradeFns {
		log.Debug("Attempting config upgrade step")
		data, err = fn(ctx, data)
		if err != nil {
			return nil, err
		}
		log.Debug("Config upgrade step successful")
	}

	if err = fs.write(ctx, data); err != nil {
		return nil, err
	}

	// Do a final update of the version
	cfg, err := fs.doLoad(ctx)
	if err != nil {
		return nil, err
	}

	log.Debug("Setting config.version", lga.Val, targetVersion)
	cfg.Version = targetVersion

	err = fs.Save(ctx, cfg)
	if err != nil {
		return nil, err
	}

	return cfg, nil
}

// backupFilePath returns the path of the backup file for the config
// file at cfgPath, named for the config version being backed up:
// /path/to/sq.yml + v0.53.0 -> /path/to/sq.v0.53.0.bak.yml. It is
// used both for pre-upgrade backups (doUpgrade) and for the downgrade
// guard (Store.backupNewerConfig). The name deliberately does not end
// in ".sq.yml": Store.loadExt treats any such file in the config dir
// as ext config. A backup file from a prior upgrade of the same
// version is overwritten; it holds the same from-version content.
func backupFilePath(cfgPath, fromVersion string) string {
	base := filepath.Base(cfgPath)
	base = strings.TrimSuffix(base, filepath.Ext(base))
	return filepath.Join(filepath.Dir(cfgPath), fmt.Sprintf("%s.%s.bak.yml", base, fromVersion))
}

// getUpgradeFuncs returns the funcs required to upgrade from startingVersion
// to targetVersion. We iterate over the set of registered funcs; if the
// version (the key) is greater than startingVersion, and less than or equal
// to targetVersion, that UpgradeFunc will be included in the return value.
func (r UpgradeRegistry) getUpgradeFuncs(startingVersion, targetVersion string) []UpgradeFunc {
	if len(r) == 0 {
		return nil
	}

	var vers []string
	for k := range r {
		if semver.Compare(k, startingVersion) > 0 && semver.Compare(k, targetVersion) <= 0 {
			vers = append(vers, k)
		}
	}

	if len(vers) == 0 {
		return nil
	}

	semver.Sort(vers)

	upgradeFns := make([]UpgradeFunc, len(vers))
	for i, v := range vers {
		upgradeFns[i] = r[v]
	}

	return upgradeFns
}

// highestVersion returns the highest version key in the registry,
// which is the config schema version known to this build. Returns
// empty string if the registry is empty.
func (r UpgradeRegistry) highestVersion() string {
	var highest string
	for k := range r {
		if highest == "" || semver.Compare(k, highest) > 0 {
			highest = k
		}
	}
	return highest
}

// LoadVersionFromFile loads the version from the config file.
// If the field is not present, minConfigVersion (and no error) is returned.
func LoadVersionFromFile(path string) (string, error) {
	bytes, err := os.ReadFile(path)
	if err != nil {
		return "", errz.Wrapf(err, "failed to load file: %s", path)
	}

	m := map[string]any{}
	err = ioz.UnmarshallYAML(bytes, &m)
	if err != nil {
		return "", errz.Wrap(err, "failed to unmarshal config YAML")
	}

	// These are all the historical names for the version field
	// in the config YAML.
	candidates := []string{"version", "config_version", "config.version"}

	for _, field := range candidates {
		if v, ok := m[field]; ok {
			// Legacy "version" field.
			s, ok := v.(string)
			if !ok {
				return "", errz.Errorf("invalid value for {%s} field: %s", field, v)
			}

			s = strings.TrimSpace(s)
			if s == "" {
				continue
			}

			if !semver.IsValid(s) {
				return "", errz.Errorf("invalid semver value for {%s} field: %s", field, s)
			}

			return s, nil
		}
	}

	return "", errz.Errorf("config file does not have a version field: %v", path)
}

// errConfigVersionNewerThanBuild is a sentinel error returned by
// checkNeedsUpgrade when the config file's version is newer than the
// current sq build version. This typically occurs when a user downgrades
// to an older sq version for testing or compatibility purposes.
//
// Callers should handle this error gracefully by logging a warning and
// continuing execution, rather than failing outright. This allows users
// to run older sq versions against configs created by newer versions,
// which is useful for:
//   - Testing regressions in older versions
//   - Temporary downgrades while waiting for bug fixes
//   - Development and debugging scenarios
//
// Note that prerelease builds (e.g., v0.0.0-dev) are exempt from this
// check and will not trigger this error.
//
// Because the config was written by a newer sq version, it may contain
// fields unknown to this build's config.Config struct, which a Save
// would silently drop. Store.Save guards against that data loss by
// writing a verbatim backup of the config file before its first
// overwrite: see Store.backupNewerConfig.
//
// config.version is a config schema version: it identifies the sq
// version in which the config schema last changed in a way that
// requires migration (the highest registered UpgradeFunc version),
// and is stamped only when an upgrade func runs. Earlier sq releases
// instead stamped config.version with the sq binary version on every
// release, so configs in the wild carry "inflated" versions
// (e.g. v0.48.0 with a v0.34.0-era schema). The comparisons here
// tolerate that: an inflated version triggers neither an upgrade nor
// this error, as long as it doesn't exceed the build version.
var errConfigVersionNewerThanBuild = errors.New("config: config version is newer than sq version")

// checkNeedsUpgrade checks the config file's version against the
// store's UpgradeRegistry and the current sq build version, and
// determines if the config needs to be upgraded.
//
// Returns:
//   - needsUpgrade: true if config version < the highest version in
//     the UpgradeRegistry, i.e. at least one registered upgrade func
//     is outstanding.
//   - foundVers: the semver version found in the config file.
//   - err: non-nil on version parsing errors, or
//     errConfigVersionNewerThanBuild if config version > build version
//     (for non-prerelease builds).
//
// Prerelease builds (e.g., v0.0.0-dev) are exempt from the
// newer-version check.
func (fs *Store) checkNeedsUpgrade(ctx context.Context) (needsUpgrade bool, foundVers string, err error) {
	foundVers, err = LoadVersionFromFile(fs.Path)
	if err != nil {
		return false, "", err
	}

	lg.FromContext(ctx).Debug("Found config version in file",
		lga.Version, foundVers, lga.Path, fs.Path)

	if semver.Compare(foundVers, MinConfigVersion) < 0 {
		return false, foundVers, errz.Errorf("version %q is less than minimum value %q",
			foundVers, MinConfigVersion)
	}

	schemaVers := fs.UpgradeRegistry.highestVersion()
	needsUpgrade = schemaVers != "" && semver.Compare(foundVers, schemaVers) < 0

	buildVers := buildinfo.Version
	if semver.Compare(foundVers, buildVers) > 0 && semver.Prerelease(buildVers) == "" {
		// The config was written by a newer sq version; the caller
		// handles this gracefully. See errConfigVersionNewerThanBuild.
		return needsUpgrade, foundVers, errConfigVersionNewerThanBuild
	}

	return needsUpgrade, foundVers, nil
}
