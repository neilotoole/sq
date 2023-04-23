package yamlstore

import (
	"context"
	"os"

	"github.com/neilotoole/sq/libsq/core/ioz"
	"github.com/neilotoole/sq/libsq/core/lg"
	"github.com/neilotoole/sq/libsq/core/lg/lga"

	"github.com/neilotoole/sq/libsq/core/errz"
)

// ExecUpgrade_v0_34_0 does the following:
// - "version" is renamed to "config_version".
// - "active" is renamed to "active_source".
// - "defaults" is renamed to "options".
// - "sources.items[].type" is renamed to "sources.items[].driver".
// - "sources.items" is renamed to "sources.sources".
// - "sources" is renamed to "collection".
// - "config_version" is set to "v0.34.0".
//
// FIXME: WIP.
func ExecUpgrade_v0_34_0(ctx context.Context, fs *YAMLFileStore) error { //nolint:revive,stylecheck
	const vers = "v0.34.0"
	log := lg.FromContext(ctx)
	log.Info("Starting config upgrade", lga.To, vers)

	data, err := os.ReadFile(fs.Path)
	if err != nil {
		return errz.Wrap(err, "failed to read config file")
	}

	// Load data
	var m map[string]any
	if err = ioz.UnmarshallYAML(data, &m); err != nil {
		return errz.Wrapf(err, "failed to unmarshal config file: %s", fs.Path)
	}

	// Do your actions

	m["config_version"] = m["version"]
	delete(m, "version")

	m["options"] = m["defaults"]
	delete(m, "defaults")

	opts, ok := m["options"].(map[string]any)
	if !ok {
		return errz.Errorf("corrupt config: invalid 'options' field")
	}

	opts["format"] = opts["output_format"]
	delete(opts, "output_format")
	opts["format.header"] = opts["output_header"]
	delete(opts, "output_header")
	opts["ping.timeout"] = opts["ping_timeout"]
	delete(opts, "ping_timeout")
	opts["shell-completion.timeout"] = opts["shell_completion_timeout"]
	delete(opts, "shell_completion_timeout")

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

		srcDriver := typ
		src["driver"] = srcDriver
		delete(src, "type")

		srcOpts, ok := src["options"].(map[string]any)
		if ok && srcOpts != nil {
			if headers, ok := srcOpts["header"].([]interface{}); ok && len(headers) >= 1 {
				if b, ok := headers[0].(bool); ok {
					switch srcDriver {
					case "csv", "tsv":
						srcOpts["driver.csv.header"] = b
					case "xlsx":
						srcOpts["driver.xlsx.header"] = b
					}
				}
			}
			delete(srcOpts, "header")
		}
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
	if err = fs.Write(data); err != nil {
		return errz.Wrapf(err, "failed to save config file: %s", fs.Path)
	}

	// fs.log.Debugf("SUCCESS: config upgraded to v0.34.0")
	return nil
}
