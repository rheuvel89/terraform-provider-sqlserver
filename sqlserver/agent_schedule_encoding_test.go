package sqlserver

import (
	"reflect"
	"strings"
	"terraform-provider-sqlserver/sqlserver/model"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
)

func TestAgentScheduleEncoding(t *testing.T) {
	base := func(s agentScheduleSpec) agentScheduleSpec {
		if s.RecurrenceFactor == 0 {
			s.RecurrenceFactor = 1
		}
		if s.SubdayType == "" {
			s.SubdayType = "once"
		}
		s.ActiveStartDate = "2027-01-01"
		s.ActiveEndDate = "9999-12-31"
		if s.ActiveStartTime == "" {
			s.ActiveStartTime = "02:30:00"
		}
		if s.ActiveEndTime == "" {
			s.ActiveEndTime = "23:59:59"
		}
		return s
	}
	msdb := func(s model.AgentSchedule) model.AgentSchedule {
		s.ActiveStartDate = 20270101
		s.ActiveEndDate = 99991231
		if s.ActiveStartTime == 0 {
			s.ActiveStartTime = 23000
		}
		if s.ActiveEndTime == 0 {
			s.ActiveEndTime = 235959
		}
		return s
	}

	tests := []struct {
		name string
		spec agentScheduleSpec
		want model.AgentSchedule
	}{
		{"once", base(agentScheduleSpec{FrequencyType: "once"}), msdb(model.AgentSchedule{FreqType: 1})},
		{"agent_start", base(agentScheduleSpec{FrequencyType: "agent_start"}), msdb(model.AgentSchedule{FreqType: 64})},
		{"idle", base(agentScheduleSpec{FrequencyType: "idle"}), msdb(model.AgentSchedule{FreqType: 128})},
		{"daily", base(agentScheduleSpec{FrequencyType: "daily"}),
			msdb(model.AgentSchedule{FreqType: 4, FreqInterval: 1, FreqSubdayType: 1})},
		{"every 3 days", base(agentScheduleSpec{FrequencyType: "daily", RecurrenceFactor: 3}),
			msdb(model.AgentSchedule{FreqType: 4, FreqInterval: 3, FreqSubdayType: 1})},
		{"every 5 minutes", base(agentScheduleSpec{FrequencyType: "daily", SubdayType: "minutes", SubdayInterval: 5}),
			msdb(model.AgentSchedule{FreqType: 4, FreqInterval: 1, FreqSubdayType: 4, FreqSubdayInterval: 5})},
		{"every 10 seconds", base(agentScheduleSpec{FrequencyType: "daily", SubdayType: "seconds", SubdayInterval: 10}),
			msdb(model.AgentSchedule{FreqType: 4, FreqInterval: 1, FreqSubdayType: 2, FreqSubdayInterval: 10})},
		{"weekdays", base(agentScheduleSpec{FrequencyType: "weekly", WeeklyDays: []string{"monday", "tuesday", "wednesday", "thursday", "friday"}}),
			msdb(model.AgentSchedule{FreqType: 8, FreqInterval: 62, FreqSubdayType: 1, FreqRecurrenceFactor: 1})},
		{"every day of the week", base(agentScheduleSpec{FrequencyType: "weekly", WeeklyDays: []string{"sunday", "monday", "tuesday", "wednesday", "thursday", "friday", "saturday"}}),
			msdb(model.AgentSchedule{FreqType: 8, FreqInterval: 127, FreqSubdayType: 1, FreqRecurrenceFactor: 1})},
		{"every other friday", base(agentScheduleSpec{FrequencyType: "weekly", RecurrenceFactor: 2, WeeklyDays: []string{"friday"}}),
			msdb(model.AgentSchedule{FreqType: 8, FreqInterval: 32, FreqSubdayType: 1, FreqRecurrenceFactor: 2})},
		{"quarterly", base(agentScheduleSpec{FrequencyType: "monthly", RecurrenceFactor: 3, MonthlyDay: 1}),
			msdb(model.AgentSchedule{FreqType: 16, FreqInterval: 1, FreqSubdayType: 1, FreqRecurrenceFactor: 3})},
		{"first sunday", base(agentScheduleSpec{FrequencyType: "monthly_relative", MonthlyRelativeWeek: "first", MonthlyRelativeDay: "sunday"}),
			msdb(model.AgentSchedule{FreqType: 32, FreqInterval: 1, FreqRelativeInterval: 1, FreqSubdayType: 1, FreqRecurrenceFactor: 1})},
		{"last day", base(agentScheduleSpec{FrequencyType: "monthly_relative", MonthlyRelativeWeek: "last", MonthlyRelativeDay: "day"}),
			msdb(model.AgentSchedule{FreqType: 32, FreqInterval: 8, FreqRelativeInterval: 16, FreqSubdayType: 1, FreqRecurrenceFactor: 1})},
		{"first weekend day every 6h", base(agentScheduleSpec{FrequencyType: "monthly_relative", MonthlyRelativeWeek: "first", MonthlyRelativeDay: "weekend_day", SubdayType: "hours", SubdayInterval: 6}),
			msdb(model.AgentSchedule{FreqType: 32, FreqInterval: 10, FreqRelativeInterval: 1, FreqSubdayType: 8, FreqSubdayInterval: 6, FreqRecurrenceFactor: 1})},
		{"window across midnight", base(agentScheduleSpec{FrequencyType: "daily", SubdayType: "hours", SubdayInterval: 1, ActiveStartTime: "22:00:00", ActiveEndTime: "02:00:00"}),
			msdb(model.AgentSchedule{FreqType: 4, FreqInterval: 1, FreqSubdayType: 8, FreqSubdayInterval: 1, ActiveStartTime: 220000, ActiveEndTime: 20000})},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := validateAgentScheduleSpec(tt.spec, nil); err != nil {
				t.Fatalf("spec should be valid: %v", err)
			}
			got, err := encodeAgentSchedule(tt.spec)
			if err != nil {
				t.Fatalf("encode: %v", err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("encode:\n got  %+v\n want %+v", got, tt.want)
			}
			if back := decodeAgentSchedule(got); !reflect.DeepEqual(back, tt.spec) {
				t.Fatalf("round trip:\n got  %+v\n want %+v", back, tt.spec)
			}
		})
	}
}

func TestAgentScheduleDecodeNonRecurring(t *testing.T) {
	// sp_add_schedule zeroes the frequency fields of once/agent_start/idle schedules, including freq_subday_type.
	got := decodeAgentSchedule(model.AgentSchedule{FreqType: 1, ActiveStartDate: 20261205, ActiveEndDate: 99991231, ActiveStartTime: 220000, ActiveEndTime: 235959})
	want := agentScheduleSpec{FrequencyType: "once", RecurrenceFactor: 1, SubdayType: "once",
		ActiveStartDate: "2026-12-05", ActiveEndDate: "9999-12-31", ActiveStartTime: "22:00:00", ActiveEndTime: "23:59:59"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func TestAgentDateTimeValidation(t *testing.T) {
	for _, s := range []string{"2026-02-30", "2026-13-01", "2025-02-29", "1989-12-31", "2026-2-01", "2026/01/01", "tomorrow"} {
		if _, err := parseAgentDate(s); err == nil {
			t.Errorf("date %q should be rejected", s)
		}
	}
	for s, want := range map[string]int{"2028-02-29": 20280229, "1990-01-01": 19900101, "9999-12-31": 99991231, "": 0} {
		if got, err := parseAgentDate(s); err != nil || got != want {
			t.Errorf("date %q: got %d, %v; want %d", s, got, err, want)
		}
	}
	for _, s := range []string{"24:00:00", "12:61:00", "12:00:60", "7:00:00", "12:00"} {
		if _, err := parseAgentTime(s); err == nil {
			t.Errorf("time %q should be rejected", s)
		}
	}
	for s, want := range map[string]int{"23:59:59": 235959, "00:00:00": 0, "02:30:00": 23000} {
		if got, err := parseAgentTime(s); err != nil || got != want {
			t.Errorf("time %q: got %d, %v; want %d", s, got, err, want)
		}
	}
}

func TestAgentMonthlyDayValidation(t *testing.T) {
	for day := 0; day <= 32; day++ {
		diags := validateAgentMonthlyDay(day, nil)
		switch {
		case day < 1 || day > 31:
			if !diags.HasError() {
				t.Errorf("day %d should be an error", day)
			}
		case day >= 29:
			if diags.HasError() || len(diags) != 1 || diags[0].Severity != diag.Warning {
				t.Errorf("day %d should be a single warning, got %v", day, diags)
			}
		default:
			if len(diags) != 0 {
				t.Errorf("day %d should be accepted silently, got %v", day, diags)
			}
		}
	}
}

func TestAgentScheduleSpecValidation(t *testing.T) {
	valid := agentScheduleSpec{FrequencyType: "daily", RecurrenceFactor: 1, SubdayType: "once", ActiveEndDate: "9999-12-31", ActiveStartTime: "00:00:00", ActiveEndTime: "23:59:59"}
	with := func(f func(*agentScheduleSpec)) agentScheduleSpec {
		s := valid
		f(&s)
		return s
	}

	tests := []struct {
		name    string
		spec    agentScheduleSpec
		unknown map[string]bool
		wantErr string
	}{
		{"valid daily", valid, nil, ""},
		{"weekly without days", with(func(s *agentScheduleSpec) { s.FrequencyType = "weekly" }), nil, "weekly_days is required"},
		{"weekly days unknown", with(func(s *agentScheduleSpec) { s.FrequencyType = "weekly" }), map[string]bool{"weekly_days": true}, ""},
		{"daily with days", with(func(s *agentScheduleSpec) { s.WeeklyDays = []string{"monday"} }), nil, "weekly_days can't be set"},
		{"monthly without day", with(func(s *agentScheduleSpec) { s.FrequencyType = "monthly" }), nil, "monthly_day is required"},
		{"monthly_relative without week", with(func(s *agentScheduleSpec) { s.FrequencyType = "monthly_relative"; s.MonthlyRelativeDay = "day" }), nil, "monthly_relative_week is required"},
		{"once with recurrence", with(func(s *agentScheduleSpec) { s.FrequencyType = "once"; s.RecurrenceFactor = 2 }), nil, "recurrence_factor can't be set"},
		{"agent_start with subday", with(func(s *agentScheduleSpec) { s.FrequencyType = "agent_start"; s.SubdayType = "hours" }), nil, "subday_type can't be set"},
		{"interval with once", with(func(s *agentScheduleSpec) { s.SubdayInterval = 5 }), nil, "subday_interval can't be set when subday_type is \"once\""},
		{"minutes without interval", with(func(s *agentScheduleSpec) { s.SubdayType = "minutes" }), nil, "subday_interval is required"},
		{"5 seconds", with(func(s *agentScheduleSpec) { s.SubdayType = "seconds"; s.SubdayInterval = 5 }), nil, "at least 10"},
		{"10 seconds", with(func(s *agentScheduleSpec) { s.SubdayType = "seconds"; s.SubdayInterval = 10 }), nil, ""},
		{"end date before start", with(func(s *agentScheduleSpec) { s.ActiveStartDate = "2027-01-02"; s.ActiveEndDate = "2027-01-01" }), nil, "active_end_date (2027-01-01) is before"},
		{"end date before start, start unknown", with(func(s *agentScheduleSpec) { s.ActiveStartDate = "2027-01-02"; s.ActiveEndDate = "2027-01-01" }), map[string]bool{"active_start_date": true}, ""},
		{"end time midnight", with(func(s *agentScheduleSpec) { s.ActiveEndTime = "00:00:00" }), nil, "use \"23:59:59\""},
		{"equal times with interval", with(func(s *agentScheduleSpec) {
			s.SubdayType = "hours"
			s.SubdayInterval = 1
			s.ActiveStartTime = "02:00:00"
			s.ActiveEndTime = "02:00:00"
		}), nil, "can't be equal"},
		{"equal times once a day", with(func(s *agentScheduleSpec) { s.ActiveStartTime = "23:59:59" }), nil, ""},
		{"end time before start time", with(func(s *agentScheduleSpec) {
			s.SubdayType = "hours"
			s.SubdayInterval = 1
			s.ActiveStartTime = "22:00:00"
			s.ActiveEndTime = "02:00:00"
		}), nil, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateAgentScheduleSpec(tt.spec, tt.unknown)
			switch {
			case tt.wantErr == "" && err != nil:
				t.Fatalf("unexpected error: %v", err)
			case tt.wantErr != "" && err == nil:
				t.Fatalf("expected error containing %q", tt.wantErr)
			case tt.wantErr != "" && !strings.Contains(err.Error(), tt.wantErr):
				t.Fatalf("expected error containing %q, got %v", tt.wantErr, err)
			}
		})
	}
}
