// Package main contains sq's main function.
package main

import (
	"context"
	"os"
	"os/signal"
	"time"

	"github.com/neilotoole/sq/cli"
	"github.com/neilotoole/sq/libsq/core/errz"
)

func main() {
	const shutdownTimeout = time.Second * 2

	var err error
	ctx, cancelFn := context.WithCancel(context.Background())
	defer func() {
		cancelFn()
		if err != nil {
			os.Exit(1)
		}
	}()

	go func() {
		// Listen for interrupt signal (Ctrl-C) and call cancelFn.
		stopCh := make(chan os.Signal, 1)
		signal.Notify(stopCh, os.Interrupt)

		<-stopCh

		// The context cancellation should propagate down the stack,
		// and cli.Execute should return, with the context.Canceled error.
		cancelFn()

		// But... in theory the main goroutine could be blocked on something.
		// So, we have some hard shutdown possibilities.
		select {
		case <-time.After(shutdownTimeout):
			// We've waited long enough for a graceful shutdown.
			cli.PrintError(ctx, nil, errz.New("hard shutdown (timeout)"))
		case <-stopCh:
			// We received a second interrupt from the user: they're really
			// serious about exiting.
			cli.PrintError(ctx, nil, errz.New("hard shutdown"))
		}
		os.Exit(1)
	}()

	err = cli.Execute(ctx, os.Stdin, os.Stdout, os.Stderr, os.Args[1:])
}
