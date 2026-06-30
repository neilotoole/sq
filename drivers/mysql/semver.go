package mysql

import (
	"context"
	"regexp"
	"strings"

	"golang.org/x/mod/semver"

	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/sqlz"
)

// semverRx matches a leading dotted-numeric version token (up to three parts),
// e.g. the "8.0.36" in MySQL's "8.0.36-0ubuntu0.22.04.1".
var semverRx = regexp.MustCompile(`^v?(\d+(?:\.\d+){0,2})`)

// parseSemver normalizes a MySQL or MariaDB @@version string to a canonical
// semver value (e.g. "v8.0.36"), comparable via golang.org/x/mod/semver.
//
// Vanilla MySQL: "8.0.36-0ubuntu0.22.04.1" -> "v8.0.36".
// MariaDB:       "5.5.5-10.6.4-MariaDB"    -> "v10.6.4". The leading "5.5.5-" is
// a replication-protocol sentinel, not the real version, so it is stripped
// first. Modern MariaDB ("10.11.2-MariaDB-...") has no sentinel and parses
// directly.
func parseSemver(raw string) (string, error) {
	s := strings.TrimPrefix(strings.TrimSpace(raw), "5.5.5-")
	m := semverRx.FindStringSubmatch(s)
	if m == nil {
		return "", errz.Errorf("no semver in mysql version string: %q", raw)
	}
	v := semver.Canonical("v" + m[1])
	if !semver.IsValid(v) {
		return "", errz.Errorf("invalid mysql semver %q from %q", v, raw)
	}
	return v, nil
}

// DBSemver implements driver.SQLDriver.
func (d *driveri) DBSemver(ctx context.Context, db sqlz.DB) (string, error) {
	var raw string
	if err := db.QueryRowContext(ctx, "SELECT @@GLOBAL.version").Scan(&raw); err != nil {
		return "", errw(err)
	}
	return parseSemver(raw)
}
