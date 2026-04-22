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
# Quick start (Docker): make run-detached && make ping  →  http://localhost:8080
# See Common.make for Docker targets and help.
# -----------------------------------------------------------------------------------------------------------

include Common.make

# -----------------------------------------------------------------------------------------------------------
# Local / CI targets (no Docker)
# -----------------------------------------------------------------------------------------------------------
# These use the same ".PHONY: target ## description" pattern as Common.make so they appear in `make help`.
# The "site-*" names avoid clashing with Common.make `build` (Docker image) and `test` (Docker validate + bun).
# -----------------------------------------------------------------------------------------------------------

.PHONY: deps ## Install dependencies (bun install)
deps:
	bun install

.PHONY: site-test ## Run lint suite (bun run test; scripts, styles, markdown, links)
site-test:
	bun run test

.PHONY: site-build ## Build production site to public/ (bun run build; Hugo)
site-build:
	bun run build

.PHONY: site-dev ## Start local dev server (bun start; http://localhost:1313)
site-dev:
	bun start

.PHONY: site-lighthouse-serve ## Build prod site and serve it locally with gzip/brotli on LH_PORT (default 8090); Ctrl+C to stop
site-lighthouse-serve:
	@LH_PORT="$${LH_PORT:-8090}"; \
	echo "Building production site with baseURL http://localhost:$$LH_PORT/ …"; \
	bun run clean; \
	node_modules/.bin/hugo/hugo --gc --minify -b "http://localhost:$$LH_PORT/"; \
	[ -f static/_redirects ] && cat static/_redirects >> public/_redirects 2>/dev/null || true; \
	echo ""; echo "Serving public/ on http://localhost:$$LH_PORT (gzip/brotli enabled). Ctrl+C to stop."; \
	bun scripts/lighthouse-server.ts --port "$$LH_PORT" --dir public

.PHONY: site-lighthouse ## Build prod site, serve with gzip/brotli, run Lighthouse (mobile + desktop), tear down; overrides LH_PORT (8090) and LH_URL (built-in)
site-lighthouse:
	@LH_PORT="$${LH_PORT:-8090}"; \
	LH_URL="http://localhost:$$LH_PORT"; \
	echo "[1/4] Building production site (hugo --gc --minify -b $$LH_URL/) …"; \
	bun run clean; \
	node_modules/.bin/hugo/hugo --gc --minify -b "$$LH_URL/" >/dev/null; \
	[ -f static/_redirects ] && cat static/_redirects >> public/_redirects 2>/dev/null || true; \
	echo "[2/4] Starting prod-like static server (gzip/brotli) on $$LH_URL …"; \
	bun scripts/lighthouse-server.ts --port "$$LH_PORT" --dir public >/tmp/lighthouse-server.log 2>&1 & \
	SERVER_PID=$$!; \
	trap "kill $$SERVER_PID 2>/dev/null || true" EXIT INT TERM; \
	for i in 1 2 3 4 5 6 7 8 9 10 11 12 13 14 15; do \
	  sleep 0.3; \
	  curl -fsS -o /dev/null "$$LH_URL/" 2>/dev/null && break; \
	done; \
	curl -fsS -o /dev/null "$$LH_URL/" || { echo "ERROR: server did not come up"; tail -20 /tmp/lighthouse-server.log; exit 1; }; \
	mkdir -p lighthouse-reports; \
	echo "[3/4] Running Lighthouse (mobile, throttled)…"; \
	bunx lighthouse "$$LH_URL" --form-factor=mobile \
	  --output=html --output=json \
	  --output-path=lighthouse-reports/mobile \
	  --chrome-flags="--headless" --quiet; \
	echo "[4/4] Running Lighthouse (desktop)…"; \
	bunx lighthouse "$$LH_URL" --preset=desktop \
	  --output=html --output=json \
	  --output-path=lighthouse-reports/desktop \
	  --chrome-flags="--headless" --quiet; \
	kill $$SERVER_PID 2>/dev/null || true; \
	echo ""; \
	echo "Reports:"; \
	echo "  → lighthouse-reports/mobile.report.html"; \
	echo "  → lighthouse-reports/desktop.report.html"

.PHONY: ci ## Install deps, lint, and production build (matches GitHub Site CI workflow)
ci: deps site-test site-build
