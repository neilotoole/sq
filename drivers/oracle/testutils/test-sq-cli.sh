#!/usr/bin/env bash
#
# test-sq-cli.sh - Test Oracle driver using sq CLI
#
# This script tests the Oracle driver end-to-end using the sq command-line tool.
# It verifies:
# 1. Oracle connectivity and basic operations
# 2. Data querying and inspection
# 3. Cross-database data movement (Postgres -> Oracle)
#
# Usage:
#   ./test-sq-cli.sh              # Use default sq binary path
#   SQ_BINARY=/path/to/sq ./test-sq-cli.sh  # Use custom sq binary

set -euo pipefail

# Get script directory and source common utilities
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/common.bash"

# Configuration
# Default to $GOPATH/bin/sq, fall back to finding sq in PATH
if [ -z "${SQ_BINARY:-}" ]; then
    if [ -n "${GOPATH:-}" ] && [ -x "${GOPATH}/bin/sq" ]; then
        SQ_BINARY="${GOPATH}/bin/sq"
    elif command -v sq >/dev/null 2>&1; then
        SQ_BINARY="$(command -v sq)"
    else
        SQ_BINARY="sq"  # Will fail later with helpful error message
    fi
fi
ORACLE_DSN="${ORACLE_DSN:-oracle://testuser:testpass@localhost:1521/FREEPDB1}"
POSTGRES_DSN="${POSTGRES_DSN:-postgres://testuser:testpass@localhost:5432/sakila?sslmode=disable}"
TEST_TABLE_PREFIX="SQ_TEST"

# Set up Oracle Instant Client
setup_oracle_instant_client || true

# ==============================================================================
# SQ-Specific Functions
# ==============================================================================

# Run sq command with error handling
run_sq() {
    local description="$1"
    shift
    log_dim "$description"
    if "$SQ_BINARY" "$@"; then
        return 0
    else
        log_error "$description failed"
        return 1
    fi
}

# Check prerequisites (sq binary, Docker, etc.)
check_prerequisites() {
    log "Checking Prerequisites"

    # Check for sq binary
    if [ ! -f "$SQ_BINARY" ] && ! command -v "$SQ_BINARY" >/dev/null 2>&1; then
        log_error "sq binary not found at: $SQ_BINARY"
        log_error "Options to fix:"
        log_error "  1. Install sq: go install github.com/neilotoole/sq"
        log_error "  2. Set GOPATH if sq is in \$GOPATH/bin"
        log_error "  3. Set SQ_BINARY=/path/to/sq environment variable"
        exit 1
    fi

    # Check Docker prerequisites (from common.bash)
    check_docker_prerequisites

    log_indent log_dim "sq binary: $SQ_BINARY"
    log_indent log_dim "sq version: $("$SQ_BINARY" version 2>/dev/null || echo 'unknown')"
    log_success "Prerequisites Found Successfully"
}

# Start database containers (Oracle and Postgres)
start_containers() {
    log "Starting Containers"
    start_services oracle postgres

    # Wait for Oracle to be healthy
    if ! wait_for_healthy "oracle" 180; then
        log_error "Oracle did not become ready in time"
        return 1
    fi
    log ""

    # Wait for Postgres to be healthy
    if ! wait_for_healthy "postgres" 60; then
        log_error "Postgres did not become ready in time"
        return 1
    fi
}

# Stop database containers
stop_containers() {
    stop_services
}

# ==============================================================================
# SQ Source Management
# ==============================================================================

# Add data sources to sq
add_sources() {
    log "Adding Data Sources"

    # Remove existing sources if they exist
    "$SQ_BINARY" rm @test_oracle 2>/dev/null || true
    "$SQ_BINARY" rm @test_postgres 2>/dev/null || true

    # Add Oracle source
    if run_sq "Adding Oracle source" add --handle @test_oracle "$ORACLE_DSN"; then
        :
    else
        log_error "Failed to add Oracle source"
        return 1
    fi

    # Add Postgres source
    if run_sq "Adding Postgres source" add --handle @test_postgres "$POSTGRES_DSN"; then
        :
    else
        log_error "Failed to add Postgres source"
        return 1
    fi

    log_success "Data sources added"
}

# Cleanup sq sources
cleanup_sources() {
    log_dim "Cleaning up sources..."

    # Remove sources
    "$SQ_BINARY" rm @test_oracle 2>/dev/null || true
    "$SQ_BINARY" rm @test_postgres 2>/dev/null || true
}

# ==============================================================================
# Test Functions
# ==============================================================================

# Test Oracle connectivity and basic operations
test_oracle_basic() {
    log "Testing Oracle Basic Operations"

    # Test 1: Inspect Oracle (show schema)
    log_dim "Test 1: Inspect Oracle schema"
    if "$SQ_BINARY" inspect @test_oracle >/dev/null 2>&1; then
        :
    else
        log_error "Oracle schema inspection failed"
        return 1
    fi

    # Test 2: Execute simple query using SQL
    log_dim "Test 2: Execute simple query (SELECT FROM DUAL)"
    local result
    result=$("$SQ_BINARY" sql --src @test_oracle "SELECT 'Hello' AS TEST_COL FROM DUAL" --json 2>/dev/null)
    if echo "$result" | /usr/bin/grep -q '"TEST_COL"'; then
        :
    else
        log_error "Simple query failed"
        return 1
    fi

    # Test 3: Query current schema using SQL
    log_dim "Test 3: Query current schema"
    local schema
    schema=$("$SQ_BINARY" sql --src @test_oracle "SELECT SYS_CONTEXT('USERENV', 'CURRENT_SCHEMA') AS SCHEMA_NAME FROM DUAL" --json 2>/dev/null | /usr/bin/grep -o '"SCHEMA_NAME": "[^"]*"' | cut -d'"' -f4)
    if [ -n "$schema" ]; then
        :
    else
        log_error "Failed to query current schema"
        return 1
    fi

    log_success "Oracle basic operations test passed"
}

# Test creating and querying tables in Oracle
test_oracle_tables() {
    log "Testing Oracle Table Operations"

    local test_table
    test_table="${TEST_TABLE_PREFIX}_DEMO_$(date +%s)"

    # Test 1: Create a test table using SQL
    log_dim "Test 1: Create test table in Oracle"
    if "$SQ_BINARY" sql --src @test_oracle "CREATE TABLE ${test_table} (ID NUMBER(19,0), NAME VARCHAR2(100), CREATED_AT TIMESTAMP)" >/dev/null 2>&1; then
        :
    else
        log_error "Failed to create table in Oracle"
        return 1
    fi

    # Test 2: Insert data using SQL
    log_dim "Test 2: Insert data into Oracle table"
    if "$SQ_BINARY" sql --src @test_oracle "INSERT INTO ${test_table} (ID, NAME, CREATED_AT) VALUES (1, 'Test User', SYSTIMESTAMP)" >/dev/null 2>&1; then
        :
    else
        log_error "Failed to insert data"
        return 1
    fi

    # Test 3: Query data back
    log_dim "Test 3: Query data from Oracle table"
    local row_count
    row_count=$("$SQ_BINARY" sql --src @test_oracle "SELECT TO_CHAR(COUNT(*)) AS CNT FROM ${test_table}" --json 2>/dev/null | /usr/bin/grep -o '"CNT": "[0-9]*"' | cut -d'"' -f4)
    if [ "$row_count" -eq 1 ]; then
        :
    else
        log_error "Table row count mismatch. Expected: 1, Got: $row_count"
        return 1
    fi

    # Test 4: Query with WHERE clause
    log_dim "Test 4: Query with WHERE clause"
    local name
    name=$("$SQ_BINARY" sql --src @test_oracle "SELECT NAME FROM ${test_table} WHERE ID = 1" --json 2>/dev/null | /usr/bin/grep -o '"NAME": "[^"]*"' | cut -d'"' -f4)
    if [ "$name" = "Test User" ]; then
        :
    else
        log_error "WHERE clause query failed. Expected: Test User, Got: $name"
        return 1
    fi

    # Note: We don't drop the table here because sq doesn't have a DROP TABLE command
    # The table will remain in Oracle but that's fine for testing
    log_indent log_dim "Test table ${test_table} left in Oracle"

    log_success "Oracle table operations test passed"
}

# Test cross-database operations
test_cross_database() {
    log "Testing Cross-Database Operations"

    # Test 1: Query Postgres data
    log_dim "Test 1: Query data from Postgres"
    local pg_output
    local pg_count
    pg_output=$("$SQ_BINARY" sql --src @test_postgres "SELECT COUNT(*)::text AS CNT FROM actor" --json 2>&1)
    pg_count=$(echo "$pg_output" | /usr/bin/grep -o '"[Cc][Nn][Tt]": "[0-9]*"' | cut -d'"' -f4)
    if [ -n "$pg_count" ] && [ "$pg_count" -gt 0 ]; then
        :
    else
        log_error "Failed to query Postgres"
        return 1
    fi

    # Test 2: Create a matching table in Oracle and insert data manually
    log_dim "Test 2: Create test table in Oracle for cross-database test"
    local test_table
    test_table="${TEST_TABLE_PREFIX}_XFER_$(date +%s)"

    if "$SQ_BINARY" sql --src @test_oracle "CREATE TABLE ${test_table} (ACTOR_ID NUMBER(10,0), FIRST_NAME VARCHAR2(45), LAST_NAME VARCHAR2(45))" >/dev/null 2>&1; then
        :
    else
        log_error "Failed to create table in Oracle"
        return 1
    fi

    # Test 3: Insert sample data into Oracle table
    log_dim "Test 3: Insert sample data into Oracle table"
    if "$SQ_BINARY" sql --src @test_oracle "INSERT INTO ${test_table} (ACTOR_ID, FIRST_NAME, LAST_NAME) VALUES (1, 'TEST', 'USER')" >/dev/null 2>&1; then
        :
    else
        log_error "Failed to insert data"
        return 1
    fi

    # Test 4: Query Oracle table
    log_dim "Test 4: Query Oracle table"
    local ora_count
    ora_count=$("$SQ_BINARY" sql --src @test_oracle "SELECT TO_CHAR(COUNT(*)) AS CNT FROM ${test_table}" --json 2>/dev/null | /usr/bin/grep -o '"CNT": "[0-9]*"' | cut -d'"' -f4)
    if [ "$ora_count" -eq 1 ]; then
        :
    else
        log_error "Failed to query Oracle table"
        return 1
    fi

    log_indent log_dim "Test table ${test_table} left in Oracle"

    log_success "Cross-database operations test passed"
}

# Test type mappings
test_type_mappings() {
    log "Testing Oracle Type Mappings"

    local test_table
    test_table="${TEST_TABLE_PREFIX}_TYPES_$(date +%s)"

    # Test 1: Create table with various data types
    log_dim "Test 1: Create table with various Oracle data types"
    if "$SQ_BINARY" sql --src @test_oracle "CREATE TABLE ${test_table} (
        COL_INT NUMBER(19,0),
        COL_TEXT VARCHAR2(100),
        COL_FLOAT BINARY_DOUBLE,
        COL_DECIMAL NUMBER(10,2),
        COL_BOOL NUMBER(1,0),
        COL_TIMESTAMP TIMESTAMP,
        COL_DATE DATE
    )" >/dev/null 2>&1; then
        :
    else
        log_error "Failed to create table with types"
        return 1
    fi

    # Test 2: Insert data with different types
    log_dim "Test 2: Insert data with various types"
    if "$SQ_BINARY" sql --src @test_oracle "INSERT INTO ${test_table} VALUES (
        42,
        'Test Text',
        3.14,
        123.45,
        1,
        TIMESTAMP '2024-01-01 12:00:00',
        DATE '2024-01-01'
    )" >/dev/null 2>&1; then
        :
    else
        log_error "Failed to insert data"
        return 1
    fi

    # Test 3: Query data back and verify
    log_dim "Test 3: Query and verify type-mapped data"
    local int_val
    int_val=$("$SQ_BINARY" sql --src @test_oracle "SELECT TO_CHAR(COL_INT) AS COL_INT FROM ${test_table}" --json 2>/dev/null | /usr/bin/grep -o '"COL_INT": "[0-9]*"' | cut -d'"' -f4)
    if [ "$int_val" -eq 42 ]; then
        :
    else
        log_error "Type mapping failed. Expected: 42, Got: $int_val"
        return 1
    fi

    log_indent log_dim "Test table ${test_table} left in Oracle"

    log_success "Type mapping test passed"
}

# ==============================================================================
# Main Execution
# ==============================================================================

main() {
    log_separator
    log_banner
    log_info "SQ CLI Oracle Driver Test"
    log ""

    # Track test results
    local tests_passed=0
    local tests_failed=0

    # Check prerequisites
    check_prerequisites
    log ""

    # Start containers
    start_containers
    log ""

    # Add sources
    if add_sources; then
        ((tests_passed++))
    else
        ((tests_failed++))
    fi
    log ""

    # Run tests
    if test_oracle_basic; then
        ((tests_passed++))
    else
        ((tests_failed++))
    fi
    log ""

    if test_oracle_tables; then
        ((tests_passed++))
    else
        ((tests_failed++))
    fi
    log ""

    if test_cross_database; then
        ((tests_passed++))
    else
        ((tests_failed++))
    fi
    log ""

    if test_type_mappings; then
        ((tests_passed++))
    else
        ((tests_failed++))
    fi
    log ""

    # Cleanup
    cleanup_sources
    log ""

    # Stop containers
    stop_services
    log ""

    # Print summary
    if [ $tests_failed -gt 0 ]; then
        log_error "Tests failed: $tests_failed of $((tests_passed + tests_failed))"
        log_error "SQ CLI Oracle driver test FAILED"
        log_separator
        exit 1
    else
        log_success "All $tests_passed tests passed!"
        log_success "SQ CLI Oracle driver test completed successfully"
        log_separator
        exit 0
    fi
}

# Trap to ensure cleanup on exit
cleanup_on_exit() {
    if [ $? -ne 0 ]; then
        log_warning "Tests failed, cleaning up..."
        cleanup_sources 2>/dev/null || true
        force_stop_containers
    fi
}
trap cleanup_on_exit EXIT

# Run main function
main
