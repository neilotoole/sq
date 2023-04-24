// Package v0_34_0 upgrades YAML config to v0.34.0.
package v0_34_0 //nolint:stylecheck,revive,cyclop

import (
	"context"
	"strconv"

	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/ioz"
	"github.com/neilotoole/sq/libsq/core/lg"
	"github.com/neilotoole/sq/libsq/core/lg/lga"
)

const Version = "v0.34.0"

// Upgrade does the following:
// - "version" is renamed to "config.version".
// - "defaults" is renamed to "options".
// - "sources.active" is renamed to "active.source".
// - "sources.active_group" is renamed to "active.group"
// - "sources.items[].type" is renamed to "sources.items[].driver".
// - "sources.items" is renamed to "sources.sources".
// - "sources" is renamed to "collection".
// - "config.version" is set to "v0.34.0".
func Upgrade(ctx context.Context, before []byte) (after []byte, err error) {
	log := lg.FromContext(ctx)
	log.Info("Starting config upgrade step", lga.To, Version)

	// Load data
	var m map[string]any
	if err = ioz.UnmarshallYAML(before, &m); err != nil {
		return nil, errz.Wrap(err, "failed to unmarshal config")
	}

	// Do your actions

	m["config.version"] = m["version"]
	delete(m, "version")

	m["options"] = m["defaults"]
	delete(m, "defaults")

	opts, ok := m["options"].(map[string]any)
	if !ok {
		return nil, errz.Errorf("corrupt config: invalid 'options' field")
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
		return nil, errz.Errorf("corrupt config: invalid 'sources' field")
	}

	sources["active.source"] = sources["active"]
	delete(sources, "active")
	sources["active.group"] = sources["active_group"]
	delete(sources, "active_group")

	items, ok := sources["items"].([]any)
	if !ok {
		return nil, errz.Errorf("corrupt config: invalid 'sources.items' field")
	}

	for i := 0; i < len(items); i++ {
		src, ok := items[i].(map[string]any)
		if !ok {
			return nil, errz.Errorf("corrupt config: invalid 'sources.items[%d]' field", i)
		}

		typ, ok := src["type"].(string)
		if !ok {
			return nil, errz.Errorf("corrupt config: invalid 'sources.items[%d].type' field", i)
		}

		srcDriver := typ
		src["driver"] = srcDriver
		delete(src, "type")

		srcOpts, ok := src["options"].(map[string]any)
		if ok && srcOpts != nil {
			if headers, ok := srcOpts["header"].([]interface{}); ok && len(headers) >= 1 {
				var hasHeader bool
				switch v := headers[0].(type) {
				case bool:
					hasHeader = v
				case string:
					hasHeader, _ = strconv.ParseBool(v)
				default:
					return nil, errz.Errorf("corrupt config: invalid 'sources.items[%d].options.header' field", i)
				}

				switch srcDriver {
				case "csv", "tsv":
					srcOpts["driver.csv.header"] = hasHeader
				case "xlsx":
					srcOpts["driver.xlsx.header"] = hasHeader
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

	m["config.version"] = Version

	// Marshal m back into []byte
	after, err = ioz.MarshalYAML(m)
	if err != nil {
		return nil, errz.Wrapf(err, "failed to upgrade config to %s", Version)
	}

	log.Info("SUCCESS: Config upgrade step completed", lga.To, Version)

	return after, nil
}
