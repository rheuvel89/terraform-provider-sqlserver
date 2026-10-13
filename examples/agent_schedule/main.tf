#
# Common SQL Server Agent schedules. The schedules themselves are in schedules.tf, which the
# provider's acceptance tests also apply, so every example here is known to work.
#
# Times are in the SQL Server's local time zone (UTC by default on Azure SQL Managed Instance).
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
