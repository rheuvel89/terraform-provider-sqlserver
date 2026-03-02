terraform {
  required_version = "~> 1.5"
  required_providers {
    docker = {
      source  = "kreuzwerker/docker"
      version = "~> 3.0"
    }
    sqlserver = {
      source  = "rheuvel89/sqlserver"
      version = "0.1.1"
    }
    time = {
      source  = "hashicorp/time"
      version = "~> 0.10"
    }
  }
}

provider "docker" {}

provider "sqlserver" {
  debug = true
  host  = docker_container.mssql.network_data[0].ip_address
  login {
    username = local.local_username
    password = local.local_password
  }
}

#
# Creates a SQL Server running in a docker container on the local machine.
#
locals {
  local_username = "sa"
  local_password = "!!up3R!!3cR37"
}

resource "docker_image" "mssql" {
  name         = "mcr.microsoft.com/mssql/server:2022-latest"
  keep_locally = true
}

resource "docker_container" "mssql" {
  name  = "mssql-resource-governor"
  image = docker_image.mssql.image_id
  env   = ["ACCEPT_EULA=Y", "SA_PASSWORD=${local.local_password}", "MSSQL_PID=Enterprise"]
}

resource "time_sleep" "wait_for_sql" {
  depends_on = [docker_container.mssql]

  create_duration = "10s"
}

#
# Resource Governor Configuration
#
# This example demonstrates how to use Resource Governor to manage workloads.
# We create two resource pools: one for reporting workloads and one for OLTP workloads.
# Each pool has workload groups that define policies for requests.
#

# Resource Pool for reporting/analytics workloads
# Limits CPU and memory to prevent reports from impacting transactional workloads
resource "sqlserver_resource_pool" "reporting" {
  name               = "ReportingPool"
  min_cpu_percent    = 0
  max_cpu_percent    = 30
  min_memory_percent = 0
  max_memory_percent = 30
  cap_cpu_percent    = 30

  depends_on = [time_sleep.wait_for_sql]
}

# Resource Pool for OLTP/transactional workloads
# Guarantees minimum resources for critical business operations
resource "sqlserver_resource_pool" "oltp" {
  name               = "OLTPPool"
  min_cpu_percent    = 50
  max_cpu_percent    = 100
  min_memory_percent = 50
  max_memory_percent = 100
  cap_cpu_percent    = 100

  depends_on = [time_sleep.wait_for_sql]
}

# Workload Group for ad-hoc reporting queries
# Low importance, limited parallelism and memory grants
resource "sqlserver_workload_group" "adhoc_reports" {
  name                             = "AdhocReports"
  resource_pool_name               = sqlserver_resource_pool.reporting.name
  importance                       = "LOW"
  request_max_memory_grant_percent = 15
  request_max_cpu_time_sec         = 120
  max_dop                          = 2
  group_max_requests               = 5
}

# Workload Group for scheduled/batch reporting
# Medium importance, more resources than ad-hoc
resource "sqlserver_workload_group" "scheduled_reports" {
  name                             = "ScheduledReports"
  resource_pool_name               = sqlserver_resource_pool.reporting.name
  importance                       = "MEDIUM"
  request_max_memory_grant_percent = 25
  request_max_cpu_time_sec         = 300
  max_dop                          = 4
  group_max_requests               = 10
}

# Workload Group for OLTP transactions
# High importance, guaranteed resources
resource "sqlserver_workload_group" "oltp_transactions" {
  name                             = "OLTPTransactions"
  resource_pool_name               = sqlserver_resource_pool.oltp.name
  importance                       = "HIGH"
  request_max_memory_grant_percent = 25
  request_max_cpu_time_sec         = 0  # unlimited
  max_dop                          = 1  # OLTP typically single-threaded
  group_max_requests               = 0  # unlimited
}

#
# Classifier Function
#
# Routes incoming sessions to appropriate workload groups based on application name
resource "sqlserver_classifier_function" "workload_classifier" {
  name        = "WorkloadClassifier"
  schema_name = "dbo"
  function_body = <<-EOF
    DECLARE @grp_name SYSNAME
    DECLARE @app_name SYSNAME = APP_NAME()
    
    -- Route based on application name
    IF @app_name LIKE 'Report%' OR @app_name LIKE '%SSRS%'
        SET @grp_name = 'AdhocReports'
    ELSE IF @app_name LIKE 'ETL%' OR @app_name LIKE 'SSIS%'
        SET @grp_name = 'ScheduledReports'
    ELSE IF @app_name LIKE 'App%' OR @app_name LIKE 'Web%'
        SET @grp_name = 'OLTPTransactions'
    ELSE
        SET @grp_name = 'default'
    
    RETURN @grp_name
EOF

  depends_on = [
    sqlserver_workload_group.adhoc_reports,
    sqlserver_workload_group.scheduled_reports,
    sqlserver_workload_group.oltp_transactions,
  ]
}

# Enable Resource Governor with the classifier function
resource "sqlserver_resource_governor" "config" {
  enabled             = true
  classifier_function = sqlserver_classifier_function.workload_classifier.fully_qualified_name

  depends_on = [sqlserver_classifier_function.workload_classifier]
}

#
# Outputs
#
output "resource_pools" {
  value = {
    reporting = {
      name    = sqlserver_resource_pool.reporting.name
      pool_id = sqlserver_resource_pool.reporting.pool_id
    }
    oltp = {
      name    = sqlserver_resource_pool.oltp.name
      pool_id = sqlserver_resource_pool.oltp.pool_id
    }
  }
}

output "workload_groups" {
  value = {
    adhoc_reports = {
      name     = sqlserver_workload_group.adhoc_reports.name
      group_id = sqlserver_workload_group.adhoc_reports.group_id
    }
    scheduled_reports = {
      name     = sqlserver_workload_group.scheduled_reports.name
      group_id = sqlserver_workload_group.scheduled_reports.group_id
    }
    oltp_transactions = {
      name     = sqlserver_workload_group.oltp_transactions.name
      group_id = sqlserver_workload_group.oltp_transactions.group_id
    }
  }
}

output "resource_governor_enabled" {
  value = sqlserver_resource_governor.config.enabled
}
