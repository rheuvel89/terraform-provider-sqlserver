package sql

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestWithRetry(t *testing.T) {
	retryable := errors.New("retryable")
	fatal := errors.New("fatal")
	noDelay := func(int) time.Duration { return 0 }
	isRetryable := func(err error) bool { return errors.Is(err, retryable) }

	tests := []struct {
		name      string
		errs      []error
		wantCalls int
		wantErr   error
	}{
		{"success", []error{nil}, 1, nil},
		{"non-retryable error is returned at once", []error{fatal}, 1, fatal},
		{"retryable errors, then success", []error{retryable, retryable, nil}, 3, nil},
		{"retryable, then non-retryable", []error{retryable, fatal}, 2, fatal},
		{"gives up after the last attempt", []error{retryable, retryable, retryable, nil}, 3, retryable},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			calls := 0
			err := withRetry(context.Background(), 3, noDelay, isRetryable, func(context.Context) error {
				calls++
				return tt.errs[calls-1]
			})
			if calls != tt.wantCalls || !errors.Is(err, tt.wantErr) || (tt.wantErr == nil && err != nil) {
				t.Fatalf("got %d calls and %v, want %d calls and %v", calls, err, tt.wantCalls, tt.wantErr)
			}
		})
	}

	t.Run("cancelled while waiting", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		calls := 0
		err := withRetry(ctx, 5, func(int) time.Duration { return time.Hour }, isRetryable, func(context.Context) error {
			calls++
			cancel()
			return retryable
		})
		if calls != 1 || !errors.Is(err, context.Canceled) {
			t.Fatalf("got %d calls and %v, want 1 call and context.Canceled", calls, err)
		}
	})

	t.Run("delay gets the attempt number", func(t *testing.T) {
		var attempts []int
		_ = withRetry(context.Background(), 3, func(a int) time.Duration { attempts = append(attempts, a); return 0 }, isRetryable,
			func(context.Context) error { return retryable })
		if len(attempts) != 2 || attempts[0] != 1 || attempts[1] != 2 {
			t.Fatalf("delay was called for attempts %v, want [1 2]", attempts)
		}
	})
}
