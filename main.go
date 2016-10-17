package main

import (
	// bootstrap is the first import, do not move
	_ "github.com/neilotoole/sq/cmd/bootstrap"

	"os"
	"strings"

	"github.com/neilotoole/go-lg/lg"
	"github.com/neilotoole/sq/cmd"
)

func main() {

	str := "\n" + strings.Repeat("*", 80) + "\n > " + strings.Join(os.Args, " ") + "\n" + strings.Repeat("*", 80)
	lg.Debugf(str)

	cmd.Execute()

}
