package sqlserver

import (
	"fmt"
	"reflect"
	"strings"
	"sync"
	"terraform-provider-sqlserver/sqlserver/model"
	"testing"
)

// step builds a step whose actions default to go to next step / quit with failure.
func step(id int, name string, opts ...func(*model.AgentJobStep)) model.AgentJobStep {
	s := model.AgentJobStep{StepID: id, Name: name, OnSuccessAction: agentActionGoToNextStep, OnFailAction: agentActionQuitWithFailure}
	for _, o := range opts {
		o(&s)
	}
	return s
}

func onSuccess(action, target int) func(*model.AgentJobStep) {
	return func(s *model.AgentJobStep) { s.OnSuccessAction, s.OnSuccessStepID = action, target }
}

func onFail(action, target int) func(*model.AgentJobStep) {
	return func(s *model.AgentJobStep) { s.OnFailAction, s.OnFailStepID = action, target }
}

func TestAgentJumpSnapshot(t *testing.T) {
	steps := []model.AgentJobStep{
		step(1, "A", onSuccess(agentActionGoToStep, 3)),
		step(2, "B"),
		step(3, "C", onFail(agentActionGoToStep, 1)),
		step(4, "D", onFail(agentActionGoToStep, 9)), // dangling target: ignored
	}
	want := []agentJump{
		{Source: "A", Outcome: agentOutcomeSuccess, Target: "C"},
		{Source: "C", Outcome: agentOutcomeFailure, Target: "A"},
	}
	if got := snapshotAgentJumps(steps); !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func TestAgentJumpRewrite(t *testing.T) {
	before := []model.AgentJobStep{
		step(1, "A", onSuccess(agentActionGoToStep, 3), onFail(agentActionGoToStep, 3)),
		step(2, "B"),
		step(3, "C"),
		step(4, "D", onFail(agentActionGoToStep, 2)),
	}
	snapshot := snapshotAgentJumps(before)

	tests := []struct {
		name  string
		after []model.AgentJobStep
		skip  string
		want  []agentJumpUpdate
	}{
		{
			// What sp_delete_jobstep really does when B (2) is deleted (spike result S1): A's jumps to C, the step
			// right after B, are reset to quit, and D's jump to B is reset too.
			name: "delete B, msdb bug",
			after: []model.AgentJobStep{
				step(1, "A", onSuccess(agentActionQuitWithSuccess, 0), onFail(agentActionQuitWithFailure, 0)),
				step(2, "C"),
				step(3, "D", onFail(agentActionQuitWithFailure, 0)),
			},
			want: []agentJumpUpdate{
				{StepID: 1, Outcome: agentOutcomeSuccess, Action: agentActionGoToStep, Target: 2},
				{StepID: 1, Outcome: agentOutcomeFailure, Action: agentActionGoToStep, Target: 2},
			},
		},
		{
			name: "delete C, the target",
			after: []model.AgentJobStep{
				step(1, "A", onSuccess(agentActionQuitWithSuccess, 0), onFail(agentActionQuitWithFailure, 0)),
				step(2, "B"),
				step(3, "D", onFail(agentActionGoToStep, 2)),
			},
			want: []agentJumpUpdate{
				// Falls back to the provider's default for success, not msdb's quit with success.
				{StepID: 1, Outcome: agentOutcomeSuccess, Action: agentActionGoToNextStep, Target: 0},
			},
		},
		{
			// sp_add_jobstep renumbers correctly, so an insert needs no updates.
			name: "insert X at 2",
			after: []model.AgentJobStep{
				step(1, "A", onSuccess(agentActionGoToStep, 4), onFail(agentActionGoToStep, 4)),
				step(2, "X"),
				step(3, "B"),
				step(4, "C"),
				step(5, "D", onFail(agentActionGoToStep, 3)),
			},
		},
		{
			name: "jumps of the moved step itself are skipped",
			after: []model.AgentJobStep{
				step(1, "B"),
				step(2, "A"),
				step(3, "C"),
				step(4, "D", onFail(agentActionGoToStep, 1)),
			},
			skip: "A",
		},
		{
			name: "deleted source",
			after: []model.AgentJobStep{
				step(1, "B"),
				step(2, "C"),
				step(3, "D", onFail(agentActionGoToStep, 1)),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := rewriteAgentJumps(snapshot, tt.after, tt.skip); !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("got %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestAgentJumpDirection(t *testing.T) {
	steps := []model.AgentJobStep{
		step(1, "A", onSuccess(agentActionGoToStep, 3)),
		step(2, "B"),
		step(3, "C", onFail(agentActionGoToStep, 1)),
	}
	if target, backward := agentJumpDirection(steps, steps[0], agentOutcomeSuccess); backward || target.Name != "C" {
		t.Errorf("A -> C should be forward, got %v %v", target, backward)
	}
	if target, backward := agentJumpDirection(steps, steps[2], agentOutcomeFailure); !backward || target.Name != "A" {
		t.Errorf("C -> A should be backward, got %v %v", target, backward)
	}
	if target, _ := agentJumpDirection(steps, steps[1], agentOutcomeSuccess); target != nil {
		t.Errorf("B has no jump, got %v", target)
	}
}

func TestAgentJobStepID(t *testing.T) {
	meta := sqlserverProvider{host: "localhost", port: "1433"}
	id := getAgentJobStepID(meta, "8f1c2a2e-1111-4a4a-9b9b-0123456789ab", "load / transform")
	if !strings.HasPrefix(id, "sqlserver://localhost:1433/agent_job/8F1C2A2E-1111-4A4A-9B9B-0123456789AB/step/") {
		t.Fatalf("unexpected id %q", id)
	}
	jobID, name, err := parseAgentJobStepID(id)
	if err != nil || jobID != "8F1C2A2E-1111-4A4A-9B9B-0123456789AB" || name != "load / transform" {
		t.Fatalf("parse %q: got %q %q %v", id, jobID, name, err)
	}
	for _, bad := range []string{"", "sqlserver://h:1/agent_job/x", "sqlserver://h:1/agent_job/x/step/"} {
		if _, _, err := parseAgentJobStepID(bad); err == nil {
			t.Errorf("%q should be rejected", bad)
		}
	}
}

func TestAgentPlanRegistry(t *testing.T) {
	const job = "JOB"
	tests := []struct {
		name    string
		claims  [][2]string // previous, step
		wantErr string
	}{
		{"insert correctly", [][2]string{{"extract", "X"}, {"X", "transform"}}, ""},
		{"insert, forget the next step", [][2]string{{"extract", "X"}, {"extract", "transform"}}, `steps "X" and "transform" both follow "extract"`},
		{"swap two steps", [][2]string{{"extract", "load"}, {"load", "transform"}}, ""},
		{"two head steps", [][2]string{{"", "a"}, {"", "b"}}, `steps "a" and "b" are both the first step`},
		{"same step planned twice", [][2]string{{"extract", "X"}, {"extract", "X"}}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := newAgentPlanRegistry()
			var err error
			for _, c := range tt.claims {
				if err = r.claimFollows(job, job, c[0], c[1]); err != nil {
					break
				}
			}
			switch {
			case tt.wantErr == "" && err != nil:
				t.Fatalf("unexpected error: %v", err)
			case tt.wantErr != "" && (err == nil || !strings.Contains(err.Error(), tt.wantErr)):
				t.Fatalf("expected error containing %q, got %v", tt.wantErr, err)
			}
		})
	}

	r := newAgentPlanRegistry()
	if err := r.claimHandles(job, job, agentOutcomeFailure, "extract", "notify"); err != nil {
		t.Fatal(err)
	}
	if err := r.claimHandles(job, job, agentOutcomeSuccess, "extract", "cleanup"); err != nil {
		t.Fatalf("success and failure are separate claims: %v", err)
	}
	if err := r.claimHandles(job, job, agentOutcomeFailure, "extract", "alert"); err == nil || !strings.Contains(err.Error(), `steps "notify" and "alert" both handle the failure of step "extract"`) {
		t.Fatalf("expected a double claim error, got %v", err)
	}
	if err := r.claimHandles("OTHER", "OTHER", agentOutcomeFailure, "extract", "alert"); err != nil {
		t.Fatalf("claims of different jobs don't clash: %v", err)
	}
}

func TestAgentStepFlags(t *testing.T) {
	// The bits documented for sp_add_jobstep's @flags.
	want := map[string]int{
		"append_output_file":             2,
		"include_step_output_in_history": 4,
		"log_to_table":                   8,
		"append_log_to_table":            16,
	}
	if !reflect.DeepEqual(agentStepFlags, want) {
		t.Fatalf("got %v, want %v", agentStepFlags, want)
	}
	data := resourceAgentJobStep().TestResourceData()
	for prop := range want {
		if err := data.Set(prop, prop == "log_to_table" || prop == "include_step_output_in_history"); err != nil {
			t.Fatal(err)
		}
	}
	if got := agentStepFlagsFromData(data); got != 12 {
		t.Fatalf("log_to_table + include_step_output_in_history: got flags %d, want 12", got)
	}
}

func TestAgentPlanRegistryKeys(t *testing.T) {
	r := newAgentPlanRegistry()
	if err := r.claimFollows("8f1c2a2e-1111-4a4a-9b9b-0123456789ab", "job", "a", "b"); err != nil {
		t.Fatal(err)
	}
	// Job IDs are compared case-insensitively: msdb returns them in upper case, a config can hold either.
	if err := r.claimFollows("8F1C2A2E-1111-4A4A-9B9B-0123456789AB", "job", "a", "c"); err == nil {
		t.Fatal("the same job in another case should clash")
	}
	// The same previous step name in another job is a different claim.
	if err := r.claimFollows("00000000-0000-0000-0000-000000000000", "other", "a", "c"); err != nil {
		t.Fatalf("claims of different jobs don't clash: %v", err)
	}
	// A step can follow one step and handle the outcome of the same step: different kinds of claim.
	if err := r.claimHandles("8F1C2A2E-1111-4A4A-9B9B-0123456789AB", "job", agentOutcomeFailure, "a", "b"); err != nil {
		t.Fatalf("a follows claim and a handles claim don't clash: %v", err)
	}
	// A new registry, as each Terraform command gets one, starts empty.
	if err := newAgentPlanRegistry().claimFollows("8f1c2a2e-1111-4a4a-9b9b-0123456789ab", "job", "a", "c"); err != nil {
		t.Fatalf("a new registry should hold no claims: %v", err)
	}
}

func TestAgentPlanRegistryConcurrent(t *testing.T) {
	// Terraform plans resources in parallel. Run with -race to check the locking; whatever the order, exactly one of
	// the steps claiming the same previous step wins and every other one gets an error.
	r := newAgentPlanRegistry()
	const claimants = 50
	errs := make(chan error, claimants)
	var wg sync.WaitGroup
	for i := 0; i < claimants; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			errs <- r.claimFollows("JOB", "job", "a", fmt.Sprintf("step%d", i))
		}(i)
	}
	wg.Wait()
	close(errs)
	failed := 0
	for err := range errs {
		if err != nil {
			failed++
		}
	}
	if failed != claimants-1 {
		t.Fatalf("%d of %d claims failed, want %d", failed, claimants, claimants-1)
	}
}
