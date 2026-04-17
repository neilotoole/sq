#!/bin/bash
#
# validate-bun-runtime.sh
#
# Quick validation script to test that all Bun scripts work correctly
# after migrating from Node.js/npm to Bun.
#
# Usage: ./validate-bun-runtime.sh
#

set -e

# Resolve script directory for reliable sourcing
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

print_test() {
    log_indent log_info_dim "Testing: $1"
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
    log "Validates the Bun runtime and project tooling for the sq.io website."
    log "Run from the repo root. Ensures Bun is installed, dependencies install"
    log "correctly, and all package.json scripts (check, clean, lint, build) succeed."
    log ""
    log "What is tested:"
    log "  • Bun installation and version"
    log "  • Project directory (sq-web package.json)"
    log "  • bun install (dependency install)"
    log "  • bun run check (Hugo version)"
    log "  • bun run clean"
    log "  • bun run lint:scripts, lint:styles, lint:markdown"
    log "  • bun run build (Hugo production build, public/ output)"
    log "  • Postbuild: public/_redirects present"
    log ""
    log "Options:"
    log "  -h, --help  Show this help message"
}

while [[ $# -gt 0 ]]; do
    case "$1" in
        -h|--help)
            usage
            exit 0
            ;;
        *)
            log_error "Unknown option: $1"
            usage
            exit 1
            ;;
    esac
done

log_separator
log_info "Bun Runtime Validation"
log ""

# Check Bun is installed
print_test "Bun installation"
if command -v bun &> /dev/null; then
    BUN_VERSION=$(bun --version)
    print_pass "Bun installed (v$BUN_VERSION)"
else
    print_fail "Bun not installed"
    log_info "Install Bun: 'curl -fsSL https://bun.sh/install | bash' or 'brew install bun'"
    exit 1
fi

# Check we're in the right directory
print_test "Project directory"
if [[ -f "package.json" ]] && command grep -q '"name": "sq.io"' package.json; then
    print_pass "In sq-web project directory"
else
    print_fail "Not in sq-web project directory"
    exit 1
fi

log ""
log "Installing dependencies"

print_test "bun install"
if bun install > /dev/null 2>&1; then
    print_pass "Dependencies installed"
else
    print_fail "bun install failed"
fi

log ""
log "Testing package.json scripts"

# Test: bun run check
print_test "bun run check (Hugo version)"
if bun run check > /dev/null 2>&1; then
    HUGO_VERSION=$(bun run --silent check 2>&1 | command grep -o 'hugo v[0-9.]*' | head -1)
    print_pass "Hugo check passed ($HUGO_VERSION)"
else
    print_fail "bun run check failed"
fi

# Test: bun run clean
print_test "bun run clean"
if bun run clean > /dev/null 2>&1; then
    print_pass "Clean completed"
else
    print_fail "bun run clean failed"
fi

# Test: bun run lint:scripts
print_test "bun run lint:scripts (ESLint)"
if bun run lint:scripts > /dev/null 2>&1; then
    print_pass "ESLint passed"
else
    print_fail "bun run lint:scripts failed"
fi

# Test: bun run lint:styles
print_test "bun run lint:styles (Stylelint)"
if bun run lint:styles > /dev/null 2>&1; then
    print_pass "Stylelint passed"
else
    print_fail "bun run lint:styles failed"
fi

# Test: bun run lint:markdown
print_test "bun run lint:markdown (markdownlint)"
if bun run lint:markdown > /dev/null 2>&1; then
    print_pass "markdownlint passed"
else
    print_fail "bun run lint:markdown failed"
fi

# Test: bun run build
print_test "bun run build (Hugo production build)"
if bun run build > /dev/null 2>&1; then
    if [[ -d "public" ]] && [[ -f "public/index.html" ]]; then
        PAGE_COUNT=$(find public -name "*.html" | wc -l | tr -d ' ')
        print_pass "Build completed ($PAGE_COUNT HTML files)"
    else
        print_fail "Build completed but public/ is missing or empty"
    fi
else
    print_fail "bun run build failed"
fi

# Verify public/_redirects exists
print_test "postbuild (_redirects appended)"
if [[ -f "public/_redirects" ]]; then
    print_pass "_redirects file exists"
else
    print_fail "public/_redirects missing"
fi

log ""
log "Summary"
log ""
log_info "Passed: $PASSED"
log_info "Failed: $FAILED"
log ""

if [[ $FAILED -eq 0 ]]; then
    log_success "All tests passed! Bun migration is working correctly."
    exit 0
else
    log_error "Some tests failed. Review the output above."
    exit 1
fi
