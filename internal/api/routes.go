package api

import (
	"github.com/neverlless/taskshed/internal/logger"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
)

func InitRoutes(authEnabled bool) *mux.Router {
	router := mux.NewRouter()

	router.Use(loggingMiddleware)

	router.HandleFunc("/", CalendarHandler).Methods("GET")
	router.HandleFunc("/tasks-view", TasksViewHandler).Methods("GET")

	api := router.PathPrefix("/api").Subrouter()
	api.HandleFunc("/tasks", CreateTask).Methods("POST")
	api.HandleFunc("/tasks/{id}", UpdateTask).Methods("PUT")
	api.HandleFunc("/tasks/{id}", DeleteTask).Methods("DELETE")
	api.HandleFunc("/tasks", GetTasks).Methods("GET")

	// Сервинг статических файлов
	router.PathPrefix("/static/").Handler(http.StripPrefix("/static/", http.FileServer(http.Dir("./web/static/"))))

	return router
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		logger.Log.WithFields(logrus.Fields{
			"method": r.Method,
			"url":    r.URL.String(),
		}).Info("Received request")
		next.ServeHTTP(w, r)
	})
}

func CalendarHandler(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "web/templates/calendar.html")
}

func TasksViewHandler(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "web/templates/tasks-view.html")
}
