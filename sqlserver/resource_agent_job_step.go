package sqlserver

import (
	"context"
	"fmt"
	"net/url"
	"sort"
	"strings"
	"terraform-provider-sqlserver/sqlserver/model"

	"github.com/hashicorp/go-cty/cty"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
	"github.com/pkg/errors"
)

type AgentJobStepConnector interface {
	EnsureAgentAvailable(ctx context.Context) error
	ListAgentJobSteps(ctx context.Context, jobID string) ([]model.AgentJobStep, error)
	AddAgentJobStep(ctx context.Context, s *model.AgentJobStep) error
	UpdateAgentJobStep(ctx context.Context, jobID string, stepID int, changes map[string]interface{}) error
	DeleteAgentJobStep(ctx context.Context, jobID string, stepID int) error
	GetAgentRunAsInfo(ctx context.Context, jobID string) (callerIsSysadmin bool, owner string, ownerIsSysadmin bool, err error)
}

// agentStepOutcomeProps are the attributes that describe each outcome of a step.
var agentStepOutcomeProps = map[agentOutcome]struct{ action, stepName, handles string }{
	agentOutcomeSuccess: {agentOnSuccessActionProp, agentOnSuccessStepNameProp, agentHandlesSuccessOfProp},
	agentOutcomeFailure: {agentOnFailActionProp, agentOnFailStepNameProp, agentHandlesFailureOfProp},
}

func resourceAgentJobStep() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceAgentJobStepCreate,
		ReadContext:   resourceAgentJobStepRead,
		UpdateContext: resourceAgentJobStepUpdate,
		DeleteContext: resourceAgentJobStepDelete,
		CustomizeDiff: resourceAgentJobStepCustomizeDiff,
		Schema: map[string]*schema.Schema{
			agentJobIdProp: {
				Type:             schema.TypeString,
				Optional:         true,
				Computed:         true,
				ExactlyOneOf:     []string{agentJobIdProp, agentPreviousStepIdProp},
				Description:      "The job_id of the job. Set it on the first step of the job only; every other step sets previous_step_id.",
				ValidateDiagFunc: validation.ToDiagFunc(validation.IsUUID),
			},
			agentPreviousStepIdProp: {
				Type:             schema.TypeString,
				Optional:         true,
				Description:      "The id of the step that runs before this one. Changing it moves the step.",
				ValidateDiagFunc: validateAgentJobStepID,
			},
			agentNameProp: {
				Type:             schema.TypeString,
				Required:         true,
				ForceNew:         true,
				Description:      "The name of the step, unique within the job.",
				ValidateDiagFunc: validation.ToDiagFunc(validation.StringIsNotWhiteSpace),
			},
			agentCommandProp: {
				Type:        schema.TypeString,
				Required:    true,
				Description: "The T-SQL the step runs.",
				DiffSuppressFunc: func(_, old, new string, _ *schema.ResourceData) bool {
					// sp_update_jobstep trims the command.
					return strings.TrimSpace(old) == strings.TrimSpace(new)
				},
			},
			agentDatabaseNameProp: {
				Type:        schema.TypeString,
				Optional:    true,
				Default:     agentDefaultStepDatabase,
				Description: "The database the step runs in.",
			},
			agentDatabaseUserNameProp: {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "The database user the step runs as. Only honoured when the job owner is sysadmin, and only a sysadmin can set it.",
			},
			agentOnSuccessActionProp: {
				Type:             schema.TypeString,
				Optional:         true,
				Default:          "go_to_next_step",
				Description:      "What to do when the step succeeds: quit_with_success, quit_with_failure, go_to_next_step or go_to_step.",
				ValidateDiagFunc: enumValidation(agentStepActions),
			},
			agentOnSuccessStepNameProp: {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "The earlier step to go to on success. Required when on_success_action is go_to_step. To jump to a later step, set handles_success_of on that step.",
			},
			agentOnFailActionProp: {
				Type:             schema.TypeString,
				Optional:         true,
				Default:          "quit_with_failure",
				Description:      "What to do when the step fails: quit_with_success, quit_with_failure, go_to_next_step or go_to_step.",
				ValidateDiagFunc: enumValidation(agentStepActions),
			},
			agentOnFailStepNameProp: {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "The earlier step to go to on failure. Required when on_fail_action is go_to_step. To jump to a later step, set handles_failure_of on that step.",
			},
			agentHandlesSuccessOfProp: {
				Type:        schema.TypeSet,
				Optional:    true,
				Elem:        &schema.Schema{Type: schema.TypeString},
				Description: "Names of earlier steps that go to this step when they succeed.",
			},
			agentHandlesFailureOfProp: {
				Type:        schema.TypeSet,
				Optional:    true,
				Elem:        &schema.Schema{Type: schema.TypeString},
				Description: "Names of earlier steps that go to this step when they fail.",
			},
			agentRetryAttemptsProp: {
				Type:             schema.TypeInt,
				Optional:         true,
				Default:          0,
				Description:      "How many times to retry the step when it fails.",
				ValidateDiagFunc: validation.ToDiagFunc(validation.IntAtLeast(0)),
			},
			agentRetryIntervalProp: {
				Type:             schema.TypeInt,
				Optional:         true,
				Default:          0,
				Description:      "Minutes to wait between retries.",
				ValidateDiagFunc: validation.ToDiagFunc(validation.IntAtLeast(0)),
			},
			agentOutputFileNameProp: {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "A file on the server to write the step output to. Not supported on Azure SQL Managed Instance.",
			},
			agentAppendOutputFileProp: {
				Type:        schema.TypeBool,
				Optional:    true,
				Default:     false,
				Description: "Append to output_file_name instead of overwriting it.",
			},
			agentLogToTableProp: {
				Type:        schema.TypeBool,
				Optional:    true,
				Default:     false,
				Description: "Write the step output to msdb.dbo.sysjobstepslogs, overwriting the previous entry.",
			},
			agentAppendLogToTableProp: {
				Type:        schema.TypeBool,
				Optional:    true,
				Default:     false,
				Description: "Write the step output to msdb.dbo.sysjobstepslogs, appending to the previous entry.",
			},
			agentStepOutputHistoryProp: {
				Type:        schema.TypeBool,
				Optional:    true,
				Default:     false,
				Description: "Include the step output in the job history.",
			},
			agentStepIdProp: {
				Type:        schema.TypeInt,
				Computed:    true,
				Description: "The step's current position in the job.",
			},
			agentStepUidProp: {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "The GUID of the step. It changes when the step moves.",
			},
		},
		Timeouts: &schema.ResourceTimeout{
			Default: defaultTimeout,
			Read:    defaultTimeout,
			Create:  defaultTimeout,
			Update:  defaultTimeout,
			Delete:  defaultTimeout,
		},
	}
}

func getAgentJobStepID(meta interface{}, jobID, name string) string {
	provider := meta.(sqlserverProvider)
	return fmt.Sprintf("sqlserver://%s:%s/agent_job/%s/step/%s", provider.host, provider.port, strings.ToUpper(jobID), url.PathEscape(name))
}

// parseAgentJobStepID returns the job GUID and step name in a step id.
func parseAgentJobStepID(id string) (jobID, name string, err error) {
	const jobMarker, stepMarker = "/agent_job/", "/step/"
	j := strings.Index(id, jobMarker)
	s := strings.Index(id, stepMarker)
	if j < 0 || s < j {
		return "", "", fmt.Errorf("%q is not the id of a sqlserver_agent_job_step", id)
	}
	jobID = id[j+len(jobMarker) : s]
	if name, err = url.PathUnescape(id[s+len(stepMarker):]); err != nil || name == "" {
		return "", "", fmt.Errorf("%q is not the id of a sqlserver_agent_job_step", id)
	}
	return strings.ToUpper(jobID), name, nil
}

func validateAgentJobStepID(i interface{}, path cty.Path) diag.Diagnostics {
	if _, _, err := parseAgentJobStepID(i.(string)); err != nil {
		return diag.Diagnostics{{Severity: diag.Error, Summary: err.Error(), AttributePath: path}}
	}
	return nil
}

func resourceAgentJobStepCustomizeDiff(_ context.Context, diff *schema.ResourceDiff, meta interface{}) error {
	name := diff.Get(agentNameProp).(string)

	for o, props := range agentStepOutcomeProps {
		if !diff.NewValueKnown(props.action) || !diff.NewValueKnown(props.stepName) {
			continue
		}
		action := diff.Get(props.action).(string)
		target := diff.Get(props.stepName).(string)
		switch {
		case action == "go_to_step" && target == "":
			return fmt.Errorf("%s is required when %s is \"go_to_step\"", props.stepName, props.action)
		case action != "go_to_step" && target != "":
			return fmt.Errorf("%s can only be set when %s is \"go_to_step\"", props.stepName, props.action)
		case target != "" && target == name:
			return fmt.Errorf("step %q can't go to itself on %s", name, o)
		}
		if diff.NewValueKnown(props.handles) && diff.Get(props.handles).(*schema.Set).Contains(name) {
			return fmt.Errorf("step %q can't handle its own %s", name, o)
		}
	}

	jobID, previous, known, fromConfig := agentStepPlacement(diff)
	if diff.Id() != "" {
		oldJobID, _ := diff.GetChange(agentJobIdProp)
		switch {
		case fromConfig && !known:
			// A new job: the step can only be created in it.
			if err := diff.ForceNew(agentJobIdProp); err != nil {
				return err
			}
		case known && !strings.EqualFold(oldJobID.(string), jobID):
			if !fromConfig {
				if err := diff.SetNew(agentJobIdProp, jobID); err != nil {
					return err
				}
			}
			if err := diff.ForceNew(agentJobIdProp); err != nil {
				return err
			}
		}
	} else if known && !fromConfig {
		if err := diff.SetNew(agentJobIdProp, jobID); err != nil {
			return err
		}
	}

	provider, ok := meta.(sqlserverProvider)
	if !ok || provider.agentRegistry == nil || !known {
		return nil
	}
	if previous != nil {
		if err := provider.agentRegistry.claimFollows(jobID, jobID, *previous, name); err != nil {
			return err
		}
	}
	for o, props := range agentStepOutcomeProps {
		if !diff.NewValueKnown(props.handles) {
			continue
		}
		for _, source := range diff.Get(props.handles).(*schema.Set).List() {
			if err := provider.agentRegistry.claimHandles(jobID, jobID, o, source.(string), name); err != nil {
				return err
			}
		}
	}
	return nil
}

// agentStepPlacement returns the job of the step and the step it follows, from job_id (first step) or
// previous_step_id. previous is nil when it isn't known yet, and "" for the first step. known is false when the
// job isn't known yet; fromConfig is true when job_id is set in the config.
func agentStepPlacement(diff *schema.ResourceDiff) (jobID string, previous *string, known, fromConfig bool) {
	raw := diff.GetRawConfig()
	if !raw.IsNull() && raw.IsKnown() {
		if v := raw.GetAttr(agentJobIdProp); !v.IsNull() {
			if !v.IsKnown() {
				return "", nil, false, true
			}
			head := ""
			return strings.ToUpper(v.AsString()), &head, true, true
		}
	}
	if !diff.NewValueKnown(agentPreviousStepIdProp) {
		return "", nil, false, false
	}
	jobID, prev, err := parseAgentJobStepID(diff.Get(agentPreviousStepIdProp).(string))
	if err != nil {
		return "", nil, false, false
	}
	return jobID, &prev, true, false
}

// agentStepTarget returns the job of the step and the name of the step it follows ("" for the first step), from the config.
func agentStepTarget(data *schema.ResourceData) (jobID, previous string, err error) {
	if prev := data.Get(agentPreviousStepIdProp).(string); prev != "" {
		return parseAgentJobStepID(prev)
	}
	return strings.ToUpper(data.Get(agentJobIdProp).(string)), "", nil
}

func agentStepFlagsFromData(data *schema.ResourceData) int {
	flags := 0
	for prop, bit := range agentStepFlags {
		if data.Get(prop).(bool) {
			flags |= bit
		}
	}
	return flags
}

// resolveAgentStepAction returns the msdb action and target step number of one outcome of the step at position.
// A go_to_step target must be an earlier step; forward jumps are declared on the target.
func resolveAgentStepAction(data *schema.ResourceData, o agentOutcome, steps []model.AgentJobStep, position int) (int, int, error) {
	props := agentStepOutcomeProps[o]
	action := agentStepActions[data.Get(props.action).(string)]
	if action != agentActionGoToStep {
		return action, 0, nil
	}
	name := data.Get(agentNameProp).(string)
	targetName := data.Get(props.stepName).(string)
	target := findAgentStep(steps, targetName)
	if target == nil {
		return 0, 0, fmt.Errorf("%s: job has no step %q", props.stepName, targetName)
	}
	if target.StepID >= position {
		return 0, 0, fmt.Errorf("%s: step %q comes after %q; %s can only go to an earlier step. To go to a later step, add %q to %s on step %q instead",
			props.stepName, targetName, name, props.stepName, name, props.handles, targetName)
	}
	return action, target.StepID, nil
}

// checkAgentStepRunAs makes sure database_user_name will be honoured: SQL Server silently ignores it when the job
// owner isn't sysadmin, and sp_update_jobstep silently drops it when the caller isn't (spike result S3b).
func checkAgentStepRunAs(ctx context.Context, connector AgentJobStepConnector, data *schema.ResourceData, jobID string) error {
	user := data.Get(agentDatabaseUserNameProp).(string)
	if user == "" {
		return nil
	}
	callerIsSysadmin, owner, ownerIsSysadmin, err := connector.GetAgentRunAsInfo(ctx, jobID)
	if err != nil {
		return errors.Wrap(err, "unable to check the job owner for database_user_name")
	}
	name := data.Get(agentNameProp).(string)
	if !callerIsSysadmin {
		return fmt.Errorf("step %q sets database_user_name, but the login the provider connects with isn't sysadmin; only a sysadmin can set database_user_name", name)
	}
	if !ownerIsSysadmin {
		return fmt.Errorf("step %q sets database_user_name, but its job is owned by %q, which isn't sysadmin; SQL Server only honours database_user_name for jobs owned by a sysadmin. Remove database_user_name or make the owner sysadmin", name, owner)
	}
	return nil
}

func applyAgentJumpUpdates(ctx context.Context, connector AgentJobStepConnector, jobID string, updates []agentJumpUpdate) error {
	for _, u := range updates {
		if err := connector.UpdateAgentJobStep(ctx, jobID, u.StepID, u.Outcome.params(u.Action, u.Target)); err != nil {
			return errors.Wrapf(err, "unable to restore the %s jump of step %d", u.Outcome, u.StepID)
		}
	}
	return nil
}

// applyAgentForwardJumps points the success or failure action of every step in handles_*_of at this step, and
// resets the steps that were removed from the list if they still point here.
func applyAgentForwardJumps(ctx context.Context, connector AgentJobStepConnector, data *schema.ResourceData, jobID string, steps []model.AgentJobStep) error {
	name := data.Get(agentNameProp).(string)
	me := findAgentStep(steps, name)
	if me == nil {
		return fmt.Errorf("step %q not found in job %s", name, jobID)
	}
	for _, o := range agentOutcomes {
		props := agentStepOutcomeProps[o]
		oldSet, newSet := data.GetChange(props.handles)
		for _, removed := range oldSet.(*schema.Set).Difference(newSet.(*schema.Set)).List() {
			source := findAgentStep(steps, removed.(string))
			if source == nil {
				continue
			}
			if action, target := o.action(*source); action == agentActionGoToStep && target == me.StepID {
				if err := connector.UpdateAgentJobStep(ctx, jobID, source.StepID, o.params(o.defaultAction(), 0)); err != nil {
					return errors.Wrapf(err, "unable to reset the %s action of step %q", o, source.Name)
				}
			}
		}
		for _, sourceName := range newSet.(*schema.Set).List() {
			source := findAgentStep(steps, sourceName.(string))
			if source == nil {
				return fmt.Errorf("%s: job has no step %q", props.handles, sourceName)
			}
			if source.StepID >= me.StepID {
				return fmt.Errorf("%s: step %q comes after %q; a step can only handle the %s of an earlier step. Set %s = \"go_to_step\" and %s = %q on step %q instead",
					props.handles, source.Name, name, o, props.action, props.stepName, name, source.Name)
			}
			action, target := o.action(*source)
			if action == agentActionGoToStep && target == me.StepID {
				continue
			}
			if action != o.defaultAction() {
				return fmt.Errorf("%s: step %q already has %s = %q; remove it from that step's config so that %q can handle its %s",
					props.handles, source.Name, props.action, enumName(agentStepActions, action), name, o)
			}
			if err := connector.UpdateAgentJobStep(ctx, jobID, source.StepID, o.params(agentActionGoToStep, me.StepID)); err != nil {
				return errors.Wrapf(err, "unable to point the %s action of step %q at %q", o, source.Name, name)
			}
		}
	}
	return nil
}

// insertAgentJobStep adds the step from the config at its place in the chain and restores every jump that the
// insert renumbered. The job lock must be held.
func insertAgentJobStep(ctx context.Context, connector AgentJobStepConnector, data *schema.ResourceData, jobID, previous string) error {
	name := data.Get(agentNameProp).(string)
	steps, err := connector.ListAgentJobSteps(ctx, jobID)
	if err != nil {
		return errors.Wrapf(err, "unable to list the steps of job %s", jobID)
	}
	if findAgentStep(steps, name) != nil {
		return fmt.Errorf("job %s already has a step called %q", jobID, name)
	}
	position := 1
	if previous != "" {
		prev := findAgentStep(steps, previous)
		if prev == nil {
			return fmt.Errorf("previous_step_id: job %s has no step %q", jobID, previous)
		}
		position = prev.StepID + 1
	}

	step := &model.AgentJobStep{
		JobID:            jobID,
		StepID:           position,
		Name:             name,
		Command:          data.Get(agentCommandProp).(string),
		DatabaseName:     data.Get(agentDatabaseNameProp).(string),
		DatabaseUserName: data.Get(agentDatabaseUserNameProp).(string),
		RetryAttempts:    data.Get(agentRetryAttemptsProp).(int),
		RetryInterval:    data.Get(agentRetryIntervalProp).(int),
		OutputFileName:   data.Get(agentOutputFileNameProp).(string),
		Flags:            agentStepFlagsFromData(data),
	}
	if step.OnSuccessAction, step.OnSuccessStepID, err = resolveAgentStepAction(data, agentOutcomeSuccess, steps, position); err != nil {
		return err
	}
	if step.OnFailAction, step.OnFailStepID, err = resolveAgentStepAction(data, agentOutcomeFailure, steps, position); err != nil {
		return err
	}

	snapshot := snapshotAgentJumps(steps)
	if err = connector.AddAgentJobStep(ctx, step); err != nil {
		return err
	}
	if steps, err = connector.ListAgentJobSteps(ctx, jobID); err != nil {
		return err
	}
	return applyAgentJumpUpdates(ctx, connector, jobID, rewriteAgentJumps(snapshot, steps, name))
}

// removeAgentJobStep deletes the step by name, if it exists, and restores the jumps that the delete broke. Jumps to
// the deleted step fall back to their default action. The job lock must be held.
func removeAgentJobStep(ctx context.Context, connector AgentJobStepConnector, jobID, name string) error {
	steps, err := connector.ListAgentJobSteps(ctx, jobID)
	if err != nil {
		return errors.Wrapf(err, "unable to list the steps of job %s", jobID)
	}
	me := findAgentStep(steps, name)
	if me == nil {
		return nil
	}
	snapshot := snapshotAgentJumps(steps)
	if err = connector.DeleteAgentJobStep(ctx, jobID, me.StepID); err != nil {
		return err
	}
	if steps, err = connector.ListAgentJobSteps(ctx, jobID); err != nil {
		return err
	}
	return applyAgentJumpUpdates(ctx, connector, jobID, rewriteAgentJumps(snapshot, steps, ""))
}

func resourceAgentJobStepCreate(ctx context.Context, data *schema.ResourceData, meta interface{}) diag.Diagnostics {
	logger := loggerFromMeta(meta, "agent_job_step", "create")
	name := data.Get(agentNameProp).(string)
	logger.Debug().Msgf("Create agent job step [%s]", name)

	connector, err := getAgentJobStepConnector(meta, data)
	if err != nil {
		return diag.FromErr(err)
	}
	if err = connector.EnsureAgentAvailable(ctx); err != nil {
		return diag.FromErr(err)
	}

	jobID, previous, err := agentStepTarget(data)
	if err != nil {
		return diag.FromErr(err)
	}

	unlock := lockAgentJob(jobID)
	defer unlock()

	if err = checkAgentStepRunAs(ctx, connector, data, jobID); err != nil {
		return diag.FromErr(err)
	}
	if err = insertAgentJobStep(ctx, connector, data, jobID, previous); err != nil {
		return diag.FromErr(errors.Wrapf(err, "unable to create agent job step [%s]", name))
	}
	steps, err := connector.ListAgentJobSteps(ctx, jobID)
	if err != nil {
		return diag.FromErr(err)
	}
	if err = applyAgentForwardJumps(ctx, connector, data, jobID, steps); err != nil {
		return diag.FromErr(err)
	}

	logger.Info().Msgf("created agent job step [%s] in job [%s]", name, jobID)

	if err = data.Set(agentJobIdProp, jobID); err != nil {
		return diag.FromErr(err)
	}
	data.SetId(getAgentJobStepID(meta, jobID, name))

	return resourceAgentJobStepRead(ctx, data, meta)
}

func resourceAgentJobStepRead(ctx context.Context, data *schema.ResourceData, meta interface{}) diag.Diagnostics {
	logger := loggerFromMeta(meta, "agent_job_step", "read")
	logger.Debug().Msgf("Read %s", data.Id())

	connector, err := getAgentJobStepConnector(meta, data)
	if err != nil {
		return diag.FromErr(err)
	}

	jobID, name, err := parseAgentJobStepID(data.Id())
	if err != nil {
		return diag.FromErr(err)
	}
	steps, err := connector.ListAgentJobSteps(ctx, jobID)
	if err != nil {
		return diag.FromErr(errors.Wrapf(err, "unable to read the steps of job %s", jobID))
	}
	me := findAgentStep(steps, name)
	if me == nil {
		logger.Info().Msgf("No step [%s] found in job [%s]", name, jobID)
		data.SetId("")
		return nil
	}

	previous := ""
	if prev := findAgentStepByID(steps, me.StepID-1); prev != nil {
		previous = getAgentJobStepID(meta, jobID, prev.Name)
	}
	values := map[string]interface{}{
		agentJobIdProp:            jobID,
		agentPreviousStepIdProp:   previous,
		agentStepIdProp:           me.StepID,
		agentStepUidProp:          me.StepUID,
		agentCommandProp:          me.Command,
		agentDatabaseNameProp:     me.DatabaseName,
		agentDatabaseUserNameProp: me.DatabaseUserName,
		agentRetryAttemptsProp:    me.RetryAttempts,
		agentRetryIntervalProp:    me.RetryInterval,
		agentOutputFileNameProp:   me.OutputFileName,
	}
	for prop, bit := range agentStepFlags {
		values[prop] = me.Flags&bit != 0
	}
	for _, o := range agentOutcomes {
		props := agentStepOutcomeProps[o]
		action, _ := o.action(*me)
		if action == agentActionGoToStep {
			// A jump to a later step belongs to that step (handles_*_of), so this step's own action stays as configured.
			if target, backward := agentJumpDirection(steps, *me, o); backward {
				values[props.action] = "go_to_step"
				values[props.stepName] = target.Name
			}
		} else {
			values[props.action] = enumName(agentStepActions, action)
			values[props.stepName] = ""
		}
		var sources []string
		for _, s := range steps {
			if a, t := o.action(s); a == agentActionGoToStep && t == me.StepID && s.StepID < me.StepID {
				sources = append(sources, s.Name)
			}
		}
		sort.Strings(sources)
		values[props.handles] = sources
	}
	for k, v := range values {
		if err = data.Set(k, v); err != nil {
			return diag.FromErr(err)
		}
	}

	return nil
}

func resourceAgentJobStepUpdate(ctx context.Context, data *schema.ResourceData, meta interface{}) diag.Diagnostics {
	logger := loggerFromMeta(meta, "agent_job_step", "update")
	logger.Debug().Msgf("Update %s", data.Id())
	name := data.Get(agentNameProp).(string)

	connector, err := getAgentJobStepConnector(meta, data)
	if err != nil {
		return diag.FromErr(err)
	}

	jobID, previous, err := agentStepTarget(data)
	if err != nil {
		return diag.FromErr(err)
	}
	oldJobID, _, _ := parseAgentJobStepID(data.Id())
	if !strings.EqualFold(oldJobID, jobID) {
		// The step moves to another job, which happens when a whole chain moves: remove it from the old one first.
		unlockOld := lockAgentJob(oldJobID)
		err = removeAgentJobStep(ctx, connector, oldJobID, name)
		unlockOld()
		if err != nil {
			return diag.FromErr(errors.Wrapf(err, "unable to remove agent job step [%s] from job [%s]", name, oldJobID))
		}
		data.SetId("")
		return resourceAgentJobStepCreate(ctx, data, meta)
	}

	unlock := lockAgentJob(jobID)
	defer unlock()

	if err = checkAgentStepRunAs(ctx, connector, data, jobID); err != nil {
		return diag.FromErr(err)
	}

	steps, err := connector.ListAgentJobSteps(ctx, jobID)
	if err != nil {
		return diag.FromErr(err)
	}
	me := findAgentStep(steps, name)
	inPlace := false
	if me != nil {
		if previous == "" {
			inPlace = me.StepID == 1
		} else if prev := findAgentStep(steps, previous); prev != nil {
			inPlace = me.StepID == prev.StepID+1
		}
	}

	if !inPlace {
		// Move: delete the step and add it again at its new place, restoring every jump around it.
		snapshot := snapshotAgentJumps(steps)
		if me != nil {
			if err = connector.DeleteAgentJobStep(ctx, jobID, me.StepID); err != nil {
				return diag.FromErr(errors.Wrapf(err, "unable to move agent job step [%s]", name))
			}
		}
		if err = insertAgentJobStep(ctx, connector, data, jobID, previous); err != nil {
			return diag.FromErr(errors.Wrapf(err, "unable to move agent job step [%s]", name))
		}
		if steps, err = connector.ListAgentJobSteps(ctx, jobID); err != nil {
			return diag.FromErr(err)
		}
		if err = applyAgentJumpUpdates(ctx, connector, jobID, rewriteAgentJumps(snapshot, steps, name)); err != nil {
			return diag.FromErr(err)
		}
	} else {
		changes := map[string]interface{}{}
		for prop, param := range map[string]string{
			agentCommandProp:          "command",
			agentDatabaseNameProp:     "database_name",
			agentDatabaseUserNameProp: "database_user_name",
			agentRetryAttemptsProp:    "retry_attempts",
			agentRetryIntervalProp:    "retry_interval",
			agentOutputFileNameProp:   "output_file_name",
		} {
			// Only changed parameters are sent, so a forward jump set by another step is never overwritten.
			// An empty string clears database_user_name and output_file_name.
			if data.HasChange(prop) {
				changes[param] = data.Get(prop)
			}
		}
		for prop := range agentStepFlags {
			if data.HasChange(prop) {
				changes["flags"] = agentStepFlagsFromData(data)
			}
		}
		for _, o := range agentOutcomes {
			props := agentStepOutcomeProps[o]
			if data.HasChange(props.action) || data.HasChange(props.stepName) {
				action, target, err := resolveAgentStepAction(data, o, steps, me.StepID)
				if err != nil {
					return diag.FromErr(err)
				}
				for k, v := range o.params(action, target) {
					changes[k] = v
				}
			}
		}
		if err = connector.UpdateAgentJobStep(ctx, jobID, me.StepID, changes); err != nil {
			return diag.FromErr(errors.Wrapf(err, "unable to update agent job step [%s]", name))
		}
		if steps, err = connector.ListAgentJobSteps(ctx, jobID); err != nil {
			return diag.FromErr(err)
		}
	}

	if err = applyAgentForwardJumps(ctx, connector, data, jobID, steps); err != nil {
		return diag.FromErr(err)
	}

	logger.Info().Msgf("updated agent job step [%s]", name)

	return resourceAgentJobStepRead(ctx, data, meta)
}

func resourceAgentJobStepDelete(ctx context.Context, data *schema.ResourceData, meta interface{}) diag.Diagnostics {
	logger := loggerFromMeta(meta, "agent_job_step", "delete")
	logger.Debug().Msgf("Delete %s", data.Id())

	connector, err := getAgentJobStepConnector(meta, data)
	if err != nil {
		return diag.FromErr(err)
	}

	jobID, name, err := parseAgentJobStepID(data.Id())
	if err != nil {
		return diag.FromErr(err)
	}

	unlock := lockAgentJob(jobID)
	defer unlock()

	if err = removeAgentJobStep(ctx, connector, jobID, name); err != nil {
		return diag.FromErr(errors.Wrapf(err, "unable to delete agent job step [%s]", name))
	}

	logger.Info().Msgf("deleted agent job step [%s] from job [%s]", name, jobID)

	data.SetId("")

	return nil
}

func getAgentJobStepConnector(meta interface{}, data *schema.ResourceData) (AgentJobStepConnector, error) {
	provider := meta.(model.Provider)
	connector, err := provider.GetConnector(data)
	if err != nil {
		return nil, err
	}
	return connector.(AgentJobStepConnector), nil
}
