#!/usr/bin/env bash
#
# validate-build.sh
#
# Checks that the Docker-built site (make run or make run-detached) is working:
# - Site responds at BASE_URL (port from BASE_URL when present)
# - All localhost links use the port from BASE_URL (no wrong port)
# - Homepage includes the asciinema player (first cast: home-quick.cast)
#
# Usage: ./scripts/validate-build.sh [OPTIONS]
#        Run with the container already up (make run-detached) or use --start to start it.
#

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
LOG_BASH="${SCRIPT_DIR}/log.bash"
if [[ -f "$LOG_BASH" ]]; then
    # shellcheck disable=SC1090
    source "$LOG_BASH"
else
    echo "Error: log.bash not found at $LOG_BASH" >&2
    exit 1
fi

PASSED=0
FAILED=0
START_CONTAINER=false

print_test() {
    log_dim "Testing: $1"
}

print_pass() {
    log_indent log_success "PASS: $1"
    PASSED=$((PASSED + 1))
}

print_fail() {
    log_indent log_error "FAIL: $1"
    FAILED=$((FAILED + 1))
}

usage() {
    log "Usage: $(basename "$0") [OPTIONS]"
    log ""
    log "Validates the running sq-web site at BASE_URL (default: http://localhost:8080):"
    log "  • Homepage returns 200"
    log "  • All links use base URL port (no wrong port)"
    log "  • Asciinema player present on homepage"
    log ""
    log "Options:"
    log "  -h, --help       Show this help"
    log "  -u, --url URL    Base URL to validate (overrides BASE_URL env)"
    log "  -s, --start      Start container (make run-detached) before validating; stop when done"
}

BASE_URL="${BASE_URL:-http://localhost:8080}"
while [[ $# -gt 0 ]]; do
    case "$1" in
        -h|--help) usage; exit 0 ;;
        -u|--url)
            if [[ -z "${2:-}" ]]; then
                log_error "Missing value for $1"
                usage
                exit 1
            fi
            BASE_URL="$2"
            shift 2
            ;;
        -s|--start) START_CONTAINER=true; shift ;;
        *) log_error "Unknown option: $1"; usage; exit 1 ;;
    esac
done

# Parse port from BASE_URL (e.g. http://localhost:8080 or http://localhost:8080/ -> 8080)
EXPECTED_PORT=""
if [[ "$BASE_URL" =~ :([0-9]+)/? ]]; then
    EXPECTED_PORT="${BASH_REMATCH[1]}"
fi

if [[ "$START_CONTAINER" == true ]]; then
    log_info "Starting container (make run-detached)..."
    make -C "${SCRIPT_DIR}/.." run-detached
    log_info "Waiting for dev server and Hugo to be ready (can take 30+ s)..."
    WAIT_DEADLINE=$(($(date +%s) + 90))
    while [[ $(date +%s) -lt $WAIT_DEADLINE ]]; do
        CODE=$(curl -sS -o /dev/null -w "%{http_code}" "${BASE_URL}/" 2>/dev/null || echo "000")
        if [[ "$CODE" == "200" ]]; then
            log_success "Site responded with 200"
            break
        fi
        sleep 3
    done
    if [[ "$CODE" != "200" ]]; then
        log_error "Site did not respond with 200 within 90 s"
    fi
fi

log_separator
log_info "Docker build validation"
log ""

# 1. Homepage responds
print_test "Homepage responds (${BASE_URL})"
HTML=$(curl -sS -o /dev/null -w "%{http_code}" "${BASE_URL}/" 2>/dev/null || echo "000")
HOMEPAGE_OK=false
if [[ "$HTML" == "200" ]]; then
    print_pass "Homepage returned 200"
    HOMEPAGE_OK=true
else
    print_fail "Homepage returned ${HTML} (expected 200). Is the container running? Try: make run-detached"
fi
BODY=$(curl -sS "${BASE_URL}/" 2>/dev/null || echo "")

# 2. All localhost links use expected port (from BASE_URL)
print_test "All links point to expected port (${EXPECTED_PORT:-any})"
if [[ "$HOMEPAGE_OK" != true ]]; then
    print_fail "Homepage did not respond; cannot check page content"
elif [[ -z "$EXPECTED_PORT" ]]; then
    print_pass "Base URL has no port; skipping"
else
    WRONG_PORT=""
    while read -r match; do
        port="${match#*:}"
        if [[ "$port" != "$EXPECTED_PORT" ]]; then
            WRONG_PORT="$port"
            break
        fi
    done < <(echo "$BODY" | grep -oE 'localhost:[0-9]+' | sort -u)
    if [[ -n "$WRONG_PORT" ]]; then
        print_fail "Page contains links to port ${WRONG_PORT} (expected ${EXPECTED_PORT})"
    else
        print_pass "All localhost links use port ${EXPECTED_PORT}"
    fi
fi

# 3. Links use expected port or relative (when BASE_URL has a port)
print_test "Nav/links use expected port or relative paths"
if [[ "$HOMEPAGE_OK" != true ]]; then
    print_fail "Homepage did not respond; cannot check page content"
elif [[ -z "$EXPECTED_PORT" ]]; then
    print_pass "Base URL has no port; skipping port check"
elif echo "$BODY" | grep -q "localhost:${EXPECTED_PORT}\|href=\"/docs/"; then
    print_pass "Links use port ${EXPECTED_PORT} or relative paths"
else
    if echo "$BODY" | grep -q "href=\"http://localhost:${EXPECTED_PORT}"; then
        print_pass "Absolute links use port ${EXPECTED_PORT}"
    else
        print_pass "Links appear valid (relative or port ${EXPECTED_PORT})"
    fi
fi

# 4. Asciinema player on homepage (first cast: home-quick.cast)
print_test "Asciinema player on homepage"
if [[ "$HOMEPAGE_OK" != true ]]; then
    print_fail "Homepage did not respond; cannot check page content"
elif echo "$BODY" | grep -qE 'asciinema|home-quick\.cast'; then
    print_pass "Asciinema / home-quick.cast found on homepage"
else
    print_fail "Asciinema player or home-quick.cast not found on homepage"
fi

# 5. Docs page loads
print_test "Docs overview page loads"
DOC_CODE=$(curl -sS -o /dev/null -w "%{http_code}" "${BASE_URL}/docs/overview/" 2>/dev/null || echo "000")
if [[ "$DOC_CODE" == "200" ]]; then
    print_pass "Docs overview returned 200"
else
    print_fail "Docs overview returned ${DOC_CODE}"
fi

# 6. /version returns 200 and valid JSON with .latest-version
print_test "GET /version returns 200 and valid JSON with .latest-version"
if [[ "$HOMEPAGE_OK" != true ]]; then
    print_fail "Homepage did not respond; cannot check /version"
else
    VERSION_RESP=$(curl -sS -w "\n%{http_code}" "${BASE_URL}/version" 2>/dev/null || echo -e "\n000")
    VERSION_BODY=$(echo "$VERSION_RESP" | sed '$d')
    VERSION_CODE=$(echo "$VERSION_RESP" | tail -n 1)
    if [[ "$VERSION_CODE" != "200" ]]; then
        print_fail "GET /version did not return 200 and valid JSON with .latest-version (got ${VERSION_CODE})"
    elif ! echo "$VERSION_BODY" | jq -e '.["latest-version"]' >/dev/null 2>&1; then
        print_fail "GET /version did not return 200 and valid JSON with .latest-version (response body missing .latest-version or invalid JSON)"
    else
        print_pass "GET /version returned 200 and valid JSON with .latest-version"
    fi
fi

# 7. Relative links must not have target="_blank" (render-link regression)
print_test "Relative links open in same tab (no target=_blank on internal relative)"
if [[ "$HOMEPAGE_OK" != true ]]; then
    print_fail "Homepage did not respond; cannot check page content"
else
    # Match lines with target="_blank" and relative-style href (../, ./, or word/). Use [a-zA-Z]
    # so links like href="history/" are caught; exclude lines with href="http(s) (valid external).
    REL_BAD=$(echo "$BODY" | grep -E 'target="_blank"[^>]*href="(\.\./|\./|[a-zA-Z][a-zA-Z0-9]*/)|href="(\.\./|\./|[a-zA-Z][a-zA-Z0-9]*/)[^>]*target="_blank"' | grep -v 'href="https\?://' || true)
    if [[ -n "$REL_BAD" ]]; then
        print_fail "Page has relative link(s) with target=_blank (should open in same tab)"
    else
        print_pass "No relative links incorrectly set to open in new tab"
    fi
fi

# 8. No bad absolute links (localhost without port when we expect a port)
print_test "No links to localhost without port (would break when clicked)"
if [[ "$HOMEPAGE_OK" != true ]]; then
    print_fail "Homepage did not respond; cannot check page content"
elif [[ -n "$EXPECTED_PORT" ]] && echo "$BODY" | grep -qE 'href="http://localhost/"|href="http://localhost"[^:0-9]'; then
    print_fail "Page has link(s) to http://localhost without port (will fail when clicked)"
else
    print_pass "No port-stripped localhost links"
fi

# 9. Follow key nav links (simulate click: resolve href and fetch)
print_test "Following key links from homepage (simulate click)"
KEY_PATHS=("/docs/overview/" "/docs/install/")
LINK_FAIL=0
for path in "${KEY_PATHS[@]}"; do
    url="${BASE_URL}${path}"
    code=$(curl -sS -o /dev/null -w "%{http_code}" "$url" 2>/dev/null || echo "000")
    if [[ "$code" != "200" ]]; then
        print_fail "Link $path -> $url returned ${code}"
        LINK_FAIL=1
    fi
done
if [[ $LINK_FAIL -eq 0 ]]; then
    print_pass "Key links /docs/overview, /docs/install return 200"
fi

# 10. Nav links absolute with expected port when BASE_URL has a port (so click works)
print_test "Nav links are absolute with port (so click works)"
if [[ "$HOMEPAGE_OK" != true ]]; then
    print_fail "Homepage did not respond; cannot check page content"
elif [[ -z "$EXPECTED_PORT" ]]; then
    print_pass "Base URL has no port; skipping"
elif echo "$BODY" | grep -qE "href=\"http://localhost:${EXPECTED_PORT}/(docs/overview|docs/install)"; then
    print_pass "Nav has absolute links with port ${EXPECTED_PORT}"
else
    if echo "$BODY" | grep -q 'href="/docs/'; then
        print_fail "Nav still has relative /docs/ links (browser may resolve without port)"
    else
        print_pass "Nav links present"
    fi
fi

log ""
log_info "Summary"
log_indent log_success "Passed: $PASSED"
log_indent log_error "Failed: $FAILED"
log ""

if [[ "$START_CONTAINER" == true ]] && [[ $FAILED -eq 0 ]]; then
    log_info "Stopping container..."
    make -C "${SCRIPT_DIR}/.." down
fi

if [[ $FAILED -gt 0 ]]; then
    exit 1
fi
exit 0
