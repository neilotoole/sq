package main

import (
	"fmt"
	"regexp"

	"github.com/neilotoole/go-lg/lg"
)

func main() {

	i := 5
	lg.Debugf("i: %v", i)
	lg.Debugf("hello world")

	pattern := regexp.MustCompile(`\A[@][a-zA-Z][a-zA-Z0-9_]*`)

	matched := pattern.MatchString("@hello")

	//matched, err := regexp.MatchString(`\A[@][a-zA-Z][a-zA-Z0-9_]*`, "@_hello")
	fmt.Println(matched)
}
