# Tools

This directory contains subdirectories for each Go tool dependency, using the
Go 1.24+ `tool` directive in isolated `go.mod` files.

## Why separate modules?

We use this subdirectory pattern to **isolate tool dependencies from the main
project's dependency tree**. This approach is recommended by the
[`golangci-lint` team](https://golangci-lint.run/docs/welcome/install/local/) and
is considered a best practice (well, by some people).

Note that the `golangci-lint` team don't *really* recommend this practice, but
we're doing it anyway until the Go community settle on a best practice. Per the
`golangci-lint` team:

> But if you want to use `go tool` to install and run `golangci-lint` (once again
> we don’t recommend that), the best approach is to use a dedicated module or
> module file to isolate `golangci-lint` from other tools or dependencies.
> This approach avoids modifying your project dependencies and the
> `golangci-lint` dependencies.

### The problem: dependency conflicts

When tool dependencies are added to your main `go.mod`, their transitive
dependencies mix with your project's dependencies. This can cause issues:

1. **Version conflicts**: A linting tool might require `gopkg.in/yaml.v3 v3.0.0`
   while your project needs `v3.0.1`. Go's module resolution will force a single
   version, potentially breaking either the tool or your project.

2. **Dependency bloat**: Tool dependencies appear in your `go.sum`, increasing
   its size and potentially confusing dependency scanners.

3. **Unexpected upgrades**: Running `go get -u` can inadvertently upgrade tool
   dependencies, creating untested combinations.

4. **Consumer pollution**: Downstream consumers of your module may see
   tool-related indirect dependencies, even though Go's module graph pruning
   won't download unused dependencies.

### The solution: isolated tool modules

By placing each tool in its own subdirectory with a separate `go.mod`:

- Tool dependencies are completely isolated from the main project
- Each tool can have its own dependency versions without conflicts
- The main `go.mod` stays clean and focused on application dependencies
- Tools can be updated independently without affecting the project

## Structure

```text
tools/
├── README.md
├── betteralign/
│   ├── go.mod
│   └── go.sum
├── gofumpt/
│   ├── go.mod
│   └── go.sum
├── goimports-reviser/
│   ├── go.mod
│   └── go.sum
├── golangci-lint/
│   ├── go.mod
│   └── go.sum
└── tparse/
    ├── go.mod
    └── go.sum
```

Each subdirectory contains a minimal `go.mod` with:

- A module declaration (e.g., `module github.com/neilotoole/sq/tools/golangci-lint`)
- A `go` version directive
- A `tool` directive pointing to the tool's main package
- The tool's dependencies (managed automatically by Go)

## Usage

Tools are invoked using the `-modfile` flag to specify which isolated module
to use:

```bash
go tool -modfile=tools/golangci-lint/go.mod golangci-lint run ./...
go tool -modfile=tools/gofumpt/go.mod gofumpt -w .
go tool -modfile=tools/tparse/go.mod tparse -all
go tool -modfile=tools/betteralign/go.mod betteralign -apply ./...
go tool -modfile=tools/goimports-reviser/go.mod goimports-reviser -format ./...
```

See the project [`Makefile`](../Makefile) for real-world usage examples.

## Adding a new tool

1. Create a new subdirectory under `tools/`:

   ```bash
   mkdir tools/newtool
   cd tools/newtool
   ```

2. Initialize the module:

   ```bash
   go mod init github.com/neilotoole/sq/tools/newtool
   ```

3. Add the tool using the `-tool` flag:

   ```bash
   go get -tool example.com/newtool@latest
   ```

   This adds both the `tool` directive and the required dependencies to the
   `go.mod` file.

4. Return to the project root and verify the tool works:

   ```bash
   cd ../..
   go tool -modfile=tools/newtool/go.mod newtool --version
   ```

5. Add a Makefile target if needed (see existing targets for examples).

## References

- [golangci-lint installation advice](https://golangci-lint.run/docs/welcome/install/local/)
- [Go 1.24 tool directive](https://go.dev/doc/modules/gomod-ref)
- [Managing tool dependencies in Go 1.24+](https://www.alexedwards.net/blog/how-to-manage-tool-dependencies-in-go-1.24-plus)
- [Fixing Go 1.24's tool directive biggest problem](https://aran.dev/posts/go-124/fixing-go-124-tool-directive-biggest-problem/)
