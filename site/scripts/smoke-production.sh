#!/usr/bin/env bash
# Post-deploy smoke checks against production sq.io.
#
# Usage: smoke-production.sh [BASE_URL]
# Default BASE_URL: https://sq.io
set -euo pipefail

BASE_URL="${1:-https://sq.io}"
BASE_URL="${BASE_URL%/}"

fail() {
	echo "smoke-production: FAIL: $1" >&2
	exit 1
}

pass() {
	echo "smoke-production: PASS: $1"
}

HTML=$(curl -fsS "${BASE_URL}/") || fail "homepage fetch failed"

if echo "${HTML}" | grep -q 'gh-stars\.min'; then
	fail "homepage still references gh-stars.min.js"
fi
pass "no legacy gh-stars.min.js"

if echo "${HTML}" | grep -q 'sq-version\.min'; then
	fail "homepage still references sq-version.min.js"
fi
pass "no legacy sq-version.min.js"

if ! echo "${HTML}" | grep -qE 'navbar-version[^>]*>v[0-9]+\.[0-9]+\.[0-9]+'; then
	fail "baked version badge (navbar-version > vX.Y.Z) not found"
fi
pass "baked version badge present"

if ! echo "${HTML}" | grep -qE 'gh-stars-count[^>]*>[0-9][0-9.]*k?'; then
	fail "baked star count (gh-stars-count) not found"
fi
pass "baked star count present"

VERSION_JSON=$(curl -fsS "${BASE_URL}/version") || fail "/version fetch failed"
if ! echo "${VERSION_JSON}" | jq -e '.["latest-version"] | test("^[0-9]+\\.[0-9]+\\.[0-9]+$")' >/dev/null; then
	fail "/version JSON missing valid latest-version"
fi
pass "/version JSON valid"

CSP=$(curl -fsSI "${BASE_URL}/" | tr -d '\r' | grep -i '^content-security-policy:' || true)
if [[ -z "${CSP}" ]]; then
	fail "Content-Security-Policy header missing"
fi
if ! echo "${CSP}" | grep -qi 'cdn\.vemetric\.com'; then
	fail "CSP missing cdn.vemetric.com in script-src"
fi
if ! echo "${CSP}" | grep -qi 'hub\.vemetric\.com'; then
	fail "CSP missing hub.vemetric.com in connect-src"
fi
pass "CSP includes Vemetric domains"

CACHE=$(curl -fsSI "${BASE_URL}/" | tr -d '\r' | grep -i '^cache-control:' || true)
if [[ -z "${CACHE}" ]]; then
	fail "Cache-Control header missing on /"
fi
if ! echo "${CACHE}" | grep -qiE 'max-age=0|must-revalidate'; then
	fail "homepage Cache-Control should revalidate (got: ${CACHE})"
fi
pass "homepage Cache-Control allows revalidation"

echo "smoke-production: all checks passed (${BASE_URL})"
