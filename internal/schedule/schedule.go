// Package schedule turns stored tasks (UTC clock time, UTC weekdays, optional
// duration and date) into concrete occurrences and per-viewer table rows.
package schedule

import (
	"fmt"
	"math"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/neverlless/taskshed/internal/database"
)

const (
	day        = 24 * time.Hour
	dayMinutes = 24 * 60
	// DefaultLength is how long a task without a duration is drawn on the calendar.
	DefaultLength = time.Hour
)

// Unpadded minutes ("1:5") occur in data written before validation existed.
var hmRe = regexp.MustCompile(`^(\d{1,3}):(\d{1,2})$`)

// Palette colours services; all hues are light enough for dark event text.
var Palette = []string{"#8DB8F2", "#93D3A2", "#E9A6C9", "#C2AEF5", "#7FD4CF", "#F2AE8A", "#CFD879", "#B6C2D1"}

// WeekOrder lists weekdays Monday first, as the UI shows them.
var WeekOrder = []time.Weekday{time.Monday, time.Tuesday, time.Wednesday, time.Thursday, time.Friday, time.Saturday, time.Sunday}

// Occurrence is one run of a task. Without a duration, End is Start+DefaultLength.
type Occurrence struct {
	Start, End  time.Time
	HasDuration bool
}

// parseHM parses "H:MM" up to "999:59" into minutes.
func parseHM(s string) (int, bool) {
	m := hmRe.FindStringSubmatch(strings.TrimSpace(s))
	if m == nil {
		return 0, false
	}
	h, _ := strconv.Atoi(m[1])
	mm, _ := strconv.Atoi(m[2])
	if mm > 59 {
		return 0, false
	}
	return h*60 + mm, true
}

// utcDays parses "Mon,Wed" (case-insensitive, 3-letter prefix) into weekdays.
func utcDays(s string) map[time.Weekday]bool {
	days := map[time.Weekday]bool{}
	for part := range strings.SplitSeq(s, ",") {
		part = strings.TrimSpace(part)
		if len(part) < 3 {
			continue
		}
		for _, d := range WeekOrder {
			if strings.EqualFold(part[:3], d.String()[:3]) {
				days[d] = true
			}
		}
	}
	return days
}

// Occurrences returns the runs of t that overlap [from, to), ordered by start.
// Runs that started before from but are still going are included.
func Occurrences(t database.Task, from, to time.Time) []Occurrence {
	startMin, ok := parseHM(t.Time)
	if !ok || startMin >= dayMinutes {
		return nil
	}
	durMin, hasDur := parseHM(t.Duration)
	hasDur = hasDur && durMin > 0
	length := DefaultLength
	if hasDur {
		length = time.Duration(durMin) * time.Minute
	}
	offset := time.Duration(startMin) * time.Minute

	var out []Occurrence
	emit := func(start time.Time) {
		if end := start.Add(length); start.Before(to) && end.After(from) {
			out = append(out, Occurrence{Start: start, End: end, HasDuration: hasDur})
		}
	}
	if !t.IsRecurring {
		date, err := time.Parse(time.DateOnly, t.Date)
		if err == nil {
			emit(date.Add(offset))
		}
		return out
	}
	days := utcDays(t.DaysOfWeek)
	if len(days) == 0 {
		return nil
	}
	// Truncate on a UTC time yields UTC midnight.
	for d := from.UTC().Add(-length - offset).Truncate(day); d.Before(to); d = d.Add(day) {
		if days[d.Weekday()] {
			emit(d.Add(offset))
		}
	}
	return out
}

// Running reports whether a run of t with a set duration covers now.
func Running(t database.Task, now time.Time) bool {
	for _, o := range Occurrences(t, now, now.Add(time.Nanosecond)) {
		if o.HasDuration {
			return true
		}
	}
	return false
}

// Row is a task as the viewer in Loc sees it at Now.
type Row struct {
	Task  database.Task
	Valid bool // false if the stored time cannot be parsed
	// Start is the local minute of day the task starts; Duration is 0 when unset.
	Start, Duration int
	// Days are the local weekdays of a recurring task, Monday first.
	Days []time.Weekday
	// Date is the local start of a dated one-off; zero otherwise.
	Date    time.Time
	Running bool
}

// NewRow localizes t for a viewer in loc at now. Recurring tasks are mapped
// through their actual runs in the current UTC week, so a weekday shift across
// midnight and DST are both exact for that week.
func NewRow(t database.Task, loc *time.Location, now time.Time) Row {
	r := Row{Task: t}
	startMin, ok := parseHM(t.Time)
	if !ok || startMin >= dayMinutes {
		return r
	}
	r.Valid = true
	if d, ok := parseHM(t.Duration); ok {
		r.Duration = d
	}
	r.Running = Running(t, now)

	var ref time.Time
	switch {
	case t.IsRecurring:
		today := now.UTC().Truncate(day)
		monday := today.Add(-time.Duration((int(today.Weekday())+6)%7) * day)
		seen := map[time.Weekday]bool{}
		best := time.Duration(math.MaxInt64)
		for _, o := range Occurrences(t, monday, monday.Add(7*day)) {
			if o.Start.Before(monday) {
				continue
			}
			seen[o.Start.In(loc).Weekday()] = true
			// The run nearest to now sets the clock time shown for the week.
			if d := absDur(o.Start.Sub(now)); d < best {
				best, ref = d, o.Start
			}
		}
		for _, d := range WeekOrder {
			if seen[d] {
				r.Days = append(r.Days, d)
			}
		}
	default:
		if date, err := time.Parse(time.DateOnly, t.Date); err == nil {
			ref = date.Add(time.Duration(startMin) * time.Minute)
			r.Date = ref.In(loc)
		}
	}
	if ref.IsZero() {
		// Undated one-off (or no valid days): use today's offset.
		ref = now.UTC().Truncate(day).Add(time.Duration(startMin) * time.Minute)
	}
	local := ref.In(loc)
	r.Start = local.Hour()*60 + local.Minute()
	return r
}

func absDur(d time.Duration) time.Duration {
	if d < 0 {
		return -d
	}
	return d
}

// Window formats the local window: "22:00–02:00", "09:00–11:00 (ends 2 days
// later)" for windows of a day or more, or "09:00, no set duration".
func Window(start, duration int) string {
	if duration <= 0 {
		return clock(start) + ", no set duration"
	}
	end := start + duration
	s := clock(start) + "–" + clock(end)
	if duration >= dayMinutes {
		n := end / dayMinutes
		plural := "s"
		if n == 1 {
			plural = ""
		}
		s += fmt.Sprintf(" (ends %d day%s later)", n, plural)
	}
	return s
}

// Window is the row's local window text.
func (r Row) Window() string { return Window(r.Start, r.Duration) }

// EndClock is the local end time "HH:MM", or "" without a duration.
func (r Row) EndClock() string {
	if r.Duration <= 0 {
		return ""
	}
	return clock(r.Start + r.Duration)
}

// StartClock is the local start time "HH:MM".
func (r Row) StartClock() string { return clock(r.Start) }

// DayNames lists the local weekdays as "Mon", "Tue", ...
func (r Row) DayNames() []string {
	out := make([]string, len(r.Days))
	for i, d := range r.Days {
		out[i] = d.String()[:3]
	}
	return out
}

// HasDay reports whether the task runs on local weekday d.
func (r Row) HasDay(d time.Weekday) bool { return slices.Contains(r.Days, d) }

// clock formats minutes as "HH:MM", wrapped to one day.
func clock(min int) string {
	min = ((min % dayMinutes) + dayMinutes) % dayMinutes
	return fmt.Sprintf("%02d:%02d", min/60, min%60)
}

// FormatDate formats a local date in English, e.g. "Thu, 5 Nov 2026".
func FormatDate(t time.Time) string { return t.Format("Mon, 2 Jan 2006") }

// Segment is part of the 24-hour strip, in percent of the day.
type Segment struct {
	Left, Width float64
	Point       bool // no duration: a thin mark at the start
}

// Segments draws the window on a 24h strip; windows past midnight wrap to the
// start, and anything beyond a second day is capped at a full strip.
func (r Row) Segments() []Segment {
	if r.Duration <= 0 {
		return []Segment{{Left: pct(r.Start), Point: true}}
	}
	end := r.Start + r.Duration
	segs := []Segment{{Left: pct(r.Start), Width: pct(min(end, dayMinutes) - r.Start)}}
	if end > dayMinutes {
		segs = append(segs, Segment{Width: pct(min(end-dayMinutes, dayMinutes))})
	}
	return segs
}

func pct(min int) float64 { return float64(min) / dayMinutes * 100 }

// NowPercent places now on the viewer's 24h strip.
func NowPercent(now time.Time, loc *time.Location) float64 {
	l := now.In(loc)
	return pct(l.Hour()*60 + l.Minute())
}

// Services returns the sorted unique services of tasks.
func Services(tasks []database.Task) []string {
	var out []string
	for _, t := range tasks {
		out = append(out, t.Service)
	}
	slices.Sort(out)
	return slices.Compact(out)
}

// Colors assigns palette colours to sorted services, so the legend and the
// calendar always agree.
func Colors(services []string) map[string]string {
	m := make(map[string]string, len(services))
	for i, s := range services {
		m[s] = Palette[i%len(Palette)]
	}
	return m
}
