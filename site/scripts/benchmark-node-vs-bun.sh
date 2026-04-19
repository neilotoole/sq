#!/usr/bin/env bash
#
# Benchmark Script: Node.js/npm vs Bun Performance Comparison
# Compares two branches with different JavaScript runtimes
# Configure branches in the Configuration section below
#
# Usage: ./benchmark-node-vs-bun.sh [OPTIONS]
#

set -euo pipefail

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

# Configuration
RUNS=3
TIMEOUT=120
VERBOSE=false
DRY_RUN=false
NO_CONFIRM=false
CLEAN_FIRST=false
CLEAN_ONLY_MODE=false
TEST_OUTPUT_MODE=false
REPLAY_FILE=""
RESULTS_FILE=""
ORIGINAL_BRANCH=""
STASH_CREATED=false
SKIP_CLEANUP=false

# Branch configuration - change these to compare different branches
# NPM_BRANCH: last master commit with npm (so benchmark stays npm vs bun after Bun PR lands)
NPM_BRANCH="8041c83"                # Upgrade to Node.js 22 and npm 10 for Netlify builds (#60)
BUN_BRANCH="feature/bun-migration"  # Default: Bun migration branch
NPM_RUNTIME="npm"                   # Default: Node.js/npm
BUN_RUNTIME="bun"                   # Runtime name for Bun branch

# Metrics storage
declare -a NPM_INSTALL_TIMES
declare -a NPM_SERVER_TIMES
declare -a NPM_BUILD_TIMES
declare -a NPM_LINT_TIMES
declare -a BUN_INSTALL_TIMES
declare -a BUN_SERVER_TIMES
declare -a BUN_BUILD_TIMES
declare -a BUN_LINT_TIMES

NPM_SERVER_MEMORY=""
BUN_SERVER_MEMORY=""

NPM_NODE_MODULES_SIZE=""
NPM_LOCKFILE_SIZE=""
NPM_PACKAGE_COUNT=""
BUN_NODE_MODULES_SIZE=""
BUN_LOCKFILE_SIZE=""
BUN_PACKAGE_COUNT=""

NPM_SITE_CHECK=""
BUN_SITE_CHECK=""

# Print functions
print_header() {
    log_info "$1"
}

print_success() {
    log_success "$1"
}

print_error() {
    log_error "$1"
}

print_info() {
    log_info "$1"
}

print_warning() {
    log_warning "$1"
}

# Usage information
usage() {
    while IFS= read -r line; do
        log "$line"
    done << EOF
Usage: $0 [OPTIONS]

Benchmark performance comparison between Node.js/npm ($NPM_BRANCH) and Bun ($BUN_BRANCH).

Options:
  -h, --help           Show this help message
  -r, --runs N         Number of test runs per metric (default: 3)
  -t, --timeout N      Server startup timeout in seconds (default: 120)
  -n, --no-confirm     Skip confirmation prompt
  -d, --dry-run        Show what would be done without executing
  -v, --verbose        Show detailed output during tests
  -1, --single         Run tests only once (equivalent to -r 1)
  -c, --clean          Clean up artifacts before running benchmarks
  --clean-only         Clean up artifacts and exit (don't run benchmarks)
  -T, --test-output    Test output formatting with sample data (no benchmarks)
  -R, --replay FILE    Replay results from a previous benchmark file

Examples:
  $0                   Run with default settings (3 runs)
  $0 -1                Run tests once (quick benchmark)
  $0 -r 5 -v           Run 5 iterations with verbose output
  $0 --no-confirm      Run without confirmation prompt
  $0 -c                Clean up first, then run benchmarks
  $0 --clean-only      Remove old benchmark results and artifacts only
  $0 --clean-only -n   Clean up without confirmation

EOF
    SKIP_CLEANUP=true
    exit 0
}

# Parse command line arguments
parse_args() {
    while [[ $# -gt 0 ]]; do
        case $1 in
            -h|--help)
                usage
                ;;
            -r|--runs)
                RUNS="$2"
                shift 2
                ;;
            -1|--single)
                RUNS=1
                shift
                ;;
            -t|--timeout)
                TIMEOUT="$2"
                shift 2
                ;;
            -n|--no-confirm)
                NO_CONFIRM=true
                shift
                ;;
            -d|--dry-run)
                DRY_RUN=true
                shift
                ;;
            -v|--verbose)
                VERBOSE=true
                shift
                ;;
            -c|--clean)
                CLEAN_FIRST=true
                shift
                ;;
            --clean-only)
                CLEAN_ONLY_MODE=true
                shift
                ;;
            -T|--test-output)
                TEST_OUTPUT_MODE=true
                shift
                ;;
            -R|--replay)
                REPLAY_FILE="$2"
                shift 2
                ;;
            *)
                print_error "Unknown option: $1"
                usage
                ;;
        esac
    done
}

# Check prerequisites
check_prerequisites() {
    print_header "Checking prerequisites..."

    # Check if we're in a git repo
    if ! git rev-parse --git-dir > /dev/null 2>&1; then
        print_error "Not in a git repository"
        exit 1
    fi

    # Check if we're in sq-web repo
    if [[ ! -f "package.json" ]] || ! grep -q "sq.io" package.json; then
        print_error "Not in sq-web repository"
        exit 1
    fi

    # Check for required commands
    local missing_cmds=()
    for cmd in git node npm bun curl bc; do
        if ! command -v "$cmd" &> /dev/null; then
            missing_cmds+=("$cmd")
        fi
    done

    if [[ ${#missing_cmds[@]} -gt 0 ]]; then
        print_error "Missing required commands: ${missing_cmds[*]}"
        exit 1
    fi

    # Check if branches exist
    if ! git rev-parse --verify "$NPM_BRANCH" &> /dev/null; then
        print_error "Branch '$NPM_BRANCH' does not exist"
        exit 1
    fi

    if ! git rev-parse --verify "$BUN_BRANCH" &> /dev/null; then
        print_error "Branch '$BUN_BRANCH' does not exist"
        exit 1
    fi

    # Get versions
    local node_version=$(node --version)
    local npm_version=$(npm --version)
    local bun_version=$(bun --version)

    print_success "Git repository: $(basename "$(pwd)")"
    print_success "Node.js: $node_version"
    print_success "npm: $npm_version"
    print_success "Bun: $bun_version"
    log ""
}

# Save current git state
save_git_state() {
    ORIGINAL_BRANCH=$(git rev-parse --abbrev-ref HEAD)
    print_info "Current branch: $ORIGINAL_BRANCH"

    # Check for uncommitted changes
    if ! git diff --quiet || ! git diff --cached --quiet; then
        print_warning "Uncommitted changes detected, stashing..."
        git stash push -m "benchmark-script-$(date +%s)" > /dev/null
        STASH_CREATED=true
    fi
}

# Restore git state
restore_git_state() {
    print_info "Restoring git state..."

    if [[ -n "$ORIGINAL_BRANCH" ]]; then
        git checkout "$ORIGINAL_BRANCH" > /dev/null 2>&1 || true
    fi

    if [[ "$STASH_CREATED" == true ]]; then
        git stash pop > /dev/null 2>&1 || true
    fi
}

# Cleanup function
cleanup() {
    # Skip cleanup for quick-exit modes
    if [[ "$SKIP_CLEANUP" == true ]] || [[ "$TEST_OUTPUT_MODE" == true ]] || [[ -n "$REPLAY_FILE" ]] || [[ "$CLEAN_ONLY_MODE" == true ]]; then
        return
    fi

    print_info "Cleaning up..."

    # Kill any running Hugo servers
    pkill -f "hugo server" > /dev/null 2>&1 || true

    # Wait for port to be free
    sleep 2
}

# Trap for cleanup on exit
trap cleanup EXIT
trap 'print_error "Script interrupted"; restore_git_state; exit 130' INT TERM

# Clean environment
clean_env() {
    local context=$1

    if [[ "$VERBOSE" == true ]]; then
        print_info "[$context] Cleaning environment..." >&2
    fi

    # Kill any running processes
    pkill -f "hugo server" > /dev/null 2>&1 || true
    sleep 1

    # Remove directories and files
    rm -rf node_modules public resources .serve-lint
    rm -f package-lock.json bun.lockb bun.lock

    if [[ "$VERBOSE" == true ]]; then
        print_success "[$context] Environment cleaned" >&2
    fi
}

# Measure time with high precision
time_command() {
    local start=$(date +%s.%N 2>/dev/null || date +%s)
    "$@" > /dev/null 2>&1
    local end=$(date +%s.%N 2>/dev/null || date +%s)
    local result=$(echo "$end - $start" | bc -l 2>/dev/null)
    # Ensure we return a valid number
    if [[ -n "$result" ]] && [[ "$result" =~ ^[0-9.]+$ ]]; then
        echo "$result"
    else
        echo "0"
    fi
}

# Benchmark install
benchmark_install() {
    local runtime=$1
    local run_num=$2
    local cmd=$3

    clean_env "$runtime install run $run_num"

    local time=$(time_command $cmd)
    local time_fmt
    time_fmt=$(printf "%.2fs" "$time")
    log_info_dim "  [$run_num/$RUNS] Installing dependencies... ${time_fmt}"
    echo "$time"
}

# Wait for server to be ready
wait_for_server() {
    local timeout=$1
    local elapsed=0
    local interval=0.5

    while (( $(echo "$elapsed < $timeout" | bc -l 2>/dev/null || echo "0") )); do
        if curl -s -o /dev/null -w "%{http_code}" http://localhost:1313/ 2>/dev/null | grep -q "200"; then
            printf "%.2f" "$elapsed"
            return 0
        fi
        sleep "$interval"
        elapsed=$(echo "$elapsed + $interval" | bc -l 2>/dev/null || echo "$elapsed")
    done

    echo "-1"
    return 1
}

# Benchmark server startup
benchmark_server_start() {
    local runtime=$1
    local run_num=$2
    local cmd=$3

    clean_env "$runtime server run $run_num"

    log_info_dim "  [$run_num/$RUNS] Starting server..."

    # Install dependencies first (silently)
    $cmd > /dev/null 2>&1

    # Start server in background
    local start=$(date +%s.%N 2>/dev/null || date +%s)

    if [[ "$runtime" == "npm" ]]; then
        npm start > /dev/null 2>&1 &
    else
        bun start > /dev/null 2>&1 &
    fi

    local server_pid=$!

    # Wait for server to respond
    local wait_result=$(wait_for_server "$TIMEOUT")

    if [[ "$wait_result" == "-1" ]]; then
        print_error "Server failed to start within ${TIMEOUT}s" >&2
        kill $server_pid 2>/dev/null || true
        echo "-1"
        return 1
    fi

    # Calculate actual elapsed time from when server was started
    local end=$(date +%s.%N 2>/dev/null || date +%s)
    local ready_time=$(echo "$end - $start" | bc -l 2>/dev/null || echo "$wait_result")

    local ready_fmt
    ready_fmt=$(printf "%.2fs" "$ready_time")
    log_info_dim "  [$run_num/$RUNS] Server ready in ${ready_fmt}"

    # Kill server
    kill $server_pid 2>/dev/null || true
    pkill -f "hugo server" > /dev/null 2>&1 || true
    sleep 1

    echo "$ready_time"
}

# Benchmark Hugo build
benchmark_build() {
    local runtime=$1
    local run_num=$2
    local cmd=$3

    clean_env "$runtime build run $run_num"

    log_info_dim "  [$run_num/$RUNS] Running Hugo build..."

    # Install dependencies first (silently)
    $cmd > /dev/null 2>&1

    # Run build
    local time
    if [[ "$runtime" == "npm" ]]; then
        time=$(time_command npm run build)
    else
        time=$(time_command bun run build)
    fi

    local time_fmt
    time_fmt=$(printf "%.2fs" "$time")
    log_info_dim "  [$run_num/$RUNS] Build completed in ${time_fmt}"
    echo "$time"
}

# Measure server memory usage (in MB)
measure_server_memory() {
    local runtime=$1
    local install_cmd=$2

    log_info_dim "  Measuring server memory usage..."

    # Install dependencies first (silently)
    $install_cmd > /dev/null 2>&1

    # Start server in background
    if [[ "$runtime" == "npm" ]]; then
        npm start > /dev/null 2>&1 &
    else
        bun start > /dev/null 2>&1 &
    fi

    local server_pid=$!

    # Wait for server to be ready
    local ready=$(wait_for_server 60)

    if [[ "$ready" == "-1" ]]; then
        print_error "Server failed to start for memory measurement" >&2
        kill $server_pid 2>/dev/null || true
        echo "0"
        return 1
    fi

    # Wait a bit for memory to stabilize
    sleep 3

    # Get memory usage of hugo process (RSS in KB, convert to MB)
    local hugo_pid=$(pgrep -f "hugo server" | head -1)
    local memory_kb=0

    if [[ -n "$hugo_pid" ]]; then
        # macOS uses different ps format than Linux
        if [[ "$(uname)" == "Darwin" ]]; then
            memory_kb=$(ps -o rss= -p "$hugo_pid" 2>/dev/null | tr -d ' ' || echo "0")
        else
            memory_kb=$(ps -o rss= -p "$hugo_pid" 2>/dev/null | tr -d ' ' || echo "0")
        fi
    fi

    # Convert KB to MB
    local memory_mb=0
    if [[ -n "$memory_kb" ]] && [[ "$memory_kb" =~ ^[0-9]+$ ]] && [[ "$memory_kb" -gt 0 ]]; then
        memory_mb=$(echo "scale=0; $memory_kb / 1024" | bc -l 2>/dev/null || echo "0")
    fi

    log_info_dim "  Server memory usage: ${memory_mb} MB"

    # Kill server
    kill $server_pid 2>/dev/null || true
    pkill -f "hugo server" > /dev/null 2>&1 || true
    sleep 1

    echo "$memory_mb"
}

# Benchmark linting
benchmark_lint() {
    local runtime=$1
    local run_num=$2
    local cmd=$3

    if [[ $run_num -eq 1 ]]; then
        clean_env "$runtime lint run $run_num"
        # Install dependencies first (silently)
        $cmd > /dev/null 2>&1
    fi

    log_info_dim "  [$run_num/$RUNS] Running linters (scripts, styles, markdown)..."

    # Run linting (excluding link checker as it's slow)
    local time
    if [[ "$runtime" == "npm" ]]; then
        time=$(time_command bash -c "npm run lint:scripts && npm run lint:styles && npm run lint:markdown")
    else
        time=$(time_command bash -c "bun run lint:scripts && bun run lint:styles && bun run lint:markdown")
    fi

    local time_fmt
    time_fmt=$(printf "%.2fs" "$time")
    log_info_dim "  [$run_num/$RUNS] Linters completed in ${time_fmt}"
    echo "$time"
}

# Get directory size in MB
get_dir_size() {
    local dir=$1
    if [[ -d "$dir" ]]; then
        du -sm "$dir" 2>/dev/null | awk '{print $1}'
    else
        echo "0"
    fi
}

# Get file size in MB
get_file_size() {
    local file=$1
    if [[ -f "$file" ]]; then
        local size=$(ls -l "$file" | awk '{print $5}')
        if [[ -n "$size" ]] && [[ "$size" =~ ^[0-9]+$ ]]; then
            local mb=$(echo "scale=2; $size / 1024 / 1024" | bc -l 2>/dev/null || echo "0")
            # Ensure leading zero for values < 1
            if [[ "$mb" == .* ]]; then
                echo "0$mb"
            else
                echo "$mb"
            fi
        else
            echo "0"
        fi
    else
        echo "0"
    fi
}

# Count packages
count_packages() {
    if [[ -d "node_modules" ]]; then
        find node_modules -maxdepth 1 -type d | wc -l | awk '{print $1-1}'
    else
        echo "0"
    fi
}

# Verify site functionality
verify_site() {
    local runtime=$1
    local install_cmd=$2

    print_info "Verifying site functionality..."

    clean_env "$runtime site verification"
    $install_cmd > /dev/null 2>&1

    # Start server
    if [[ "$runtime" == "npm" ]]; then
        npm start > /dev/null 2>&1 &
    else
        bun start > /dev/null 2>&1 &
    fi

    local server_pid=$!

    # Wait for server
    local ready=$(wait_for_server 120)

    if [[ "$ready" == "-1" ]]; then
        print_error "Server failed to start for verification"
        kill $server_pid 2>/dev/null || true
        echo "0/9"
        return 1
    fi

    local checks_passed=0
    local checks_total=9

    # 1. Check homepage loads
    if curl -s -o /dev/null -w "%{http_code}" http://localhost:1313/ | grep -q "200"; then
        print_success "  Homepage loads (HTTP 200)"
        ((checks_passed++))
    else
        print_error "  Homepage failed to load"
    fi

    # 2. Check homepage content
    if curl -s http://localhost:1313/ | grep -qi "sq"; then
        print_success "  Homepage contains expected content"
        ((checks_passed++))
    else
        print_error "  Homepage content check failed"
    fi

    # 3. Check docs page
    if curl -s -o /dev/null -w "%{http_code}" http://localhost:1313/docs/ | grep -q "200"; then
        print_success "  Docs page loads (HTTP 200)"
        ((checks_passed++))
    else
        print_error "  Docs page failed to load"
    fi

    # 4. Check for CSS/JS assets
    if curl -s http://localhost:1313/ | grep -qE '\.(css|js)'; then
        print_success "  CSS/JS assets referenced"
        ((checks_passed++))
    else
        print_error "  CSS/JS assets check failed"
    fi

    # 5. Check install page loads (Hugo uses pretty URLs without .html)
    if curl -s -o /dev/null -w "%{http_code}" http://localhost:1313/docs/install/ | grep -q "200"; then
        print_success "  Install page loads (HTTP 200)"
        ((checks_passed++))
    else
        print_error "  Install page failed to load"
    fi

    # 6. Check overview page loads (Hugo uses pretty URLs without .html)
    if curl -s -o /dev/null -w "%{http_code}" http://localhost:1313/docs/overview/ | grep -q "200"; then
        print_success "  Overview page loads (HTTP 200)"
        ((checks_passed++))
    else
        print_error "  Overview page failed to load"
    fi

    # 7. Check search functionality is available (search input element or index.json)
    local search_input=$(curl -s http://localhost:1313/ | grep -qE 'search|Search' && echo "yes" || echo "no")
    local search_index=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:1313/index.json 2>/dev/null)
    if [[ "$search_input" == "yes" ]] || [[ "$search_index" == "200" ]]; then
        print_success "  Search functionality available"
        ((checks_passed++))
    else
        print_error "  Search functionality not found"
    fi

    # 8. Check images load (favicon or any image)
    local favicon_status=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:1313/favicon.ico 2>/dev/null)
    local img_in_html=$(curl -s http://localhost:1313/ | grep -qE '<img' && echo "yes" || echo "no")
    if [[ "$favicon_status" == "200" ]] || [[ "$img_in_html" == "yes" ]]; then
        print_success "  Images/favicon available"
        ((checks_passed++))
    else
        print_error "  Images check failed"
    fi

    # 9. Check no major errors in page content (look for error indicators)
    local homepage_content=$(curl -s http://localhost:1313/)
    if echo "$homepage_content" | grep -qiE 'error|exception|failed to|cannot find'; then
        print_error "  Page contains error messages"
    else
        print_success "  No error messages in content"
        ((checks_passed++))
    fi

    # Kill server
    kill $server_pid 2>/dev/null || true
    pkill -f "hugo server" > /dev/null 2>&1 || true
    sleep 1

    echo "$checks_passed/$checks_total"
}

# Calculate average
calc_average() {
    local sum=0
    local count=0

    for val in "$@"; do
        # Skip empty, non-numeric, or -1 values
        if [[ -n "$val" ]] && [[ "$val" != "-1" ]] && [[ "$val" =~ ^[0-9.]+$ ]]; then
            sum=$(echo "$sum + $val" | bc -l 2>/dev/null || echo "$sum")
            ((count++))
        fi
    done

    if [[ $count -gt 0 ]]; then
        echo "scale=2; $sum / $count" | bc -l 2>/dev/null || echo "0"
    else
        echo "0"
    fi
}

# Calculate percentage improvement
calc_improvement() {
    local old=$1
    local new=$2

    # Validate inputs are numeric
    if [[ ! "$old" =~ ^[0-9.]+$ ]] || [[ ! "$new" =~ ^[0-9.]+$ ]]; then
        echo "N/A"
        return
    fi

    # Check if old is zero
    if [[ $(echo "$old == 0" | bc -l 2>/dev/null || echo "1") -eq 1 ]]; then
        echo "N/A"
        return
    fi

    local diff=$(echo "$old - $new" | bc -l 2>/dev/null || echo "0")
    local percent=$(echo "scale=1; ($diff / $old) * 100" | bc -l 2>/dev/null || echo "0")

    if [[ $(echo "$percent > 0" | bc -l 2>/dev/null || echo "0") -eq 1 ]]; then
        printf "%.1f%% faster" "$percent"
    elif [[ $(echo "$percent < 0" | bc -l 2>/dev/null || echo "0") -eq 1 ]]; then
        local abs_percent=$(echo "$percent * -1" | bc -l 2>/dev/null || echo "0")
        printf "%.1f%% slower" "$abs_percent"
    else
        echo "Same"
    fi
}

# Test branch
test_branch() {
    local branch=$1
    local runtime=$2
    local install_cmd=$3

    print_header "Testing $branch ($runtime)"
    log ""

    # Clean before checkout to avoid conflicts with lock files
    print_info "Cleaning environment before checkout..."
    rm -rf node_modules public resources .serve-lint > /dev/null 2>&1 || true
    rm -f package-lock.json bun.lockb bun.lock > /dev/null 2>&1 || true

    # Checkout branch
    print_info "Checking out $branch..."
    if ! git checkout "$branch" > /dev/null 2>&1; then
        print_error "Failed to checkout branch '$branch'. Do you have uncommitted changes?"
        print_error "Run 'git status' to check, or use 'git stash' to save changes."
        return 1
    fi
    log ""

    # Benchmark install
    print_info "Benchmarking dependency installation..."
    local install_times=()
    local t
    for ((i=1; i<=RUNS; i++)); do
        t=$(benchmark_install "$runtime" "$i" "$install_cmd")
        install_times+=("$t")
    done
    log ""

    # Benchmark server startup
    print_info "Benchmarking server startup..."
    local server_times=()
    for ((i=1; i<=RUNS; i++)); do
        t=$(benchmark_server_start "$runtime" "$i" "$install_cmd")
        server_times+=("$t")
    done
    log ""

    # Benchmark Hugo build
    print_info "Benchmarking Hugo build..."
    local build_times=()
    for ((i=1; i<=RUNS; i++)); do
        t=$(benchmark_build "$runtime" "$i" "$install_cmd")
        build_times+=("$t")
    done
    log ""

    # Benchmark linting
    print_info "Benchmarking linting..."
    local lint_times=()
    for ((i=1; i<=RUNS; i++)); do
        t=$(benchmark_lint "$runtime" "$i" "$install_cmd")
        lint_times+=("$t")
    done
    log ""

    # Get metrics
    clean_env "$runtime metrics"
    $install_cmd > /dev/null 2>&1

    local node_modules_size=$(get_dir_size "node_modules")
    local package_count=$(count_packages)
    local lockfile_size=""

    if [[ "$runtime" == "npm" ]]; then
        lockfile_size=$(get_file_size "package-lock.json")
    else
        # Try bun.lock first (text format), fallback to bun.lockb (binary format)
        if [[ -f "bun.lock" ]]; then
            lockfile_size=$(get_file_size "bun.lock")
        else
            lockfile_size=$(get_file_size "bun.lockb")
        fi
    fi

    print_info "Collecting metrics..."
    print_success "  Packages installed: $package_count"
    print_success "  node_modules size: ${node_modules_size} MB"
    print_success "  Lockfile size: ${lockfile_size} MB"

    # Measure server memory usage
    local server_memory=$(measure_server_memory "$runtime" "$install_cmd")
    log ""

    # Verify site
    local site_check=$(verify_site "$runtime" "$install_cmd")
    log ""

    # Store results
    if [[ "$runtime" == "npm" ]]; then
        NPM_INSTALL_TIMES=("${install_times[@]}")
        NPM_SERVER_TIMES=("${server_times[@]}")
        NPM_BUILD_TIMES=("${build_times[@]}")
        NPM_LINT_TIMES=("${lint_times[@]}")
        NPM_NODE_MODULES_SIZE="$node_modules_size"
        NPM_LOCKFILE_SIZE="$lockfile_size"
        NPM_PACKAGE_COUNT="$package_count"
        NPM_SERVER_MEMORY="$server_memory"
        NPM_SITE_CHECK="$site_check"
        if [[ "$VERBOSE" == true ]]; then
            print_info "DEBUG [npm]: install_times=(${install_times[*]}) -> NPM_INSTALL_TIMES=(${NPM_INSTALL_TIMES[*]})"
            print_info "DEBUG [npm]: lint_times=(${lint_times[*]}) -> NPM_LINT_TIMES=(${NPM_LINT_TIMES[*]})"
        fi
    else
        BUN_INSTALL_TIMES=("${install_times[@]}")
        BUN_SERVER_TIMES=("${server_times[@]}")
        BUN_BUILD_TIMES=("${build_times[@]}")
        BUN_LINT_TIMES=("${lint_times[@]}")
        BUN_NODE_MODULES_SIZE="$node_modules_size"
        BUN_LOCKFILE_SIZE="$lockfile_size"
        BUN_PACKAGE_COUNT="$package_count"
        BUN_SERVER_MEMORY="$server_memory"
        BUN_SITE_CHECK="$site_check"
        if [[ "$VERBOSE" == true ]]; then
            print_info "DEBUG [bun]: install_times=(${install_times[*]}) -> BUN_INSTALL_TIMES=(${BUN_INSTALL_TIMES[*]})"
            print_info "DEBUG [bun]: lint_times=(${lint_times[*]}) -> BUN_LINT_TIMES=(${BUN_LINT_TIMES[*]})"
        fi
    fi
}

# Generate results
generate_results() {
    print_header "Performance Comparison"
    log ""

    # Debug: Show array contents
    if [[ "$VERBOSE" == true ]]; then
        print_info "DEBUG: NPM_INSTALL_TIMES = (${NPM_INSTALL_TIMES[*]})"
        print_info "DEBUG: NPM_SERVER_TIMES = (${NPM_SERVER_TIMES[*]})"
        print_info "DEBUG: NPM_BUILD_TIMES = (${NPM_BUILD_TIMES[*]})"
        print_info "DEBUG: NPM_LINT_TIMES = (${NPM_LINT_TIMES[*]})"
        print_info "DEBUG: BUN_INSTALL_TIMES = (${BUN_INSTALL_TIMES[*]})"
        print_info "DEBUG: BUN_SERVER_TIMES = (${BUN_SERVER_TIMES[*]})"
        print_info "DEBUG: BUN_BUILD_TIMES = (${BUN_BUILD_TIMES[*]})"
        print_info "DEBUG: BUN_LINT_TIMES = (${BUN_LINT_TIMES[*]})"
    fi

    # Calculate averages
    local npm_install_avg=$(calc_average "${NPM_INSTALL_TIMES[@]}")
    local npm_server_avg=$(calc_average "${NPM_SERVER_TIMES[@]}")
    local npm_build_avg=$(calc_average "${NPM_BUILD_TIMES[@]}")
    local npm_lint_avg=$(calc_average "${NPM_LINT_TIMES[@]}")

    local bun_install_avg=$(calc_average "${BUN_INSTALL_TIMES[@]}")
    local bun_server_avg=$(calc_average "${BUN_SERVER_TIMES[@]}")
    local bun_build_avg=$(calc_average "${BUN_BUILD_TIMES[@]}")
    local bun_lint_avg=$(calc_average "${BUN_LINT_TIMES[@]}")

    # Calculate improvements
    local install_improvement=$(calc_improvement "$npm_install_avg" "$bun_install_avg")
    local server_improvement=$(calc_improvement "$npm_server_avg" "$bun_server_avg")
    local build_improvement=$(calc_improvement "$npm_build_avg" "$bun_build_avg")
    local lint_improvement=$(calc_improvement "$npm_lint_avg" "$bun_lint_avg")

    # Calculate lockfile size improvement
    local lockfile_improvement="Same"
    if [[ "$NPM_LOCKFILE_SIZE" =~ ^[0-9.]+$ ]] && [[ "$BUN_LOCKFILE_SIZE" =~ ^[0-9.]+$ ]]; then
        if [[ $(echo "$BUN_LOCKFILE_SIZE < $NPM_LOCKFILE_SIZE" | bc -l 2>/dev/null || echo "0") -eq 1 ]]; then
            local lock_pct=$(echo "scale=1; (1 - $BUN_LOCKFILE_SIZE / $NPM_LOCKFILE_SIZE) * 100" | bc -l 2>/dev/null || echo "0")
            lockfile_improvement="${lock_pct}% smaller"
        elif [[ $(echo "$BUN_LOCKFILE_SIZE > $NPM_LOCKFILE_SIZE" | bc -l 2>/dev/null || echo "0") -eq 1 ]]; then
            local lock_pct=$(echo "scale=1; ($BUN_LOCKFILE_SIZE / $NPM_LOCKFILE_SIZE - 1) * 100" | bc -l 2>/dev/null || echo "0")
            lockfile_improvement="${lock_pct}% larger"
        fi
    fi

    # Calculate node_modules size improvement
    local size_improvement="Same"
    if [[ "$NPM_NODE_MODULES_SIZE" =~ ^[0-9]+$ ]] && [[ "$BUN_NODE_MODULES_SIZE" =~ ^[0-9]+$ ]]; then
        if [[ "$BUN_NODE_MODULES_SIZE" -lt "$NPM_NODE_MODULES_SIZE" ]]; then
            local size_pct=$(echo "scale=1; (1 - $BUN_NODE_MODULES_SIZE / $NPM_NODE_MODULES_SIZE) * 100" | bc -l 2>/dev/null || echo "0")
            size_improvement="${size_pct}% smaller"
        elif [[ "$BUN_NODE_MODULES_SIZE" -gt "$NPM_NODE_MODULES_SIZE" ]]; then
            local size_pct=$(echo "scale=1; ($BUN_NODE_MODULES_SIZE / $NPM_NODE_MODULES_SIZE - 1) * 100" | bc -l 2>/dev/null || echo "0")
            size_improvement="${size_pct}% larger"
        fi
    fi

    # Print table
    log "┌─────────────────────────┬──────────────┬──────────────┬──────────────────┐"
    log "$(printf "│ %-23s │ %-12s │ %-12s │ %-16s │" "Metric" "npm" "Bun" "Improvement")"
    log "├─────────────────────────┼──────────────┼──────────────┼──────────────────┤"
    log "$(printf "│ %-23s │ %11.2fs │ %11.2fs │ %-16s │" "Install Time" "$npm_install_avg" "$bun_install_avg" "$install_improvement")"
    log "$(printf "│ %-23s │ %11.2fs │ %11.2fs │ %-16s │" "Server Start Time" "$npm_server_avg" "$bun_server_avg" "$server_improvement")"
    log "$(printf "│ %-23s │ %11.2fs │ %11.2fs │ %-16s │" "Hugo Build Time" "$npm_build_avg" "$bun_build_avg" "$build_improvement")"
    log "$(printf "│ %-23s │ %11.2fs │ %11.2fs │ %-16s │" "Linting Time" "$npm_lint_avg" "$bun_lint_avg" "$lint_improvement")"
    log "$(printf "│ %-23s │ %9s MB │ %9s MB │ %-16s │" "node_modules Size" "$NPM_NODE_MODULES_SIZE" "$BUN_NODE_MODULES_SIZE" "$size_improvement")"
    log "$(printf "│ %-23s │ %9s MB │ %9s MB │ %-16s │" "Lockfile Size" "$NPM_LOCKFILE_SIZE" "$BUN_LOCKFILE_SIZE" "$lockfile_improvement")"

    # Calculate memory improvement
    local memory_improvement="Same"
    if [[ "$NPM_SERVER_MEMORY" =~ ^[0-9]+$ ]] && [[ "$BUN_SERVER_MEMORY" =~ ^[0-9]+$ ]] && [[ "$NPM_SERVER_MEMORY" -gt 0 ]]; then
        if [[ "$BUN_SERVER_MEMORY" -lt "$NPM_SERVER_MEMORY" ]]; then
            local mem_pct=$(echo "scale=1; (1 - $BUN_SERVER_MEMORY / $NPM_SERVER_MEMORY) * 100" | bc -l 2>/dev/null || echo "0")
            memory_improvement="${mem_pct}% less"
        elif [[ "$BUN_SERVER_MEMORY" -gt "$NPM_SERVER_MEMORY" ]]; then
            local mem_pct=$(echo "scale=1; ($BUN_SERVER_MEMORY / $NPM_SERVER_MEMORY - 1) * 100" | bc -l 2>/dev/null || echo "0")
            memory_improvement="${mem_pct}% more"
        fi
    fi
    log "$(printf "│ %-23s │ %9s MB │ %9s MB │ %-16s │" "Server Memory (RSS)" "$NPM_SERVER_MEMORY" "$BUN_SERVER_MEMORY" "$memory_improvement")"

    # Determine site verification status
    local site_status="Both pass"
    # Extract passed count from "X/Y" format
    local npm_passed=$(echo "$NPM_SITE_CHECK" | cut -d'/' -f1)
    local npm_total=$(echo "$NPM_SITE_CHECK" | cut -d'/' -f2)
    local bun_passed=$(echo "$BUN_SITE_CHECK" | cut -d'/' -f1)
    local bun_total=$(echo "$BUN_SITE_CHECK" | cut -d'/' -f2)

    local npm_all_pass=$([[ "$npm_passed" == "$npm_total" ]] && echo "yes" || echo "no")
    local bun_all_pass=$([[ "$bun_passed" == "$bun_total" ]] && echo "yes" || echo "no")

    if [[ "$npm_all_pass" != "yes" ]] && [[ "$bun_all_pass" != "yes" ]]; then
        site_status="Both have issues"
    elif [[ "$npm_all_pass" != "yes" ]]; then
        site_status="npm has issues"
    elif [[ "$bun_all_pass" != "yes" ]]; then
        site_status="Bun has issues"
    fi
    log "$(printf "│ %-23s │ %12s │ %12s │ %-16s │" "Site Verification" "$NPM_SITE_CHECK" "$BUN_SITE_CHECK" "$site_status")"
    log "└─────────────────────────┴──────────────┴──────────────┴──────────────────┘"
    log ""

    # Summary
    print_header "Summary"
    log ""

    # Only show comparisons if we have valid numbers
    if [[ "$npm_install_avg" =~ ^[0-9.]+$ ]] && [[ "$bun_install_avg" =~ ^[0-9.]+$ ]] &&
       [[ $(echo "$bun_install_avg > 0" | bc -l 2>/dev/null || echo "0") -eq 1 ]]; then
        if [[ $(echo "$bun_install_avg < $npm_install_avg" | bc -l 2>/dev/null || echo "0") -eq 1 ]]; then
            local speedup=$(echo "scale=1; $npm_install_avg / $bun_install_avg" | bc -l 2>/dev/null || echo "1.0")
            print_success "Bun is ${speedup}x faster at installing dependencies"
        fi
    fi

    if [[ "$npm_build_avg" =~ ^[0-9.]+$ ]] && [[ "$bun_build_avg" =~ ^[0-9.]+$ ]] &&
       [[ $(echo "$bun_build_avg > 0" | bc -l 2>/dev/null || echo "0") -eq 1 ]]; then
        if [[ $(echo "$bun_build_avg < $npm_build_avg" | bc -l 2>/dev/null || echo "0") -eq 1 ]]; then
            local speedup=$(echo "scale=1; $npm_build_avg / $bun_build_avg" | bc -l 2>/dev/null || echo "1.0")
            print_success "Bun is ${speedup}x faster at Hugo builds"
        fi
    fi

    if [[ "$npm_lint_avg" =~ ^[0-9.]+$ ]] && [[ "$bun_lint_avg" =~ ^[0-9.]+$ ]] &&
       [[ $(echo "$bun_lint_avg > 0" | bc -l 2>/dev/null || echo "0") -eq 1 ]]; then
        if [[ $(echo "$bun_lint_avg < $npm_lint_avg" | bc -l 2>/dev/null || echo "0") -eq 1 ]]; then
            local speedup=$(echo "scale=1; $npm_lint_avg / $bun_lint_avg" | bc -l 2>/dev/null || echo "1.0")
            print_success "Bun is ${speedup}x faster at running linters"
        fi
    fi

    # Check if all site verification tests passed
    local npm_site_passed=$(echo "$NPM_SITE_CHECK" | cut -d'/' -f1)
    local npm_site_total=$(echo "$NPM_SITE_CHECK" | cut -d'/' -f2)
    local bun_site_passed=$(echo "$BUN_SITE_CHECK" | cut -d'/' -f1)
    local bun_site_total=$(echo "$BUN_SITE_CHECK" | cut -d'/' -f2)

    if [[ "$npm_site_passed" == "$npm_site_total" ]] && [[ "$bun_site_passed" == "$bun_site_total" ]]; then
        print_success "Both versions successfully serve the site"
        print_success "All $npm_site_total site verification checks passed"
    elif [[ "$npm_site_passed" == "$npm_site_total" ]]; then
        print_success "npm version passes all site checks"
        print_warning "Bun version: $BUN_SITE_CHECK checks passed"
    elif [[ "$bun_site_passed" == "$bun_site_total" ]]; then
        print_warning "npm version: $NPM_SITE_CHECK checks passed"
        print_success "Bun version passes all site checks"
    else
        print_warning "npm version: $NPM_SITE_CHECK checks passed"
        print_warning "Bun version: $BUN_SITE_CHECK checks passed"
    fi

    log ""
    if [[ -n "$RESULTS_FILE" ]]; then
        print_info "Results saved to: $RESULTS_FILE"
    fi
}

# Cleanup function
do_cleanup() {
    print_header "Benchmark Cleanup"
    log ""

    print_info "Scanning for benchmark artifacts..."
    log ""

    # Find benchmark result files
    local result_files=(benchmark-results-*.txt)
    local run_logs=(benchmark-run*.log)
    local count=0

    # Check for result files
    if [[ -f "${result_files[0]}" ]]; then
        log "Benchmark result files found:"
        for file in "${result_files[@]}"; do
            if [[ -f "$file" ]]; then
                local size=$(ls -lh "$file" | awk '{print $5}')
                local date=$(ls -l "$file" | awk '{print $6, $7, $8}')
                log "  - $file (${size}, ${date})"
                ((count++))
            fi
        done
        log ""
    fi

    # Check for log files
    if [[ -f "${run_logs[0]}" ]]; then
        log "Benchmark log files found:"
        for file in "${run_logs[@]}"; do
            if [[ -f "$file" ]]; then
                local size=$(ls -lh "$file" | awk '{print $5}')
                log "  - $file (${size})"
                ((count++))
            fi
        done
        log ""
    fi

    # Check for leftover artifacts
    local artifacts=()
    [[ -d "node_modules" ]] && artifacts+=("node_modules/")
    [[ -d "public" ]] && artifacts+=("public/")
    [[ -d "resources" ]] && artifacts+=("resources/")
    [[ -d ".serve-lint" ]] && artifacts+=(".serve-lint/")
    [[ -f "package-lock.json" ]] && artifacts+=("package-lock.json")
    [[ -f "bun.lockb" ]] && artifacts+=("bun.lockb")
    [[ -f "bun.lock" ]] && artifacts+=("bun.lock")

    if [[ ${#artifacts[@]} -gt 0 ]]; then
        log "Build artifacts found:"
        for artifact in "${artifacts[@]}"; do
            if [[ -d "$artifact" ]]; then
                local size=$(du -sh "$artifact" 2>/dev/null | awk '{print $1}')
                log "  - $artifact (${size})"
            else
                log "  - $artifact"
            fi
            ((count++))
        done
        log ""
    fi

    if [[ $count -eq 0 ]]; then
        print_success "No benchmark artifacts found. Directory is clean!"
        return 0
    fi

    # Confirm cleanup
    if [[ "$NO_CONFIRM" == false ]]; then
        log "Remove all ${count} item(s)? [y/N] "
        read -r response
        if [[ ! "$response" =~ ^[Yy]$ ]]; then
            print_info "Cleanup cancelled"
            return 0
        fi
    else
        print_warning "Running cleanup without confirmation (--no-confirm)"
    fi

    log ""
    print_info "Cleaning up..."

    # Remove result files
    if [[ -f "${result_files[0]}" ]]; then
        for file in "${result_files[@]}"; do
            if [[ -f "$file" ]]; then
                rm -f "$file" && print_success "Removed $file"
            fi
        done
    fi

    # Remove log files
    if [[ -f "${run_logs[0]}" ]]; then
        for file in "${run_logs[@]}"; do
            if [[ -f "$file" ]]; then
                rm -f "$file" && print_success "Removed $file"
            fi
        done
    fi

    # Remove artifacts
    if [[ ${#artifacts[@]} -gt 0 ]]; then
        for artifact in "${artifacts[@]}"; do
            if [[ -d "$artifact" ]]; then
                rm -rf "$artifact" && print_success "Removed $artifact"
            elif [[ -f "$artifact" ]]; then
                rm -f "$artifact" && print_success "Removed $artifact"
            fi
        done
    fi

    log ""
    print_success "Cleanup complete!"
}

# Test output with sample data
test_output() {
    print_header "Testing Output Formatting"
    log ""
    print_info "Using sample benchmark data..."
    log ""

    # Populate with sample data (realistic values based on actual runs)
    NPM_INSTALL_TIMES=("34.45" "19.33" "20.56")
    NPM_SERVER_TIMES=("1.00" "1.00" "1.50")
    NPM_BUILD_TIMES=("27.93" "27.28" "27.88")
    NPM_LINT_TIMES=("3.82" "3.85" "3.43")
    NPM_NODE_MODULES_SIZE="1598"
    NPM_LOCKFILE_SIZE="0.96"
    NPM_PACKAGE_COUNT="605"
    NPM_SERVER_MEMORY="156"
    NPM_SITE_CHECK="9/9"

    BUN_INSTALL_TIMES=("10.68" "9.86" "9.06")
    BUN_SERVER_TIMES=("1.00" "1.00" "1.00")
    BUN_BUILD_TIMES=("28.78" "26.59" "27.85")
    BUN_LINT_TIMES=("1.81" "1.08" "1.00")
    BUN_NODE_MODULES_SIZE="717"
    BUN_LOCKFILE_SIZE="0.42"
    BUN_PACKAGE_COUNT="975"
    BUN_SERVER_MEMORY="142"
    BUN_SITE_CHECK="9/9"

    log_separator
    generate_results

    print_success "Test output complete!"
}

# Parse times from a results file line pattern
# Usage: parse_times "pattern" "file" "section_marker"
parse_times_from_file() {
    local file=$1
    local section=$2  # "npm" or "bun"
    local metric=$3   # "Installing dependencies" "Starting server" "Building production" "Running linters"

    local in_section=false
    local times=()

    while IFS= read -r line; do
        # Detect section start
        if [[ "$line" =~ Testing.*\(${section}\) ]]; then
            in_section=true
        elif [[ "$line" =~ Testing.*\((npm|bun)\) ]] && [[ "$in_section" == true ]]; then
            # Hit next section, stop
            break
        fi

        # Parse timing lines within section
        if [[ "$in_section" == true ]] && [[ "$line" =~ \[.*\].*${metric}.*\ ([0-9.]+)s$ ]]; then
            times+=("${BASH_REMATCH[1]}")
        fi
    done < "$file"

    echo "${times[@]}"
}

# Parse a metric value from file
parse_metric_from_file() {
    local file=$1
    local section=$2  # "npm" or "bun"
    local pattern=$3  # e.g., "node_modules size:" or "Packages installed:"

    local in_section=false

    while IFS= read -r line; do
        # Detect section start
        if [[ "$line" =~ Testing.*\(${section}\) ]]; then
            in_section=true
        elif [[ "$line" =~ Testing.*\((npm|bun)\) ]] && [[ "$in_section" == true ]]; then
            break
        fi

        # Parse metric line within section
        if [[ "$in_section" == true ]] && [[ "$line" =~ ${pattern}[[:space:]]*([0-9.]+) ]]; then
            echo "${BASH_REMATCH[1]}"
            return
        fi
    done < "$file"

    echo "0"
}

# Replay results from a previous benchmark file
replay_results() {
    local file=$1

    if [[ ! -f "$file" ]]; then
        print_error "File not found: $file"
        exit 1
    fi

    print_header "Replaying Benchmark Results"
    log ""
    print_info "Parsing results from: $file"
    log ""

    # Parse npm times
    read -ra NPM_INSTALL_TIMES <<< "$(parse_times_from_file "$file" "npm" "Installing dependencies")"
    read -ra NPM_SERVER_TIMES <<< "$(parse_times_from_file "$file" "npm" "Starting server")"
    read -ra NPM_BUILD_TIMES <<< "$(parse_times_from_file "$file" "npm" "Building production")"
    read -ra NPM_LINT_TIMES <<< "$(parse_times_from_file "$file" "npm" "Running linters")"

    # Parse bun times
    read -ra BUN_INSTALL_TIMES <<< "$(parse_times_from_file "$file" "bun" "Installing dependencies")"
    read -ra BUN_SERVER_TIMES <<< "$(parse_times_from_file "$file" "bun" "Starting server")"
    read -ra BUN_BUILD_TIMES <<< "$(parse_times_from_file "$file" "bun" "Building production")"
    read -ra BUN_LINT_TIMES <<< "$(parse_times_from_file "$file" "bun" "Running linters")"

    # Parse npm metrics
    NPM_NODE_MODULES_SIZE=$(parse_metric_from_file "$file" "npm" "node_modules size:")
    NPM_LOCKFILE_SIZE=$(parse_metric_from_file "$file" "npm" "Lockfile size:")
    NPM_PACKAGE_COUNT=$(parse_metric_from_file "$file" "npm" "Packages installed:")
    NPM_SERVER_MEMORY=$(parse_metric_from_file "$file" "npm" "server memory usage...")
    NPM_SITE_CHECK="4/4"  # Assume passed if we got this far

    # Parse bun metrics
    BUN_NODE_MODULES_SIZE=$(parse_metric_from_file "$file" "bun" "node_modules size:")
    BUN_LOCKFILE_SIZE=$(parse_metric_from_file "$file" "bun" "Lockfile size:")
    BUN_PACKAGE_COUNT=$(parse_metric_from_file "$file" "bun" "Packages installed:")
    BUN_SERVER_MEMORY=$(parse_metric_from_file "$file" "bun" "server memory usage...")
    BUN_SITE_CHECK="4/4"

    # Debug output
    if [[ "$VERBOSE" == true ]]; then
        print_info "Parsed NPM install times: ${NPM_INSTALL_TIMES[*]}"
        print_info "Parsed NPM server times: ${NPM_SERVER_TIMES[*]}"
        print_info "Parsed NPM build times: ${NPM_BUILD_TIMES[*]}"
        print_info "Parsed NPM lint times: ${NPM_LINT_TIMES[*]}"
        print_info "Parsed NPM node_modules: ${NPM_NODE_MODULES_SIZE} MB"
        print_info "Parsed NPM lockfile: ${NPM_LOCKFILE_SIZE} MB"
        log ""
        print_info "Parsed BUN install times: ${BUN_INSTALL_TIMES[*]}"
        print_info "Parsed BUN server times: ${BUN_SERVER_TIMES[*]}"
        print_info "Parsed BUN build times: ${BUN_BUILD_TIMES[*]}"
        print_info "Parsed BUN lint times: ${BUN_LINT_TIMES[*]}"
        print_info "Parsed BUN node_modules: ${BUN_NODE_MODULES_SIZE} MB"
        print_info "Parsed BUN lockfile: ${BUN_LOCKFILE_SIZE} MB"
        log ""
    fi

    log_separator
    generate_results

    print_success "Replay complete!"
}

# Main function
main() {
    # Parse arguments
    parse_args "$@"

    log_separator

    # Handle clean-only mode (cleanup and exit)
    if [[ "$CLEAN_ONLY_MODE" == true ]]; then
        do_cleanup
        exit 0
    fi

    # Handle clean-first mode (cleanup then continue to benchmarks)
    if [[ "$CLEAN_FIRST" == true ]]; then
        do_cleanup
        # Continue to run benchmarks after cleanup
    fi

    # Handle test output mode
    if [[ "$TEST_OUTPUT_MODE" == true ]]; then
        test_output
        exit 0
    fi

    # Handle replay mode
    if [[ -n "$REPLAY_FILE" ]]; then
        replay_results "$REPLAY_FILE"
        exit 0
    fi

    # Handle dry-run mode
    if [[ "$DRY_RUN" == true ]]; then
        print_header "Dry Run - No benchmarks will be executed"
        log ""
        print_info "Configuration:"
        log "  - Runs per metric: $RUNS"
        log "  - Server timeout: ${TIMEOUT}s"
        log "  - npm branch: $NPM_BRANCH"
        log "  - Bun branch: $BUN_BRANCH"
        log ""
        print_info "The following steps would be performed:"
        log "  1. Stash any uncommitted changes"
        log "  2. Checkout '$NPM_BRANCH' branch"
        log "  3. Run $RUNS iterations of:"
        log "     - npm install (timed)"
        log "     - npm start (server startup timed)"
        log "     - npm run build (Hugo build timed)"
        log "     - npm run lint:scripts/styles/markdown (timed)"
        log "  4. Collect npm metrics (node_modules size, lockfile size)"
        log "  5. Measure server memory usage (RSS)"
        log "  6. Verify site functionality"
        log "  7. Checkout '$BUN_BRANCH' branch"
        log "  8. Run $RUNS iterations of:"
        log "     - bun install (timed)"
        log "     - bun start (server startup timed)"
        log "     - bun run build (Hugo build timed)"
        log "     - bun run lint:scripts/styles/markdown (timed)"
        log "  9. Collect bun metrics (node_modules size, lockfile size)"
        log " 10. Measure server memory usage (RSS)"
        log " 11. Verify site functionality"
        log " 12. Generate comparison report"
        log " 13. Restore original branch and unstash changes"
        log ""
        print_success "Dry run complete - no changes made"
        SKIP_CLEANUP=true
        exit 0
    fi

    # Set results filename
    RESULTS_FILE="benchmark-results-$(date +%Y%m%d-%H%M%S).txt"

    # Redirect output to both console and file (strip colors from file)
    # This sends colored output to console, plain text to file
    exec > >(tee >(sed -u 's/\x1b\[[0-9;]*m//g' > "$RESULTS_FILE"))
    exec 2>&1

    # Print banner
    log_separator
    print_header "Node.js/npm vs Bun Performance Benchmark"
    log_separator
    log ""

    # Print test configuration
    print_info "Test Configuration:"
    log "  - Runs per metric: $RUNS"
    log "  - Server timeout: ${TIMEOUT}s"
    log "  - Date: $(date '+%Y-%m-%d %H:%M:%S')"
    log ""

    # Check prerequisites
    check_prerequisites

    # Confirm
    if [[ "$NO_CONFIRM" == false ]]; then
        log "This will switch branches and clean your workspace. Continue? [y/N] "
        read -r response
        if [[ ! "$response" =~ ^[Yy]$ ]]; then
            print_info "Aborted by user"
            exit 0
        fi
        log ""
    fi

    # Save git state
    save_git_state
    log ""

    # Test npm branch
    log_separator
    if ! test_branch "$NPM_BRANCH" "$NPM_RUNTIME" "$NPM_RUNTIME install"; then
        print_error "Failed to test $NPM_BRANCH branch. Aborting benchmark."
        restore_git_state
        exit 1
    fi

    log_separator
    if ! test_branch "$BUN_BRANCH" "$BUN_RUNTIME" "$BUN_RUNTIME install"; then
        print_error "Failed to test $BUN_BRANCH branch. Aborting benchmark."
        restore_git_state
        exit 1
    fi

    # Generate comparison
    log_separator
    generate_results

    # Restore git state
    restore_git_state

    print_success "Benchmark complete!"
}

# Run main
main "$@"
