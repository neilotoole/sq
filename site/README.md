# sq.io website

[![Netlify Status](https://api.netlify.com/api/v1/badges/7caea069-2a8d-4f0b-bafe-b053bbc5eb08/deploy-status)](https://app.netlify.com/sites/sq-web/deploys)

This is the repo for the [sq.io](https://sq.io) website, which
hosts documentations for [sq](https://github.com/neilotoole/sq).

This site is built using:

- [Hugo](https://gohugo.io) site generator
- [Doks](https://getdoks.org) theme
- [Bun](https://bun.sh) tooling
- [Netlify](https://www.netlify.com) hosting

Changes to the `master` branch kick off a redeploy on Netlify.

## Contributing

You could [open an issue](https://github.com/neilotoole/sq-web/issues), but ideally you'd submit a
pull request.

### 1. Clone this repo

```bash
git clone https://github.com/neilotoole/sq-web.git && cd sq-web
```

### 2. Install dependencies

```bash
bun install
```

### 3. Make changes and test locally

```bash
# Start a local webserver on http://localhost:1313 to test you changes.
# Note that it may take a minute or longer for the site to be served (and
# you may not receive any indication of this). Be patient.
bun start

# Run linters, link checks, etc.
bun test
```

### 4. Submit a Pull Request

Create a [Pull Request](https://github.com/neilotoole/sq-web/pulls), providing context
for your changes.

## Content Style Guide

### Alerts

Use Hugo alert shortcodes to highlight important information:

```gotemplate
{{< alert icon="👉" >}}
This is an important note for the reader.
{{< /alert >}}
```

## Development

### CI Workflow

The project uses GitHub Actions and Netlify for continuous integration:

| Trigger                               | Action                                                |
|---------------------------------------|-------------------------------------------------------|
| Push to `master` or `develop`         | GitHub Actions runs linting and build checks          |
| Pull request to `master` or `develop` | GitHub Actions CI + Netlify deploy preview            |
| Merge to `master`                     | Automatic production deploy to [sq.io](https://sq.io) |

Netlify provides deploy previews for every PR with Lighthouse audits for performance,
accessibility, best practices, and SEO. Before merging, click through to the
deploy preview (e.g., `https://deploy-preview-59--sq-web.netlify.app`) to verify
your changes look correct.

### Commands

Key Bun scripts defined in `package.json`:

| Command                  | Description                                             |
|--------------------------|---------------------------------------------------------|
| `bun start`              | Start local dev server with live reload                 |
| `bun run build`          | Build production site                                   |
| `bun run preview`        | Build and serve locally at http://localhost:1313        |
| `bun run lint`           | Run all linters (scripts, styles, markdown, links)      |
| `bun run gen:cmd-help`   | Regenerate command help files in `content/en/docs/cmd/` |
| `bun run gen:syntax-css` | Regenerate syntax highlighting CSS                      |

### Regenerating Command Documentation

The `gen:cmd-help` script (`./content/en/docs/cmd/generate-cmd-help.sh`) regenerates the `.help.txt`
files in `content/en/docs/cmd/`. These files contain the help text for each `sq` command and are
included in the documentation pages.

### Link Checking

The `bun run lint` command includes link checking via [linkinator](https://github.com/JustinBeckwith/linkinator).
Some sites (e.g., StackOverflow) block automated crawlers, returning 403 errors in CI. These domains
are excluded in `linkinator.config.json`.

## Redirects

- You can use the Hugo [alias](https://gohugo.io/content-management/urls/#aliases) mechanism to
  maintain an old path that will redirect to the new path.
- If you need a redirect that's not associated with Hugo content, add an entry to
  the [`static/_redirects`](/static/_redirects) file. This is what the site uses to
  serve the [sq.io/install.sh](https://sq.io/install.sh) script.

## Misc

- Doks comes with [commands](https://getdoks.org/docs/prologue/commands/) for common tasks.
- Use `bun run gen:syntax-css` to regenerate the syntax highlight theme. The themes (light and dark)
  are specified in [generate-syntax-css.sh](./generate-syntax-css.sh).

### Asciinema

The site makes use of [asciinema](https://asciinema.org) via
the [gohugo-asciinema](https://github.com/cljoly/gohugo-asciinema) Hugo module.

Typically, casts are stored in `./static/casts`. To include a cast, use this shortcode:

```gotemplate
{{< asciicast src="/casts/home-quick.cast" theme="monokai" poster="npt:0:20" rows="10" speed="1.5" idleTimeLimit="3" >}}
```

- `poster="npt:0:20"` specifies that the "poster" or cover image should be taken from 0m20s into the
  cast.
- Add `autoPlay="true"` if the cast should start immediately. This is usually not the case.

If you see this problem:

```shell
Error: Error building site: "content/en/_index.md:9:1": failed to extract shortcode: template for shortcode "asciicast" not found
```

It probably means that the hugo module cache is out of whack. Run `bun install` and try again.

## Documentation

- [Netlify](https://docs.netlify.com/)
- [Hugo](https://gohugo.io/documentation/)
- [Doks](https://getdoks.org/)

## Communities

- [sq discussions](https://github.com/neilotoole/sq/discussions)
- [Netlify Community](https://community.netlify.com/)
- [Hugo Forums](https://discourse.gohugo.io/)
- [Doks Discussions](https://github.com/h-enk/doks/discussions)

## Acknowledgements

- Special thanks to [Netlify](https://www.netlify.com), who provide
  free hosting for [sq.io](https://sq.io) via
  their [open source program](https://www.netlify.com/open-source/).
- A bunch of stuff has been lifted from the [Docsy theme](https://www.docsy.dev).

[![Deploys by Netlify](https://www.netlify.com/v3/img/components/netlify-dark.svg)](https://www.netlify.com)
