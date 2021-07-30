// Package main contains sq's main function.
package main

import (
	"context"
	"os"
	"os/signal"

	"github.com/neilotoole/sq/cli"
)

func main() {
	var err error
	ctx, cancelFn := context.WithCancel(context.Background())
	defer func() {
		cancelFn()
		if err != nil {
			os.Exit(1)
		}
	}()

	go func() {
		stopCh := make(chan os.Signal, 1)
		signal.Notify(stopCh, os.Interrupt)

		<-stopCh
		cancelFn()
	}()

	err = cli.Execute(ctx, os.Stdin, os.Stdout, os.Stderr, os.Args[1:])
}
