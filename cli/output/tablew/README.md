# Package `tablew`

Package `tablew` implements text table output writers.

The actual rendering of the text table is handled by a modified version of
[`olekukonko/tablewriter`](https://github.com/olekukonko/tablewriter),
which can be found in the `internal` sub-package. At the time, `tablewriter`
didn't provide all the functionality that sq required. However,
that package has been significantly developed since then
fork, and it may be possible that we could dispense with the forked
version entirely and directly use a newer version of `tablewriter`.

This entire package could use a rewrite, a lot has changed with sq
since this package was first created. So, if you see code in here
that doesn't make sense to you, you're probably judging it correctly.
