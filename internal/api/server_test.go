package api

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/neverlless/taskshed/internal/database"
	"github.com/neverlless/taskshed/web"
)

func newTestServer(t *testing.T, token string) http.Handler {
	t.Helper()
	store, err := database.OpenSQLite(filepath.Join(t.TempDir(), "api.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.Close() })
	return NewServer(store, Options{APIToken: token, Logger: slog.New(slog.DiscardHandler)})
}

func do(t *testing.T, h http.Handler, method, path, body string, header ...string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	for i := 0; i+1 < len(header); i += 2 {
		req.Header.Set(header[i], header[i+1])
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

const validTask = `{"name":"backup","service":"db","time":"2:00","days_of_week":"mon,fri","is_recurring":true,"extra":"ignored"}`

func TestListEmpty(t *testing.T) {
	rec := do(t, newTestServer(t, ""), "GET", "/api/tasks", "")
	if rec.Code != 200 || strings.TrimSpace(rec.Body.String()) != "[]" {
		t.Fatalf("%d %q", rec.Code, rec.Body)
	}
}

func TestTaskLifecycle(t *testing.T) {
	h := newTestServer(t, "")

	rec := do(t, h, "POST", "/api/tasks", validTask)
	if rec.Code != 200 {
		t.Fatalf("create: %d %s", rec.Code, rec.Body)
	}
	var created database.Task
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil || created.ID == 0 {
		t.Fatalf("create body %s: %v", rec.Body, err)
	}
	if created.Time != "02:00" || created.DaysOfWeek != "Mon,Fri" {
		t.Errorf("not normalized: %+v", created)
	}

	if rec := do(t, h, "GET", "/api/tasks/1", ""); rec.Code != 200 || !strings.Contains(rec.Body.String(), `"name":"backup"`) {
		t.Errorf("get: %d %s", rec.Code, rec.Body)
	}
	if rec := do(t, h, "PUT", "/api/tasks/1", strings.Replace(validTask, "backup", "restore", 1)); rec.Code != 200 || !strings.Contains(rec.Body.String(), `"id":1`) {
		t.Errorf("update: %d %s", rec.Code, rec.Body)
	}
	if rec := do(t, h, "DELETE", "/api/tasks/1", ""); rec.Code != 204 {
		t.Errorf("delete: %d %s", rec.Code, rec.Body)
	}
	if rec := do(t, h, "GET", "/api/tasks/1", ""); rec.Code != 404 {
		t.Errorf("get deleted: %d", rec.Code)
	}
}

func TestErrors(t *testing.T) {
	h := newTestServer(t, "")
	tests := []struct {
		method, path, body string
		status             int
		msg                string
	}{
		{"POST", "/api/tasks", `{"name":"","service":"db","time":"02:00","days_of_week":"Mon","is_recurring":true}`, 400, "name is required"},
		{"POST", "/api/tasks", `{not json`, 400, "invalid JSON body"},
		{"POST", "/api/tasks", `{"name":"` + strings.Repeat("a", 2<<20) + `"}`, 413, "request body too large"},
		{"PUT", "/api/tasks/99", validTask, 404, "task not found"},
		{"DELETE", "/api/tasks/99", "", 404, "task not found"},
		{"GET", "/api/tasks/abc", "", 400, "invalid task id"},
		{"DELETE", "/api/tasks/0", "", 400, "invalid task id"},
	}
	for _, tt := range tests {
		rec := do(t, h, tt.method, tt.path, tt.body)
		var body struct{ Error string }
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatalf("%s %s: body %q is not JSON: %v", tt.method, tt.path, rec.Body, err)
		}
		if rec.Code != tt.status || body.Error != tt.msg || rec.Header().Get("Content-Type") != "application/json" {
			t.Errorf("%s %s = %d %q; want %d %q", tt.method, tt.path, rec.Code, body.Error, tt.status, tt.msg)
		}
	}
}

func TestAuth(t *testing.T) {
	h := newTestServer(t, "s3cret")

	rec := do(t, h, "POST", "/api/tasks", validTask)
	if rec.Code != 401 || rec.Header().Get("WWW-Authenticate") != "Bearer" || !strings.Contains(rec.Body.String(), `"error"`) {
		t.Fatalf("no token: %d %v %s", rec.Code, rec.Header(), rec.Body)
	}
	if rec := do(t, h, "DELETE", "/api/tasks/1", "", "Authorization", "Bearer wrong"); rec.Code != 401 {
		t.Fatalf("wrong token: %d", rec.Code)
	}
	if rec := do(t, h, "POST", "/api/tasks", validTask, "Authorization", "Bearer s3cret"); rec.Code != 200 {
		t.Fatalf("with token: %d %s", rec.Code, rec.Body)
	}
	if rec := do(t, h, "GET", "/api/tasks", ""); rec.Code != 200 {
		t.Fatalf("reads stay open: %d", rec.Code)
	}
}

func TestHealthz(t *testing.T) {
	rec := do(t, newTestServer(t, ""), "GET", "/healthz", "")
	if rec.Code != 200 || rec.Body.String() != "ok" {
		t.Fatalf("%d %q", rec.Code, rec.Body)
	}
}

func TestPagesAndStatic(t *testing.T) {
	h := newTestServer(t, "")

	for _, path := range []string{"/", "/tasks-view"} {
		rec := do(t, h, "GET", path, "")
		if rec.Code != 200 || !strings.Contains(rec.Header().Get("Content-Type"), "text/html") {
			t.Fatalf("%s: %d %s", path, rec.Code, rec.Header().Get("Content-Type"))
		}
		for k, v := range map[string]string{
			"Content-Security-Policy": pageCSP,
			"X-Content-Type-Options":  "nosniff",
			"Referrer-Policy":         "no-referrer",
			"X-Frame-Options":         "DENY",
		} {
			if got := rec.Header().Get(k); got != v {
				t.Errorf("%s %s = %q; want %q", path, k, got, v)
			}
		}
	}

	rec := do(t, h, "GET", "/static/css/styles.css", "")
	body, _ := io.ReadAll(rec.Body)
	if rec.Code != 200 || len(body) == 0 || !strings.HasPrefix(rec.Header().Get("Content-Type"), "text/css") {
		t.Fatalf("static: %d %s", rec.Code, rec.Header().Get("Content-Type"))
	}
	if rec.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Error("static missing security headers")
	}

	for _, path := range []string{"/nope", "/static/", "/static/js/", "/static/missing.js"} {
		if rec := do(t, h, "GET", path, ""); rec.Code != 404 {
			t.Errorf("%s = %d; want 404", path, rec.Code)
		}
	}
}

func TestPanicRecovery(t *testing.T) {
	s := &server{log: slog.New(slog.DiscardHandler)}
	h := s.recoverPanics(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { panic("boom") }))
	rec := do(t, h, "GET", "/", "")
	if rec.Code != 500 || !strings.Contains(rec.Body.String(), "internal server error") {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
}

func TestCompression(t *testing.T) {
	h := newTestServer(t, "")
	want, err := web.FS.ReadFile("static/css/styles.css")
	if err != nil {
		t.Fatal(err)
	}

	rec := do(t, h, "GET", "/static/css/styles.css", "", "Accept-Encoding", "gzip")
	if rec.Header().Get("Content-Encoding") != "gzip" || rec.Body.Len() >= len(want) {
		t.Fatalf("not compressed: encoding %q, %d bytes", rec.Header().Get("Content-Encoding"), rec.Body.Len())
	}
	zr, err := gzip.NewReader(rec.Body)
	if err != nil {
		t.Fatal(err)
	}
	got, err := io.ReadAll(zr)
	if err != nil || !bytes.Equal(got, want) {
		t.Fatalf("decompressed body differs (err %v)", err)
	}

	// Clients that don't ask for gzip get the plain file, and tiny responses stay uncompressed.
	if rec := do(t, h, "GET", "/static/css/styles.css", ""); rec.Header().Get("Content-Encoding") != "" || !bytes.Equal(rec.Body.Bytes(), want) {
		t.Errorf("plain request got encoding %q", rec.Header().Get("Content-Encoding"))
	}
	if rec := do(t, h, "GET", "/healthz", "", "Accept-Encoding", "gzip"); rec.Header().Get("Content-Encoding") != "" || rec.Body.String() != "ok" {
		t.Errorf("healthz: encoding %q body %q", rec.Header().Get("Content-Encoding"), rec.Body.String())
	}
}

func TestAssets(t *testing.T) {
	h := newTestServer(t, "")
	tests := []struct {
		path, cache string
		code        int
	}{
		{web.Asset("css/styles.css"), "public, max-age=31536000, immutable", 200},
		{web.Asset("fonts/ibm-plex-sans-latin-400-normal.woff2"), "public, max-age=31536000, immutable", 200},
		{"/assets/0ld0ld0ld0ld/css/styles.css", "no-cache", 200}, // page from a previous build
		{"/static/css/styles.css", "no-cache", 200},
		{web.Asset("css/"), "", 404},
	}
	for _, tt := range tests {
		rec := do(t, h, "GET", tt.path, "")
		if rec.Code != tt.code || rec.Header().Get("Cache-Control") != tt.cache {
			t.Errorf("%s: %d %q; want %d %q", tt.path, rec.Code, rec.Header().Get("Cache-Control"), tt.code, tt.cache)
		}
	}
	page := do(t, h, "GET", "/", "").Body.String()
	for _, want := range []string{`href="` + web.Asset("css/styles.css") + `"`, `rel="preload" href="` + web.Asset("fonts/ibm-plex-sans-latin-400-normal.woff2") + `"`} {
		if !strings.Contains(page, want) {
			t.Errorf("page lacks %s", want)
		}
	}
}
