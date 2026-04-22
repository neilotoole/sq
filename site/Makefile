# -----------------------------------------------------------------------------------------------------------
# Makefile for the sq.io site (Hugo + Bun), living under site/ in the sq monorepo.
# -----------------------------------------------------------------------------------------------------------
# Includes Common.make: Docker-based local server (make build = Docker image; make run / run-detached).
#
# For day-to-day and CI-style work without Docker, prefer:
#   make deps        — bun install
#   make site-test   — bun run test (lint, link check, etc.; same as Site CI)
#   make site-build  — bun run build (Hugo → public/)
#   make site-dev    — bun start (local dev server)
#   make ci          — deps, then site-test, then site-build (matches GitHub Site CI)
#
# Lighthouse harness (simulates Netlify-served production with gzip/brotli):
#   make site-lighthouse        — build, serve, run Lighthouse (mobile + desktop), tear down
#   make site-lighthouse-serve  — same build + server, stays running for manual DevTools audits
#
# Quick start (Docker): make run-detached && make ping  →  http://localhost:8080
# See Common.make for Docker targets and help.
# -----------------------------------------------------------------------------------------------------------

include Common.make

# -----------------------------------------------------------------------------------------------------------
# Configuration
# -----------------------------------------------------------------------------------------------------------
# Port used by the Lighthouse harness (prod build + gzip/brotli static server).
# Chosen to avoid :8080 (Docker via `make run`) and :1313 (Hugo dev via `make site-dev`).
LH_PORT ?= 8090
LH_URL  ?= http://localhost:$(LH_PORT)


# -----------------------------------------------------------------------------------------------------------
# Local / CI targets (no Docker)
# -----------------------------------------------------------------------------------------------------------
# These use the same ".PHONY: target ## description" pattern as Common.make so they appear in `make help`.
# The "site-*" names avoid clashing with Common.make `build` (Docker image) and `test` (Docker validate + bun).
# Logging uses the shared log.bash helpers via $(LOGGER) (defined in Common.make).
# -----------------------------------------------------------------------------------------------------------

.PHONY: deps ## Install dependencies (bun install)
deps:
	@$(LOGGER) log_separator
	@$(LOGGER) log_info "Installing site dependencies (bun install)"
	bun install
	@$(LOGGER) log_success "Dependencies installed"

.PHONY: site-test ## Run lint suite (bun run test; scripts, styles, markdown, links)
site-test:
	@$(LOGGER) log_separator
	@$(LOGGER) log_info "Running site lint suite"
	@$(LOGGER) log_indent log_dim "scripts, styles, markdown, links"
	bun run test
	@$(LOGGER) log_success "Lint suite passed"

.PHONY: site-build ## Build production site to public/ (bun run build; Hugo)
site-build:
	@$(LOGGER) log_separator
	@$(LOGGER) log_info "Building production site to public/"
	bun run build
	@$(LOGGER) log_success "Site built to public/"

.PHONY: site-dev ## Start local dev server (bun start; http://localhost:1313)
site-dev:
	@$(LOGGER) log_separator
	@$(LOGGER) log_info "Starting Hugo dev server"
	@$(LOGGER) log_indent log_dim "http://localhost:1313 (Ctrl+C to stop)"
	bun start

.PHONY: site-lighthouse-serve ## Build prod site and serve it locally with gzip/brotli on LH_PORT (default 8090); Ctrl+C to stop
site-lighthouse-serve:
	@set -e; \
	source scripts/log.bash; \
	log_separator; \
	log_info "Building production site and serving with gzip/brotli"; \
	log_indent log_dim "baseURL: $(LH_URL)"; \
	log_indent log_dim "[1/2] bun run clean && hugo --gc --minify"; \
	bun run clean >/dev/null; \
	node_modules/.bin/hugo/hugo --gc --minify -b "$(LH_URL)/" >/dev/null; \
	{ [ -f static/_redirects ] && cat static/_redirects >> public/_redirects 2>/dev/null; } || true; \
	log_indent log_dim "[2/2] bun scripts/lighthouse-server.ts"; \
	log_info "Serving public/ on $(LH_URL) (gzip/brotli enabled)"; \
	log_indent log_dim "Ctrl+C to stop"; \
	bun scripts/lighthouse-server.ts --port "$(LH_PORT)" --dir public

.PHONY: site-lighthouse ## Build prod site, serve with gzip/brotli, run Lighthouse (mobile + desktop), tear down; override LH_PORT (default 8090)
site-lighthouse:
	@set -e; \
	source scripts/log.bash; \
	log_separator; \
	log_info "Running Lighthouse against a local prod-like build"; \
	log_indent log_dim "URL: $(LH_URL)"; \
	log_indent log_dim "[1/4] Building production site (hugo --gc --minify)…"; \
	bun run clean >/dev/null; \
	node_modules/.bin/hugo/hugo --gc --minify -b "$(LH_URL)/" >/dev/null; \
	{ [ -f static/_redirects ] && cat static/_redirects >> public/_redirects 2>/dev/null; } || true; \
	log_indent log_dim "[2/4] Starting prod-like static server (gzip/brotli)…"; \
	bun scripts/lighthouse-server.ts --port "$(LH_PORT)" --dir public >/tmp/lighthouse-server.log 2>&1 & \
	SERVER_PID=$$!; \
	trap "kill $$SERVER_PID 2>/dev/null || true" EXIT INT TERM; \
	for i in 1 2 3 4 5 6 7 8 9 10 11 12 13 14 15; do \
	  sleep 0.3; \
	  curl -fsS -o /dev/null "$(LH_URL)/" 2>/dev/null && break; \
	done; \
	curl -fsS -o /dev/null "$(LH_URL)/" || { \
	  log_error "Server did not come up; see /tmp/lighthouse-server.log"; \
	  tail -20 /tmp/lighthouse-server.log; \
	  exit 1; \
	}; \
	mkdir -p lighthouse-reports; \
	log_indent log_dim "[3/4] Running Lighthouse (mobile, throttled)…"; \
	bunx lighthouse "$(LH_URL)" --form-factor=mobile \
	  --output=html --output=json \
	  --output-path=lighthouse-reports/mobile \
	  --chrome-flags="--headless" --quiet; \
	log_indent log_dim "[4/4] Running Lighthouse (desktop)…"; \
	bunx lighthouse "$(LH_URL)" --preset=desktop \
	  --output=html --output=json \
	  --output-path=lighthouse-reports/desktop \
	  --chrome-flags="--headless" --quiet; \
	kill $$SERVER_PID 2>/dev/null || true; \
	log_success "Lighthouse reports written to lighthouse-reports/"; \
	log_indent log_dim "→ lighthouse-reports/mobile.report.html"; \
	log_indent log_dim "→ lighthouse-reports/desktop.report.html"

.PHONY: ci ## Install deps, lint, and production build (matches GitHub Site CI workflow)
ci: deps site-test site-build
	@$(LOGGER) log_separator
	@$(LOGGER) log_success "CI pipeline completed (deps, lint, build)"
