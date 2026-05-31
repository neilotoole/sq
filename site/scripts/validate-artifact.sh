#!/usr/bin/env bash
# Validate a local site/public build artifact (no HTTP server required).
#
# Usage: validate-artifact.sh [PUBLIC_DIR]
# Default PUBLIC_DIR: site/public (relative to this script's parent directory).
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SITE_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
PUBLIC_DIR="${1:-${SITE_DIR}/public}"
INDEX="${PUBLIC_DIR}/index.html"

fail() {
	echo "validate-artifact: FAIL: $1" >&2
	exit 1
}

pass() {
	echo "validate-artifact: PASS: $1"
}

[[ -d "${PUBLIC_DIR}" ]] || fail "directory not found: ${PUBLIC_DIR}"
[[ -f "${INDEX}" ]] || fail "public/index.html missing (run make site-build first)"

BODY=$(cat "${INDEX}")

if echo "${BODY}" | grep -q 'gh-stars\.min'; then
	fail "index.html references gh-stars.min.js"
fi
pass "no legacy gh-stars.min.js"

if echo "${BODY}" | grep -q 'sq-version\.min'; then
	fail "index.html references sq-version.min.js"
fi
pass "no legacy sq-version.min.js"

if echo "${BODY}" | grep -q 'api\.github\.com'; then
	fail "index.html references api.github.com (runtime fetch should be gone)"
fi
pass "no api.github.com in homepage HTML"

if ! echo "${BODY}" | grep -qE 'navbar-version[^>]*>v[0-9]+\.[0-9]+\.[0-9]+'; then
	fail "version badge (navbar-version > vX.Y.Z) not found in index.html"
fi
pass "baked version badge present"

if ! echo "${BODY}" | grep -qE 'gh-stars-count[^>]*>[0-9][0-9.]*k?'; then
	fail "star count (gh-stars-count) not found in index.html"
fi
pass "baked star count present"

[[ -f "${PUBLIC_DIR}/version.json" ]] || fail "public/version.json missing"
if ! jq -e '.["latest-version"] | test("^[0-9]+\\.[0-9]+\\.[0-9]+$")' "${PUBLIC_DIR}/version.json" >/dev/null; then
	fail "public/version.json has invalid latest-version"
fi
pass "public/version.json valid"

echo "validate-artifact: all checks passed (${PUBLIC_DIR})"
