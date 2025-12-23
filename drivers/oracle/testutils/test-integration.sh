#!/usr/bin/env bash
#
# test-integration.sh - Run Oracle driver integration tests
#
# This script:
# 1. Starts Oracle (and optionally Postgres) via docker-compose
# 2. Waits for databases to be healthy
# 3. Runs integration tests
# 4. Cleans up containers
#
# Usage:
#   ./test-integration.sh              # Run all integration tests (Oracle only)
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
      echo "Usage: $0 [OPTIONS]"
      echo ""
      echo "Options:"
      echo "  --with-pg, --with-postgres  Start Postgres and run cross-database tests"
      echo "  --keep                      Keep containers running after tests"
      echo "  --pattern PATTERN           Run only tests matching PATTERN (e.g., TestSmoke)"
      echo "  --timeout DURATION          Test timeout (default: 10m)"
      echo "  --help, -h                  Show this help message"
      echo ""
      echo "Examples:"
      echo "  $0                          # Run all Oracle integration tests"
      echo "  $0 --with-pg                # Run all tests including cross-database"
      echo "  $0 --pattern TestSmoke      # Run only TestSmoke"
      echo "  $0 --keep                   # Keep containers running after tests"
      exit 0
      ;;
    *)
      log_error "Unknown option: $1"
      echo "Use --help for usage information"
      exit 1
      ;;
  esac
done

# Change to script directory
cd "$SCRIPT_DIR"

# Check prerequisites (including Go for this script)
check_prerequisites() {
    log_info "Checking prerequisites..."

    # Check Docker prerequisites (from common.bash)
    check_docker_prerequisites

    # Also need Go for running tests
    if ! command_exists go; then
        log_error "go is not installed or not in PATH"
        exit 1
    fi

    log_success "Prerequisites check passed"
}

# Function to start containers
start_containers() {
    local services="oracle"

    if [ "$WITH_POSTGRES" = true ]; then
        services="oracle postgres"
        log_info "Starting Oracle and Postgres containers..."
    else
        log_info "Starting Oracle container..."
    fi

    start_services $services
}

# Function to stop containers
stop_containers() {
    stop_services
}

# Function to run tests
run_tests() {
    log_info "Running integration tests..."

    # Set up Oracle Instant Client
    setup_oracle_instant_client || true

    # Build test command - run from driver directory where Go files are located
    local test_cmd="go test -v -timeout $TIMEOUT"

    if [ -n "$TEST_PATTERN" ]; then
        test_cmd="$test_cmd -run $TEST_PATTERN"
    fi

    # Show what we're running
    log_info "Running tests in: $DRIVER_DIR"
    log_info "Test command: $test_cmd"
    echo ""

    # Run the tests from the driver directory
    pushd "$DRIVER_DIR" > /dev/null
    local result=0
    if eval "$test_cmd"; then
        echo ""
        log_success "All tests passed!"
    else
        echo ""
        log_error "Some tests failed"
        result=1
    fi
    popd > /dev/null
    return $result
}

# Main execution
main() {
    log_separator
    log_banner
    log_info "Oracle Integration Test Runner"
    echo ""

    # Check prerequisites
    check_prerequisites
    echo ""

    # Start containers
    start_containers
    echo ""

    # Wait for Oracle to be healthy
    if ! wait_for_healthy "oracle" 180; then
        log_error "Oracle failed to become healthy"
        show_service_logs oracle 50
        stop_containers
        exit 1
    fi
    echo ""

    # Wait for Postgres if needed
    if [ "$WITH_POSTGRES" = true ]; then
        if ! wait_for_healthy "postgres" 60; then
            log_error "Postgres failed to become healthy"
            show_service_logs postgres 50
            stop_containers
            exit 1
        fi
        echo ""
    fi

    # Run tests
    local test_result=0
    run_tests || test_result=$?
    echo ""

    # Cleanup
    stop_containers
    echo ""

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
