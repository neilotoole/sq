#!/bin/bash

# -----------------------------------------------------------------------------------------------------------
# Script Name: log.bash
# Version: 1.8
#
# Description: A collection of logging utility functions for bash scripts that
#              provide colored and formatted console output.
#
# Functions:
#   get_terminal_width : Get current terminal width (max 120 chars)
#   log                : Basic white text logging
#   log_dim            : Dimmed white text logging
#   log_info           : Blue text for info messages
#   log_info_dim       : Dimmed blue text for info messages
#   log_success        : Green checkmark with dimmed green text for success messages
#   log_error          : Red X symbol with dimmed red text for error messages
#   log_warning        : Yellow triangle with dimmed yellow text for warnings
#   log_indent          : Indent (2 spaces) and call any log function (usage: log_indent log_success "message")
#   log_separator      : Print a separator line across terminal width
#   log_centered       : Print centered text
#   log_verbose        : Log message only if VERBOSE environment variable is true
#   log_banner         : Display ASCII art banner with colored text
# -----------------------------------------------------------------------------------------------------------
# Usage Example: source scripts/log.bash
# -----------------------------------------------------------------------------------------------------------

# -----------------------------------------------------------------------------------------------------------
# Global variables
# -----------------------------------------------------------------------------------------------------------
VERBOSE=false

# -----------------------------------------------------------------------------------------------------------
# ANSI color codes
# -----------------------------------------------------------------------------------------------------------
RED='\033[91m'
GREEN='\033[92m'
YELLOW='\033[93m'
BLUE='\033[94m'
WHITE='\033[97m'
RESET='\033[0m'
DIM='\033[2m'


# -----------------------------------------------------------------------------------------------------------
# Function: get_terminal_width
# Description: Get current terminal width (max 120 chars)
# -----------------------------------------------------------------------------------------------------------
get_terminal_width() {
    local width=80

    # Non-interactive shells (for example CI) may not have TERM set or a TTY.
    # In that case, avoid tput errors and use a safe default.
    if [[ -n "${TERM:-}" ]] && [[ -t 1 ]] && command -v tput >/dev/null 2>&1; then
        width=$(tput cols 2>/dev/null || echo "")
    fi

    if ! [[ "$width" =~ ^[0-9]+$ ]]; then
        width=80
    fi

    if [ "$width" -gt 120 ]; then
        width=120
    fi
    echo "$width"
}

# -----------------------------------------------------------------------------------------------------------
# Basic Logging functions for consistent console output
# -----------------------------------------------------------------------------------------------------------

log() {
    echo -e "${WHITE} $1${RESET}"
}

log_dim() {
    echo -e "${DIM}${WHITE} $1${RESET}"
}

log_info() {
    echo -e "${BLUE} $1${RESET}"
}

log_info_dim() {
    echo -e "${DIM}${BLUE} $1${RESET}"
}

log_success() {
    echo -e " ${GREEN}✔${RESET} ${DIM}${GREEN} $1${RESET}"
}

log_error() {
    echo -e " ${RED}🅇${RESET}  ${DIM}${RED}$1${RESET}"
}

log_warning() {
    echo -e " ${YELLOW}▲${RESET}  ${DIM}${YELLOW}$1${RESET}"
}

# -----------------------------------------------------------------------------------------------------------
# Function: log_separator
# Description: Print a separator line across terminal width
# -----------------------------------------------------------------------------------------------------------
log_separator() {
    local terminal_width
    terminal_width=$(get_terminal_width)
    printf "=-%.0s" $(seq 1 $((terminal_width / 2)))
    echo "="
}

# -----------------------------------------------------------------------------------------------------------
# Function: log_indent
# Description: Indent (2 spaces) and call any log function (usage: log_indent log_success "message")
# -----------------------------------------------------------------------------------------------------------
log_indent() {
    local log_func=$1
    shift
    printf "  "
    $log_func "$@"
}

# -----------------------------------------------------------------------------------------------------------
# Function: log_centered
# Description: Center a message in the terminal.
# -----------------------------------------------------------------------------------------------------------
log_centered() {
    local terminal_width
    local message="$1"
    terminal_width=$(get_terminal_width)

    # Calculate padding
    local padding=$(((terminal_width - ${#message}) / 2))

    # Create padding string
    local pad_str
    pad_str=$(printf '%*s' "$padding" '')

    # Print centered message
    echo -e "${pad_str}${message}"
}

# -----------------------------------------------------------------------------------------------------------
# Function: log_verbose
# Description: Log message only if VERBOSE environment variable is true
# -----------------------------------------------------------------------------------------------------------
log_verbose() {
    if [[ "${VERBOSE:-false}" == "true" ]]; then
        log_info_dim "$*"
    fi
}

# -----------------------------------------------------------------------------------------------------------
# Function: log_banner
# Description: Show script banner. A bit of je ne sais quoi for your project.
#
# To generate your project name logo use this site:
#   https://patorjk.com/software/taag/#p=display&f=ANSI+Shadow&t=Code+Red
#
# -----------------------------------------------------------------------------------------------------------
log_banner() {

    echo -e "
    ${DIM}${YELLOW}███████╗ ██████╗     ██╗    ██╗███████╗██████╗
    ${DIM}${YELLOW}██╔════╝██╔═══██╗    ██║    ██║██╔════╝██╔══██╗
    ${DIM}${YELLOW}███████╗██║   ██║    ██║ █╗ ██║█████╗  ██████╔╝
    ${DIM}${YELLOW}╚════██║██║▄▄ ██║    ██║███╗██║██╔══╝  ██╔══██╗
    ${DIM}${YELLOW}███████║╚██████╔╝    ╚███╔███╔╝███████╗██████╔╝
    ${DIM}${YELLOW}╚══════╝ ╚══▀▀═╝      ╚══╝╚══╝ ╚══════╝╚═════╝
    ${RESET}"
}

# -----------------------------------------------------------------------------------------------------------
# Example usage
# -----------------------------------------------------------------------------------------------------------
if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
    log_separator
    log_banner
    log "This is a normal message."
    log_dim "This is a dim message."
    log_info "This is an info message."
    log_info_dim "This is a dim info message."
    log_success "This is a success message."
    log_warning "This is a warning message."
    log_error "This is an error message."
    log_indent log_success "This is an indented message."
    log_centered "This is a centered message"
    log_separator
fi
