package oracle

import (
	"context"
	"regexp"
	"strings"

	"golang.org/x/mod/semver"

	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/sqlz"
)

// semverRx matches a leading dotted-numeric version token (up to three parts).
var semverRx = regexp.MustCompile(`^v?(\d+(?:\.\d+){0,2})`)

// parseSemver normalizes an Oracle version string to canonical semver
// (e.g. "v23.26.1"). Oracle versions are five-part; the regex caps at three.
func parseSemver(raw string) (string, error) {
	m := semverRx.FindStringSubmatch(strings.TrimSpace(raw))
	if m == nil {
		return "", errz.Errorf("no semver in oracle version string: %q", raw)
	}
	v := semver.Canonical("v" + m[1])
	if !semver.IsValid(v) {
		return "", errz.Errorf("invalid oracle semver %q from %q", v, raw)
	}
	return v, nil
}

// DBSemver implements driver.SQLDriver. It mirrors getSourceMetadata's version
// preference: product_component_version.version_full (readable by every user),
// falling back to v$instance.version (visible only to DBAs).
func (d *driveri) DBSemver(ctx context.Context, db sqlz.DB) (string, error) {
	const qFull = "SELECT version_full FROM product_component_version WHERE ROWNUM = 1"
	const qInst = "SELECT version FROM v$instance WHERE ROWNUM = 1"

	var raw string
	if err := db.QueryRowContext(ctx, qFull).Scan(&raw); err != nil || raw == "" {
		if err := db.QueryRowContext(ctx, qInst).Scan(&raw); err != nil {
			return "", errw(err)
		}
	}
	return parseSemver(raw)
}
