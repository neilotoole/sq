package config

import (
	"os"

	"github.com/neilotoole/sq/libsq/core/errz"
	"gopkg.in/yaml.v3"
)

// execUpgradeExample_v0_15_0 is an example of how to do a config file upgrade.
func execUpgradeExample_v0_15_0(fs *YAMLFileStore) error { //nolint:unused,revive,stylecheck
	data, err := os.ReadFile(fs.Path)
	if err != nil {
		return errz.Wrap(err, "failed to read config file")
	}

	// Do something with data
	var m map[string]any
	if err = yaml.Unmarshal(data, &m); err != nil {
		return errz.Wrapf(err, "failed to unmarshal config file: %s", fs.Path)
	}

	// Example: Let's say we want to set the default timeout
	// to 10m for some reason.
	var defaults map[any]any

	// Note that we need to be defensive about the existence of fields in m.
	defaults, ok := m["defaults"].(map[any]any)
	if !ok || defaults == nil {
		// "defaults" doesn't exist, so we'll create it.
		defaults = map[any]any{}
		m["defaults"] = defaults
	}

	// Make the change
	defaults["timeout"] = "10m"

	// Always upgrade the version
	m["version"] = "v0.15.0"

	// Marshal m back into []byte
	data, err = yaml.Marshal(m)
	if err != nil {
		return errz.Wrapf(err, "failed to upgrade %s to v0.15.0", fs.Path)
	}

	// Then write results
	if err = fs.doSave(data); err != nil {
		return errz.Wrapf(err, "failed to save config file: %s", fs.Path)
	}

	// fs.log.Debugf("SUCCESS: config upgraded to v0.12.0")
	return nil
}

// execUpgrade_v0_34_0 does the following:
// - "defaults" is renamed to "options".
//
// FIXME: WIP.
func execUpgrade_v0_34_0(fs *YAMLFileStore) error { //nolint:revive,stylecheck
	data, err := os.ReadFile(fs.Path)
	if err != nil {
		return errz.Wrap(err, "failed to read config file")
	}

	// Do something with data
	var m map[string]any
	if err = yaml.Unmarshal(data, &m); err != nil {
		return errz.Wrapf(err, "failed to unmarshal config file: %s", fs.Path)
	}

	// Marshal m back into []byte
	data, err = yaml.Marshal(m)
	if err != nil {
		return errz.Wrapf(err, "failed to upgrade %s to v0.34.0", fs.Path)
	}

	// Then write results
	if err = fs.doSave(data); err != nil {
		return errz.Wrapf(err, "failed to save config file: %s", fs.Path)
	}

	// fs.log.Debugf("SUCCESS: config upgraded to v0.34.0")
	return nil
}
