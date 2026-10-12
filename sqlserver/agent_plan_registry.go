package sqlserver

import (
	"fmt"
	"strings"
	"sync"
)

// agentPlanRegistry catches forks in step chains at plan time. Each step's CustomizeDiff records the claims in
// its config: "in job J, the step after P is me" and "I handle the success/failure of step S". SQL Server jobs
// can't fork, so two steps making the same claim is an error. Claims come from config rather than the current
// order in msdb, so inserting a step in the middle never causes a false error.
//
// The registry lives on the configured provider, so it holds the claims of one Terraform command: a plan plans
// every resource in one provider process, and an apply re-plans the changed resources in a new one.
type agentPlanRegistry struct {
	mu     sync.Mutex
	claims map[string]string // claim key -> name of the step that made it
}

func newAgentPlanRegistry() *agentPlanRegistry {
	return &agentPlanRegistry{claims: map[string]string{}}
}

// claimFollows records that step follows previous in the job; previous is "" for the first step.
func (r *agentPlanRegistry) claimFollows(jobID, jobName, previous, step string) error {
	key := fmt.Sprintf("%s/after/%s", strings.ToUpper(jobID), previous)
	if other := r.claim(key, step); other != "" {
		if previous == "" {
			return fmt.Errorf("steps %q and %q are both the first step of job %s; SQL Server jobs can't fork, so only one step can set job_id and the others must set previous_step_id", other, step, jobName)
		}
		return fmt.Errorf("steps %q and %q both follow %q in job %s; SQL Server jobs can't fork, so each step needs a different previous_step_id", other, step, previous, jobName)
	}
	return nil
}

// claimHandles records that step is the target of source's jump for the outcome (handles_success_of / handles_failure_of).
func (r *agentPlanRegistry) claimHandles(jobID, jobName string, outcome agentOutcome, source, step string) error {
	key := fmt.Sprintf("%s/handles/%s/%s", strings.ToUpper(jobID), outcome, source)
	if other := r.claim(key, step); other != "" {
		return fmt.Errorf("steps %q and %q both handle the %s of step %q in job %s; a step can only jump to one step per outcome", other, step, outcome, source, jobName)
	}
	return nil
}

// claim records key for owner and returns the other owner when a different step already holds it.
func (r *agentPlanRegistry) claim(key, owner string) string {
	r.mu.Lock()
	defer r.mu.Unlock()
	if other, ok := r.claims[key]; ok && other != owner {
		return other
	}
	r.claims[key] = owner
	return ""
}
