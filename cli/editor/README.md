## Package editor

This package implements editor functionality: calling out to the editor
defined in envar `$EDITOR`.

The code is copied almost verbatim from
[kubectl](https://github.com/kubernetes/kubectl/tree/master/pkg/cmd/util/editor),
but with some modifications to eliminate dependencies on external packages.
