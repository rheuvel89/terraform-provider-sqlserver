package sqlserver

import (
	"strings"
	"sync"
)

// agentJobLocks serialises the step changes of one job within this provider process. Step numbers shift on
// every insert and delete, so a step's number is only valid while the lock is held. Steps of different jobs
// still change in parallel. Other processes (another apply, SSMS) aren't covered.
var agentJobLocks sync.Map

// lockAgentJob locks the job and returns the function that unlocks it.
func lockAgentJob(jobID string) func() {
	mu, _ := agentJobLocks.LoadOrStore(strings.ToUpper(jobID), &sync.Mutex{})
	mu.(*sync.Mutex).Lock()
	return mu.(*sync.Mutex).Unlock
}
