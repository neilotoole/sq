package main

import (
	"fmt"

	udiff "github.com/neilotoole/sq/cli/diff/libdiff/internal/go-udiff"
)

func main() {
	a := "Hello, world!\n"
	b := "Hello, Go!\nSay hi to µDiff"

	edits := udiff.Strings(a, b)
	final, err := udiff.Apply(a, edits)
	if err != nil {
		panic(err)
	}

	fmt.Println(final)
}
