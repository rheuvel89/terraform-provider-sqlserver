package sqlserver

const (
	databaseProp           = "database"
	principalIdProp        = "principal_id"
	usernameProp           = "username"
	loginNameProp          = "login_name"
	objectIdProp           = "object_id"
	passwordProp           = "password"
	sidStrProp             = "sid"
	clientIdProp           = "client_id"
	loginSourceTypeProp    = "login_source_type"
	authenticationTypeProp = "authentication_type"
	rolesProp              = "roles"

	LoginSourceTypeSQL      = "sql_login"
	LoginSourceTypeExternal = "external_login"

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
)
