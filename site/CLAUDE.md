# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This is the sq.io website repository - a Hugo-based documentation site for the [sq](https://github.com/neilotoole/sq) CLI tool. The site uses the Doks theme, Bun tooling, and is deployed on Netlify.

**Tech Stack:**
- Hugo 0.122.0+ (extended version) - static site generator
- Bun 1.2+ - JavaScript runtime and package manager
- Doks theme (Hugo theme for documentation sites)
- Netlify for hosting and CI/CD

## Development Commands

### Local Development
```bash
# Install dependencies (includes Hugo via hugo-installer)
bun install

# Start local dev server at http://localhost:1313
# Note: May take 1+ minute to start, be patient
bun start

# Build production site (output to ./public)
bun run build

# Build and preview locally
bun run preview
```

### Linting and Testing
```bash
# Run all linters (scripts, styles, markdown, links)
bun test
# or
bun run lint

# Individual linters
bun run lint:scripts       # ESLint on assets/js
bun run lint:styles        # Stylelint on SCSS files
bun run lint:markdown      # markdownlint on MD files
bun run lint:links         # Link checker (starts local server)

# Auto-fix
bun run lint:scripts-fix
bun run lint:styles-fix
bun run lint:markdown-fix
```

### Content Generation
```bash
# Regenerate sq command help documentation
# Requires sq CLI to be installed and in PATH
bun run gen:cmd-help

# Regenerate syntax highlighting CSS (Chroma themes)
bun run gen:syntax-css
```

## Architecture

### Content Organization

**Content structure:**
- `content/en/docs/` - Main documentation in English
  - `cmd/` - Auto-generated command documentation (see below)
  - `drivers/` - Database driver docs
  - `output/` - Output format docs
  - `inspect/` - Inspect command docs
  - `config/` - Configuration docs
  - `overview.md`, `install.md` - Key landing pages

**Hugo modules:**
- Uses `cj.rs/gohugo-asciinema` module for embedded terminal recordings
- Module defined in `go.mod`, cached by Hugo

### Command Documentation System

The `content/en/docs/cmd/` directory contains auto-generated help text for sq commands:

1. **Generation script:** `content/en/docs/cmd/generate-cmd-help.sh`
   - Runs `sq <command> --help` for each command
   - Outputs to `.help.txt` files (e.g., `add.help.txt`, `driver-ls.help.txt`)
   - Also generates config option help in `./options/*.help.txt`

2. **Markdown integration:** Corresponding `.md` files include the help text via Hugo shortcodes

3. **Regeneration:** Run `bun run gen:cmd-help` when sq commands change (requires sq installed)

### Layouts and Templates

- `layouts/_default/` - Base templates
- `layouts/shortcodes/` - Custom shortcodes for documentation
  - Includes asciinema player shortcode for terminal recordings
- `layouts/partials/` - Reusable template partials

### Assets Pipeline

- `assets/scss/` - SCSS stylesheets compiled by Hugo Pipes
- `assets/js/` - JavaScript bundled with esbuild
- `static/` - Static assets served as-is
  - `static/casts/` - Asciinema terminal recordings (.cast files)
  - `static/_redirects` - Netlify redirects (e.g., sq.io/install.sh)

### Configuration

Hugo configuration is split by environment:
- `config/_default/` - Base config (config.toml, params.toml, menus, etc.)
- `config/production/` - Production overrides
- `config/next/` - Staging/next environment

### Syntax Highlighting

Uses Hugo's built-in Chroma (not highlight.js):
- Themes: Nord (light and dark modes)
- Generation: `generate-syntax-css.sh` creates SCSS files
- Output: `assets/scss/components/_syntax.scss` and `_syntax-dark.scss`

### Link Checking

`linkinator.sh`:
1. Builds a fresh site with Hugo
2. Starts a local server at http://localhost:31317
3. Runs linkinator against the local build
4. Configuration in `linkinator.config.json` (excludes domains that block crawlers)

## CI/CD Workflow

**Triggers and actions:**
- Push to `master` or `develop` → GitHub Actions runs linting and builds
- PR to `master` or `develop` → CI + Netlify deploy preview
- Merge to `master` → Auto-deploy to sq.io

**Netlify configuration** (`netlify.toml`):
- Build command: `bun run build`
- Publish directory: `public`
- Plugins: Lighthouse audits, sitemap submission
- Deploy previews include full Lighthouse reports
- Netlify automatically detects `bun.lock` and uses `bun install`

## Content Style Guide

### Asciinema Terminal Recordings

Include terminal recordings using the asciicast shortcode:

```gotemplate
{{< asciicast src="/casts/filename.cast" theme="monokai" poster="npt:0:20" rows="10" speed="1.5" idleTimeLimit="3" >}}
```

- Store casts in `static/casts/`
- `poster="npt:0:20"` sets cover image at 20 seconds
- Usually do NOT set `autoPlay="true"`

**Module cache issue:** If you see "shortcode 'asciicast' not found", run `bun install` to refresh Hugo modules.

### Alerts

Use Hugo alert shortcodes for callouts:

```gotemplate
{{< alert icon="👉" >}}
Important note for readers.
{{< /alert >}}
```

## Redirects

- **Hugo aliases:** Use Hugo's `alias` front matter for content-based redirects
- **Static redirects:** Add entries to `static/_redirects` for non-content redirects (Netlify format)
  - Automatically appended to `public/_redirects` during build

## Important Notes

- **Hugo version:** Extended version required (specified in package.json otherDependencies)
- **Bun version:** Requires Bun 1.2+ (enforced in package.json engines)
- **Build time:** Initial local server start can take 1+ minute
- **Command help:** The `gen:cmd-help` script requires the `sq` CLI installed locally
- **Module cache:** Run `bun install` if Hugo module issues occur
- **Lockfile:** Project uses `bun.lock` (text-based lockfile, introduced in Bun 1.2+) for dependency management
