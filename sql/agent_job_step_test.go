package sql

import (
	"context"
	"strings"
	"testing"
)

func TestAgentJobStepRefusesStepZero(t *testing.T) {
	// The connector has no server: the guard must fail before any connection is made.
	c := &Connector{}
	for _, stepID := range []int{0, -1} {
		if err := c.DeleteAgentJobStep(context.Background(), "job", stepID); err == nil || !strings.Contains(err.Error(), "refusing to delete") {
			t.Errorf("DeleteAgentJobStep(%d): expected a refusal, got %v", stepID, err)
		}
		if err := c.UpdateAgentJobStep(context.Background(), "job", stepID, map[string]interface{}{"command": "SELECT 1"}); err == nil || !strings.Contains(err.Error(), "refusing to update") {
			t.Errorf("UpdateAgentJobStep(%d): expected a refusal, got %v", stepID, err)
		}
	}
	if err := c.UpdateAgentJobStep(context.Background(), "job", 1, map[string]interface{}{"subsystem": "CmdExec"}); err == nil || !strings.Contains(err.Error(), "not supported") {
		t.Errorf("UpdateAgentJobStep: expected unsupported parameters to be rejected, got %v", err)
	}
}
