This directory contains subdirectories for each Go tool that we want to use
per the `go.mod` `tool` directive.

We use this subdirectory pattern per the
[advice](https://golangci-lint.run/docs/welcome/install/#install-from-sources)
given by the `golangci-lint` team.

```
Warning

We don’t recommend using go tool.
But if you want to use go tool to install and run golangci-lint (once again we don’t recommend that), the best approach is to use a dedicated module or module file to isolate golangci-lint from other tools or dependencies.
This approach avoids modifying your project dependencies and the golangci-lint dependencies

```
