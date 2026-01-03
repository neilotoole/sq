# Integration Test Helpers

This directory contains shell-based integration test utilities for `sq`.

Unlike the Go-based `testh` package (which provides test helpers for Go unit tests),
`itesth` provides bash scripts for end-to-end CLI testing.

## Contents

| File                  | Description                                                            |
| --------------------- | ---------------------------------------------------------------------- |
| `log.bash`            | Logging utilities with colored output (success/error/warning/info)     |
| `common.bash`         | Common utilities (Docker Compose, container management, prerequisites) |

## Usage

Source the utilities in your test scripts:

```bash
#!/usr/bin/env bash
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/log.bash"
source "${SCRIPT_DIR}/common.bash"
```

## Logging Functions

```bash
log "Normal message"
log_dim "Dimmed message"
log_info "Info message (blue)"
log_success "Success message (green checkmark)"
log_error "Error message (red X)"
log_warning "Warning message (yellow triangle)"
log_indent log_success "Indented success"
log_separator  # Print separator line
log_banner     # Print ASCII art banner
```

## See Also

- `testh/` - Go test helper package for unit tests
- `drivers/*/testutils/` - Driver-specific integration test utilities
