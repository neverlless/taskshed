package schedule

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/neverlless/taskshed/internal/database"
)

// Piece is the part of one occurrence that falls on one local day.
type Piece struct {
	Task database.Task
	Run  Occurrence
	// Start and End are local minutes within the day (End ≤ 1440).
	Start, End int
	// Continued: the run began on an earlier day; Continues: it goes past this day.
	Continued, Continues bool
	// Col and Cols place overlapping pieces side by side.
	Col, Cols int
}

// Day is one local day of a week view.
type Day struct {
	Date   time.Time // local midnight
	Pieces []Piece   // ordered by start
	Groups []Group   // ordered by start
}

const (
	// MaxLanes is how many runs of one service may overlap before they are
	// drawn as a Group: past that, side-by-side boxes become unreadable slivers.
	MaxLanes = 3
	// ProfileStep is the resolution of a Group's load profile, in minutes.
	ProfileStep = 30
)

// Group stands in for heavily overlapping runs of one service on one day.
type Group struct {
	Service    string
	Pieces     []Piece
	Start, End int // local minutes within the day
	// Continued: it starts at midnight with a run from an earlier day; Continues: a run goes past this day.
	Continued, Continues bool
	Peak                 int   // most runs going at once
	Profile              []int // runs going in each ProfileStep slot from Start
	Col, Cols            int
}

// WeekStart returns local midnight of the Monday of the week containing t in loc.
func WeekStart(t time.Time, loc *time.Location) time.Time {
	t = t.In(loc)
	back := (int(t.Weekday()) + 6) % 7
	return time.Date(t.Year(), t.Month(), t.Day()-back, 0, 0, 0, 0, loc)
}

// Week splits the runs of tasks into the seven local days starting at monday
// (local midnight). Runs crossing midnight appear on each day they touch.
func Week(tasks []database.Task, monday time.Time) []Day {
	loc := monday.Location()
	days := make([]Day, 7)
	for i := range days {
		start := time.Date(monday.Year(), monday.Month(), monday.Day()+i, 0, 0, 0, 0, loc)
		end := time.Date(monday.Year(), monday.Month(), monday.Day()+i+1, 0, 0, 0, 0, loc)
		days[i].Date = start
		for _, t := range tasks {
			for _, o := range Occurrences(t, start, end) {
				p := Piece{Task: t, Run: o, End: dayMinutes}
				if o.Start.After(start) {
					p.Start = minuteOfDay(o.Start.In(loc))
				} else {
					p.Continued = !o.Start.Equal(start)
				}
				if o.End.Before(end) {
					p.End = max(minuteOfDay(o.End.In(loc)), p.Start)
				} else {
					p.Continues = o.End.After(end)
				}
				days[i].Pieces = append(days[i].Pieces, p)
			}
		}
		slices.SortStableFunc(days[i].Pieces, func(a, b Piece) int { return a.Start - b.Start })
		days[i].Pieces, days[i].Groups = group(days[i].Pieces)
		pack(days[i].Pieces, days[i].Groups)
	}
	return days
}

// minVisualMinutes is how much of the day a short event occupies on screen,
// so packing also separates events whose boxes would touch.
const minVisualMinutes = 30

func (p Piece) span() Span { return Span{p.Start, max(p.End, p.Start+minVisualMinutes)} }

// group replaces each cluster of one service's overlapping pieces that would
// need more than MaxLanes columns with a Group. ps must be ordered by start.
func group(ps []Piece) ([]Piece, []Group) {
	var services []string
	byService := map[string][]Piece{}
	for _, p := range ps {
		if _, ok := byService[p.Task.Service]; !ok {
			services = append(services, p.Task.Service)
		}
		byService[p.Task.Service] = append(byService[p.Task.Service], p)
	}
	var kept []Piece
	var groups []Group
	for _, svc := range services {
		svcPieces := byService[svc]
		for len(svcPieces) > 0 {
			// The cluster runs while each next piece starts before the cluster ends.
			n, end := 1, svcPieces[0].span().End
			for n < len(svcPieces) && svcPieces[n].Start < end {
				end = max(end, svcPieces[n].span().End)
				n++
			}
			cluster := svcPieces[:n]
			svcPieces = svcPieces[n:]
			if g := newGroup(cluster); g.Peak > MaxLanes {
				groups = append(groups, g)
			} else {
				kept = append(kept, cluster...)
			}
		}
	}
	slices.SortStableFunc(kept, func(a, b Piece) int { return a.Start - b.Start })
	slices.SortStableFunc(groups, func(a, b Group) int { return a.Start - b.Start })
	return kept, groups
}

func newGroup(ps []Piece) Group {
	g := Group{Service: ps[0].Task.Service, Pieces: ps, Start: ps[0].Start}
	type edge struct{ at, delta int }
	var edges []edge
	for _, p := range ps {
		sp := p.span()
		g.End = max(g.End, min(sp.End, dayMinutes))
		g.Continued = g.Continued || (p.Continued && p.Start == 0)
		g.Continues = g.Continues || p.Continues
		edges = append(edges, edge{sp.Start, 1}, edge{sp.End, -1})
	}
	// Ends before starts at the same minute: back-to-back runs don't overlap.
	slices.SortFunc(edges, func(a, b edge) int {
		if a.at != b.at {
			return a.at - b.at
		}
		return a.delta - b.delta
	})
	running := 0
	for _, e := range edges {
		running += e.delta
		g.Peak = max(g.Peak, running)
	}
	for from := g.Start; from < g.End; from += ProfileStep {
		to, n := from+ProfileStep, 0
		for _, p := range ps {
			if sp := p.span(); sp.Start < to && sp.End > from {
				n++
			}
		}
		g.Profile = append(g.Profile, n)
	}
	return g
}

func pack(ps []Piece, gs []Group) {
	spans := make([]Span, 0, len(ps)+len(gs))
	for _, p := range ps {
		spans = append(spans, p.span())
	}
	for _, g := range gs {
		spans = append(spans, Span{g.Start, g.End})
	}
	placements := Pack(spans)
	for i := range ps {
		ps[i].Col, ps[i].Cols = placements[i].Col, placements[i].Cols
	}
	for i := range gs {
		pl := placements[len(ps)+i]
		gs[i].Col, gs[i].Cols = pl.Col, pl.Cols
	}
}

// TimeLabel is the visible time of a group: "20:00–07:40", "from 20:00" when
// it runs past midnight, "until 07:40" when it began the day before.
func (g Group) TimeLabel() string {
	switch {
	case g.Continued && g.Continues:
		return "all day"
	case g.Continued:
		return "until " + clock(g.End)
	case g.Continues:
		return "from " + clock(g.Start)
	}
	return clock(g.Start) + "–" + clock(g.End)
}

// Label is "56 Bareos jobs".
func (g Group) Label() string { return fmt.Sprintf("%d %s jobs", len(g.Pieces), g.Service) }

// PeakLabel is "up to 41 at once".
func (g Group) PeakLabel() string { return fmt.Sprintf("up to %d at once", g.Peak) }

// Tooltip names the first few runs; the tasks page lists them all.
func (g Group) Tooltip() string {
	lines := []string{g.Label(), g.TimeLabel() + ", " + g.PeakLabel()}
	const shown = 5
	for i, p := range g.Pieces {
		if i == shown {
			lines = append(lines, fmt.Sprintf("and %d more", len(g.Pieces)-shown))
			break
		}
		lines = append(lines, p.Task.Name)
	}
	return strings.Join(lines, "\n")
}

// Span is a half-open interval [Start, End) of minutes.
type Span struct{ Start, End int }

// Placement is the column of a span within its overlap cluster.
type Placement struct{ Col, Cols int }

// Pack lays out overlapping spans side by side: transitively overlapping
// spans form a cluster, each span takes the first column free at its start,
// and every member of a cluster shares the cluster's column count.
func Pack(spans []Span) []Placement {
	order := make([]int, len(spans))
	for i := range order {
		order[i] = i
	}
	slices.SortStableFunc(order, func(a, b int) int {
		if d := spans[a].Start - spans[b].Start; d != 0 {
			return d
		}
		return spans[b].End - spans[a].End
	})
	out := make([]Placement, len(spans))
	var (
		cluster  []int
		colEnds  []int // end of the last span in each column
		clusterE int
	)
	flush := func() {
		for _, i := range cluster {
			out[i].Cols = len(colEnds)
		}
		cluster, colEnds = cluster[:0], colEnds[:0]
	}
	for _, i := range order {
		s := spans[i]
		if len(cluster) > 0 && s.Start >= clusterE {
			flush()
		}
		col := slices.IndexFunc(colEnds, func(end int) bool { return end <= s.Start })
		if col < 0 {
			col = len(colEnds)
			colEnds = append(colEnds, s.End)
		} else {
			colEnds[col] = s.End
		}
		out[i].Col = col
		if len(cluster) == 0 || s.End > clusterE {
			clusterE = s.End
		}
		cluster = append(cluster, i)
	}
	flush()
	return out
}

func minuteOfDay(t time.Time) int { return t.Hour()*60 + t.Minute() }

// When describes the run's local window: "Mon 02:00–03:30".
func (p Piece) When(loc *time.Location) string {
	start := p.Run.Start.In(loc)
	return start.Format("Mon") + " " + Window(minuteOfDay(start), p.durationMinutes())
}

// TimeLabel is the visible time of a piece: "02:00–03:30", or "until 01:00"
// on the days after a run began.
func (p Piece) TimeLabel(loc *time.Location) string {
	end := clock(minuteOfDay(p.Run.End.In(loc)))
	if p.Continued {
		if p.Run.HasDuration {
			return "until " + end
		}
		return "continued"
	}
	if !p.Run.HasDuration {
		return clock(minuteOfDay(p.Run.Start.In(loc)))
	}
	return clock(minuteOfDay(p.Run.Start.In(loc))) + "–" + end
}

func (p Piece) durationMinutes() int {
	if !p.Run.HasDuration {
		return 0
	}
	return int(p.Run.End.Sub(p.Run.Start) / time.Minute)
}

// Tooltip is the multi-line hover text of a calendar event.
func (p Piece) Tooltip(loc *time.Location) string {
	start := p.Run.Start.In(loc)
	when := Window(minuteOfDay(start), p.durationMinutes())
	if !p.Task.IsRecurring {
		when = "One-off, " + FormatDate(start) + ", " + when
	}
	lines := []string{p.Task.Name, p.Task.Service, when}
	if p.Task.Hosts != "" {
		lines = append(lines, "Hosts: "+Hosts(p.Task.Hosts))
	}
	if p.Task.Description != "" {
		lines = append(lines, p.Task.Description)
	}
	return strings.Join(lines, "\n")
}

// Hosts turns "a,b, c" into "a, b, c".
func Hosts(s string) string {
	parts := strings.Split(s, ",")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	return strings.Join(parts, ", ")
}

// WeekTitle formats the week like "26 Oct – 1 Nov 2026" (en-GB).
func WeekTitle(monday time.Time) string {
	sunday := monday.AddDate(0, 0, 6)
	switch {
	case monday.Year() != sunday.Year():
		return fmt.Sprintf("%s – %s", monday.Format("2 Jan 2006"), sunday.Format("2 Jan 2006"))
	case monday.Month() != sunday.Month():
		return fmt.Sprintf("%s – %s", monday.Format("2 Jan"), sunday.Format("2 Jan 2006"))
	default:
		return fmt.Sprintf("%d – %s", monday.Day(), sunday.Format("2 Jan 2006"))
	}
}
