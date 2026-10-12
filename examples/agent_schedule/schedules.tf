#
# Schedule examples, as shown in docs/resources/agent_schedule.md. recurrence_factor (1) and subday_type ("once")
# are left out where their defaults apply.
#
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

# Quarterly: 1st of Jan/Apr/Jul/Oct at 07:00
resource "sqlserver_agent_schedule" "quarterly" {
  name              = "Quarterly report - 1st of quarter 07:00"
  frequency_type    = "monthly"
  recurrence_factor = 3
  monthly_day       = 1
  active_start_time = "07:00:00"
  active_start_date = "2027-01-01" # anchors the quarters to January
}

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
