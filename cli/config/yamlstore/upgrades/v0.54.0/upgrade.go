// Package v0_54_0 upgrades YAML config to v0.54.0.
package v0_54_0 //nolint:revive

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/ioz"
	"github.com/neilotoole/sq/libsq/core/lg"
	"github.com/neilotoole/sq/libsq/core/lg/lga"
	"github.com/neilotoole/sq/libsq/core/secret"
)

// Version is the target config version this upgrade produces.
const Version = "v0.54.0"

// Upgrade renames the "redact" option to "secrets.reveal" and flips
// polarity: secrets.reveal = !redact. The translation is applied at
// the top-level options map and to each source's per-source options.
// It also escapes '$' as '$$' in source locations that the new
// placeholder-template machinery would otherwise reinterpret: see
// escapeLocation.
//
// Redact cases:
//   - redact: true (the historical default): drop the key. The new
//     option's default is secrets.reveal: false, which means the same
//     thing (redaction on). Leaving the key absent keeps the config
//     minimal.
//   - redact: false: write secrets.reveal: true.
//   - redact set to a non-bool value (corrupt): drop the key and warn.
//     The new option falls back to its default (redaction on).
//   - Already-migrated configs (no "redact" key): no-op for that map.
//
// A corrupt shape for the top-level "options" or "collection" entries
// (present but not a map) returns an error rather than silently
// skipping, matching the precedent set by the v0.34.0 upgrade.
//
// The "config.version" field is set to v0.54.0 on success.
func Upgrade(ctx context.Context, before []byte) (after []byte, err error) {
	log := lg.FromContext(ctx)
	log.Info("Starting config upgrade step", lga.To, Version)

	var m map[string]any
	if err = ioz.UnmarshallYAML(before, &m); err != nil {
		return nil, errz.Wrap(err, "failed to unmarshal config")
	}

	if raw, present := m["options"]; present {
		opts, ok := raw.(map[string]any)
		if !ok {
			return nil, errz.Errorf("corrupt config: invalid 'options' field (want map, got %T)", raw)
		}
		translateRedact(log, opts, "options")
	}

	if raw, present := m["collection"]; present {
		collection, ok := raw.(map[string]any)
		if !ok {
			return nil, errz.Errorf("corrupt config: invalid 'collection' field (want map, got %T)", raw)
		}
		if rawSources, present := collection["sources"]; present {
			sources, ok := rawSources.([]any)
			if !ok {
				return nil, errz.Errorf("corrupt config: invalid 'collection.sources' field (want list, got %T)", rawSources)
			}
			for i, rawSrc := range sources {
				src, ok := rawSrc.(map[string]any)
				if !ok {
					return nil, errz.Errorf("corrupt config: invalid 'collection.sources[%d]' field", i)
				}
				escapeLocation(log, src, i)
				if rawSrcOpts, present := src["options"]; present {
					srcOpts, ok := rawSrcOpts.(map[string]any)
					if !ok {
						return nil, errz.Errorf(
							"corrupt config: invalid 'collection.sources[%d].options' field (want map, got %T)",
							i, rawSrcOpts)
					}
					handle, _ := src["handle"].(string)
					ctxLabel := fmt.Sprintf("collection.sources[%d] (%s)", i, handle)
					translateRedact(log, srcOpts, ctxLabel)
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

// escapeLocation escapes '$' as '$$' in-place in src's "location"
// field when the location would not survive v0.54.0
// placeholder-template interpretation byte-identically. Source
// locations predating v0.54.0 are literal strings: the placeholder
// syntax did not exist, so any ${scheme:path}-shaped text in them is
// accidental. Without escaping, a legacy location containing a
// well-formed ref would be silently substituted at connect time
// (sending wrong credentials), malformed placeholder syntax would fail
// every connect, and a literal "$$" would be unescaped to "$"
// (see https://github.com/neilotoole/sq/issues/782).
//
// Locations whose only dollars are lone '$' characters (e.g. an inline
// password like "p$ssW0rd") already expand to themselves, and are left
// untouched so the config bytes don't churn. The location value is
// deliberately not logged: it may contain credentials.
func escapeLocation(log *slog.Logger, src map[string]any, i int) {
	loc, ok := src["location"].(string)
	if !ok || !strings.Contains(loc, "$") {
		return
	}

	if !strings.Contains(loc, "$$") {
		refs, err := secret.ExtractRefs(loc)
		if err == nil && len(refs) == 0 {
			// Lone dollars only: expands to itself; nothing to escape.
			return
		}
	}

	src["location"] = secret.Escape(loc)
	handle, _ := src["handle"].(string)
	log.Info(
		"config upgrade: escaped '$' as '$$' in source location so it connects byte-identically",
		lga.Loc, fmt.Sprintf("collection.sources[%d] (%s)", i, handle),
	)
}

// translateRedact applies the redact → secrets.reveal flip in-place
// on the given options map. ctxLabel identifies the map for log
// messages (e.g. "options" or "collection.sources[2] (@prod/pg)").
// Behavior:
//   - "redact" absent: no-op.
//   - "redact" true: key dropped (matches new default secrets.reveal:
//     false).
//   - "redact" false: write "secrets.reveal": true.
//   - "redact" present but not a bool: drop the key, log a warning,
//     and fall back to the new default. This shouldn't happen for a
//     typed option, but if a hand-edited config introduces a bad
//     value, surfacing it in the log lets the user understand why
//     redaction behavior may have changed after the upgrade.
func translateRedact(log *slog.Logger, opts map[string]any, ctxLabel string) {
	raw, present := opts["redact"]
	if !present {
		return
	}
	delete(opts, "redact")

	redact, ok := raw.(bool)
	if !ok {
		log.Warn(
			"config upgrade: dropping non-bool 'redact' value; falling back to secrets.reveal default",
			lga.Key, "redact",
			lga.Loc, ctxLabel,
			lga.Val, fmt.Sprintf("%T:%v", raw, raw),
		)
		return
	}
	if !redact {
		opts["secrets.reveal"] = true
	}
}
