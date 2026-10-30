package api

import (
	"cmp"
	"encoding/csv"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/a-h/templ"

	"github.com/neverlless/taskshed/internal/database"
	"github.com/neverlless/taskshed/internal/schedule"
	"github.com/neverlless/taskshed/internal/ui"
)

const maxQueryRunes = 200

// viewerLocation reads the IANA zone app.js stores in the tz cookie. The name
// is only ever passed to time.LoadLocation; anything it rejects means UTC.
// hasCookie is false when the browser has not set the cookie yet.
func viewerLocation(r *http.Request) (loc *time.Location, hasCookie bool) {
	c, err := r.Cookie("tz")
	if err != nil {
		return time.UTC, false
	}
	// LoadLocation also accepts "Local" and "", which would mean the server's zone.
	if c.Value == "" || c.Value == "Local" || len(c.Value) > 64 {
		return time.UTC, true
	}
	loc, err = time.LoadLocation(c.Value)
	if err != nil {
		return time.UTC, true
	}
	return loc, true
}

// viewerTheme is the theme switch's cookie; any other value means follow the system.
func viewerTheme(r *http.Request) string {
	if c, err := r.Cookie("theme"); err == nil && (c.Value == "light" || c.Value == "dark") {
		return c.Value
	}
	return ""
}

// isFragment: htmx swaps only the fragment, except when restoring history,
// where it replaces the whole body.
func isFragment(r *http.Request) bool {
	return r.Header.Get("HX-Request") == "true" && r.Header.Get("HX-History-Restore-Request") != "true"
}

func (s *server) render(w http.ResponseWriter, r *http.Request, status int, c templ.Component) {
	h := w.Header()
	h.Set("Content-Security-Policy", pageCSP)
	h.Set("Cache-Control", "no-cache")
	h.Set("Vary", "HX-Request, Cookie")
	h.Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	if err := c.Render(ui.WithTheme(r.Context(), viewerTheme(r)), w); err != nil {
		s.log.Error("render failed", "path", r.URL.Path, "err", err)
	}
}

// taskQuery is the validated filter and sort of the tasks page and CSV.
type taskQuery struct{ q, service, sort, order string }

func parseTaskQuery(r *http.Request) taskQuery {
	v := r.URL.Query()
	q := taskQuery{
		q:       strings.TrimSpace(v.Get("q")),
		service: v.Get("service"),
		sort:    v.Get("sort"),
		order:   v.Get("order"),
	}
	if r := []rune(q.q); len(r) > maxQueryRunes {
		q.q = string(r[:maxQueryRunes])
	}
	if !slices.Contains([]string{"name", "service", "start"}, q.sort) {
		q.sort = "start"
	}
	if q.order != "desc" {
		q.order = "asc"
	}
	return q
}

// rows filters and sorts tasks as the viewer in loc sees them at now.
func (q taskQuery) rows(tasks []database.Task, loc *time.Location, now time.Time) []schedule.Row {
	needle := strings.ToLower(q.q)
	var rows []schedule.Row
	for _, t := range tasks {
		if q.service != "" && t.Service != q.service {
			continue
		}
		if needle != "" && !strings.Contains(strings.ToLower(t.Name), needle) &&
			!strings.Contains(strings.ToLower(t.Service), needle) &&
			!strings.Contains(strings.ToLower(t.Hosts), needle) {
			continue
		}
		rows = append(rows, schedule.NewRow(t, loc, now))
	}
	slices.SortStableFunc(rows, func(a, b schedule.Row) int {
		var c int
		switch q.sort {
		case "name":
			c = strings.Compare(strings.ToLower(a.Task.Name), strings.ToLower(b.Task.Name))
		case "service":
			c = strings.Compare(strings.ToLower(a.Task.Service), strings.ToLower(b.Task.Service))
		default:
			// Unparseable times sort last.
			c = cmp.Compare(sortStart(a), sortStart(b))
		}
		if q.order == "desc" {
			return -c
		}
		return c
	})
	return rows
}

func sortStart(r schedule.Row) int {
	if !r.Valid {
		return 1 << 30
	}
	return r.Start
}

func (s *server) tasksPage(w http.ResponseWriter, r *http.Request) {
	loc, hasTZ := viewerLocation(r)
	now := s.now()
	q := parseTaskQuery(r)
	d := ui.TasksData{Query: q.q, Service: q.service, Sort: q.sort, Order: q.order,
		NowPercent: schedule.NowPercent(now, loc), AutoLoad: !hasTZ}
	status := http.StatusOK
	tasks, err := s.store.List(r.Context())
	if err != nil {
		s.log.Error("list tasks for page", "path", r.URL.Path, "err", err)
		d.Failed, status = true, http.StatusInternalServerError
	} else {
		d.Services = schedule.Services(tasks)
		d.Total = len(tasks)
		d.Rows = q.rows(tasks, loc, now)
	}
	if isFragment(r) {
		s.render(w, r, status, ui.TaskResults(d))
		return
	}
	s.render(w, r, status, ui.TasksPage(d))
}

// csvCell keeps spreadsheet apps from evaluating task text as a formula.
func csvCell(s string) string {
	if s != "" && strings.ContainsRune("=+-@", rune(s[0])) {
		return "'" + s
	}
	return s
}

func (s *server) tasksCSV(w http.ResponseWriter, r *http.Request) {
	loc, _ := viewerLocation(r)
	tasks, err := s.store.List(r.Context())
	if err != nil {
		s.log.Error("list tasks for CSV", "err", err)
		http.Error(w, "Tasks could not be loaded.", http.StatusInternalServerError)
		return
	}
	h := w.Header()
	h.Set("Content-Type", "text/csv; charset=utf-8")
	h.Set("Content-Disposition", `attachment; filename="tasks.csv"`)
	h.Set("Cache-Control", "no-store")
	h.Set("Vary", "Cookie")
	// The BOM makes Excel read UTF-8 (Cyrillic task names) correctly.
	_, _ = w.Write([]byte("\xEF\xBB\xBF"))
	cw := csv.NewWriter(w)
	cw.Comma = ';'
	cw.UseCRLF = true
	_ = cw.Write([]string{"Name", "Service", "Start (local)", "Duration", "End (local)", "Days (local)", "Date (local)", "Recurring", "Description", "Hosts"})
	for _, row := range parseTaskQuery(r).rows(tasks, loc, s.now()) {
		t := row.Task
		date, recurring := "", "no"
		if !row.Date.IsZero() {
			date = row.Date.Format(time.DateOnly)
		}
		if t.IsRecurring {
			recurring = "yes"
		}
		start := t.Time
		if row.Valid {
			start = row.StartClock()
		}
		record := []string{t.Name, t.Service, start, t.Duration, row.EndClock(), strings.Join(row.DayNames(), ","),
			date, recurring, t.Description, t.Hosts}
		for i := range record {
			record[i] = csvCell(record[i])
		}
		_ = cw.Write(record)
	}
	cw.Flush()
	if err := cw.Error(); err != nil {
		s.log.Error("write CSV", "err", err)
	}
}

func (s *server) calendarPage(w http.ResponseWriter, r *http.Request) {
	loc, hasTZ := viewerLocation(r)
	now := s.now()
	monday := schedule.WeekStart(now, loc)
	if week := r.URL.Query().Get("week"); week != "" {
		// Any date selects its week; an invalid one falls back to this week.
		if day, err := time.ParseInLocation(time.DateOnly, week, loc); err == nil && day.Year() > 1970 && day.Year() < 3000 {
			monday = schedule.WeekStart(day, loc)
		}
	}
	d := ui.CalendarData{Loc: loc, Now: now, Monday: monday, AutoLoad: !hasTZ}
	status := http.StatusOK
	tasks, err := s.store.List(r.Context())
	if err != nil {
		s.log.Error("list tasks for page", "path", r.URL.Path, "err", err)
		d.Failed, status = true, http.StatusInternalServerError
	} else {
		d.Empty = len(tasks) == 0
		d.Services = schedule.Services(tasks)
		d.Colors = schedule.Colors(d.Services)
		d.Days = schedule.Week(tasks, monday)
		for _, t := range tasks {
			if !t.IsRecurring && t.Date == "" {
				d.Undated++
			}
		}
	}
	if isFragment(r) {
		s.render(w, r, status, ui.CalendarView(d))
		return
	}
	s.render(w, r, status, ui.CalendarPage(d))
}
