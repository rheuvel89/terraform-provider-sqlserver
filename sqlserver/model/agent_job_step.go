package model

// AgentJobStep is a job step in raw msdb form (see msdb.dbo.sysjobsteps).
type AgentJobStep struct {
	JobID            string
	StepID           int
	StepUID          string
	Name             string
	Command          string
	DatabaseName     string
	DatabaseUserName string
	OnSuccessAction  int
	OnSuccessStepID  int
	OnFailAction     int
	OnFailStepID     int
	RetryAttempts    int
	RetryInterval    int
	OutputFileName   string
	Flags            int
}
