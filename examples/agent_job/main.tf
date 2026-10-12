#
# A nightly ETL job: an operator who is emailed on failure, a job with three chained steps, a step that runs when
# extract or transform fails, and one schedule shared by two jobs.
#
terraform {
  required_version = "~> 1.5"
  required_providers {
    sqlserver = {
      source  = "rheuvel89/sqlserver"
      version = "0.5.0"
    }
  }
}

provider "sqlserver" {
  host = "localhost"
  login {
    username = "sa"
    password = var.sa_password
  }
}

variable "sa_password" {
  type      = string
  sensitive = true
}

resource "sqlserver_database" "warehouse" {
  name = "warehouse"
}

# Mail is sent by Database Mail, which has to be set up separately (see docs/resources/agent_operator.md).
resource "sqlserver_agent_operator" "dba" {
  name          = "dba"
  email_address = "dba@example.com"
}

resource "sqlserver_agent_job" "nightly_etl" {
  name                       = "nightly_etl"
  description                = "Loads the warehouse every night"
  notify_level_email         = "on_failure"
  notify_email_operator_name = sqlserver_agent_operator.dba.name
}

# The first step sets job_id; every later step sets previous_step_id.
resource "sqlserver_agent_job_step" "extract" {
  job_id         = sqlserver_agent_job.nightly_etl.job_id
  name           = "extract"
  database_name  = sqlserver_database.warehouse.name
  command        = "SELECT 'extract'"
  retry_attempts = 3
  retry_interval = 5
}

resource "sqlserver_agent_job_step" "transform" {
  previous_step_id = sqlserver_agent_job_step.extract.id
  name             = "transform"
  database_name    = sqlserver_database.warehouse.name
  command          = "SELECT 'transform'"
}

resource "sqlserver_agent_job_step" "load" {
  previous_step_id = sqlserver_agent_job_step.transform.id
  name             = "load"
  database_name    = sqlserver_database.warehouse.name
  command          = "SELECT 'load'"
  # Stop here when everything went well, so the cleanup step below only runs after a failure.
  on_success_action = "quit_with_success"
}

# A forward jump is declared on the step that is jumped to.
resource "sqlserver_agent_job_step" "cleanup_after_failure" {
  previous_step_id   = sqlserver_agent_job_step.load.id
  name               = "cleanup_after_failure"
  database_name      = sqlserver_database.warehouse.name
  command            = "SELECT 'cleanup'"
  handles_failure_of = [sqlserver_agent_job_step.extract.name, sqlserver_agent_job_step.transform.name]
  on_success_action  = "quit_with_failure" # the job still reports the failure
}

resource "sqlserver_agent_job" "update_stats" {
  name = "update_stats"
}

resource "sqlserver_agent_job_step" "update_stats" {
  job_id        = sqlserver_agent_job.update_stats.job_id
  name          = "update_stats"
  database_name = sqlserver_database.warehouse.name
  command       = "EXEC sp_updatestats"
}

resource "sqlserver_agent_schedule" "nightly" {
  name              = "Daily 02:30"
  frequency_type    = "daily"
  active_start_time = "02:30:00"
}

resource "sqlserver_agent_job_schedule" "nightly_etl" {
  job_id      = sqlserver_agent_job.nightly_etl.job_id
  schedule_id = sqlserver_agent_schedule.nightly.schedule_id
}

resource "sqlserver_agent_job_schedule" "update_stats" {
  job_id      = sqlserver_agent_job.update_stats.job_id
  schedule_id = sqlserver_agent_schedule.nightly.schedule_id
}
