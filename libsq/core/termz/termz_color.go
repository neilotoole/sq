package termz

import (
	"os"
	"strings"
)

// colorEnvOverride reports an explicit color preference derived from the
// NO_COLOR, FORCE_COLOR, and TERM environment variables. The returned ok is
// false when no environment variable forces a decision, in which case the
// caller should fall back to terminal detection.
//
// Precedence follows the de-facto convention: NO_COLOR (any non-empty value)
// disables color and takes precedence over everything; FORCE_COLOR enables
// color, except when its value is "0" or "false", which disables it; TERM=dumb
// disables color.
//
// One deliberate divergence from supports-color: an empty-but-present
// FORCE_COLOR= is treated as unset (fall through to terminal detection) rather
// than "force on". Go cannot cheaply tell an empty value from an unset one, and
// an inherited empty FORCE_COLOR= is far more likely accidental than an
// intentional request to force color, so the safer reading is "no preference".
//
// [NO_COLOR]: https://no-color.org/
// [FORCE_COLOR]: https://force-color.org/
func colorEnvOverride() (enabled, ok bool) {
	if os.Getenv("NO_COLOR") != "" {
		return false, true
	}

	if v := os.Getenv("FORCE_COLOR"); v != "" {
		switch strings.ToLower(strings.TrimSpace(v)) {
		case "0", "false":
			// FORCE_COLOR=0 (or "false") explicitly disables color, per the
			// force-color.org convention. The previous implementation treated
			// any non-empty value (including "0") as "force on", which is the
			// opposite of the documented behavior.
			return false, true
		default:
			return true, true
		}
	}

	if os.Getenv("TERM") == "dumb" {
		return false, true
	}

	return false, false
}
