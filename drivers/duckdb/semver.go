package duckdb

import (
	"context"
	"regexp"
	"strings"

	"golang.org/x/mod/semver"

	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/sqlz"
)

// semverRx matches a leading dotted-numeric version token (up to three parts).
// The optional leading "v" covers DuckDB's v-prefixed version() output.
var semverRx = regexp.MustCompile(`^v?(\d+(?:\.\d+){0,2})`)

// parseSemver normalizes a DuckDB version() string to canonical semver (e.g.
// "v1.5.2"). DuckDB's version() is already v-prefixed.
func parseSemver(raw string) (string, error) {
	m := semverRx.FindStringSubmatch(strings.TrimSpace(raw))
	if m == nil {
		return "", errz.Errorf("no semver in duckdb version string: %q", raw)
	}
	v := semver.Canonical("v" + m[1])
	if !semver.IsValid(v) {
		return "", errz.Errorf("invalid duckdb semver %q from %q", v, raw)
	}
	return v, nil
}

// DBSemver implements driver.SQLDriver.
func (d *driveri) DBSemver(ctx context.Context, db sqlz.DB) (string, error) {
	var raw string
	if err := db.QueryRowContext(ctx, stmtVersion).Scan(&raw); err != nil {
		return "", errw(err)
	}
	return parseSemver(raw)
}
