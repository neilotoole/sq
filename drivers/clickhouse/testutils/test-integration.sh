#!/usr/bin/env bash
#
# test-integration.sh - Run ClickHouse driver integration tests
#
# This script:
# 1. Starts ClickHouse (and optionally Postgres) via docker-compose
# 2. Waits for databases to be healthy
# 3. Runs integration tests
# 4. Cleans up containers
#
# Usage:
#   ./test-integration.sh              # Run all integration tests (ClickHouse only)
#   ./test-integration.sh --with-pg    # Run all integration tests including cross-database tests
#   ./test-integration.sh --keep       # Don't stop containers after tests
#   ./test-integration.sh --help       # Show help

set -euo pipefail

# Get script directory and source common utilities
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/common.bash"

# Configuration
WITH_POSTGRES=false
KEEP_CONTAINERS=false
TEST_PATTERN=""
TIMEOUT="10m"

# Parse command line arguments
while [[ $# -gt 0 ]]; do
  case $1 in
    --with-pg|--with-postgres)
      WITH_POSTGRES=true
      shift
      ;;
    --keep)
      KEEP_CONTAINERS=true
      shift
      ;;
    --pattern)
      TEST_PATTERN="$2"
      shift 2
      ;;
    --timeout)
      TIMEOUT="$2"
      shift 2
      ;;
    --help|-h)
      log_separator
      log_info "ClickHouse Integration Test Runner"
      log "Usage: $0 [OPTIONS]"
      log_dim ""
      log_info "Options:"
      log "  --with-pg, --with-postgres  ${DIM}Start Postgres and run cross-database tests"
      log "  --keep                      ${DIM}Keep containers running after tests"
      log "  --pattern PATTERN           ${DIM}Run only tests matching PATTERN (e.g., TestSmoke)"
      log "  --timeout DURATION          ${DIM}Test timeout (default: 10m)"
      log "  --help, -h                  ${DIM}Show this help message"
      log ""
      log_info "Examples:"
      log "  $0                          ${DIM}# Run all ClickHouse integration tests"
      log "  $0 --with-pg                ${DIM}# Run all tests including cross-database"
      log "  $0 --pattern TestSmoke      ${DIM}# Run only TestSmoke"
      log "  $0 --keep                   ${DIM}# Keep containers running after tests"
      exit 0
      ;;
    *)
      log_error "Unknown option: $1"
      log "Use --help for usage information"
      exit 1
      ;;
  esac
done

# Change to script directory
cd "$SCRIPT_DIR"

# Check prerequisites (including Go for this script)
check_prerequisites() {
    log "Checking Prerequisites"

    # Check Docker prerequisites (from common.bash)
    check_docker_prerequisites

    # Also need Go for running tests
    if ! command_exists go; then
        log_error "go is not installed or not in PATH"
        exit 1
    fi

    log_success "Prerequisites Found Successfully"
}

# Function to start containers
start_containers() {
    local services="clickhouse"

    if [ "$WITH_POSTGRES" = true ]; then
        services="clickhouse postgres"
    fi

    log "Starting Containers"

    start_services $services
}

# Function to stop containers
stop_containers() {
    stop_services
}

# Function to run tests
run_tests() {
    log "Running integration tests..."

    # Build test command - run from driver directory where Go files are located
    local test_cmd="go test -v -timeout $TIMEOUT"

    if [ -n "$TEST_PATTERN" ]; then
        test_cmd="$test_cmd -run $TEST_PATTERN"
    fi

    # Show what we're running
    log "Running tests in: $DRIVER_DIR"
    log_indent log_dim "Test command: $test_cmd"
    echo ""

    # Run the tests from the driver directory
    pushd "$DRIVER_DIR" > /dev/null || return 1
    local result=0
    echo -en "${DIM}"
    eval "$test_cmd"
    result=$?
    echo -en "${RESET}"
    if [ $result -eq 0 ]; then
        echo ""
        log_success "All tests passed!"
    else
        echo ""
        log_error "Some tests failed"
    fi
    popd > /dev/null || return 1
    return $result
}

# Main execution
main() {
    log_separator
    log_banner
    log_info "ClickHouse Integration Test Runner"
    log ""

    # Check prerequisites
    check_prerequisites
    log ""

    # Start containers
    start_containers
    log ""

    # Wait for ClickHouse to be healthy
    if ! wait_for_healthy "clickhouse" 180; then
        log_error "ClickHouse failed to become healthy"
        show_service_logs clickhouse 50
        stop_containers
        exit 1
    fi
    log ""

    # Wait for Postgres if needed
    if [ "$WITH_POSTGRES" = true ]; then
        if ! wait_for_healthy "postgres" 60; then
            log_error "Postgres failed to become healthy"
            show_service_logs postgres 50
            stop_containers
            exit 1
        fi
        log ""
    fi

    # Run tests
    local test_result=0
    run_tests || test_result=$?
    log ""

    # Cleanup
    stop_containers
    log ""

    # Exit with test result
    if [ $test_result -eq 0 ]; then
        log_success "Integration test run completed successfully"
        log_separator
        exit 0
    else
        log_error "Integration test run failed"
        log_separator
        exit 1
    fi
}

# Trap to ensure cleanup on exit
cleanup_on_exit() {
    if [ $? -ne 0 ] && [ "$KEEP_CONTAINERS" != true ]; then
        log_warning "Tests failed, stopping containers..."
        force_stop_containers
    fi
}
trap cleanup_on_exit EXIT

# Run main function
main
