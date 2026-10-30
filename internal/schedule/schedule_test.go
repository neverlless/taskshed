package schedule

import (
	"slices"
	"testing"
	"time"

	"github.com/neverlless/taskshed/internal/database"
)

func zone(t *testing.T, name string) *time.Location {
	t.Helper()
	loc, err := time.LoadLocation(name)
	if err != nil {
		t.Fatal(err)
	}
	return loc
}

func utc(y int, m time.Month, d, h, min int) time.Time {
	return time.Date(y, m, d, h, min, 0, 0, time.UTC)
}

func recurring(at, dur, days string) database.Task {
	return database.Task{Name: "t", Service: "s", Time: at, Duration: dur, DaysOfWeek: days, IsRecurring: true}
}

var (
	sun, mon, tue, wed, thu = time.Sunday, time.Monday, time.Tuesday, time.Wednesday, time.Thursday
)

func TestRowDayShift(t *testing.T) {
	kyiv, ny := zone(t, "Europe/Kyiv"), zone(t, "America/New_York")
	summer := utc(2026, 7, 15, 12, 0) // Kyiv UTC+3
	winter := utc(2026, 11, 4, 12, 0) // New York UTC-5
	tests := []struct {
		name      string
		task      database.Task
		loc       *time.Location
		now       time.Time
		start     int
		days      []time.Weekday
		window    string
		weekLabel []string
	}{
		{"late UTC evening moves to next local day", recurring("22:30", "", "Mon,Wed"), kyiv, summer,
			90, []time.Weekday{tue, thu}, "01:30, no set duration", []string{"Tue", "Thu"}},
		{"early UTC morning moves to previous local day", recurring("02:00", "", "Mon"), ny, winter,
			21 * 60, []time.Weekday{sun}, "21:00, no set duration", []string{"Sun"}},
		{"window past local midnight", recurring("03:00", "04:00", "Mon"), ny, winter,
			22 * 60, []time.Weekday{sun}, "22:00–02:00", []string{"Sun"}},
		{"multi-day duration", recurring("09:00", "50:00", "Wed"), time.UTC, winter,
			9 * 60, []time.Weekday{wed}, "09:00–11:00 (ends 2 days later)", []string{"Wed"}},
		{"Sunday maps to Monday, Monday first", recurring("23:00", "01:00", "Sun,Mon"), kyiv, winter,
			60, []time.Weekday{mon, tue}, "01:00–02:00", []string{"Mon", "Tue"}},
	}
	for _, tt := range tests {
		r := NewRow(tt.task, tt.loc, tt.now)
		if !r.Valid || r.Start != tt.start || !slices.Equal(r.Days, tt.days) || r.Window() != tt.window ||
			!slices.Equal(r.DayNames(), tt.weekLabel) || !r.Date.IsZero() {
			t.Errorf("%s: got start %d days %v window %q names %v date %v", tt.name, r.Start, r.Days, r.Window(), r.DayNames(), r.Date)
		}
	}
}

func TestRunning(t *testing.T) {
	ny := zone(t, "America/New_York")
	night := recurring("03:00", "04:00", "Mon") // Sun 22:00 – Mon 02:00 in New York
	monday := func(h, m int) time.Time { return time.Date(2026, 11, 2, h, m, 0, 0, ny) }
	if monday(0, 30).Weekday() != mon {
		t.Fatal("2 Nov 2026 is a Monday")
	}
	for _, tt := range []struct {
		now  time.Time
		want bool
	}{
		{monday(0, 30), true}, // window started yesterday (local)
		{time.Date(2026, 11, 1, 22, 0, 0, 0, ny), true},
		{monday(2, 0), false}, // end is exclusive
		{monday(21, 59), false},
	} {
		if got := NewRow(night, ny, tt.now).Running; got != tt.want {
			t.Errorf("running at %v = %v; want %v", tt.now, got, tt.want)
		}
	}
	// Three-day window started two UTC days ago.
	long := recurring("09:00", "72:00", "Mon")
	if !Running(long, utc(2026, 11, 4, 20, 0)) || Running(long, utc(2026, 11, 5, 9, 0)) {
		t.Error("multi-day running window")
	}
	if Running(recurring("03:00", "", "Mon"), utc(2026, 11, 2, 3, 30)) {
		t.Error("a task without duration is never running")
	}
}

func TestDatedOneOff(t *testing.T) {
	kyiv := zone(t, "Europe/Kyiv") // UTC+2 in November
	task := database.Task{Time: "22:30", Duration: "01:00", Date: "2026-11-05"}
	r := NewRow(task, kyiv, utc(2026, 10, 29, 12, 0))
	if r.Date.Day() != 6 || r.Date.Hour() != 0 || r.Start != 30 || len(r.Days) != 0 || FormatDate(r.Date) != "Fri, 6 Nov 2026" {
		t.Fatalf("got %+v", r)
	}
	start := utc(2026, 11, 5, 22, 30)
	occ := Occurrences(task, start.Add(-30*24*time.Hour), start.Add(30*24*time.Hour))
	if len(occ) != 1 || !occ[0].Start.Equal(start) || occ[0].End.Sub(occ[0].Start) != time.Hour || !occ[0].HasDuration {
		t.Fatalf("occurrences %+v", occ)
	}
	if !Running(task, start) || Running(task, start.Add(time.Hour)) || Running(task, start.Add(-7*24*time.Hour)) {
		t.Error("running bounds")
	}
	if got := Occurrences(task, start.Add(2*time.Hour), start.Add(48*time.Hour)); len(got) != 0 {
		t.Errorf("out of range: %+v", got)
	}
}

func TestUndatedOneOff(t *testing.T) {
	for _, date := range []string{"", "5 Nov"} {
		task := database.Task{Time: "10:00", Date: date}
		if occ := Occurrences(task, time.Time{}, utc(2100, 1, 1, 0, 0)); len(occ) != 0 {
			t.Errorf("%q: occurrences %+v", date, occ)
		}
		r := NewRow(task, time.UTC, utc(2026, 11, 2, 10, 30))
		if !r.Date.IsZero() || r.Days != nil || r.Running || r.Start != 600 {
			t.Errorf("%q: row %+v", date, r)
		}
	}
}

func TestOccurrencesRecurring(t *testing.T) {
	task := recurring("23:00", "02:00", "Sun,Wed")
	// Mon 2 Nov 00:30 UTC: Sunday's run (23:00–01:00) is still going.
	got := Occurrences(task, utc(2026, 11, 2, 0, 30), utc(2026, 11, 9, 0, 0))
	want := []time.Time{utc(2026, 11, 1, 23, 0), utc(2026, 11, 4, 23, 0), utc(2026, 11, 8, 23, 0)}
	if len(got) != len(want) {
		t.Fatalf("got %+v", got)
	}
	for i, o := range got {
		if !o.Start.Equal(want[i]) || o.End.Sub(o.Start) != 2*time.Hour {
			t.Errorf("%d: %+v", i, o)
		}
	}
	// Without a duration the default hour is used but flagged.
	o := Occurrences(recurring("10:00", "", "Mon"), utc(2026, 11, 2, 0, 0), utc(2026, 11, 3, 0, 0))
	if len(o) != 1 || o[0].HasDuration || o[0].End.Sub(o[0].Start) != DefaultLength {
		t.Errorf("default length %+v", o)
	}
	if Occurrences(recurring("10:00", "", ""), utc(2026, 11, 2, 0, 0), utc(2026, 11, 9, 0, 0)) != nil {
		t.Error("no days, no occurrences")
	}
}

// Occurrences keep their UTC instants across a DST switch; the local clock moves.
func TestDST(t *testing.T) {
	ny, berlin := zone(t, "America/New_York"), zone(t, "Europe/Berlin")
	daily := recurring("12:00", "01:00", "Mon,Tue,Wed,Thu,Fri,Sat,Sun")
	for _, o := range Occurrences(daily, utc(2026, 10, 30, 0, 0), utc(2026, 11, 3, 0, 0)) {
		if o.Start.UTC().Hour() != 12 {
			t.Errorf("UTC start moved: %v", o.Start)
		}
		wantHour := 8 // EDT until 1 Nov 06:00 UTC
		if o.Start.After(utc(2026, 11, 1, 6, 0)) {
			wantHour = 7
		}
		if h := o.Start.In(ny).Hour(); h != wantHour {
			t.Errorf("%v in New York = %d:00; want %d:00", o.Start, h, wantHour)
		}
	}
	// The table shows the clock time of the run nearest now.
	if r := NewRow(daily, ny, utc(2026, 10, 28, 12, 0)); r.Start != 8*60 {
		t.Errorf("before switch: %d", r.Start)
	}
	if r := NewRow(daily, ny, utc(2026, 11, 1, 12, 0)); r.Start != 7*60 {
		t.Errorf("after switch: %d", r.Start)
	}

	// 04:30 UTC Sunday is 00:30 Sunday in New York on 1 Nov (still EDT) but
	// 23:30 Saturday a week later (EST).
	sunday := recurring("04:30", "00:30", "Sun")
	if r := NewRow(sunday, ny, utc(2026, 10, 29, 0, 0)); !slices.Equal(r.Days, []time.Weekday{sun}) || r.Start != 30 {
		t.Errorf("DST week: %v %d", r.Days, r.Start)
	}
	if r := NewRow(sunday, ny, utc(2026, 11, 5, 0, 0)); !slices.Equal(r.Days, []time.Weekday{time.Saturday}) || r.Start != 23*60+30 {
		t.Errorf("week after: %v %d", r.Days, r.Start)
	}

	// Berlin leaves CEST on 25 Oct 2026 at 01:00 UTC.
	early := recurring("00:30", "", "Sun")
	got := Occurrences(early, utc(2026, 10, 25, 0, 0), utc(2026, 11, 2, 0, 0))
	if len(got) != 2 || got[0].Start.In(berlin).Hour() != 2 || got[1].Start.In(berlin).Hour() != 1 {
		t.Errorf("Berlin: %+v", got)
	}
}

func TestSegments(t *testing.T) {
	tests := []struct {
		row  Row
		want []Segment
	}{
		{Row{Start: 360}, []Segment{{Left: 25, Point: true}}},
		{Row{Start: 360, Duration: 360}, []Segment{{Left: 25, Width: 25}}},
		{Row{Start: 18 * 60, Duration: 12 * 60}, []Segment{{Left: 75, Width: 25}, {Width: 25}}},
		{Row{Start: 18 * 60, Duration: 168 * 60}, []Segment{{Left: 75, Width: 25}, {Width: 100}}},
	}
	for _, tt := range tests {
		if got := tt.row.Segments(); !slices.Equal(got, tt.want) {
			t.Errorf("%+v: %+v; want %+v", tt.row, got, tt.want)
		}
	}
	if p := NowPercent(utc(2026, 11, 2, 22, 0), zone(t, "Europe/Kyiv")); p != 0 {
		t.Errorf("now percent %v", p)
	}
}

func TestWindow(t *testing.T) {
	for _, tt := range []struct {
		start, dur int
		want       string
	}{
		{9 * 60, 50 * 60, "09:00–11:00 (ends 2 days later)"},
		{22 * 60, 240, "22:00–02:00"},
		{0, 24 * 60, "00:00–00:00 (ends 1 day later)"},
		{90, 0, "01:30, no set duration"},
	} {
		if got := Window(tt.start, tt.dur); got != tt.want {
			t.Errorf("Window(%d, %d) = %q; want %q", tt.start, tt.dur, got, tt.want)
		}
	}
}

func TestColors(t *testing.T) {
	tasks := []database.Task{{Service: "Web"}, {Service: "AWX"}, {Service: "Web"}, {Service: "DB"}}
	services := Services(tasks)
	if !slices.Equal(services, []string{"AWX", "DB", "Web"}) {
		t.Fatal(services)
	}
	c := Colors(services)
	if c["AWX"] != "#8DB8F2" || c["DB"] != "#93D3A2" || c["Web"] != "#E9A6C9" {
		t.Error(c)
	}
	many := make([]string, 9)
	for i := range many {
		many[i] = string(rune('a' + i))
	}
	if Colors(many)["i"] != Palette[0] {
		t.Error("palette wraps")
	}
}
