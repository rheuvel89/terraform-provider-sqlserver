# sqlserver_agent_schedule

The `sqlserver_agent_schedule` resource creates and manages a SQL Server Agent schedule. A schedule starts the jobs it's
attached to with [`sqlserver_agent_job_schedule`](agent_job_schedule.md); one schedule can be shared by several jobs.

Schedules use readable attributes, such as `frequency_type = "weekly"` and `weekly_days = ["monday"]`, which the
provider translates to and from the numbers msdb stores.

## Example Usage

These examples are also in [`examples/agent_schedule`](../../examples/agent_schedule/schedules.tf), and the provider's
acceptance tests apply every one of them.

### Daily

```hcl
# Every day at 02:30
resource "sqlserver_agent_schedule" "nightly_0230" {
  name              = "Daily 02:30"
  frequency_type    = "daily"
  active_start_time = "02:30:00"
}

# Every 3 days at 04:00, counting from a fixed date
resource "sqlserver_agent_schedule" "every_3_days" {
  name              = "Every 3 days at 04:00"
  frequency_type    = "daily"
  recurrence_factor = 3
  active_start_time = "04:00:00"
  active_start_date = "2026-11-01" # anchors the cycle
}

# Every 5 minutes, only during a date range (Black Friday to Cyber Monday)
resource "sqlserver_agent_schedule" "black_friday_watch" {
  name              = "Black Friday 2026 - every 5 min"
  frequency_type    = "daily"
  subday_type       = "minutes"
  subday_interval   = 5
  active_start_date = "2026-11-26"
  active_end_date   = "2026-11-30"
}
```

### Weekly

```hcl
# Sunday 01:00 (weekly full backup)
resource "sqlserver_agent_schedule" "weekly_full_backup" {
  name              = "Weekly full backup - Sunday 01:00"
  frequency_type    = "weekly"
  weekly_days       = ["sunday"]
  active_start_time = "01:00:00"
}

# Every 15 minutes, 08:00-18:00, Monday to Friday
resource "sqlserver_agent_schedule" "office_hours_15m" {
  name              = "Weekdays every 15 min, 08:00-18:00"
  frequency_type    = "weekly"
  weekly_days       = ["monday", "tuesday", "wednesday", "thursday", "friday"]
  subday_type       = "minutes"
  subday_interval   = 15
  active_start_time = "08:00:00"
  active_end_time   = "18:00:00"
}

# Every other Friday at 17:30 (payroll)
resource "sqlserver_agent_schedule" "biweekly_payroll" {
  name              = "Payroll export - every other Friday 17:30"
  frequency_type    = "weekly"
  recurrence_factor = 2
  weekly_days       = ["friday"]
  active_start_time = "17:30:00"
  active_start_date = "2026-10-16" # a payroll Friday; anchors which Fridays count
}
```

### Monthly, on a Fixed Day

```hcl
# Quarterly: 1st of Jan/Apr/Jul/Oct at 07:00
resource "sqlserver_agent_schedule" "quarterly" {
  name              = "Quarterly report - 1st of quarter 07:00"
  frequency_type    = "monthly"
  recurrence_factor = 3
  monthly_day       = 1
  active_start_time = "07:00:00"
  active_start_date = "2027-01-01" # anchors the quarters to January
}
```

### Monthly, on a Relative Day

```hcl
# First Sunday of the month at 03:00
resource "sqlserver_agent_schedule" "first_sunday" {
  name                  = "First Sunday of the month 03:00"
  frequency_type        = "monthly_relative"
  monthly_relative_week = "first"
  monthly_relative_day  = "sunday"
  active_start_time     = "03:00:00"
}

# Second Wednesday at 06:00 (post-patch checks). Not always the day after
#    Patch Tuesday: if the 1st is a Wednesday, it falls before the second Tuesday.
resource "sqlserver_agent_schedule" "post_patch_tuesday" {
  name                  = "Second Wednesday - post-patch checks 06:00"
  frequency_type        = "monthly_relative"
  monthly_relative_week = "second"
  monthly_relative_day  = "wednesday"
  active_start_time     = "06:00:00"
}

# Last weekday (Mon-Fri) of the month at 20:00 (month-end close).
#    Use monthly_relative_day = "day" for the last calendar day.
resource "sqlserver_agent_schedule" "month_end_close" {
  name                  = "Last weekday of the month 20:00"
  frequency_type        = "monthly_relative"
  monthly_relative_week = "last"
  monthly_relative_day  = "weekday"
  active_start_time     = "20:00:00"
}

# First weekend day (Saturday or Sunday) of the month, every 6 hours
resource "sqlserver_agent_schedule" "first_weekend_day_6h" {
  name                  = "First weekend day, every 6h"
  frequency_type        = "monthly_relative"
  monthly_relative_week = "first"
  monthly_relative_day  = "weekend_day"
  subday_type           = "hours"
  subday_interval       = 6
}
```

### One-off and Event-based

```hcl
# Once, at a fixed date and time (migration cutover)
resource "sqlserver_agent_schedule" "cutover" {
  name              = "One-off cutover"
  frequency_type    = "once"
  active_start_date = "2026-12-05"
  active_start_time = "22:00:00"
}

# Whenever SQL Server Agent starts
resource "sqlserver_agent_schedule" "on_agent_start" {
  name           = "On SQL Agent start"
  frequency_type = "agent_start"
}

# Whenever the CPU is idle (needs the idle-CPU condition in Agent's properties)
resource "sqlserver_agent_schedule" "when_idle" {
  name           = "When CPU idle"
  frequency_type = "idle"
}
```

## Argument Reference

* `name` - (Required) The name of the schedule. Names don't have to be unique. Changing it renames the schedule in place.
* `enabled` - (Optional) Whether the schedule starts jobs. Default is `true`.
* `frequency_type` - (Required) How often the schedule fires: `once`, `daily`, `weekly`, `monthly`,
  `monthly_relative`, `agent_start` (when SQL Server Agent starts) or `idle` (when the CPU is idle).
* `recurrence_factor` - (Optional) Every N days, weeks or months, for `daily`, `weekly`, `monthly` and
  `monthly_relative` schedules. Default is `1`. When it's more than 1, set `active_start_date`: see
  [Start Dates](#start-dates).
* `weekly_days` - (Optional) The days a `weekly` schedule fires on: `sunday` to `saturday`. Required for `weekly`
  schedules and not allowed otherwise.
* `monthly_day` - (Optional) The day of the month, 1 to 31, that a `monthly` schedule fires on. Required for `monthly`
  schedules and not allowed otherwise. See [Days 29 to 31](#days-29-to-31).
* `monthly_relative_week` - (Optional) For `monthly_relative` schedules: `first`, `second`, `third`, `fourth` or `last`.
* `monthly_relative_day` - (Optional) For `monthly_relative` schedules: `sunday` to `saturday`, `day` (any day),
  `weekday` (Monday to Friday) or `weekend_day` (Saturday or Sunday). For example, `last` + `day` is the last day of
  the month.
* `subday_type` - (Optional) How often the schedule fires on a day it runs: `once` (at `active_start_time`), `seconds`,
  `minutes` or `hours`. Default is `once`. Only for `daily`, `weekly`, `monthly` and `monthly_relative` schedules.
* `subday_interval` - (Optional) The number of seconds, minutes or hours between runs. Required unless `subday_type` is
  `once`. SQL Server Agent's minimum is 10 seconds.
* `active_start_date` - (Optional) The first day the schedule can fire, as `YYYY-MM-DD`. Defaults to the day the
  schedule is created.
* `active_end_date` - (Optional) The last day the schedule can fire, as `YYYY-MM-DD`. Default is `9999-12-31`.
* `active_start_time` - (Optional) The time of day the schedule starts firing, as `HH:MM:SS`. Default is `00:00:00`.
* `active_end_time` - (Optional) The time of day the schedule stops firing, as `HH:MM:SS`. Default is `23:59:59`.
  See [Time Windows](#time-windows).
* `owner_login_name` - (Optional) The login that owns the schedule. Defaults to the login the provider connects with.

## Attribute Reference

* `schedule_id` - The ID of the schedule in msdb.
* `schedule_uid` - The GUID of the schedule.

## Checks at Plan Time

The provider rejects these during `terraform plan`, rather than leaving them to a vague SQL Server error during apply:

* attributes that don't match the `frequency_type`, such as `weekly_days` on a `daily` schedule;
* dates that don't exist, such as `2026-02-30` or `2025-02-29`, dates before `1990-01-01` (SQL Server's earliest), and
  times such as `24:00:00` or `12:61:00`;
* an `active_end_date` before the `active_start_date`;
* a `subday_interval` under 10 seconds;
* an `active_end_time` equal to the `active_start_time` when the schedule fires several times a day;
* an `active_end_time` of `00:00:00`. SQL Server treats `0` as "not set" and stores `23:59:59`, so use `23:59:59` for
  "until midnight".

## Time Zone and Daylight Saving Time

Times are in the SQL Server's local time zone. On Azure SQL Managed Instance that's the time zone chosen when the
instance was created: UTC by default, and it can't be changed afterwards.

On a server whose time zone observes daylight saving time, a schedule in the hour that is skipped in spring may run at
the change or not at all that day, and around the autumn change a schedule that fires every few minutes may pause for
about an hour. To avoid surprises, don't schedule jobs between about 01:00 and 03:00 local time, or run the server in
UTC, and make jobs safe to run twice. The provider can't check this, as it doesn't know the server's time zone at plan
time.

## Start Dates

For schedules that repeat every N days, weeks or months, `active_start_date` decides which days count: the cycle is
counted from that date. It defaults to the day of the apply, so set it
explicitly, as the "every 3 days", "every other Friday" and "quarterly" examples do.

## Days 29 to 31

SQL Server Agent **skips** months that don't have the `monthly_day`: a schedule on day 31 doesn't run in February,
April, June, September and November, and one on day 29 only runs in February in leap years. The provider shows a
warning for days 29 to 31. For the last day of every month, use `frequency_type = "monthly_relative"` with
`monthly_relative_week = "last"` and `monthly_relative_day = "day"`.

## Time Windows

An `active_end_time` earlier than the `active_start_time` means a window that crosses midnight: `22:00:00` to
`02:00:00` fires from 22:00 until 02:00 the next morning. The first run is at the start time on the start date.

## Other Notes

* `idle` schedules only fire when the idle-CPU condition is set in SQL Server Agent's properties. They haven't been
  verified on Azure SQL Managed Instance.
* Deleting a schedule that is still attached to a job fails. Terraform detaches it first when the attachment is managed
  by [`sqlserver_agent_job_schedule`](agent_job_schedule.md).

## Import

Import isn't supported.
