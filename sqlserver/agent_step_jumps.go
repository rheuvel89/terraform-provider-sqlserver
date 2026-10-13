package sqlserver

import "terraform-provider-sqlserver/sqlserver/model"

// msdb step actions (on_success_action / on_fail_action).
const (
	agentActionQuitWithSuccess = 1
	agentActionQuitWithFailure = 2
	agentActionGoToNextStep    = 3
	agentActionGoToStep        = 4
)

var agentStepActions = map[string]int{
	"quit_with_success": agentActionQuitWithSuccess,
	"quit_with_failure": agentActionQuitWithFailure,
	"go_to_next_step":   agentActionGoToNextStep,
	"go_to_step":        agentActionGoToStep,
}

// agentOutcome is the outcome of a step that a jump belongs to.
type agentOutcome string

const (
	agentOutcomeSuccess agentOutcome = "success"
	agentOutcomeFailure agentOutcome = "failure"
)

// defaultAction is the action a step falls back to when its jump target goes away.
func (o agentOutcome) defaultAction() int {
	if o == agentOutcomeSuccess {
		return agentActionGoToNextStep
	}
	return agentActionQuitWithFailure
}

// action returns the step's action and target step number for this outcome.
func (o agentOutcome) action(s model.AgentJobStep) (int, int) {
	if o == agentOutcomeSuccess {
		return s.OnSuccessAction, s.OnSuccessStepID
	}
	return s.OnFailAction, s.OnFailStepID
}

// params returns the sp_update_jobstep parameters that set this outcome's action.
func (o agentOutcome) params(action, target int) map[string]interface{} {
	if o == agentOutcomeSuccess {
		return map[string]interface{}{"on_success_action": action, "on_success_step_id": target}
	}
	return map[string]interface{}{"on_fail_action": action, "on_fail_step_id": target}
}

var agentOutcomes = []agentOutcome{agentOutcomeSuccess, agentOutcomeFailure}

// agentJump is a go-to-step jump, by step name so that it survives renumbering.
type agentJump struct {
	Source  string
	Outcome agentOutcome
	Target  string
}

// agentJumpUpdate is the action a step needs for one outcome to restore a jump.
type agentJumpUpdate struct {
	StepID  int
	Outcome agentOutcome
	Action  int
	Target  int
}

func findAgentStep(steps []model.AgentJobStep, name string) *model.AgentJobStep {
	for i := range steps {
		if steps[i].Name == name {
			return &steps[i]
		}
	}
	return nil
}

func findAgentStepByID(steps []model.AgentJobStep, stepID int) *model.AgentJobStep {
	for i := range steps {
		if steps[i].StepID == stepID {
			return &steps[i]
		}
	}
	return nil
}

// snapshotAgentJumps records every jump in the job by name, before an insert, delete or move renumbers the steps.
func snapshotAgentJumps(steps []model.AgentJobStep) []agentJump {
	var jumps []agentJump
	for _, s := range steps {
		for _, o := range agentOutcomes {
			action, target := o.action(s)
			if action != agentActionGoToStep {
				continue
			}
			if t := findAgentStepByID(steps, target); t != nil {
				jumps = append(jumps, agentJump{Source: s.Name, Outcome: o, Target: t.Name})
			}
		}
	}
	return jumps
}

// rewriteAgentJumps compares the snapshot with the steps after the change and returns the updates that restore
// every jump. A jump whose target is gone falls back to the outcome's default action. Jumps from skipSource are
// left alone, because that step's own jumps come from its config. This is needed because sp_delete_jobstep
// resets jumps to the step after the deleted one (spike result S1).
func rewriteAgentJumps(snapshot []agentJump, steps []model.AgentJobStep, skipSource string) []agentJumpUpdate {
	var updates []agentJumpUpdate
	for _, j := range snapshot {
		if j.Source == skipSource {
			continue
		}
		source := findAgentStep(steps, j.Source)
		if source == nil {
			continue
		}
		wantAction, wantTarget := j.Outcome.defaultAction(), 0
		if target := findAgentStep(steps, j.Target); target != nil && target.StepID != source.StepID {
			wantAction, wantTarget = agentActionGoToStep, target.StepID
		}
		action, target := j.Outcome.action(*source)
		if action != wantAction || (wantAction == agentActionGoToStep && target != wantTarget) {
			updates = append(updates, agentJumpUpdate{StepID: source.StepID, Outcome: j.Outcome, Action: wantAction, Target: wantTarget})
		}
	}
	return updates
}

// agentJumpDirection tells whether a step's jump for an outcome goes to an earlier step (backward, owned by the
// source) or a later step (forward, owned by the target). It returns the target step and whether it's backward.
func agentJumpDirection(steps []model.AgentJobStep, s model.AgentJobStep, o agentOutcome) (target *model.AgentJobStep, backward bool) {
	action, targetID := o.action(s)
	if action != agentActionGoToStep {
		return nil, false
	}
	target = findAgentStepByID(steps, targetID)
	return target, target != nil && target.StepID < s.StepID
}
