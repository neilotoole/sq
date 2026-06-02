// Package v0_54_0 upgrades YAML config to v0.54.0.
package v0_54_0 //nolint:revive

import (
	"context"

	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/ioz"
	"github.com/neilotoole/sq/libsq/core/lg"
	"github.com/neilotoole/sq/libsq/core/lg/lga"
)

// Version is the target config version this upgrade produces.
const Version = "v0.54.0"

// Upgrade renames the "redact" option to "secrets.reveal" and flips
// polarity: secrets.reveal = !redact. The translation is applied at
// the top-level options map and to each source's per-source options.
//
// Cases:
//   - redact: true (the historical default) → drop the key. The new
//     option's default is secrets.reveal: false, which means the same
//     thing (redaction on). Leaving the key absent keeps the config
//     minimal.
//   - redact: false → write secrets.reveal: true.
//   - Already-migrated configs (no "redact" key) → no-op.
//
// The "config.version" field is set to v0.54.0.
func Upgrade(ctx context.Context, before []byte) (after []byte, err error) {
	log := lg.FromContext(ctx)
	log.Info("Starting config upgrade step", lga.To, Version)

	var m map[string]any
	if err = ioz.UnmarshallYAML(before, &m); err != nil {
		return nil, errz.Wrap(err, "failed to unmarshal config")
	}

	if opts, ok := m["options"].(map[string]any); ok {
		translateRedact(opts)
	}

	if collection, ok := m["collection"].(map[string]any); ok {
		if sources, ok := collection["sources"].([]any); ok {
			for i, raw := range sources {
				src, ok := raw.(map[string]any)
				if !ok {
					return nil, errz.Errorf("corrupt config: invalid 'collection.sources[%d]' field", i)
				}
				if srcOpts, ok := src["options"].(map[string]any); ok {
					translateRedact(srcOpts)
				}
			}
		}
	}

	m["config.version"] = Version

	after, err = ioz.MarshalYAML(m)
	if err != nil {
		return nil, errz.Wrapf(err, "failed to upgrade config to %s", Version)
	}

	log.Info("SUCCESS: Config upgrade step completed", lga.To, Version)
	return after, nil
}

// translateRedact applies the redact → secrets.reveal flip in-place
// on the given options map. If "redact" is absent, the map is left
// alone. If "redact" is true (matching the new default), the key is
// dropped; if false, "secrets.reveal" is set to true.
func translateRedact(opts map[string]any) {
	raw, ok := opts["redact"]
	if !ok {
		return
	}
	delete(opts, "redact")

	redact, ok := raw.(bool)
	if !ok {
		// Non-bool value (shouldn't happen for a typed option, but be
		// defensive). Dropping the unrecognized value is the safest
		// move; the new option will fall back to its default.
		return
	}
	if !redact {
		opts["secrets.reveal"] = true
	}
}
