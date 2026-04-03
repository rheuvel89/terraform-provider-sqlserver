package sql

import (
	"errors"
	"testing"
)

func TestIsIgnorableInactiveProcessIDError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "matches SQL Server inactive process ID error",
			err:  errors.New("mssql: Process ID 116 is not an active process ID."),
			want: true,
		},
		{
			name: "nil error is not ignorable",
			err:  nil,
			want: false,
		},
		{
			name: "other error is not ignorable",
			err:  errors.New("mssql: login failed"),
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isIgnorableInactiveProcessIDError(tt.err)
			if got != tt.want {
				t.Fatalf("isIgnorableInactiveProcessIDError() = %v, want %v", got, tt.want)
			}
		})
	}
}
