package yamlstore

import (
	"context"
	"os"
	"strings"

	"github.com/neilotoole/sq/cli/config"
	"github.com/neilotoole/sq/libsq/core/ioz"
	"github.com/neilotoole/sq/libsq/core/options"

	"github.com/neilotoole/sq/libsq/core/lg"
	"github.com/neilotoole/sq/libsq/core/lg/lga"

	"github.com/neilotoole/sq/cli/buildinfo"
	"github.com/neilotoole/sq/libsq/core/errz"
	"golang.org/x/mod/semver"
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

// doUpgrade runs all the registered upgrade funcs between cfg.Version
// and targetVersion. Typically this is checked by Load, but can be
// explicitly invoked for testing etc.
func (fs *Store) doUpgrade(ctx context.Context, optsReg *options.Registry,
	startVersion, targetVersion string,
) (*config.Config, error) {
	log := lg.From(ctx)
	log.Debug("Starting config upgrade", lga.From, startVersion, lga.To, targetVersion)

	if !semver.IsValid(targetVersion) {
		return nil, errz.Errorf("invalid semver for config version {%s}", targetVersion)
	}

	var err error
	upgradeFns := fs.UpgradeRegistry.getUpgradeFuncs(startVersion, targetVersion)

	data, err := os.ReadFile(fs.Path)
	if err != nil {
		return nil, err
	}

	for _, fn := range upgradeFns {
		log.Debug("Attempting config upgrade step")
		data, err = fn(ctx, data)
		if err != nil {
			return nil, err
		}
		log.Debug("Config upgrade step successful")
	}

	if err = fs.Write(data); err != nil {
		return nil, err
	}

	// Do a final update of the version
	cfg, err := fs.doLoad(ctx, optsReg)
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

// checkNeedsUpgrade checks on the config version, returning needsUpgrade
// if applicable. The returned foundVers is a valid semver.
func checkNeedsUpgrade(ctx context.Context, path string) (needsUpgrade bool, foundVers string, err error) {
	foundVers, err = LoadVersionFromFile(path)
	if err != nil {
		return false, "", err
	}

	lg.From(ctx).Debug("Found config version in file",
		lga.Version, foundVers, lga.Path, path)

	if semver.Compare(foundVers, MinConfigVersion) < 0 {
		return false, foundVers, errz.Errorf("version %q is less than minimum value %q",
			foundVers, MinConfigVersion)
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
