# Releasing & CHANGELOG

How `sq` releases are cut, and the conventions for
[`CHANGELOG.md`](../CHANGELOG.md). Updating the changelog at release time is a
**maintainer** task; as a PR author you generally only add an entry under
`## Unreleased` (and often not even that — see **Scope** below).

## CHANGELOG conventions

[`CHANGELOG.md`](../CHANGELOG.md) follows
[Keep a Changelog](https://keepachangelog.com/en/1.0.0/) and
[Semantic Versioning](https://semver.org/spec/v2.0.0.html) — see that spec for
the baseline format.

**Scope:** Entries describe the **`sq` CLI and core libraries** (what ships in
the release binary). Changes that **only** touch [`site/`](../site) (the sq.io
Hugo site) do **not** need a CHANGELOG entry unless a maintainer wants a
release-note line tied to the `sq` product.

### Unreleased section

Work-in-progress accumulates under an `## Unreleased` section at the top of the
file, with `Fixed` / `Changed` / `Added` subsections in that order (omit the
ones you don't use):

```markdown
## Unreleased

### Fixed

### Changed

### Added

## [v0.48.5] - 2025-01-19
```

Add entries under `## Unreleased` as CLI and library changes land. At release
time a maintainer renames it to `## [vX.Y.Z] - YYYY-MM-DD`, drops empty
subsections, and adds the version comparison link. The `## Unreleased` section
should not exist when there is no work-in-progress.

### sq conventions

Beyond the Keep a Changelog baseline:

- **Markers** at the start of an entry: ☢️ breaking change, 🐥 alpha/beta
  feature, 👉 important callout within entry text.
- **Reference issues** at the start of an entry, with link definitions at the
  bottom of the file. Use fenced code blocks with language hints (`shell`,
  `sql`, `json`, …), indented two spaces when nested under a list item:

  ```markdown
  - [#338]: The [`having`](https://sq.io/docs/query#having) function is now implemented.

    $ sq '.payment | group_by(.customer_id) | having(sum(.amount) > 200)'
  ```

  ```markdown
  [#338]: https://github.com/neilotoole/sq/issues/338
  ```

- **Link to sq.io docs** for commands, flags, and config options
  (e.g. ``[`config.option`](https://sq.io/docs/config#configoption)``); use
  backticks for bare commands (`` `sq add` ``) and flags (`` `--verbose` ``).
- **Version links** at the bottom compare against the previous tag
  (`[vX.Y.Z]: https://github.com/neilotoole/sq/compare/vA.B.C...vX.Y.Z`); the
  first release in a sequence links to its release tag
  (`[v0.15.2]: https://github.com/neilotoole/sq/releases/tag/v0.15.2`).
- Use **US English** ("honors" not "honours", "color" not "colour"); present
  tense for features ("Adds…"), past tense for fixes ("Fixed…").

## Release procedure

Releases are cut by pushing a `v*` tag. A maintainer:

1. **Finalizes the CHANGELOG** — renames `## Unreleased` to
   `## [vX.Y.Z] - YYYY-MM-DD`, drops empty subsections, and adds the version
   comparison link at the bottom (per the conventions above).
2. **Tags and pushes** `vX.Y.Z` on `master`.

Pushing the tag triggers the release path of the **Main Pipeline**
([`main.yml`](../.github/workflows/main.yml)): the full test suites run, then
per-platform [GoReleaser](https://goreleaser.com) builds
(`.goreleaser-*.yml`) produce the binaries, `publish` cuts the GitHub release,
`docker-publish` pushes the `ghcr.io` image, and `test-install` smoke-tests the
published artifacts as a post-publish canary. When the GitHub release is
published, [`site-publish-release.yml`](../.github/workflows/site-publish-release.yml)
auto-deploys [sq.io](https://sq.io).

For the job-by-job detail of that pipeline (fast loop vs. full suites, what
gates `publish`), see [`docs/WORKFLOW.md`](./WORKFLOW.md#main-pipeline-go-build--test--release).
