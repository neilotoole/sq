#!/bin/bash

# -----------------------------------------------------------------------------------------------------------
# Script Name: checkenv.bash
# Version: 1.8.1
#
# This script checks for the existence of .env files and ensures that all required environment variables are set.
#
# For each file, check the following:
#
#  1. If the .env file doesn't exist, it creates one from .env.example
#  2. Checks that all required variables from .env.example are present
#  3. Checks that all variables have values
#
#  The script also supports a --merge option that allows you to merge
#  existing .env files with .env.example files.
#
#  When using --merge, the script will create a new file (e.g., .env.new) with the following:
#  - Existing values in your .env file will be preserved
#  - Missing variables will be filled with values from .env.example
#  - Empty variables in your .env will be filled with values from .env.example
#  - Comments and formatting from .env.example are preserved
#
#  Note: when using --merge, no user interaction is required. Useful for CI/CD pipelines.
#
# -----------------------------------------------------------------------------------------------------------
# Usage:
#   ./checkenv.bash [OPTIONS] [env_file_path1] [env_file_path2] ...
#
# Arguments:
#   env_file_path   Path to .env file to check (default: .env)
#
# Options:
#   -h, --help      Show this help message and exit
#   -v, --verbose   Enable verbose output
#   --merge         Automatically create a .env.new file by merging existing .env with missing variables from .env.example
#
# Examples:
#   ./checkenv.bash                                    # Check only .env
#   ./checkenv.bash .env.test                          # Check only .env.test
#   ./checkenv.bash .env apps/frontend/.env            # Check both files in order
#   ./checkenv.bash --merge                            # Merge .env with .env.example, output to .env.new
#   ./checkenv.bash --merge .env                       # Merge .env with .env.example, output to .env.new
#   ./checkenv.bash --verbose .env.test                # Verbose output while checking .env.test
# -----------------------------------------------------------------------------------------------------------

# shellcheck source=scripts/log.bash
# shellcheck disable=SC1091,SC2034
# shellcheck disable=SC1091
source "$(dirname "$0")/log.bash"

# -----------------------------------------------------------------------------------------------------------
# Global variables
# -----------------------------------------------------------------------------------------------------------
ENV_FILES=()
MERGE_MODE=false
VERBOSE=false

# -----------------------------------------------------------------------------------------------------------
# Function: show_help
# Description: Display help message and exit
# -----------------------------------------------------------------------------------------------------------
show_help() {
    log_separator
    log "${BLUE}Usage${RESET}: $0 [OPTIONS] [env_file_path1] [env_file_path2] ..."
    log ""
    log "${BLUE}Arguments${RESET}:"
    log "  env_file_path   ${DIM}Path to .env file to check (default: .env)${RESET}"
    log ""
    log "${BLUE}Options${RESET}:"
    log "  -h, --help      ${DIM}Show this help message and exit${RESET}"
    log "  -v, --verbose   ${DIM}Enable verbose output${RESET}"
    log "  --merge         ${DIM}Automatically create a .env.new file by merging existing .env with missing variables from .env.example${RESET}"
    log ""
    log "${BLUE}Examples${RESET}:"
    log_dim "  $0                                    # Check only .env"
    log_dim "  $0 .env.test                          # Check only .env.test"
    log_dim "  $0 .env apps/frontend/.env            # Check both files in order"
    log_dim "  $0 --merge                            # Merge .env with .env.example, output to .env.new"
    log_dim "  $0 --merge .env                       # Merge .env with .env.example, output to .env.new"
    log_dim "  $0 --verbose .env.test                # Verbose output while checking .env.test"
    log ""
    log "${BLUE}What this script does${RESET}:"
    log_dim "  1. Checks if .env files exist, creates them from .env.example if missing"
    log_dim "  2. Validates that all required variables from .env.example are present"
    log_dim "  3. Ensures all variables have non-empty values"
    log_dim "  4. In merge mode: syncs .env from .env.example, then validates (no prompts; for make check-env)"
}


# -----------------------------------------------------------------------------------------------------------
# Function: parse_arguments
# Description: Parse command line arguments
# -----------------------------------------------------------------------------------------------------------
parse_arguments() {
    while [[ $# -gt 0 ]]; do
        case $1 in
            -h|--help)
                show_help
                exit 0
                ;;
            -v|--verbose)
                VERBOSE=true
                shift
                ;;
            --merge)
                MERGE_MODE=true
                shift
                ;;
            -*)
                log_error "Unknown option $1"
                log ""
                log "Use --help for usage information"
                exit 1
                ;;
            *)
                ENV_FILES+=("$1")
                shift
                ;;
        esac
    done

    # If no files provided, default to .env
    if [[ ${#ENV_FILES[@]} -eq 0 ]]; then
        ENV_FILES=(.env)
    fi
}

# -----------------------------------------------------------------------------------------------------------
# Main functional methods
# -----------------------------------------------------------------------------------------------------------

# -----------------------------------------------------------------------------------------------------------
# Function: main
# Description: Main function that orchestrates the environment checking process
# -----------------------------------------------------------------------------------------------------------
main() {
    # Parse command line arguments first
    parse_arguments "$@"

    log_separator
    log_info "Check environment variables"

    # Process each file
    process_env_files
}

# -----------------------------------------------------------------------------------------------------------
# Function: process_env_files
# Description: Process all specified environment files
# -----------------------------------------------------------------------------------------------------------
process_env_files() {
    # Get fancy!
    formatted_files=""
    for file in "${ENV_FILES[@]}"; do
        if [ -n "$formatted_files" ]; then
            formatted_files="$formatted_files,"
        fi
        formatted_files="$formatted_files'$file'"
    done
    log "Checking environment variables for [${DIM}$formatted_files${RESET}]"

    # Process each file in order
    for env_file in "${ENV_FILES[@]}"; do
        log_indent log_verbose "Processing '$env_file'"

        # Protection: Never process .env.example as a target file
        if [ "$env_file" = ".env.example" ]; then
            log_error " Cannot process .env.example as a target file - this is a template file."
            log_error " Please specify a different target file (e.g., .env)."
            log_error " Skipping '$env_file'..."
            log
            continue
        fi

        if [ "$MERGE_MODE" = true ]; then
            # Merge mode: sync keys from .env.example, then validate (no prompts).
            merge_env_files "$env_file"
            check_env_file "$env_file" || exit 1
        else
            # Normal mode: check and validate environment file
            check_env_file "$env_file" || exit 1
        fi
    done
}

# -----------------------------------------------------------------------------------------------------------
# Function: check_env_file
# Description: Check and validate a single environment file
# -----------------------------------------------------------------------------------------------------------
check_env_file() {
    local env_file="$1"

    # Check if .env file exists, and if not attempt to create one.
    check_env_exists "$env_file"

    # Check if all environment variables are present
    if ! check_env_vars "$env_file"; then
        # If check failed and user didn't choose to merge, continue to next file
        return 1
    fi

    # Check if all environment variables have values
    if ! check_env_values "$env_file"; then
        # If check failed and user didn't choose to merge, continue to next file
        return 1
    fi

    log
}

# Function to get value for a variable from a file
get_var_value() {
  local file="$1"
  local var="$2"
  # Find the first line that starts with VAR=, ignore comments and blank lines, and extract everything after the first =
  grep -E "^${var}=" "$file" 2>/dev/null | head -n 1 | cut -d '=' -f 2-
}

# Function to get the example file name based on the env file name
get_example_file() {
  local env_file="$1"
  local env_dir
  local env_basename
  local example_file

  env_dir=$(dirname "$env_file")
  [ "$env_dir" = "." ] && env_dir="$(pwd)"
  env_basename=$(basename "$env_file")

  # Replace .env with .env.example, or add .example before the extension
  if [[ "$env_basename" =~ ^\.env$ ]]; then
    example_file="$env_dir/.env.example"
  else
    # For files like .env.tunnels, create .env.tunnels.example
    example_file="$env_dir/${env_basename}.example"
  fi

  echo "$example_file"
}

# Function to get complete value for a variable from a file, handling multi-line values
get_complete_var_value() {
  local file="$1"
  local var="$2"
  local value=""
  local line
  local in_var=false

  while IFS= read -r line || [ -n "$line" ]; do
    # Check if this line starts with our variable (using exact match to avoid issues with = in values)
    if [[ "$line" =~ ^${var}= ]]; then
      in_var=true
      # Extract everything after the first =
      value="${line#*=}"
      # If the line doesn't end with a backslash, this is a complete value
      if [[ "$line" != *"\\" ]]; then
        echo "$value"
        return 0
      fi
    elif [ "$in_var" = true ]; then
      # We're in a multi-line value, append this line
      if [[ "$line" =~ ^[[:space:]]*\\ ]]; then
        # Line starts with backslash, append without the backslash
        value="${value}${line#*\\}"
      else
        # No backslash, this is the end of the multi-line value
        value="${value}${line}"
        echo "$value"
        return 0
      fi
    fi
  done <"$file"

  # If we get here, return the value we have (might be empty)
  echo "$value"
}

# Function to check if a variable has a value in any of the processed files
get_cascaded_value() {
  local var="$1"
  local value
  local file

  # Check each file in order for a value
  for file in "${ENV_FILES[@]}"; do
    value=$(get_var_value "$file" "$var")
    if [ -n "$value" ]; then
      echo "$value"
      return 0
    fi
  done
  return 1
}

#
# Check if .env file exists, if not, create it from .env.example
#
check_env_exists() {
  local env_file="$1"
  local example_file

  example_file=$(get_example_file "$env_file")

  if [ ! -f "$env_file" ]; then
    if [ -f "$example_file" ]; then
      log_dim " Creating $env_file from $example_file..."
      cp "$example_file" "$env_file"
      log_dim " ${GREEN}✓${RESET} $env_file created successfully"
    else
      log_error "${DIM}$example_file${RESET} not found. Cannot create ${WHITE}${DIM}$env_file${RESET}"
      log "Please add ${DIM}.env.example${RESET} file with all required environment variables"
      exit 1
    fi
  else
    log_dim " ${GREEN}✓${RESET} ${DIM}$env_file${RESET} file already exists"
  fi
}

#
# Check for missing environment variables in .env file compared to .env.example
#
check_env_vars() {
  local env_file="$1"
  local example_file
  local missing_vars=""
  local var
  local value

  example_file=$(get_example_file "$env_file")

  if [ -f "$env_file" ]; then
    log_verbose " Checking if all environment variables are set..."

    if [ -f "$example_file" ]; then

      missing_vars=$(diff <(grep -v '^#' "$example_file" | grep -E '^[A-Za-z_][A-Za-z0-9_]*=' | sed 's/=.*//' | sort -u) <(grep -v '^#' "$env_file" | grep -E '^[A-Za-z_][A-Za-z0-9_]*=' | sed 's/=.*//' | sort -u) | grep '<' | sed 's/< //' | sort -u)

      # Check if there are any missing variables
      if [ -n "$missing_vars" ]; then
         log_indent log_warning " The following environment variables are missing, add them to your ${WHITE}${DIM}$env_file${YELLOW} file:"
        for var in $missing_vars; do
          log_indent log_warning " - $var"
        done

        if [ "$MERGE_MODE" = true ]; then
          log_verbose "Merge mode: syncing missing keys from ${DIM}$example_file${RESET}..."
          merge_env_files "$env_file"
          return 0
        fi

        # Ask user if they want to create a merged file
        log " " # Insert a blank line for readability
        log "Would you like to create a merged file with missing variables from ${DIM}$example_file${RESET} ?"
        log_dim "This will create $env_file.new with your existing values plus missing variables."
        read -p "Create merged file? (y/N): " -n 1 -r
        echo
        if [[ $REPLY =~ ^[Yy]$ ]]; then
          log_verbose "Creating merged file..."
          merge_env_files "$env_file"
          return 0
        else
          log_dim "Skipping merge. Please add the missing variables manually"
          return 1
        fi
      else
        log_dim " ${GREEN}✓${RESET} All required environment variables keys found"
        return 0
      fi
    else
       log_error "${DIM}$example_file${RESET} not found. Cannot compare for missing environment variables"
      return 1
    fi
  else
    log_error "${DIM}$env_file${RESET} file not found. Cannot check for missing environment variables"
    return 1
  fi
}

#
# Check for empty environment variables values
#
check_env_values() {
  local env_file="$1"
  local empty_vars=""
  local var
  local value
  local example_file

  example_file=$(get_example_file "$env_file")

  log_verbose " Checking if all environment variables have values..."

  # Check each line in the file
  while IFS= read -r line; do
    if [ -n "$line" ] && [ "${line#\#}" = "$line" ]; then
      var="${line%%=*}"
      value="${line#*=}"
      if [ -z "$value" ]; then
        empty_vars="$empty_vars $var"
      fi
    fi
  done <"$env_file"

  if [ -n "$empty_vars" ]; then
    log_warning " The following environment variables are empty in ${DIM}$env_file${RESET}${YELLOW} file:"
    for var in $empty_vars; do
      log_warning "  - $var"
    done

    if [ -f "$example_file" ]; then
      if [ "$MERGE_MODE" = true ]; then
        log_error "Can not continue with empty values in ${DIM}$env_file${RESET} (fill secrets; see ${DIM}$example_file${RESET})."
        return 1
      fi

      # Offer to merge values from .env.example (preserves any existing values the user has set)
      log
      log_info "Would you like to fill empty variables with values from ${DIM}$example_file${RESET}${BLUE}?"
      log_info "This will create ${DIM}$env_file.new${RESET}${BLUE} — your existing values will not be overwritten."
      read -p "Create merged file? (y/N/i=ignore): " -n 1 -r
      echo
      if [[ $REPLY =~ ^[Yy]$ ]]; then
        log_verbose "Creating merged file..."
        merge_env_files "$env_file"
        return 0
      elif [[ $REPLY =~ ^[Ii]$ ]]; then
        log_dim "Ignoring empty environment variables and continuing"
        return 0
      else
        log_dim "Skipping merge. Please add values for the empty variables manually"
        log_error "Can not continue with empty values in $env_file file."
        return 1
      fi
    else
      log_error "Can not continue with empty values in $env_file file."
      return 1
    fi
  else
    log_dim " ${GREEN}✓${RESET} All required environment variables have values"
    return 0
  fi
}

#
# Smart merge existing .env with .env.example preserving comments and formatting
#
merge_env_files() {
  local env_file="$1"
  local example_file
  local output_file="${env_file}.new"
  local var
  local value
  local example_value

  example_file=$(get_example_file "$env_file")

  # Protection: Never overwrite .env.example
  if [ "$env_file" = ".env.example" ] || [ "$env_file" = "$example_file" ]; then
    log_error "Cannot merge .env.example file - this would overwrite the template."
    log_error " Please specify a different target file (e.g., .env)."
    return 1
  fi

  if [ ! -f "$env_file" ]; then
     log_error "${DIM}$env_file${RESET} not found. Cannot merge files."
    return 1
  fi

  if [ ! -f "$example_file" ]; then
    log_error "${DIM}$example_file${RESET} not found. Cannot merge files."
    return 1
  fi

  log_dim " Smart merging $env_file with $example_file..."
  log_dim " Output will be written to $output_file"
  log_dim " Preserving comments, blank lines, and formatting..."

  # Clear output file
  : >"$output_file"

  # Read .env.example line by line and process each line
  # Use a different approach to handle files that might not end with newline
  while IFS= read -r line || [ -n "$line" ]; do
    # Check if this is a variable assignment line (starts with VAR=)
    # Use a more precise pattern to extract variable name before the first =
    if [[ "$line" =~ ^[A-Za-z_][A-Za-z0-9_]*= ]]; then
      # Extract variable name - everything before the first = sign
      var="${line%%=*}"

      # Get complete value from .env.example (handling multi-line values)
      example_value=$(get_complete_var_value "$example_file" "$var")

      # Check if this variable exists in .env and has a non-empty value
      if grep -q "^${var}=" "$env_file"; then
        # Variable exists in .env, get its value
        value=$(get_complete_var_value "$env_file" "$var")
        # Redact sensitive variable values in log output (*_TOKEN, *_KEY, *_SECRET).
        case "$var" in
          *_TOKEN | *_KEY | *_SECRET) display_value="[redacted]" ;;
          *) display_value="$value" ;;
        esac
        case "$var" in
          *_TOKEN | *_KEY | *_SECRET) display_example="[redacted]" ;;
          *) display_example="$example_value" ;;
        esac
        # Check if the value is empty
        if [ -n "$value" ]; then
          # Variable has a non-empty value, use that value
          log_dim "  ${GREEN}✓${RESET} $var ${DIM}(using value ${display_value} from $env_file)"
          echo "$var=$value" >>"$output_file"
        else
          # Variable exists but has empty value, use value from .env.example
          log_dim "  ${YELLOW}~${RESET} $var ${DIM}(empty in $env_file, using ${display_example} from $example_file)"
          echo "$var=$example_value" >>"$output_file"
        fi
      else
        # Variable doesn't exist in .env, use value from .env.example
        case "$var" in
          *_TOKEN | *_KEY | *_SECRET) display_example="[redacted]" ;;
          *) display_example="$example_value" ;;
        esac
        log_dim "  ${RED}+${RESET} $var ${DIM}(missing from $env_file, using ${display_example} from $example_file)"
        echo "$var=$example_value" >>"$output_file"
      fi
    else
      # This is a comment, blank line, or other content - preserve as-is
      echo "$line" >>"$output_file"
    fi
  done <"$example_file"

  log "${GREEN}✓${RESET} Smart merge completed. New file created ${DIM}$output_file"

  # If in merge mode, automatically replace the original file
  if [ "$MERGE_MODE" = true ]; then
    log_verbose "Creating backup of ${DIM}$env_file as ${DIM}${env_file}.bak..."
    cp "$env_file" "${env_file}.bak"
    log_verbose "Replacing ${DIM}$env_file${RESET} with merged version..."
    cp "$output_file" "$env_file"
    rm -f "$output_file"
    log_dim " ${GREEN}✓${RESET} Successfully replaced $env_file with merged version."
    log_verbose " Backup saved as ${DIM}${env_file}.bak${RESET}"
  else
    # Ask user if they want to replace the original file
    log
    log_info "Would you like to replace ${DIM}$env_file${RESET}${BLUE} with the merged version?"
    log_info "This will create a backup of your current ${DIM}$env_file${RESET}${BLUE} as ${DIM}${env_file}.bak${RESET}"
    read -p "Replace $env_file with merged version? (y/N): " -n 1 -r
    echo
    if [[ $REPLY =~ ^[Yy]$ ]]; then
      log_verbose "Creating backup of ${DIM}$env_file${RESET}${BLUE} as ${DIM}${env_file}.bak${RESET}..."
      cp "$env_file" "${env_file}.bak"
      log_dim "Replacing $env_file with merged version..."
      cp "$output_file" "$env_file"
      rm -f "$output_file"
      log_verbose "Successfully replaced ${DIM}$env_file${RESET} with merged version."
      log_verbose "Backup saved as ${DIM}${env_file}.bak${RESET}"
    else
      log_dim "Keeping original $env_file unchanged"
      log_dim "Merged version remains available as $output_file"
    fi
  fi
}

# -----------------------------------------------------------------------------------------------------------
# Script entry point
# -----------------------------------------------------------------------------------------------------------
main "$@"
