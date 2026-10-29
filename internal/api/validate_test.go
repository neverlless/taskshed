package api

import (
	"strings"
	"testing"

	"github.com/neverlless/taskshed/internal/database"
)

func TestValidateTask(t *testing.T) {
	valid := func() database.Task {
		return database.Task{Name: "backup", Service: "db", Time: "02:00", DaysOfWeek: "Mon", IsRecurring: true}
	}
	tests := []struct {
		name    string
		edit    func(*database.Task)
		wantErr string
		check   func(database.Task) bool
	}{
		{"valid", func(*database.Task) {}, "", nil},
		{"trims name", func(t *database.Task) { t.Name = "  x  " }, "", func(t database.Task) bool { return t.Name == "x" }},
		{"empty name", func(t *database.Task) { t.Name = "   " }, "name is required", nil},
		{"empty service", func(t *database.Task) { t.Service = "" }, "service is required", nil},
		{"long name", func(t *database.Task) { t.Name = strings.Repeat("я", 201) }, "name must be at most 200", nil},
		{"200 runes ok", func(t *database.Task) { t.Name = strings.Repeat("я", 200) }, "", nil},
		{"long hosts", func(t *database.Task) { t.Hosts = strings.Repeat("h", 2001) }, "hosts must be at most 2000", nil},
		{"long description", func(t *database.Task) { t.Description = strings.Repeat("d", 2001) }, "description must be at most 2000", nil},
		{"pads time", func(t *database.Task) { t.Time = "8:05" }, "", func(t database.Task) bool { return t.Time == "08:05" }},
		{"bad time", func(t *database.Task) { t.Time = "24:00" }, "time must be between", nil},
		{"bad minutes", func(t *database.Task) { t.Time = "10:60" }, "time must be between", nil},
		{"time format", func(t *database.Task) { t.Time = "10.30" }, "time must be HH:MM", nil},
		{"pads duration", func(t *database.Task) { t.Duration = "1:30" }, "", func(t database.Task) bool { return t.Duration == "01:30" }},
		{"long duration", func(t *database.Task) { t.Duration = "168:00" }, "", func(t database.Task) bool { return t.Duration == "168:00" }},
		{"too long duration", func(t *database.Task) { t.Duration = "168:01" }, "duration must be between", nil},
		{"zero duration", func(t *database.Task) { t.Duration = "0:00" }, "duration must be between", nil},
		{"duration minutes", func(t *database.Task) { t.Duration = "1:75" }, "duration must be between", nil},
		{"duration format", func(t *database.Task) { t.Duration = "90" }, "duration must be H:MM", nil},
		{"normalizes days", func(t *database.Task) { t.DaysOfWeek = " monday, WED,fri,Mon ,, " }, "",
			func(t database.Task) bool { return t.DaysOfWeek == "Mon,Wed,Fri" }},
		{"keeps day order", func(t *database.Task) { t.DaysOfWeek = "sun,mon" }, "", func(t database.Task) bool { return t.DaysOfWeek == "Sun,Mon" }},
		{"unknown day", func(t *database.Task) { t.DaysOfWeek = "Mon,Funday" }, `unknown day "Funday"`, nil},
		{"short day", func(t *database.Task) { t.DaysOfWeek = "Mo" }, "unknown day", nil},
		{"recurring needs days", func(t *database.Task) { t.DaysOfWeek = "" }, "days_of_week is required", nil},
		{"one-off without days", func(t *database.Task) { t.IsRecurring = false; t.DaysOfWeek = ""; t.Date = "2026-11-05" }, "", nil},
		{"bad date", func(t *database.Task) { t.Date = "2026-13-01" }, "date must be YYYY-MM-DD", nil},
		{"date format", func(t *database.Task) { t.Date = "05.11.2026" }, "date must be YYYY-MM-DD", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			task := valid()
			tt.edit(&task)
			err := validateTask(&task)
			switch {
			case tt.wantErr == "" && err != nil:
				t.Fatalf("unexpected error: %v", err)
			case tt.wantErr != "" && (err == nil || !strings.Contains(err.Error(), tt.wantErr)):
				t.Fatalf("error = %v; want %q", err, tt.wantErr)
			case tt.check != nil && !tt.check(task):
				t.Fatalf("not normalized: %+v", task)
			}
		})
	}
}
