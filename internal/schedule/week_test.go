package schedule

import (
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/neverlless/taskshed/internal/database"
)

func TestPack(t *testing.T) {
	tests := []struct {
		name  string
		spans []Span
		want  []Placement
	}{
		{"no overlap", []Span{{0, 60}, {60, 120}, {200, 300}},
			[]Placement{{0, 1}, {0, 1}, {0, 1}}},
		{"two overlapping", []Span{{0, 60}, {30, 90}, {120, 180}},
			[]Placement{{0, 2}, {1, 2}, {0, 1}}},
		{"chain A-B-C, A and C apart", []Span{{0, 60}, {30, 90}, {60, 120}},
			[]Placement{{0, 2}, {1, 2}, {0, 2}}},
		{"identical times", []Span{{60, 120}, {60, 120}, {60, 120}},
			[]Placement{{0, 3}, {1, 3}, {2, 3}}},
		{"unsorted input keeps index order", []Span{{30, 90}, {0, 60}},
			[]Placement{{1, 2}, {0, 2}}},
		{"empty", nil, []Placement{}},
	}
	for _, tt := range tests {
		if got := Pack(tt.spans); !slices.Equal(got, tt.want) {
			t.Errorf("%s: %v; want %v", tt.name, got, tt.want)
		}
	}
}

func TestWeekStart(t *testing.T) {
	berlin := zone(t, "Europe/Berlin")
	for _, in := range []time.Time{
		time.Date(2026, 10, 26, 0, 0, 0, 0, berlin),
		time.Date(2026, 11, 1, 23, 59, 0, 0, berlin),
		utc(2026, 10, 25, 23, 30), // already Monday in Berlin
	} {
		if got := WeekStart(in, berlin); !got.Equal(time.Date(2026, 10, 26, 0, 0, 0, 0, berlin)) {
			t.Errorf("WeekStart(%v) = %v", in, got)
		}
	}
}

func TestWeekSplitsAndPacks(t *testing.T) {
	berlin := zone(t, "Europe/Berlin") // UTC+1 from 25 Oct 2026
	monday := time.Date(2026, 10, 26, 0, 0, 0, 0, berlin)
	tasks := []database.Task{
		recurring("21:00", "03:00", "Tue"),     // Tue 22:00 – Wed 01:00 local
		recurring("23:30", "00:30", "Mon,Tue"), // 00:30–01:00 on Tue, Wed
		{Name: "drill", Time: "09:00", Duration: "02:00", Date: "2026-10-30"},
		{Name: "undated", Time: "09:00"},
		recurring("23:30", "02:00", "Sun"), // Sun 25 Oct 23:30 UTC runs into Mon 00:30–02:30 local
	}
	days := Week(tasks, monday)
	if len(days) != 7 || !days[6].Date.Equal(time.Date(2026, 11, 1, 0, 0, 0, 0, berlin)) {
		t.Fatalf("days %v", days)
	}
	type got struct {
		start, end           int
		continued, continues bool
		col, cols            int
	}
	summary := func(d Day) []got {
		var out []got
		for _, p := range d.Pieces {
			out = append(out, got{p.Start, p.End, p.Continued, p.Continues, p.Col, p.Cols})
		}
		return out
	}
	check := func(i int, want []got) {
		t.Helper()
		if g := summary(days[i]); !slices.Equal(g, want) {
			t.Errorf("day %d: %+v; want %+v", i, g, want)
		}
	}
	check(0, []got{{30, 150, false, false, 0, 1}}) // Monday: Sunday-UTC run, local
	check(1, []got{{30, 60, false, false, 0, 1}, {22 * 60, 1440, false, true, 0, 1}})
	check(2, []got{{0, 60, true, false, 0, 2}, {30, 60, false, false, 1, 2}})
	check(4, []got{{600, 720, false, false, 0, 1}})
	check(6, nil)

	cont := days[2].Pieces[0]
	if cont.TimeLabel(berlin) != "until 01:00" || cont.When(berlin) != "Tue 22:00–01:00" {
		t.Errorf("continuation: %q %q", cont.TimeLabel(berlin), cont.When(berlin))
	}
	if l := days[1].Pieces[1].TimeLabel(berlin); l != "22:00–01:00" {
		t.Errorf("label %q", l)
	}
}

func TestTooltip(t *testing.T) {
	p := Piece{
		Task: database.Task{Name: "Failover drill", Service: "Platform", Hosts: "db-1,db-2", Description: "Planned switch"},
		Run:  Occurrence{Start: utc(2026, 10, 30, 9, 0), End: utc(2026, 10, 30, 11, 0), HasDuration: true},
	}
	want := "Failover drill\nPlatform\nOne-off, Fri, 30 Oct 2026, 10:00–12:00\nHosts: db-1, db-2\nPlanned switch"
	if got := p.Tooltip(zone(t, "Europe/Berlin")); got != want {
		t.Errorf("%q", got)
	}
	p.Task.IsRecurring, p.Task.Hosts, p.Task.Description = true, "", ""
	p.Run.HasDuration = false
	if got := p.Tooltip(time.UTC); got != "Failover drill\nPlatform\n09:00, no set duration" {
		t.Errorf("%q", got)
	}
}

func TestWeekTitle(t *testing.T) {
	for in, want := range map[string]string{
		"2026-10-26": "26 Oct – 1 Nov 2026",
		"2026-11-02": "2 – 8 Nov 2026",
		"2026-12-28": "28 Dec 2026 – 3 Jan 2027",
	} {
		d, _ := time.Parse(time.DateOnly, in)
		if got := WeekTitle(d); got != want {
			t.Errorf("%s: %q; want %q", in, got, want)
		}
	}
}

func TestWeekGroupsCrowdedService(t *testing.T) {
	monday := time.Date(2026, 10, 26, 0, 0, 0, 0, time.UTC)
	backup := func(name, at, dur string) database.Task {
		t := recurring(at, dur, "Mon")
		t.Name, t.Service = name, "Bareos"
		return t
	}
	patch := recurring("21:00", "01:00", "Mon")
	patch.Name, patch.Service = "patch", "AWX"
	tasks := []database.Task{
		backup("a", "20:00", "02:00"), // 20:00–22:00
		backup("b", "20:00", "01:00"), // 20:00–21:00
		backup("c", "20:30", "01:00"), // 20:30–21:30
		backup("d", "20:45", "00:45"), // 20:45–21:30: four at once from 20:45
		backup("e", "21:45", "03:00"), // overlaps a, runs past midnight
		backup("lonely", "06:00", "01:00"),
		patch,
	}
	day := Week(tasks, monday)[0]

	if len(day.Groups) != 1 {
		t.Fatalf("groups = %+v", day.Groups)
	}
	g := day.Groups[0]
	if g.Service != "Bareos" || len(g.Pieces) != 5 || g.Peak != 4 || g.Start != 20*60 || g.End != 24*60 ||
		g.Continued || !g.Continues || g.TimeLabel() != "from 20:00" || g.Label() != "5 Bareos jobs" {
		t.Errorf("group = %+v (%s)", g, g.TimeLabel())
	}
	// 30-minute slots from 20:00: a,b | a,b,c,d | a,c,d | a,e | e | e | e | e
	if want := []int{2, 4, 3, 2, 1, 1, 1, 1}; !slices.Equal(g.Profile, want) {
		t.Errorf("profile = %v; want %v", g.Profile, want)
	}
	// The lone backup and the AWX patch stay single events; the patch sits beside the group.
	var names []string
	for _, p := range day.Pieces {
		names = append(names, p.Task.Name)
	}
	if !slices.Equal(names, []string{"lonely", "patch"}) {
		t.Errorf("pieces = %v", names)
	}
	if p := day.Pieces[1]; p.Cols != 2 || g.Cols != 2 || p.Col == g.Col {
		t.Errorf("patch col %d/%d, group col %d/%d", p.Col, p.Cols, g.Col, g.Cols)
	}
	if !strings.HasPrefix(g.Tooltip(), "5 Bareos jobs\nfrom 20:00, up to 4 at once\na\n") {
		t.Errorf("tooltip = %q", g.Tooltip())
	}
}

func TestWeekKeepsFewOverlapsAsEvents(t *testing.T) {
	monday := time.Date(2026, 10, 26, 0, 0, 0, 0, time.UTC)
	tasks := []database.Task{
		recurring("20:00", "02:00", "Mon"),
		recurring("20:00", "02:00", "Mon"),
		recurring("20:00", "02:00", "Mon"),
	}
	day := Week(tasks, monday)[0]
	if len(day.Groups) != 0 || len(day.Pieces) != 3 || day.Pieces[0].Cols != 3 {
		t.Errorf("groups %d, pieces %d", len(day.Groups), len(day.Pieces))
	}
}

func TestWeekBackToBackRunsDoNotGroup(t *testing.T) {
	monday := time.Date(2026, 10, 26, 0, 0, 0, 0, time.UTC)
	var tasks []database.Task
	for _, at := range []string{"20:00", "21:00", "22:00", "23:00"} {
		tasks = append(tasks, recurring(at, "01:00", "Mon"))
	}
	if day := Week(tasks, monday)[0]; len(day.Groups) != 0 || len(day.Pieces) != 4 {
		t.Errorf("groups %d, pieces %d", len(day.Groups), len(day.Pieces))
	}
}
