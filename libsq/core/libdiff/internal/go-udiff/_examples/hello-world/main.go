package main

import (
	"fmt"

	udiff "github.com/neilotoole/sq/libsq/core/libdiff/internal/go-udiff"
)

func main() {
	a := "Hello, world!\n"
	b := "Hello, Go!\nSay hi to µDiff"
	d := udiff.Unified("a.txt", "b.txt", a, b)
	fmt.Println(d)
}
