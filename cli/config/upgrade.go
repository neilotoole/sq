package config

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/neilotoole/sq/libsq/core/ioz"

	"github.com/neilotoole/sq/libsq/core/lg"
	"github.com/neilotoole/sq/libsq/core/lg/lga"

	"github.com/neilotoole/sq/cli/buildinfo"
	"github.com/neilotoole/sq/libsq/core/errz"
	"golang.org/x/mod/semver"
)

// minConfigVersion is the minimum semver value of Config.Version.
// This is basically how far back in time the config upgrade process
// can support. If the config dates from prior to this (unlikely),
// then the user needs to start with a new config.
const minConfigVersion = "v0.0.0-dev"

// upgradeFunc performs a (single) upgrade of the config file. Typically
// a func will read the config data from disk, perform some transformation
// on it, and write it back out to disk. Note that the func should not
// bind the config file YAML to the Config object, as they may differ
// significantly. Instead, the func should bind the YAML to a map, and
// manipulate that map directly.
type upgradeFunc func(ctx context.Context, fs *YAMLFileStore) error

// upgradeRegistry is a map of config_version to upgrade funcs.
type upgradeRegistry map[string]upgradeFunc

// defaultUpgradeReg is the default upgrade registry.
var defaultUpgradeReg = upgradeRegistry{}

func init() { //nolint:gochecknoinits
	if !buildinfo.IsDefaultVersion() && semver.Compare(buildinfo.Version, minConfigVersion) < 0 {
		panic(fmt.Sprintf("buildinfo.Version %q is less than minimum config version %q",
			buildinfo.Version, minConfigVersion))
	}

	// For each upgrade, register the upgrade func as below:
	//  defaultUpgradeReg["v0.34.0"] = ExecUpgrade_v0_34_0
	//
	// IMPORTANT: the upgrade function should not use any defined struct
	// types, as these may change between versions. Instead, the upgrade
	// function should directly manipulate the yaml/map.

	defaultUpgradeReg["v0.34.0"] = ExecUpgrade_v0_34_0
}

// UpgradeConfig runs all the registered upgrade funcs between cfg.Version
// and targetVersion. Typically this is checked by Load, but can be
// explicitly invoked for testing etc.
func (fs *YAMLFileStore) UpgradeConfig(ctx context.Context, startVersion, targetVersion string) (*Config, error) {
	if !semver.IsValid(targetVersion) {
		return nil, errz.Errorf("invalid semver for config version {%s}", targetVersion)
	}

	log := lg.FromContext(ctx)

	log.Debug("Starting config upgrade", lga.From, startVersion, lga.To, targetVersion)
	var err error
	upgradeFns := fs.upgradeReg.getUpgradeFuncs(startVersion, targetVersion)

	for _, fn := range upgradeFns {
		err = fn(ctx, fs)
		if err != nil {
			return nil, err
		}
	}

	// Do a final update of the version
	cfg, err := fs.doLoad(ctx)
	if err != nil {
		return nil, err
	}
	cfg.Version = targetVersion

	log.Debug("Setting config_version", lga.Val, targetVersion)

	err = fs.Save(ctx, cfg)
	if err != nil {
		return nil, err
	}

	return cfg, nil
}

// getUpgradeFuncs returns the funcs required to upgrade from startingVersion
// to targetVersion. We iterate over the set of registered funcs; if the
// version (the key) is greater than startingVersion, and less than or equal
// to targetVersion, that upgradeFunc will be included in the return value.
func (r upgradeRegistry) getUpgradeFuncs(startingVersion, targetVersion string) []upgradeFunc {
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

	upgradeFns := make([]upgradeFunc, len(vers))
	for i, v := range vers {
		upgradeFns[i] = r[v]
	}

	return upgradeFns
}

// loadVersion loads the version from the config file.
// If the field is not present, minConfigVersion (and no error) is returned.
func loadVersion(path string) (string, error) {
	bytes, err := os.ReadFile(path)
	if err != nil {
		return "", errz.Wrapf(err, "failed to load file: %s", path)
	}

	m := map[string]any{}
	err = ioz.UnmarshallYAML(bytes, &m)
	if err != nil {
		return "", errz.Wrap(err, "failed to unmarshal config YAML")
	}

	if v, ok := m["version"]; ok {
		// Legacy "version" field.
		s, ok := v.(string)
		if !ok {
			return "", errz.Errorf("invalid value for 'version' field: %s", v)
		}

		s = strings.TrimSpace(s)
		if s == "" {
			// We could return an error here, but it's probably slightly
			// better to carry on in the absence of the version.
			return minConfigVersion, nil
		}

		if !semver.IsValid(s) {
			return "", errz.Errorf("invalid semver value for 'version' field: %s", s)
		}

		return s, nil
	}

	if v, ok := m["config_version"]; ok {
		s, ok := v.(string)
		if !ok {
			return "", errz.Errorf("invalid value for 'config_version' field: %s", v)
		}

		s = strings.TrimSpace(s)
		if s == "" {
			// We could return an error here, but it's probably slightly
			// better to carry on in the absence of the version.
			return minConfigVersion, nil
		}

		if !semver.IsValid(s) {
			return "", errz.Errorf("invalid semver value for 'config_version' field: %s", s)
		}

		return s, nil
	}

	return minConfigVersion, nil
}

// checkNeedsUpgrade checks on the config version, returning needsUpgrade
// if applicable. The returned foundVers is a valid semver.
func checkNeedsUpgrade(path string) (needsUpgrade bool, foundVers string, err error) {
	foundVers, err = loadVersion(path)
	if err != nil {
		return false, "", err
	}

	if semver.Compare(foundVers, minConfigVersion) < 0 {
		return false, foundVers, errz.Errorf("version %q is less than minimum value %q",
			foundVers, minConfigVersion)
	}

	buildVers := buildinfo.Version

	switch semver.Compare(foundVers, buildVers) {
	case 0:
		// Versions are the same, nothing to do here
		return false, foundVers, nil
	case 1:
		// sq version is less than config version:
		// - user needs to upgrade sq
		// - but we make an exception if sq is prerelease
		if semver.Prerelease(buildVers) == "" {
			return false, foundVers, errz.Errorf("config: version %q is newer than sq version %q: upgrade sq to a newer version",
				foundVers, buildVers)
		}
		return false, foundVers, nil

	default:
		// config version is less than sq version; we need to upgrade config.
		return true, foundVers, nil
	}
}
