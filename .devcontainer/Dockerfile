# Dev container image for sq contributors. See docs/DEVELOPER.md#dev-container.
#
# The image is checked weekly and on change by .github/workflows/devcontainer.yml,
# which builds it and runs the fast local loop inside it.

# Keep in sync with the bun-version in the site and format workflows.
FROM oven/bun:1.4.3 AS bun

FROM mcr.microsoft.com/devcontainers/go:2-1.27-trixie

# Java runs the ANTLR tool behind `make gen`, and shellcheck is part of
# `make lint`. The Makefile builds SQLite with the sqlite_icu tag, which needs
# the ICU headers. The base image already has gcc (cgo, for SQLite and DuckDB),
# git, jq and make.
#
# The base image also ships Sequoia's sq (an OpenPGP tool), which would answer
# to `sq` until `make install` puts this repo's sq ahead of it on the PATH.
# Nothing needs it: dpkg-dev only suggests it, and apt verifies signatures
# with the separate sqv package.
RUN apt-get update \
  && apt-get install -y --no-install-recommends default-jre-headless libicu-dev shellcheck \
  && apt-get purge -y sq \
  && rm -rf /var/lib/apt/lists/*

COPY --from=bun /usr/local/bin/bun /usr/local/bin/bun
RUN ln -s bun /usr/local/bin/bunx

# Docker Desktop reports a bind mount's root directory as owned by root for
# about the first second after the container starts, so git's ownership check
# fails ("detected dubious ownership") if postCreateCommand reaches git in that
# window. The check guards against other users' repositories on a shared
# machine, which doesn't apply to a single-user dev container.
RUN git config --system --add safe.directory '*'

# The base image sets GOTOOLCHAIN=local, which breaks the build as soon as
# go.mod asks for a newer Go than the image ships. With auto, the go command
# downloads the toolchain that go.mod names.
ENV GOTOOLCHAIN=auto

# Keep every Go cache under ~/.cache (build cache, module cache, and
# golangci-lint's cache) so a single volume persists them across rebuilds.
ENV GOMODCACHE=/home/vscode/.cache/go-mod

# Create the volume mount points ahead of time. Docker seeds a new named volume
# from the image directory, ownership included; a missing directory would come
# up owned by root.
RUN mkdir -p /home/vscode/.cache /home/vscode/.duckdb \
  && chown vscode:vscode /home/vscode/.cache /home/vscode/.duckdb
