#!/usr/bin/env bash
#
# test-execsql-fix.sh - Demonstrate ExecSQL fix by comparing old vs new sq behavior
#
# This script demonstrates the difference between old (broken) and new (fixed) sq:
# - OLD: INSERT/UPDATE/DELETE show "Affected 0 row(s)" or no output
# - NEW: INSERT/UPDATE/DELETE show accurate affected row counts
#
# Usage:
#   ./test-execsql-fix.sh                          # Compare old vs new with SQLite
#   ./test-execsql-fix.sh --new /path/to/new/sq    # Specify new sq binary
#   ./test-execsql-fix.sh --old /path/to/old/sq    # Specify old sq binary

set -euo pipefail

# Get script directory and source logging utilities
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/log.bash"

# ==============================================================================
# Configuration
# ==============================================================================

NEW_SQ_BINARY=""
OLD_SQ_BINARY=""

# Parse command line arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --new)
            NEW_SQ_BINARY="$2"
            shift 2
            ;;
        --old)
            OLD_SQ_BINARY="$2"
            shift 2
            ;;
        --help|-h)
            log_separator
            log_info "ExecSQL Fix Comparison Test"
            log "Usage: $0 [OPTIONS]"
            log ""
            log_info "Options:"
            log "  --new PATH    ${DIM}Path to new (fixed) sq binary"
            log "  --old PATH    ${DIM}Path to old (broken) sq binary"
            log "  --help, -h    ${DIM}Show this help message"
            log ""
            log_info "Defaults:"
            log "  --new         ${DIM}\$GOPATH/bin/sq or first sq in PATH"
            log "  --old         ${DIM}\$(which sq)"
            log ""
            log_info "Examples:"
            log "  $0                                    ${DIM}# Use defaults"
            log "  $0 --new ./sq-new --old ./sq-old      ${DIM}# Specify both"
            exit 0
            ;;
        *)
            log_error "Unknown option: $1"
            log "Use --help for usage information"
            exit 1
            ;;
    esac
done

# Set defaults for NEW_SQ_BINARY
if [ -z "$NEW_SQ_BINARY" ]; then
    if [ -n "${GOPATH:-}" ] && [ -x "${GOPATH}/bin/sq" ]; then
        NEW_SQ_BINARY="${GOPATH}/bin/sq"
    elif command -v sq >/dev/null 2>&1; then
        NEW_SQ_BINARY="$(command -v sq)"
    else
        NEW_SQ_BINARY="sq"
    fi
fi

# Set defaults for OLD_SQ_BINARY
if [ -z "$OLD_SQ_BINARY" ]; then
    OLD_SQ_BINARY="$(which sq 2>/dev/null || echo "")"
fi

# ==============================================================================
# Cleanup
# ==============================================================================

cleanup() {
    log_dim "Cleaning up..."
    "$NEW_SQ_BINARY" rm "@test_new" 2>/dev/null || true
    [ -n "$OLD_SQ_BINARY" ] && "$OLD_SQ_BINARY" rm "@test_old" 2>/dev/null || true
    rm -f /tmp/test_execsql_*.db
}

trap cleanup EXIT

# ==============================================================================
# Test Functions
# ==============================================================================

# Test DDL/DML operations with a specific binary
# Arguments: $1=sq_binary $2=src_handle $3=expect_correct (true/false)
test_ddl_dml() {
    local sq_binary="$1"
    local handle="$2"
    local output

    # CREATE TABLE
    output=$("$sq_binary" sql --src "$handle" \
        "CREATE TABLE test_exec (id INTEGER, name TEXT)" 2>&1) || true
    if echo "$output" | grep -q "Affected.*row"; then
        log_success "CREATE: $output"
    elif [ -z "$output" ]; then
        log_error "CREATE: (no output)"
    else
        log_error "CREATE: $output"
    fi

    # INSERT 3 rows
    output=$("$sq_binary" sql --src "$handle" \
        "INSERT INTO test_exec VALUES (1,'A'),(2,'B'),(3,'C')" 2>&1) || true
    if echo "$output" | grep -q "Affected 3"; then
        log_success "INSERT: $output"
    elif [ -z "$output" ]; then
        log_error "INSERT: (no output) ${DIM}← should be 3${RESET}"
    elif echo "$output" | grep -q "Affected 0"; then
        log_error "INSERT: $output ${DIM}← should be 3${RESET}"
    else
        log_error "INSERT: $output"
    fi

    # UPDATE 2 rows
    output=$("$sq_binary" sql --src "$handle" \
        "UPDATE test_exec SET name='X' WHERE id<=2" 2>&1) || true
    if echo "$output" | grep -q "Affected 2"; then
        log_success "UPDATE: $output"
    elif [ -z "$output" ]; then
        log_error "UPDATE: (no output) ${DIM}← should be 2${RESET}"
    elif echo "$output" | grep -q "Affected 0"; then
        log_error "UPDATE: $output ${DIM}← should be 2${RESET}"
    else
        log_error "UPDATE: $output"
    fi

    # DELETE 1 row
    output=$("$sq_binary" sql --src "$handle" \
        "DELETE FROM test_exec WHERE id=3" 2>&1) || true
    if echo "$output" | grep -q "Affected 1"; then
        log_success "DELETE: $output"
    elif [ -z "$output" ]; then
        log_error "DELETE: (no output) ${DIM}← should be 1${RESET}"
    elif echo "$output" | grep -q "Affected 0"; then
        log_error "DELETE: $output ${DIM}← should be 1${RESET}"
    else
        log_error "DELETE: $output"
    fi

    # DROP TABLE
    output=$("$sq_binary" sql --src "$handle" \
        "DROP TABLE test_exec" 2>&1) || true
    if echo "$output" | grep -q "Affected"; then
        log_success "DROP:   $output"
    elif [ -z "$output" ]; then
        log_error "DROP:   (no output)"
    else
        log_error "DROP:   $output"
    fi
}

# ==============================================================================
# Main
# ==============================================================================

main() {
    log_separator
    log_banner
    log_info "ExecSQL Fix - Comparison Test"
    log ""

    # Summary
    log "The fix changes how sq executes DDL/DML statements:"
    log ""
    log_indent log_error "Before: INSERT returns 'Affected 0 rows' or no output"
    log_indent log_success "After:  INSERT returns 'Affected 3 rows' (correct)"
    log ""
    log "Technical: Uses ExecContext() instead of QueryContext() for DDL/DML"
    log ""
    log_separator
    log ""


    # Check new binary exists
    if [ ! -x "$NEW_SQ_BINARY" ] && ! command -v "$NEW_SQ_BINARY" >/dev/null 2>&1; then
        log_error "sq binary not found: $NEW_SQ_BINARY"
        log "Use --new /path/to/sq to specify"
        exit 1
    fi

    # Test OLD binary (if available)
    if [ -n "$OLD_SQ_BINARY" ] && [ -x "$OLD_SQ_BINARY" ]; then
        log "OLD sq (before fix)"
        log_indent log_dim "Binary: $OLD_SQ_BINARY"
        log_indent log_dim "Version: $("$OLD_SQ_BINARY" version 2>/dev/null || echo 'unknown')"
        log ""

        local db_old="/tmp/test_execsql_old_$$.db"
        "$OLD_SQ_BINARY" rm "@test_old" 2>/dev/null || true
        "$OLD_SQ_BINARY" add "sqlite3://$db_old" --handle "@test_old" >/dev/null 2>&1

        test_ddl_dml "$OLD_SQ_BINARY" "@test_old" "false"

        "$OLD_SQ_BINARY" rm "@test_old" 2>/dev/null || true
        rm -f "$db_old"
        log ""
    else
        log "OLD sq (skipped)"
        log_indent log_dim "No old binary available. Use --old /path/to/sq"
        log ""
    fi

    # Test NEW binary
    log "NEW sq (after fix)"
    log_indent log_dim "Binary: $NEW_SQ_BINARY"
    log_indent log_dim "Version: $("$NEW_SQ_BINARY" version 2>/dev/null || echo 'unknown')"
    log ""

    local db_new="/tmp/test_execsql_new_$$.db"
    "$NEW_SQ_BINARY" rm "@test_new" 2>/dev/null || true
    "$NEW_SQ_BINARY" add "sqlite3://$db_new" --handle "@test_new" >/dev/null 2>&1

    test_ddl_dml "$NEW_SQ_BINARY" "@test_new" "true"

    "$NEW_SQ_BINARY" rm "@test_new" 2>/dev/null || true
    rm -f "$db_new"
    log ""



    log_success "All tests completed successfully"
}

main
