package sql

import (
	"context"
	"errors"
	"testing"

	mssql "github.com/microsoft/go-mssqldb"
)

func TestRetryOnDeadlock(t *testing.T) {
	deadlock := mssql.Error{Number: errDeadlockVictim, Message: "deadlock victim"}
	other := mssql.Error{Number: 2627, Message: "duplicate key"}

	tests := []struct {
		name      string
		errs      []error
		wantCalls int
		wantErr   error
	}{
		{"success", []error{nil}, 1, nil},
		{"other error is not retried", []error{other}, 1, other},
		{"deadlock then success", []error{deadlock, deadlock, nil}, 3, nil},
		{"gives up after the last attempt", []error{deadlock, deadlock, deadlock, deadlock, deadlock, nil}, agentDeadlockAttempts, deadlock},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			calls := 0
			err := retryOnDeadlock(context.Background(), func() error {
				calls++
				return tt.errs[calls-1]
			})
			if calls != tt.wantCalls {
				t.Errorf("calls = %d, want %d", calls, tt.wantCalls)
			}
			if !errors.Is(err, tt.wantErr) && err != tt.wantErr {
				t.Errorf("err = %v, want %v", err, tt.wantErr)
			}
		})
	}
}
