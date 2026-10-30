// Package ui renders TaskShed's HTML pages and htmx fragments with templ.
package ui

//go:generate go tool templ generate

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/neverlless/taskshed/internal/schedule"
)

type themeKey struct{}

// WithTheme stores the viewer's theme choice for Layout: "light", "dark", or "" to follow the system.
func WithTheme(ctx context.Context, theme string) context.Context {
	return context.WithValue(ctx, themeKey{}, theme)
}

func themeFrom(ctx context.Context) string {
	t, _ := ctx.Value(themeKey{}).(string)
	return t
}

// TasksData is everything the tasks page and its results fragment show.
type TasksData struct {
	Query, Service string
	Sort, Order    string // validated: name|service|start, asc|desc
	Services       []string
	Rows           []schedule.Row // filtered and sorted
	Total          int            // tasks before filtering
	NowPercent     float64
	AutoLoad       bool // no tz cookie yet: re-render once app.js has set it
	Failed         bool
}

func (d TasksData) values(sort, order string) url.Values {
	v := url.Values{}
	if d.Query != "" {
		v.Set("q", d.Query)
	}
	if d.Service != "" {
		v.Set("service", d.Service)
	}
	v.Set("sort", sort)
	v.Set("order", order)
	return v
}

func (d TasksData) URL() string { return "/tasks-view?" + d.values(d.Sort, d.Order).Encode() }

func (d TasksData) CSVURL() string { return "/tasks.csv?" + d.values(d.Sort, d.Order).Encode() }

// SortURL sorts by col, toggling the order if col is already the sort column.
func (d TasksData) SortURL(col string) string {
	order := "asc"
	if d.Sort == col && d.Order == "asc" {
		order = "desc"
	}
	return "/tasks-view?" + d.values(col, order).Encode()
}

func (d TasksData) ariaSort(col string) string {
	switch {
	case d.Sort != col:
		return "none"
	case d.Order == "asc":
		return "ascending"
	default:
		return "descending"
	}
}

func (d TasksData) sortIcon(col string) string {
	switch d.ariaSort(col) {
	case "ascending":
		return "▲"
	case "descending":
		return "▼"
	}
	return ""
}

func (d TasksData) countText() string {
	if len(d.Rows) == d.Total {
		return fmt.Sprintf("%d tasks", d.Total)
	}
	return fmt.Sprintf("%d of %d tasks", len(d.Rows), d.Total)
}

func trigger(autoLoad bool) string {
	if autoLoad {
		return "load, every 60s"
	}
	return "every 60s"
}

func pct(v float64) string { return strconv.FormatFloat(v, 'f', 3, 64) + "%" }

func segStyle(s schedule.Segment) string {
	if s.Point {
		return "left: " + pct(s.Left)
	}
	return "left: " + pct(s.Left) + "; width: " + pct(s.Width)
}

func dayLabel(r schedule.Row) string {
	if len(r.Days) == 0 {
		return "No valid days"
	}
	names := make([]string, len(r.Days))
	for i, d := range r.Days {
		names[i] = d.String()
	}
	return strings.Join(names, ", ")
}

// CalendarData is everything the calendar page and its week fragment show.
type CalendarData struct {
	Loc      *time.Location
	Now      time.Time
	Monday   time.Time // local midnight starting the shown week
	Days     []schedule.Day
	Services []string
	Colors   map[string]string
	Undated  int // one-off tasks without a date
	Empty    bool
	Failed   bool
	AutoLoad bool
}

func weekURL(monday time.Time, current bool) string {
	if current {
		return "/"
	}
	return "/?week=" + monday.Format(time.DateOnly)
}

func (d CalendarData) isCurrent() bool {
	return d.Monday.Equal(schedule.WeekStart(d.Now, d.Loc))
}

func (d CalendarData) URL() string { return weekURL(d.Monday, d.isCurrent()) }

func (d CalendarData) shiftURL(days int) string {
	m := d.Monday.AddDate(0, 0, days)
	return weekURL(m, m.Equal(schedule.WeekStart(d.Now, d.Loc)))
}

func (d CalendarData) isToday(day schedule.Day) bool {
	y1, m1, d1 := day.Date.Date()
	y2, m2, d2 := d.Now.In(d.Loc).Date()
	return y1 == y2 && m1 == m2 && d1 == d2
}

func (d CalendarData) isPast(day schedule.Day) bool {
	return d.isCurrent() && day.Date.Before(d.Now) && !d.isToday(day)
}

func (d CalendarData) nowStyle() string {
	l := d.Now.In(d.Loc)
	return "top: " + pct(float64(l.Hour()*60+l.Minute())/1440*100)
}

// shortEvent: under ~55 minutes there is room for one line only.
func shortEvent(p schedule.Piece) bool { return p.End-p.Start < 55 }

func (d CalendarData) eventStyle(p schedule.Piece) string {
	return fmt.Sprintf("top: %s; height: %s; left: calc(%s + 2px); width: calc(%s - 4px); background: %s; border-color: %s",
		pct(float64(p.Start)/1440*100), pct(float64(p.End-p.Start)/1440*100),
		pct(float64(p.Col)/float64(p.Cols)*100), pct(100/float64(p.Cols)),
		d.Colors[p.Task.Service], d.Colors[p.Task.Service])
}

func (d CalendarData) ariaLabel(p schedule.Piece) string {
	s := p.Task.Name + ", " + p.Task.Service + ", " + p.When(d.Loc)
	if p.Continued {
		s += ", continued"
	}
	return s
}

func taskLink(name string) string { return "/tasks-view?" + url.Values{"q": {name}}.Encode() }

func serviceLink(service string) string {
	return "/tasks-view?" + url.Values{"service": {service}}.Encode()
}

// groupStyle places a group like an event and shades each ProfileStep slot by
// how many of its runs are going then, from faint to the full service colour.
func (d CalendarData) groupStyle(g schedule.Group) string {
	var b strings.Builder
	span := float64(g.End - g.Start)
	for i, n := range g.Profile {
		from := float64(i*schedule.ProfileStep) / span * 100
		to := min(float64((i+1)*schedule.ProfileStep)/span*100, 100)
		if i > 0 {
			b.WriteString(", ")
		}
		fmt.Fprintf(&b, "color-mix(in srgb, var(--c) %d%%, transparent) %s %s", 15+75*n/g.Peak, pct(from), pct(to))
	}
	return fmt.Sprintf("--c: %s; top: %s; height: %s; left: calc(%s + 2px); width: calc(%s - 4px); background: linear-gradient(to bottom, %s)",
		d.Colors[g.Service], pct(float64(g.Start)/1440*100), pct(span/1440*100),
		pct(float64(g.Col)/float64(g.Cols)*100), pct(100/float64(g.Cols)), b.String())
}

func groupAriaLabel(day schedule.Day, g schedule.Group) string {
	return g.Label() + ", " + day.Date.Format("Mon") + " " + g.TimeLabel() + ", " + g.PeakLabel()
}

// agendaItem is one row of the phone agenda: a single run or a group.
type agendaItem struct {
	Piece *schedule.Piece
	Group *schedule.Group
}

func agenda(day schedule.Day) []agendaItem {
	items := make([]agendaItem, 0, len(day.Pieces)+len(day.Groups))
	pi, gi := 0, 0
	for pi < len(day.Pieces) || gi < len(day.Groups) {
		if gi == len(day.Groups) || (pi < len(day.Pieces) && day.Pieces[pi].Start <= day.Groups[gi].Start) {
			items = append(items, agendaItem{Piece: &day.Pieces[pi]})
			pi++
		} else {
			items = append(items, agendaItem{Group: &day.Groups[gi]})
			gi++
		}
	}
	return items
}

func hourLabel(h int) string { return fmt.Sprintf("%02d:00", h) }

func undatedNote(n int) string {
	if n == 1 {
		return "1 one-off task has no date and is only listed on the "
	}
	return fmt.Sprintf("%d one-off tasks have no date and are only listed on the ", n)
}
