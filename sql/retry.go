package sql

import (
	"context"
	"time"
)

// withRetry runs op until it succeeds, fails with an error that retryable rejects, or has run maxAttempts times,
// and returns the last error. delay gives the wait after the given (1-based) attempt. When ctx is cancelled while
// waiting, it returns ctx.Err().
func withRetry(ctx context.Context, maxAttempts int, delay func(attempt int) time.Duration, retryable func(error) bool, op func(context.Context) error) error {
	var err error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		if err = op(ctx); err == nil || !retryable(err) || attempt == maxAttempts {
			return err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay(attempt)):
		}
	}
	return err
}
