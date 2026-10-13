package model

// AgentSchedule is a schedule in raw msdb form (see msdb.dbo.sysschedules).
type AgentSchedule struct {
	ScheduleID           int
	ScheduleUID          string
	Name                 string
	Enabled              bool
	FreqType             int
	FreqInterval         int
	FreqSubdayType       int
	FreqSubdayInterval   int
	FreqRelativeInterval int
	FreqRecurrenceFactor int
	ActiveStartDate      int
	ActiveEndDate        int
	ActiveStartTime      int
	ActiveEndTime        int
	OwnerLoginName       string
}
