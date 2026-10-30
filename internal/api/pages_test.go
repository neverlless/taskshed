package api

import (
	"context"
	"encoding/csv"
	"log/slog"
	"net/http"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/neverlless/taskshed/internal/database"
)

// Wednesday 4 Nov 2026, 12:00 UTC (Kyiv UTC+2, New York UTC-5).
var testNow = time.Date(2026, 11, 4, 12, 0, 0, 0, time.UTC)

func newPageServer(t *testing.T, tasks ...database.Task) (http.Handler, *database.Store) {
	t.Helper()
	store, err := database.OpenSQLite(filepath.Join(t.TempDir(), "pages.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.Close() })
	for i := range tasks {
		if err := store.Create(context.Background(), &tasks[i]); err != nil {
			t.Fatal(err)
		}
	}
	h := NewServer(store, Options{Logger: slog.New(slog.DiscardHandler), Now: func() time.Time { return testNow }})
	return h, store
}

var demoTasks = []database.Task{
	{Name: "Postgres backup", Service: "Database", Time: "22:30", Duration: "01:30", DaysOfWeek: "Mon,Wed", IsRecurring: true, Hosts: "db-1,db-2"},
	{Name: "Log rotation", Service: "Platform", Time: "05:00", DaysOfWeek: "Mon,Tue,Wed,Thu,Fri,Sat,Sun", IsRecurring: true},
	{Name: "=cmd|' /C calc'!A0", Service: "Platform", Time: "09:00", Duration: "02:00", Date: "2026-11-06", Description: `say "hi"; bye`},
	{Name: "Ad hoc", Service: "AWX", Time: "10:00"},
}

func TestTasksPageFullAndFragment(t *testing.T) {
	h, _ := newPageServer(t, demoTasks...)

	full := do(t, h, "GET", "/tasks-view", "", "Cookie", "tz=UTC")
	body := full.Body.String()
	if full.Code != 200 || !strings.HasPrefix(body, "<!doctype html>") || !strings.Contains(body, `id="filters"`) ||
		!strings.Contains(body, `<option value="Database">Database</option>`) || !strings.Contains(body, `id="task-results"`) {
		t.Fatalf("full page: %d %s", full.Code, body)
	}
	for k, v := range map[string]string{"Vary": "HX-Request, Cookie", "Content-Security-Policy": pageCSP} {
		if got := full.Header().Get(k); got != v {
			t.Errorf("%s = %q", k, got)
		}
	}

	frag := do(t, h, "GET", "/tasks-view?q=backup", "", "HX-Request", "true", "Cookie", "tz=UTC")
	body = frag.Body.String()
	if !strings.HasPrefix(body, `<div id="task-results"`) || strings.Contains(body, "<html") ||
		!strings.Contains(body, "1 of 4 tasks") || !strings.Contains(body, `href="/tasks.csv?order=asc&amp;q=backup&amp;sort=start"`) {
		t.Fatalf("fragment: %s", body)
	}

	// History restores need the whole page even though htmx sends HX-Request.
	restore := do(t, h, "GET", "/tasks-view", "", "HX-Request", "true", "HX-History-Restore-Request", "true")
	if !strings.Contains(restore.Body.String(), "<html") {
		t.Error("history restore got a fragment")
	}
}

var taskNameRe = regexp.MustCompile(`<div class="task-name">(?:<span class="running-mark".*?</span></span>)?([^<]*)</div>`)

func TestTasksPageFiltersAndSort(t *testing.T) {
	h, _ := newPageServer(t, demoTasks...)
	names := func(path string) string {
		body := do(t, h, "GET", path, "", "HX-Request", "true", "Cookie", "tz=UTC").Body.String()
		var out []string
		for _, m := range taskNameRe.FindAllStringSubmatch(body, -1) {
			out = append(out, m[1])
		}
		return strings.Join(out, ", ")
	}
	const cmd = "=cmd|&#39; /C calc&#39;!A0"
	tests := []struct{ path, want string }{
		{"/tasks-view", "Log rotation, " + cmd + ", Ad hoc, Postgres backup"},
		{"/tasks-view?sort=name&order=desc", "Postgres backup, Log rotation, Ad hoc, " + cmd},
		{"/tasks-view?sort=service", "Ad hoc, Postgres backup, Log rotation, " + cmd},
		{"/tasks-view?service=Platform", "Log rotation, " + cmd},
		{"/tasks-view?q=DB-2", "Postgres backup"},
		{"/tasks-view?sort=evil&order=sideways", "Log rotation, " + cmd + ", Ad hoc, Postgres backup"},
	}
	for _, tt := range tests {
		if got := names(tt.path); got != tt.want {
			t.Errorf("%s: %q; want %q", tt.path, got, tt.want)
		}
	}

	body := do(t, h, "GET", "/tasks-view?q=nothing", "", "HX-Request", "true").Body.String()
	if !strings.Contains(body, "No tasks match these filters.") || !strings.Contains(body, `href="/tasks-view">Clear filters`) {
		t.Errorf("no match: %s", body)
	}
	body = do(t, h, "GET", "/tasks-view?sort=name", "", "HX-Request", "true").Body.String()
	if !strings.Contains(body, `<th scope="col" aria-sort="ascending"><a class="sort" href="/tasks-view?order=desc&amp;sort=name"`) {
		t.Errorf("sort header: %s", body)
	}
}

func TestTasksPageTimeZone(t *testing.T) {
	h, _ := newPageServer(t, demoTasks[0])
	tests := []struct {
		cookie, window, days, trigger string
	}{
		{"", "22:30–00:00", "Monday, Wednesday", `hx-trigger="load, every 60s"`},
		{"tz=Europe/Kyiv", "00:30–02:00", "Tuesday, Thursday", `hx-trigger="every 60s"`},
		{"tz=America/New_York", "17:30–19:00", "Monday, Wednesday", `hx-trigger="every 60s"`},
		{"tz=Mars/Olympus", "22:30–00:00", "Monday, Wednesday", `hx-trigger="every 60s"`},
		{"tz=Local", "22:30–00:00", "Monday, Wednesday", `hx-trigger="every 60s"`},
	}
	for _, tt := range tests {
		var hdr []string
		if tt.cookie != "" {
			hdr = []string{"Cookie", tt.cookie}
		}
		body := do(t, h, "GET", "/tasks-view", "", hdr...).Body.String()
		if !strings.Contains(body, tt.window) || !strings.Contains(body, `aria-label="`+tt.days+`"`) || !strings.Contains(body, tt.trigger) {
			t.Errorf("cookie %q: want %q %q %q in %s", tt.cookie, tt.window, tt.days, tt.trigger, body)
		}
	}
}

func TestTasksPageStoreError(t *testing.T) {
	h, store := newPageServer(t)
	store.Close()
	rec := do(t, h, "GET", "/tasks-view", "")
	if rec.Code != 500 || !strings.Contains(rec.Body.String(), "Tasks could not be loaded.") {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
	if rec := do(t, h, "GET", "/", ""); rec.Code != 500 || !strings.Contains(rec.Body.String(), "The schedule could not be loaded.") {
		t.Fatalf("calendar: %d %s", rec.Code, rec.Body)
	}
}

func TestTasksCSV(t *testing.T) {
	h, _ := newPageServer(t, demoTasks...)
	rec := do(t, h, "GET", "/tasks.csv?sort=name", "", "Cookie", "tz=Europe/Kyiv")
	if rec.Code != 200 || rec.Header().Get("Content-Disposition") != `attachment; filename="tasks.csv"` ||
		rec.Header().Get("Content-Type") != "text/csv; charset=utf-8" {
		t.Fatalf("%d %v", rec.Code, rec.Header())
	}
	raw := rec.Body.String()
	body, ok := strings.CutPrefix(raw, "\xEF\xBB\xBF")
	if !ok {
		t.Fatal("missing UTF-8 BOM")
	}
	r := csv.NewReader(strings.NewReader(body))
	r.Comma = ';'
	rows, err := r.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	want := [][]string{
		{"Name", "Service", "Start (local)", "Duration", "End (local)", "Days (local)", "Date (local)", "Recurring", "Description", "Hosts"},
		{"'=cmd|' /C calc'!A0", "Platform", "11:00", "02:00", "13:00", "", "2026-11-06", "no", `say "hi"; bye`, ""},
		{"Ad hoc", "AWX", "12:00", "", "", "", "", "no", "", ""},
		{"Log rotation", "Platform", "07:00", "", "", "Mon,Tue,Wed,Thu,Fri,Sat,Sun", "", "yes", "", ""},
		{"Postgres backup", "Database", "00:30", "01:30", "02:00", "Tue,Thu", "", "yes", "", "db-1,db-2"},
	}
	if len(rows) != len(want) {
		t.Fatalf("rows %q", rows)
	}
	for i := range want {
		if strings.Join(rows[i], "|") != strings.Join(want[i], "|") {
			t.Errorf("row %d: %q; want %q", i, rows[i], want[i])
		}
	}
	if !strings.Contains(raw, `"say ""hi""; bye"`) || !strings.Contains(raw, "\r\n") {
		t.Errorf("quoting: %s", raw)
	}
	if rows := strings.Count(do(t, h, "GET", "/tasks.csv?service=AWX", "").Body.String(), "\n"); rows != 2 {
		t.Errorf("filtered CSV has %d lines", rows)
	}
}

func TestCalendarPage(t *testing.T) {
	h, _ := newPageServer(t, demoTasks...)
	rec := do(t, h, "GET", "/", "", "Cookie", "tz=Europe/Kyiv")
	body := rec.Body.String()
	for _, want := range []string{
		"<!doctype html>",
		`<li><span class="swatch" style="background: #8DB8F2;"></span>AWX</li>`,
		"1 one-off task has no date and is only listed on the ",
		`<h2 id="week-title" class="cal-title">2 – 8 Nov 2026</h2>`,
		`<span class="button" aria-disabled="true">This week</span>`,
		`href="/?week=2026-10-26"`, `href="/?week=2026-11-09"`,
		`<span class="cal-dayhead today">Wed 4</span>`,
		// 22:30 UTC Mon = Tue 00:30 Kyiv, ends 02:00.
		`aria-label="Postgres backup, Database, Tue 00:30–02:00"`,
		"title=\"Postgres backup\nDatabase\n00:30–02:00\nHosts: db-1, db-2\"",
		`href="/tasks-view?q=Postgres+backup"`,
		`class="now-line" style="top: 58.333%;"`, // 14:00 Kyiv
		`hx-trigger="every 60s"`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("missing %q", want)
		}
	}
	if rec.Header().Get("Content-Security-Policy") != pageCSP || rec.Header().Get("Vary") != "HX-Request, Cookie" {
		t.Errorf("headers %v", rec.Header())
	}
	// Two runs of the Mon,Wed task this week, the dated one-off (Fri), 7 log rotations.
	if n := strings.Count(body, `class="cal-event`); n != 2+1+7 {
		t.Errorf("%d events", n)
	}

	frag := do(t, h, "GET", "/?week=2026-11-12", "", "HX-Request", "true").Body.String()
	if !strings.HasPrefix(frag, `<section id="calendar-view"`) || strings.Contains(frag, "<html") ||
		!strings.Contains(frag, "9 – 15 Nov 2026") || !strings.Contains(frag, `hx-get="/?week=2026-11-09"`) ||
		strings.Contains(frag, "now-line") || !strings.Contains(frag, `hx-trigger="load, every 60s"`) {
		t.Errorf("fragment: %s", frag)
	}
	for _, bad := range []string{"/?week=nope", "/?week=0001-01-01", "/?week=2026-13-01"} {
		if b := do(t, h, "GET", bad, "").Body.String(); !strings.Contains(b, "2 – 8 Nov 2026") {
			t.Errorf("%s did not fall back to this week", bad)
		}
	}
}

func TestCalendarEmpty(t *testing.T) {
	h, _ := newPageServer(t)
	body := do(t, h, "GET", "/", "").Body.String()
	if !strings.Contains(body, "No tasks scheduled yet.") || strings.Contains(body, "calendar-view") {
		t.Errorf("%s", body)
	}
}

func TestStaticAssets(t *testing.T) {
	h, _ := newPageServer(t)
	for path, ctype := range map[string]string{
		"/static/vendor/htmx/htmx.min.js": "text/javascript",
		"/static/js/app.js":               "text/javascript",
	} {
		rec := do(t, h, "GET", path, "")
		if rec.Code != 200 || !strings.HasPrefix(rec.Header().Get("Content-Type"), ctype) || rec.Body.Len() == 0 {
			t.Errorf("%s: %d %s", path, rec.Code, rec.Header().Get("Content-Type"))
		}
	}
	if !strings.Contains(pageCSP, "font-src 'self';") || !strings.Contains(pageCSP, "script-src 'self';") {
		t.Errorf("CSP %q", pageCSP)
	}
}
