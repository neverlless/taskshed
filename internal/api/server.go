// Package api serves the task JSON API, health check and embedded web pages.
package api

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"io/fs"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/neverlless/taskshed/internal/database"
	"github.com/neverlless/taskshed/web"
)

const (
	maxBodyBytes = 1 << 20
	// FullCalendar injects its icon font as a data: URL.
	pageCSP = "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; font-src 'self' data:; connect-src 'self'; frame-ancestors 'none'; base-uri 'none'; form-action 'none'"
)

type Options struct {
	// APIToken, if set, is required as a Bearer token on POST/PUT/DELETE.
	APIToken string
	Logger   *slog.Logger
}

type server struct {
	store *database.Store
	log   *slog.Logger
	token string
}

func NewServer(store *database.Store, opts Options) http.Handler {
	s := &server{store: store, log: opts.Logger, token: opts.APIToken}
	if s.log == nil {
		s.log = slog.Default()
	}
	static, err := fs.Sub(web.FS, "static")
	if err != nil {
		panic(err) // embed layout is fixed at compile time
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/tasks", s.listTasks)
	mux.HandleFunc("GET /api/tasks/{id}", s.getTask)
	mux.HandleFunc("POST /api/tasks", s.requireToken(s.createTask))
	mux.HandleFunc("PUT /api/tasks/{id}", s.requireToken(s.updateTask))
	mux.HandleFunc("DELETE /api/tasks/{id}", s.requireToken(s.deleteTask))
	mux.HandleFunc("GET /healthz", s.healthz)
	mux.HandleFunc("GET /{$}", page("templates/calendar.html"))
	mux.HandleFunc("GET /tasks-view", page("templates/tasks-view.html"))
	mux.Handle("GET /static/", http.StripPrefix("/static/", staticFiles(http.FileServerFS(static))))

	return s.logRequests(s.recoverPanics(securityHeaders(mux)))
}

func (s *server) listTasks(w http.ResponseWriter, r *http.Request) {
	tasks, err := s.store.List(r.Context())
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, tasks)
}

func (s *server) getTask(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	task, err := s.store.Get(r.Context(), id)
	if err != nil {
		s.storeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, task)
}

// createTask answers 200, not 201: Ansible's uri module expects 200 by default.
func (s *server) createTask(w http.ResponseWriter, r *http.Request) {
	task, ok := decodeTask(w, r)
	if !ok {
		return
	}
	if err := s.store.Create(r.Context(), &task); err != nil {
		s.internalError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, task)
}

func (s *server) updateTask(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	task, ok := decodeTask(w, r)
	if !ok {
		return
	}
	if err := s.store.Update(r.Context(), id, &task); err != nil {
		s.storeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, task)
}

func (s *server) deleteTask(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	if err := s.store.Delete(r.Context(), id); err != nil {
		s.storeError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *server) healthz(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	if err := s.store.Ping(ctx); err != nil {
		s.log.Error("health check failed", "err", err)
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte("unavailable"))
		return
	}
	w.Write([]byte("ok"))
}

func page(name string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Security-Policy", pageCSP)
		w.Header().Set("Cache-Control", "no-cache")
		http.ServeFileFS(w, r, web.FS, name)
	}
}

// staticFiles hides directory listings and asks browsers to revalidate,
// since asset names are not content-hashed.
func staticFiles(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "" || strings.HasSuffix(r.URL.Path, "/") {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Cache-Control", "no-cache")
		h.ServeHTTP(w, r)
	})
}

func pathID(w http.ResponseWriter, r *http.Request) (int, bool) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, "invalid task id")
		return 0, false
	}
	return id, true
}

func decodeTask(w http.ResponseWriter, r *http.Request) (database.Task, bool) {
	var task database.Task
	// Unknown fields are allowed on purpose: older clients may send extras.
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxBodyBytes)).Decode(&task); err != nil {
		if _, ok := errors.AsType[*http.MaxBytesError](err); ok {
			writeError(w, http.StatusRequestEntityTooLarge, "request body too large")
		} else {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
		}
		return task, false
	}
	if err := validateTask(&task); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return task, false
	}
	return task, true
}

func (s *server) storeError(w http.ResponseWriter, r *http.Request, err error) {
	if errors.Is(err, database.ErrNotFound) {
		writeError(w, http.StatusNotFound, "task not found")
		return
	}
	s.internalError(w, r, err)
}

// internalError logs the real cause; clients never see database error text.
func (s *server) internalError(w http.ResponseWriter, r *http.Request, err error) {
	s.log.Error("request failed", "method", r.Method, "path", r.URL.Path, "err", err)
	writeError(w, http.StatusInternalServerError, "internal server error")
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func (s *server) requireToken(next http.HandlerFunc) http.HandlerFunc {
	if s.token == "" {
		return next
	}
	// Hashing first keeps the comparison constant-time regardless of length.
	want := sha256.Sum256([]byte(s.token))
	return func(w http.ResponseWriter, r *http.Request) {
		got, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
		gotSum := sha256.Sum256([]byte(got))
		if !ok || subtle.ConstantTimeCompare(gotSum[:], want[:]) != 1 {
			w.Header().Set("WWW-Authenticate", "Bearer")
			writeError(w, http.StatusUnauthorized, "missing or invalid API token")
			return
		}
		next(w, r)
	}
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Referrer-Policy", "no-referrer")
		h.Set("X-Frame-Options", "DENY")
		next.ServeHTTP(w, r)
	})
}

func (s *server) recoverPanics(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if v := recover(); v != nil {
				if v == http.ErrAbortHandler {
					panic(v)
				}
				s.log.Error("panic", "method", r.Method, "path", r.URL.Path, "panic", v)
				writeError(w, http.StatusInternalServerError, "internal server error")
			}
		}()
		next.ServeHTTP(w, r)
	})
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (w *statusRecorder) WriteHeader(code int) {
	if w.status == 0 {
		w.status = code
	}
	w.ResponseWriter.WriteHeader(code)
}

func (w *statusRecorder) Write(b []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	return w.ResponseWriter.Write(b)
}

func (w *statusRecorder) Unwrap() http.ResponseWriter { return w.ResponseWriter }

func (s *server) logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w}
		next.ServeHTTP(rec, r)
		if rec.status == 0 {
			rec.status = http.StatusOK
		}
		s.log.Info("request", "method", r.Method, "path", r.URL.Path, "status", rec.status,
			"duration_ms", float64(time.Since(start).Microseconds())/1000)
	})
}
