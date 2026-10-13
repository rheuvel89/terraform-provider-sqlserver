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
		wantErr   int32 // mssql error number, 0 for no error
	}{
		{"success", []error{nil}, 1, 0},
		{"other error is not retried", []error{other}, 1, other.Number},
		{"deadlock then success", []error{deadlock, deadlock, nil}, 3, 0},
		{"gives up after the last attempt", []error{deadlock, deadlock, deadlock, deadlock, deadlock, nil}, agentDeadlockAttempts, errDeadlockVictim},
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
			var got int32
			var sqlErr mssql.Error
			if errors.As(err, &sqlErr) {
				got = sqlErr.Number
			} else if err != nil {
				t.Fatalf("unexpected error type %T", err)
			}
			if got != tt.wantErr {
				t.Errorf("err = %v, want error number %d", err, tt.wantErr)
			}
		})
	}
}
