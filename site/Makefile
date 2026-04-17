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

.PHONY: ci ## Install deps, lint, and production build (matches GitHub Site CI workflow)
ci: deps site-test site-build
