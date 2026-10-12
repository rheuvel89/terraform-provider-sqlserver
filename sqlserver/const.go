package sqlserver

const (
	databaseProp           = "database"
	principalIdProp        = "principal_id"
	usernameProp           = "username"
	loginNameProp          = "login_name"
	objectIdProp           = "object_id"
	passwordProp           = "password"
	passwordWoProp         = "password_wo"
	passwordWoVersionProp  = "password_wo_version"
	sidStrProp             = "sid"
	clientIdProp           = "client_id"
	loginSourceTypeProp    = "login_source_type"
	authenticationTypeProp = "authentication_type"
	rolesProp              = "roles"
	isDisabledProp         = "is_disabled"

	LoginSourceTypeSQL      = "sql_login"
	LoginSourceTypeExternal = "external_login"

	// User source types
	UserSourceTypeInstance = "instance_user"
	UserSourceTypeDatabase = "database_user"
	UserSourceTypeExternal = "external_user"

	// Resource Pool properties
	resourcePoolNameProp    = "name"
	resourcePoolNameRefProp = "resource_pool_name"
	minCPUPercentProp       = "min_cpu_percent"
	maxCPUPercentProp       = "max_cpu_percent"
	minMemoryPercentProp    = "min_memory_percent"
	maxMemoryPercentProp    = "max_memory_percent"
	capCPUPercentProp       = "cap_cpu_percent"
	minIOPSPerVolumeProp    = "min_iops_per_volume"
	maxIOPSPerVolumeProp    = "max_iops_per_volume"
	poolIdProp              = "pool_id"

	// Workload Group properties
	workloadGroupNameProp            = "name"
	importanceProp                   = "importance"
	requestMaxMemoryGrantPercentProp = "request_max_memory_grant_percent"
	requestMaxCPUTimeSecProp         = "request_max_cpu_time_sec"
	requestMemoryGrantTimeoutSecProp = "request_memory_grant_timeout_sec"
	maxDOPProp                       = "max_dop"
	groupMaxRequestsProp             = "group_max_requests"
	groupIdProp                      = "group_id"

	// Resource Governor properties
	enabledProp            = "enabled"
	classifierFunctionProp = "classifier_function"

	// Classifier Function properties
	classifierFunctionNameProp = "name"
	schemaNameProp             = "schema_name"
	functionBodyProp           = "function_body"
	functionObjectIdProp       = "object_id"
	fullyQualifiedNameProp     = "fully_qualified_name"

	// Database properties
	databaseNameProp       = "name"
	collationProp          = "collation"
	recoveryModelProp      = "recovery_model"
	compatibilityLevelProp = "compatibility_level"
	ownerProp              = "owner"
)

const (
	// SQL Server Agent properties
	agentNameProp          = "name"
	agentEnabledProp       = "enabled"
	agentCategoryNameProp  = "category_name"
	agentOwnerLoginProp    = "owner_login_name"
	agentDescriptionProp   = "description"
	agentJobIdProp         = "job_id"
	agentScheduleIdProp    = "schedule_id"
	agentOperatorIdProp    = "operator_id"
	agentEmailAddressProp  = "email_address"
	agentStartStepIdProp   = "start_step_id"
	agentNotifyEventlog    = "notify_level_eventlog"
	agentNotifyEmail       = "notify_level_email"
	agentNotifyEmailOpProp = "notify_email_operator_name"
	agentDeleteLevelProp   = "delete_level"

	// Built-in categories used as defaults
	agentJobDefaultCategory      = "[Uncategorized (Local)]"
	agentOperatorDefaultCategory = "[Uncategorized]"
)

// agentNotifyLevels maps the friendly notify/delete levels of a job to their msdb values.
var agentNotifyLevels = map[string]int{
	"never":      0,
	"on_success": 1,
	"on_failure": 2,
	"always":     3,
}

const (
	// SQL Server Agent schedule properties
	agentFrequencyTypeProp       = "frequency_type"
	agentRecurrenceFactorProp    = "recurrence_factor"
	agentWeeklyDaysProp          = "weekly_days"
	agentMonthlyDayProp          = "monthly_day"
	agentMonthlyRelativeWeekProp = "monthly_relative_week"
	agentMonthlyRelativeDayProp  = "monthly_relative_day"
	agentSubdayTypeProp          = "subday_type"
	agentSubdayIntervalProp      = "subday_interval"
	agentActiveStartDateProp     = "active_start_date"
	agentActiveEndDateProp       = "active_end_date"
	agentActiveStartTimeProp     = "active_start_time"
	agentActiveEndTimeProp       = "active_end_time"
	agentScheduleUidProp         = "schedule_uid"
)
