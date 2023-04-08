// Package retry implements retry functionality.
package retry

import (
	"context"
	"math/rand"
	"strings"
	"time"

	goretry "github.com/sethvargo/go-retry"
)

// Do implements retry using fibonacci backoff, up to a total
// execution time of maxDuration (use zero to indicate no max). The
// maximum interval between retries is 5 seconds.
//
// If len(matches) is zero, retry will be attempted until ctx is canceled
// or times out, or maxDuration is reached. If one or more MatchFuncs are
// supplied, retry is performed only if the error returned by fn is
// matched by one of the MatchFunc args.
//
// For simple string matching, use retry.Match("value") for convenience.
//
//	err = retry.Do(ctx, time.Second*10, dbConnect, retry.Match("connection refused"))
func Do(ctx context.Context, maxDuration time.Duration, fn func() error, matches ...MatchFunc) error {
	b := newFibonacci(time.Millisecond * 100)
	b = goretry.WithJitterPercent(10, b)

	if maxDuration != 0 {
		b = goretry.WithMaxDuration(maxDuration, b)
	}

	return do(ctx, fn, b, matches...)
}

// DoConstant is similar to Do, but uses a constant backoff instead of fibonacci.
func DoConstant(ctx context.Context, interval, maxDuration time.Duration, fn func() error, matches ...MatchFunc) error {
	b := goretry.NewConstant(interval)
	b = goretry.WithJitterPercent(10, b)

	if maxDuration != 0 {
		b = goretry.WithMaxDuration(maxDuration, b)
	}

	return do(ctx, fn, b, matches...)
}

func do(ctx context.Context, fn func() error, b goretry.Backoff, matches ...MatchFunc) error {
	return goretry.Do(ctx, b, func(_ context.Context) error {
		err := fn()
		if err == nil {
			return nil
		}

		if len(matches) == 0 {
			return goretry.RetryableError(err)
		}

		for _, matchFn := range matches {
			if matchFn(err) {
				return goretry.RetryableError(err)
			}
		}

		return err
	})
}

// MatchFunc returns true if err matches a condition for retry.
type MatchFunc func(err error) bool

// Match is a MatchFunc that tests if the retry func's error
// string contains s.
func Match(s string) MatchFunc {
	return func(err error) bool {
		if err == nil {
			return false
		}

		return strings.Contains(err.Error(), s)
	}
}

const maxFibBackoff = time.Second * 5

type fibonnacci struct {
	fib goretry.Backoff
}

func (f fibonnacci) Next() (time.Duration, bool) {
	d, b := f.fib.Next()
	if d > maxFibBackoff {
		d = maxFibBackoff
	}
	return d, b
}

// newFibonacci returns a backoff that limits the
// max duration of the backoff to 5seconds.
func newFibonacci(d time.Duration) goretry.Backoff {
	return fibonnacci{fib: goretry.NewFibonacci(d)}
}

// Jitter returns some amount of jitter between 5ms and 25ms.
func Jitter() time.Duration {
	r := rand.Intn(20) //nolint:gosec
	j := time.Millisecond*5 + time.Millisecond*time.Duration(r)
	return j
}

// SleepJitter sleeps for a jittery amount of time.
func SleepJitter() {
	time.Sleep(Jitter())
}
