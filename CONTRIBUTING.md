`sq` welcomes new [issues](https://github.com/neilotoole/sq/issues), [pull requests](https://github.com/neilotoole/sq/pulls)
and [discussion](https://github.com/neilotoole/sq/discussions).

For user documentation, see [sq.io](https://sq.io)

## Required tooling

This documentation presumes you are on MacOS. If not, adapt appropriately.

- `go`: `brew install go`
- `make`: `brew install make`
- `shellcheck`: `brew install shellcheck`

## Makefile

Yes, we are a Go project, and shouldn't need a [Makefile](./Makefile). But, `sq` is also a fairly
complex project, with generated code, CGo, test containers, related docs [website](https://sq.io), and a bunch of other
stuff. Therefore, if for no other reason, it is recommended to use the Makefile when developing locally.

## CHANGELOG.md

The [CHANGELOG.md](./CHANGELOG.md) file is sacrosanct, in that it *must* be updated every
time there is a new release. Note that this is a task for the project
maintainers; you do not need to worry about this if creating a PR.

This project follows [Keep a Changelog](https://keepachangelog.com/en/1.0.0/)
and [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

### Unreleased Section

When there is work-in-progress, `CHANGELOG.md` uses an `## Unreleased` section
at the top for accumulating changes during development.

```markdown
## Unreleased

### Fixed

### Changed

### Added

## [v0.48.5] - 2025-01-19
...
```

**Workflow:**

1. **Starting new work**: Add an `## Unreleased` section at the top of the
   CHANGELOG with the standard subsection headers (Fixed, Changed, Added).

2. **During development**: Add entries under `## Unreleased` as changes are
   made. Each PR should update this section with its changes.

3. **At release time**: When creating a new version (e.g., `git tag v1.2.3`):
   - Replace `## Unreleased` with `## [v1.2.3] - YYYY-MM-DD`
   - Remove empty subsections
   - Add the version comparison link at the bottom of the file

The `## Unreleased` section should not exist when there is no work-in-progress.

### Version Entry Structure

```markdown
## [vX.Y.Z] - YYYY-MM-DD

Optional brief description paragraph for significant releases.

### Added

- New features go here

### Changed

- Changes to existing functionality

### Fixed

- Bug fixes
```

When present, sections should appear in this order: Added, Changed, Fixed.
Not all sections are required for every release.

### Special Markers

- ☢️ - Breaking changes, place at start of entry
- 🐥 - Alpha/beta features, place at start of entry
- 👉 - Important notes/callouts within entry text

### Entry Formatting

Reference GitHub issues at the start of entries when applicable:

```markdown
- [#123]: Description of the change.
```

Issue link definitions go at the bottom of the file:

```markdown
[#123]: https://github.com/neilotoole/sq/issues/123
```

Use fenced code blocks with language hints (`shell`, `sql`, `json`, `csv`,
`plaintext`). Indent code blocks with two spaces when nested under a list item.

Example entry with code block:

```markdown
- [#338]: The [`having`](https://sq.io/docs/query#having) function is now
  implemented.

  $ sq '.payment | group_by(.customer_id) | having(sum(.amount) > 200)'
```

**Formatting tips:**

- Link config options to docs:
  `` [`config.option`](https://sq.io/docs/config#configoption) ``
- Use backticks for commands: `` `sq add` ``
- Use backticks for flags: `` `--verbose` ``

### Breaking Changes

Prefix with ☢️ and explain what changed. Include before/after examples when
helpful:

```markdown
- ☢️ The `--old-flag` flag has been renamed to `--new-flag`.
```

### Writing Style

- Start entries with a verb or noun phrase
- Use present tense for features ("Implements...", "Adds...")
- Use past tense for fixed bugs ("Fixed...", "Resolved...")
- Provide concrete examples with shell commands
- Link to documentation for detailed features
- Be specific about what changed and why

### Version Links

At the bottom of the file, add version comparison links:

```markdown
[vX.Y.Z]: https://github.com/neilotoole/sq/compare/vA.B.C...vX.Y.Z
```

For the first release in a sequence, link to the release tag:

```markdown
[v0.15.2]: https://github.com/neilotoole/sq/releases/tag/v0.15.2
```
