package sqlserver

import (
	"fmt"
	"regexp"
	"sort"
	"terraform-provider-sqlserver/sqlserver/model"
	"time"

	"github.com/hashicorp/go-cty/cty"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
)

// Friendly schedule values and their msdb encodings (see sp_add_schedule).
var (
	agentFrequencyTypes = map[string]int{
		"once":             1,
		"daily":            4,
		"weekly":           8,
		"monthly":          16,
		"monthly_relative": 32,
		"agent_start":      64,
		"idle":             128,
	}
	agentWeekDays = map[string]int{
		"sunday":    1,
		"monday":    2,
		"tuesday":   4,
		"wednesday": 8,
		"thursday":  16,
		"friday":    32,
		"saturday":  64,
	}
	agentRelativeWeeks = map[string]int{
		"first":  1,
		"second": 2,
		"third":  4,
		"fourth": 8,
		"last":   16,
	}
	agentRelativeDays = map[string]int{
		"sunday":      1,
		"monday":      2,
		"tuesday":     3,
		"wednesday":   4,
		"thursday":    5,
		"friday":      6,
		"saturday":    7,
		"day":         8,
		"weekday":     9,
		"weekend_day": 10,
	}
	agentSubdayTypes = map[string]int{
		"once":    1,
		"seconds": 2,
		"minutes": 4,
		"hours":   8,
	}
)

const (
	// YYYY-MM-DD and HH:MM:SS (24-hour clock), as Go time layouts. Go writes a layout as its fixed reference time
	// Mon Jan 2 15:04:05 2006, where every number is a token: 2006 year, 01 month, 02 day, 15 hour (24-hour clock;
	// 03 would be 12-hour), 04 minute, 05 second.
	agentDateLayout     = time.DateOnly // "2006-01-02"
	agentTimeLayout     = time.TimeOnly // "15:04:05"
	agentMinDate        = "1990-01-01"
	agentMaxDate        = "9999-12-31"
	agentMinSubdaySecs  = 10
	agentDefaultEndTime = "23:59:59"
)

var (
	agentDatePattern = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)
	agentTimePattern = regexp.MustCompile(`^\d{2}:\d{2}:\d{2}$`)
)

// agentScheduleSpec is a schedule in the provider's friendly form.
type agentScheduleSpec struct {
	FrequencyType       string
	RecurrenceFactor    int
	WeeklyDays          []string
	MonthlyDay          int
	MonthlyRelativeWeek string
	MonthlyRelativeDay  string
	SubdayType          string
	SubdayInterval      int
	ActiveStartDate     string // "" lets SQL Server use today
	ActiveEndDate       string
	ActiveStartTime     string
	ActiveEndTime       string
}

// isRecurring reports whether the frequency type uses recurrence_factor and the sub-day settings.
func isRecurring(frequencyType string) bool {
	switch frequencyType {
	case "daily", "weekly", "monthly", "monthly_relative":
		return true
	}
	return false
}

// encodeAgentSchedule translates the friendly form into msdb values. It expects a spec that passed
// validateAgentScheduleSpec.
func encodeAgentSchedule(spec agentScheduleSpec) (model.AgentSchedule, error) {
	var s model.AgentSchedule
	var err error

	s.FreqType = agentFrequencyTypes[spec.FrequencyType]
	if s.ActiveStartDate, err = parseAgentDate(spec.ActiveStartDate); err != nil {
		return s, err
	}
	if s.ActiveEndDate, err = parseAgentDate(spec.ActiveEndDate); err != nil {
		return s, err
	}
	if s.ActiveStartTime, err = parseAgentTime(spec.ActiveStartTime); err != nil {
		return s, err
	}
	if s.ActiveEndTime, err = parseAgentTime(spec.ActiveEndTime); err != nil {
		return s, err
	}

	if !isRecurring(spec.FrequencyType) {
		return s, nil
	}

	s.FreqSubdayType = agentSubdayTypes[spec.SubdayType]
	s.FreqSubdayInterval = spec.SubdayInterval
	recurrence := spec.RecurrenceFactor
	if recurrence < 1 {
		recurrence = 1
	}

	switch spec.FrequencyType {
	case "daily":
		// For daily schedules "every N days" is stored in freq_interval.
		s.FreqInterval = recurrence
	case "weekly":
		for _, day := range spec.WeeklyDays {
			s.FreqInterval |= agentWeekDays[day]
		}
		s.FreqRecurrenceFactor = recurrence
	case "monthly":
		s.FreqInterval = spec.MonthlyDay
		s.FreqRecurrenceFactor = recurrence
	case "monthly_relative":
		s.FreqInterval = agentRelativeDays[spec.MonthlyRelativeDay]
		s.FreqRelativeInterval = agentRelativeWeeks[spec.MonthlyRelativeWeek]
		s.FreqRecurrenceFactor = recurrence
	}
	return s, nil
}

// decodeAgentSchedule translates msdb values into the friendly form. Fields that don't apply to the
// frequency type are left at their zero value, matching an unset attribute.
func decodeAgentSchedule(s model.AgentSchedule) agentScheduleSpec {
	spec := agentScheduleSpec{
		FrequencyType:    enumName(agentFrequencyTypes, s.FreqType),
		RecurrenceFactor: 1,
		SubdayType:       "once",
		ActiveStartDate:  formatAgentDate(s.ActiveStartDate),
		ActiveEndDate:    formatAgentDate(s.ActiveEndDate),
		ActiveStartTime:  formatAgentTime(s.ActiveStartTime),
		ActiveEndTime:    formatAgentTime(s.ActiveEndTime),
	}
	if !isRecurring(spec.FrequencyType) {
		return spec
	}

	if name := enumName(agentSubdayTypes, s.FreqSubdayType); name != "" {
		spec.SubdayType = name
	}
	if spec.SubdayType != "once" {
		spec.SubdayInterval = s.FreqSubdayInterval
	}

	switch spec.FrequencyType {
	case "daily":
		spec.RecurrenceFactor = s.FreqInterval
	case "weekly":
		spec.RecurrenceFactor = s.FreqRecurrenceFactor
		for day, bit := range agentWeekDays {
			if s.FreqInterval&bit != 0 {
				spec.WeeklyDays = append(spec.WeeklyDays, day)
			}
		}
		sort.Slice(spec.WeeklyDays, func(i, j int) bool {
			return agentWeekDays[spec.WeeklyDays[i]] < agentWeekDays[spec.WeeklyDays[j]]
		})
	case "monthly":
		spec.RecurrenceFactor = s.FreqRecurrenceFactor
		spec.MonthlyDay = s.FreqInterval
	case "monthly_relative":
		spec.RecurrenceFactor = s.FreqRecurrenceFactor
		spec.MonthlyRelativeDay = enumName(agentRelativeDays, s.FreqInterval)
		spec.MonthlyRelativeWeek = enumName(agentRelativeWeeks, s.FreqRelativeInterval)
	}
	return spec
}

// validateAgentScheduleSpec checks the combination of attributes against the frequency type and returns the first
// problem. Attributes listed in unknown haven't been resolved yet (plan time) and are skipped.
func validateAgentScheduleSpec(spec agentScheduleSpec, unknown map[string]bool) error {
	for _, check := range []func(agentScheduleSpec, map[string]bool) error{
		validateAgentFrequencyFields,
		validateAgentSubday,
		validateAgentActiveDates,
		validateAgentActiveTimes,
	} {
		if err := check(spec, unknown); err != nil {
			return err
		}
	}
	return nil
}

// validateAgentFrequencyFields checks that each frequency-specific attribute is set exactly when the frequency
// type uses it.
func validateAgentFrequencyFields(spec agentScheduleSpec, unknown map[string]bool) error {
	ft := spec.FrequencyType
	fields := []struct {
		name          string
		wanted, isSet bool
	}{
		{"weekly_days", ft == "weekly", len(spec.WeeklyDays) > 0},
		{"monthly_day", ft == "monthly", spec.MonthlyDay != 0},
		{"monthly_relative_week", ft == "monthly_relative", spec.MonthlyRelativeWeek != ""},
		{"monthly_relative_day", ft == "monthly_relative", spec.MonthlyRelativeDay != ""},
	}
	if !isRecurring(ft) {
		// recurrence_factor defaults to 1, so only a larger value counts as set.
		fields = append(fields, struct {
			name          string
			wanted, isSet bool
		}{"recurrence_factor", false, spec.RecurrenceFactor > 1})
	}
	for _, f := range fields {
		switch {
		case unknown[f.name] || f.wanted == f.isSet:
		case f.wanted:
			return fmt.Errorf("%s is required when frequency_type is %q", f.name, ft)
		default:
			return fmt.Errorf("%s can't be set when frequency_type is %q", f.name, ft)
		}
	}
	return nil
}

// validateAgentSubday checks subday_type and subday_interval: they only apply to recurring schedules, and the
// interval is required for anything but "once", with SQL Server Agent's minimum of 10 seconds.
func validateAgentSubday(spec agentScheduleSpec, unknown map[string]bool) error {
	if unknown["subday_type"] || unknown["subday_interval"] {
		return nil
	}
	if !isRecurring(spec.FrequencyType) {
		switch {
		case spec.SubdayType != "once":
			return fmt.Errorf("subday_type can't be set when frequency_type is %q", spec.FrequencyType)
		case spec.SubdayInterval != 0:
			return fmt.Errorf("subday_interval can't be set when frequency_type is %q", spec.FrequencyType)
		}
		return nil
	}
	switch {
	case spec.SubdayType == "once" && spec.SubdayInterval != 0:
		return fmt.Errorf("subday_interval can't be set when subday_type is \"once\"")
	case spec.SubdayType != "once" && spec.SubdayInterval < 1:
		return fmt.Errorf("subday_interval is required when subday_type is %q", spec.SubdayType)
	case spec.SubdayType == "seconds" && spec.SubdayInterval < agentMinSubdaySecs:
		return fmt.Errorf("subday_interval must be at least %d when subday_type is \"seconds\" (SQL Server Agent's minimum)", agentMinSubdaySecs)
	}
	return nil
}

// validateAgentActiveDates checks that the end date isn't before the start date. The dates themselves are checked
// by validateAgentDate.
func validateAgentActiveDates(spec agentScheduleSpec, unknown map[string]bool) error {
	if spec.ActiveStartDate == "" || spec.ActiveEndDate == "" || unknown["active_start_date"] || unknown["active_end_date"] {
		return nil
	}
	start, errStart := parseAgentDate(spec.ActiveStartDate)
	end, errEnd := parseAgentDate(spec.ActiveEndDate)
	if errStart == nil && errEnd == nil && end < start {
		return fmt.Errorf("active_end_date (%s) is before active_start_date (%s)", spec.ActiveEndDate, spec.ActiveStartDate)
	}
	return nil
}

// validateAgentActiveTimes rejects the time windows SQL Server would change or refuse. An end time before the start
// time is allowed: it's a window that crosses midnight.
func validateAgentActiveTimes(spec agentScheduleSpec, unknown map[string]bool) error {
	if !unknown["active_end_time"] && spec.ActiveEndTime == "00:00:00" {
		return fmt.Errorf("active_end_time can't be \"00:00:00\": SQL Server stores it as %q; use %q for \"until midnight\"", agentDefaultEndTime, agentDefaultEndTime)
	}
	if isRecurring(spec.FrequencyType) && spec.SubdayType != "once" && spec.ActiveStartTime == spec.ActiveEndTime &&
		!unknown["active_start_time"] && !unknown["active_end_time"] && !unknown["subday_type"] {
		return fmt.Errorf("active_start_time and active_end_time can't be equal when subday_type is %q", spec.SubdayType)
	}
	return nil
}

func parseAgentDate(s string) (int, error) {
	if s == "" {
		return 0, nil
	}
	if !agentDatePattern.MatchString(s) {
		return 0, fmt.Errorf("%q is not a date in YYYY-MM-DD format", s)
	}
	t, err := time.Parse(agentDateLayout, s)
	if err != nil {
		return 0, fmt.Errorf("%q is not a valid date", s)
	}
	if s < agentMinDate {
		return 0, fmt.Errorf("%q is before %s, the earliest date SQL Server Agent accepts", s, agentMinDate)
	}
	return t.Year()*10000 + int(t.Month())*100 + t.Day(), nil
}

func formatAgentDate(v int) string {
	if v == 0 {
		return ""
	}
	return fmt.Sprintf("%04d-%02d-%02d", v/10000, v/100%100, v%100)
}

func parseAgentTime(s string) (int, error) {
	if s == "" {
		return 0, nil
	}
	if !agentTimePattern.MatchString(s) {
		return 0, fmt.Errorf("%q is not a time in HH:MM:SS format", s)
	}
	t, err := time.Parse(agentTimeLayout, s)
	if err != nil {
		return 0, fmt.Errorf("%q is not a valid time", s)
	}
	return t.Hour()*10000 + t.Minute()*100 + t.Second(), nil
}

func formatAgentTime(v int) string {
	return fmt.Sprintf("%02d:%02d:%02d", v/10000, v/100%100, v%100)
}

func validateAgentDate(i interface{}, path cty.Path) diag.Diagnostics {
	if _, err := parseAgentDate(i.(string)); err != nil {
		return diag.Diagnostics{{Severity: diag.Error, Summary: err.Error(), AttributePath: path}}
	}
	return nil
}

func validateAgentTime(i interface{}, path cty.Path) diag.Diagnostics {
	if _, err := parseAgentTime(i.(string)); err != nil {
		return diag.Diagnostics{{Severity: diag.Error, Summary: err.Error(), AttributePath: path}}
	}
	return nil
}

// validateAgentMonthlyDay accepts 1-31 and warns for days that don't exist in every month,
// which SQL Server Agent skips (spike result S5).
func validateAgentMonthlyDay(i interface{}, path cty.Path) diag.Diagnostics {
	day := i.(int)
	if day < 1 || day > 31 {
		return diag.Diagnostics{{Severity: diag.Error, Summary: fmt.Sprintf("monthly_day must be between 1 and 31, got %d", day), AttributePath: path}}
	}
	months := map[int]string{
		29: "February, except in leap years",
		30: "February",
		31: "February, April, June, September and November",
	}[day]
	if months == "" {
		return nil
	}
	return diag.Diagnostics{{
		Severity: diag.Warning,
		Summary:  fmt.Sprintf("monthly_day %d doesn't exist in every month", day),
		Detail: fmt.Sprintf("SQL Server Agent skips months without day %d (%s). For the last day of every month, use "+
			"frequency_type = \"monthly_relative\" with monthly_relative_week = \"last\" and monthly_relative_day = \"day\".", day, months),
		AttributePath: path,
	}}
}
