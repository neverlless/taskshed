package api

import (
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/neverlless/taskshed/internal/database"
)

var (
	// Minutes may be unpadded ("1:5" = 01:05): the Bareos syncer and other 1.x clients send that.
	clockRe    = regexp.MustCompile(`^(\d{1,2}):(\d{1,2})$`)
	durationRe = regexp.MustCompile(`^(\d{1,3}):(\d{1,2})$`)
	weekdays   = []string{"Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun"}
)

const maxDurationMinutes = 168 * 60

// validateTask checks t and normalizes it in place. The error message is safe
// to show to clients.
func validateTask(t *database.Task) error {
	t.Name = strings.TrimSpace(t.Name)
	t.Service = strings.TrimSpace(t.Service)
	for _, f := range []struct {
		name, value string
		required    bool
		max         int
	}{
		{"name", t.Name, true, 200},
		{"service", t.Service, true, 200},
		{"description", t.Description, false, 2000},
		{"hosts", t.Hosts, false, 2000},
	} {
		if f.required && f.value == "" {
			return fmt.Errorf("%s is required", f.name)
		}
		if utf8.RuneCountInString(f.value) > f.max {
			return fmt.Errorf("%s must be at most %d characters", f.name, f.max)
		}
	}

	m := clockRe.FindStringSubmatch(t.Time)
	if m == nil {
		return errors.New("time must be HH:MM")
	}
	h, _ := strconv.Atoi(m[1])
	min, _ := strconv.Atoi(m[2])
	if h > 23 || min > 59 {
		return errors.New("time must be between 00:00 and 23:59")
	}
	t.Time = fmt.Sprintf("%02d:%02d", h, min)

	if t.Duration != "" {
		m := durationRe.FindStringSubmatch(t.Duration)
		if m == nil {
			return errors.New("duration must be H:MM")
		}
		h, _ := strconv.Atoi(m[1])
		min, _ := strconv.Atoi(m[2])
		if total := h*60 + min; min > 59 || total == 0 || total > maxDurationMinutes {
			return errors.New("duration must be between 0:01 and 168:00 with minutes below 60")
		}
		t.Duration = fmt.Sprintf("%02d:%02d", h, min)
	}

	days, err := normalizeDays(t.DaysOfWeek)
	if err != nil {
		return err
	}
	if t.IsRecurring && days == "" {
		return errors.New("days_of_week is required for recurring tasks")
	}
	t.DaysOfWeek = days

	if t.Date != "" {
		if _, err := time.Parse(time.DateOnly, t.Date); err != nil {
			return errors.New("date must be YYYY-MM-DD")
		}
	}
	return nil
}

// normalizeDays turns "monday, wed,FRI,mon" into "Mon,Wed,Fri".
func normalizeDays(s string) (string, error) {
	var out []string
	for part := range strings.SplitSeq(s, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		day := ""
		if len(part) >= 3 {
			for _, d := range weekdays {
				if strings.EqualFold(part[:3], d) {
					day = d
				}
			}
		}
		if day == "" {
			return "", fmt.Errorf("days_of_week: unknown day %q", part)
		}
		if !slices.Contains(out, day) {
			out = append(out, day)
		}
	}
	return strings.Join(out, ","), nil
}
