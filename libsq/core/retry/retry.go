// Package retry provides a mechanism to execute a function
// with retry logic.
package retry

import (
	"context"
	"strings"
	"time"

	goretry "github.com/sethvargo/go-retry"
)

// Do implements retry using fibonacci backoff, up to a total
// execution time of maxDuration. The retry impl is mostly handled
// by the sethvargo/go-retry package. The matched args are tested
// against the error returned by fn; if the error message contains
// any of matches, retry can be performed.
//
//	err = retry.Do(ctx, time.Second*5, loadDataFn, "not found", "timeout")
func Do(ctx context.Context, maxDuration time.Duration, fn func() error, matches ...string) error {
	b := goretry.NewFibonacci(time.Millisecond * 100)
	b = goretry.WithJitterPercent(10, b)
	b = goretry.WithMaxDuration(maxDuration, b)

	return goretry.Do(ctx, b, func(_ context.Context) error {
		err := fn()
		if err == nil {
			return nil
		}

		errStr := err.Error()
		for _, match := range matches {
			if strings.Contains(errStr, match) {
				return goretry.RetryableError(err)
			}
		}

		return err
	})
}
