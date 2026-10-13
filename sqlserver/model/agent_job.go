package model

type AgentJob struct {
	JobID                   string
	Name                    string
	Description             string
	Enabled                 bool
	CategoryName            string
	OwnerLoginName          string
	NotifyLevelEventlog     int
	NotifyLevelEmail        int
	NotifyEmailOperatorName string
	DeleteLevel             int
	StartStepID             int
}
