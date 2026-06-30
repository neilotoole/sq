package postgres

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

// parseSemver normalizes a Postgres server_version string to canonical semver
// (e.g. "v16.1.0"). Since PG 10 the version is two-part (major.minor); the
// leading token also handles any trailing distro parenthetical.
func parseSemver(raw string) (string, error) {
	m := semverRx.FindStringSubmatch(strings.TrimSpace(raw))
	if m == nil {
		return "", errz.Errorf("no semver in postgres version string: %q", raw)
	}
	v := semver.Canonical("v" + m[1])
	if !semver.IsValid(v) {
		return "", errz.Errorf("invalid postgres semver %q from %q", v, raw)
	}
	return v, nil
}

// DBSemver implements driver.SQLDriver.
func (d *driveri) DBSemver(ctx context.Context, db sqlz.DB) (string, error) {
	var raw string
	if err := db.QueryRowContext(ctx, "SELECT current_setting('server_version')").Scan(&raw); err != nil {
		return "", errw(err)
	}
	return parseSemver(raw)
}
