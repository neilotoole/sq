package config

import (
	"os"

	"github.com/neilotoole/sq/libsq/core/ioz"

	"github.com/neilotoole/sq/libsq/core/errz"
	"gopkg.in/yaml.v3"
)

// execUpgrade_v0_34_0 does the following:
// - "version" is renamed to "config_version".
// - "active" is renamed to "active_source".
// - "defaults" is renamed to "options".
// - "sources.items[].type" is renamed to "sources.items[].driver".
// - "sources.items" is renamed to "sources.sources".
// - "sources" is renamed to "collection".
// - "config_version" is set to "v0.34.0".
//
// FIXME: WIP.
func execUpgrade_v0_34_0(fs *YAMLFileStore) error { //nolint:revive,stylecheck
	const vers = "v0.34.0"
	data, err := os.ReadFile(fs.Path)
	if err != nil {
		return errz.Wrap(err, "failed to read config file")
	}

	// Load data
	var m map[string]any
	if err = yaml.Unmarshal(data, &m); err != nil {
		return errz.Wrapf(err, "failed to unmarshal config file: %s", fs.Path)
	}

	// Do your actions
	m["options"] = m["defaults"]
	delete(m, "defaults")

	m["config_version"] = m["version"]
	delete(m, "version")

	sources, ok := m["sources"].(map[string]any)
	if !ok {
		return errz.Errorf("corrupt config: invalid 'sources' field")
	}

	sources["active_source"] = sources["active"]
	delete(sources, "active")

	items, ok := sources["items"].([]any)
	if !ok {
		return errz.Errorf("corrupt config: invalid 'sources.items' field")
	}

	for i := 0; i < len(items); i++ {
		src, ok := items[i].(map[string]any)
		if !ok {
			return errz.Errorf("corrupt config: invalid 'sources.items[%d]' field", i)
		}

		typ, ok := src["type"].(string)
		if !ok {
			return errz.Errorf("corrupt config: invalid 'sources.items[%d].type' field", i)
		}

		src["driver"] = typ
		delete(src, "type")
	}

	// Rename sources.items to sources.sources.
	sources["sources"] = items
	delete(sources, "items")

	// Rename sources to collection.
	m["collection"] = sources
	delete(m, "sources")

	m["config_version"] = vers

	// Marshal m back into []byte
	data, err = ioz.MarshalYAML(m)
	if err != nil {
		return errz.Wrapf(err, "failed to upgrade %s to %s", fs.Path, vers)
	}

	// Then write results
	if err = fs.doSave(data); err != nil {
		return errz.Wrapf(err, "failed to save config file: %s", fs.Path)
	}

	// fs.log.Debugf("SUCCESS: config upgraded to v0.34.0")
	return nil
}
