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
		pack(days[i].Pieces)
	}
	return days
}

// minVisualMinutes is how much of the day a short event occupies on screen,
// so packing also separates events whose boxes would touch.
const minVisualMinutes = 30

func pack(ps []Piece) {
	spans := make([]Span, len(ps))
	for i, p := range ps {
		spans[i] = Span{p.Start, max(p.End, p.Start+minVisualMinutes)}
	}
	for i, pl := range Pack(spans) {
		ps[i].Col, ps[i].Cols = pl.Col, pl.Cols
	}
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
