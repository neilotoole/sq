# Docker image for local testing only: runs Hugo server with live reload on port 8080.
# The live site is built/served elsewhere (e.g. Netlify). This is for testing in Docker.
#
# Design:
# - Hugo listens on 8080 inside the container so all generated links use port 8080 (no 1313).
# - Hugo modules (e.g. asciinema shortcode) are downloaded at build time so they work at run.
# - At run, mount only content/layouts/assets/static/config for hot reload; no need to mount go.mod/go.sum.
FROM oven/bun:1-debian

ARG HUGO_VERSION=0.122.0

# Install Go (Hugo modules), C++ (Hugo extended), and Hugo binary
RUN apt-get update && apt-get install -y --no-install-recommends \
    golang-go \
    git \
    ca-certificates \
    curl \
    libstdc++6 \
    && rm -rf /var/lib/apt/lists/* \
    && ARCH=$(dpkg --print-architecture) \
    && curl -fsSL "https://github.com/gohugoio/hugo/releases/download/v${HUGO_VERSION}/hugo_extended_${HUGO_VERSION}_linux-${ARCH}.tar.gz" \
       | tar -xz -C /usr/local/bin hugo \
    && hugo version \
    && ln -s /usr/local/bin/bun /usr/local/bin/npx

WORKDIR /app

COPY package.json bun.lock* ./
RUN bun install --ignore-scripts

COPY . .

# Fetch Hugo modules (e.g. gohugo-asciinema) so shortcodes work at run
RUN hugo mod get

# Serve on 8080 so generated links use :8080 (no port mismatch when mapping host 8080)
ENV HUGO_BASEURL=http://localhost:8080/
EXPOSE 8080

# Dev server proxies to Hugo and serves GET /version (fetches Homebrew; no Netlify).
# SERVER_PORT=8080 so links use :8080; Hugo runs on 1314 inside the container.
CMD ["sh", "-c", "SERVER_PORT=8080 HUGO_PORT=1314 bun scripts/dev-server.js"]
