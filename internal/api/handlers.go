package api

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/neverlless/taskshed/internal/database"
	"github.com/neverlless/taskshed/internal/logger"

	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
)

func CreateTask(w http.ResponseWriter, r *http.Request) {
	var task database.Task
	err := json.NewDecoder(r.Body).Decode(&task)
	if err != nil {
		logger.Log.WithFields(logrus.Fields{
			"level":  "error",
			"ts":     time.Now().Format(time.RFC3339Nano),
			"caller": "handlers.go:14",
			"msg":    fmt.Sprintf("Failed to decode task: %v", err),
		}).Error(err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	var id int
	if database.IsPostgres {
		err = database.DB.QueryRow(
			"INSERT INTO tasks (name, service, time, days_of_week, is_recurring, description, hosts) VALUES ($1, $2, $3, $4, $5, $6, $7) RETURNING id",
			task.Name, task.Service, task.Time, task.DaysOfWeek, task.IsRecurring, task.Description, task.Hosts,
		).Scan(&id)
	} else {
		result, err := database.DB.Exec(
			"INSERT INTO tasks (name, service, time, days_of_week, is_recurring, description, hosts) VALUES (?, ?, ?, ?, ?, ?, ?)",
			task.Name, task.Service, task.Time, task.DaysOfWeek, task.IsRecurring, task.Description, task.Hosts,
		)
		if err == nil {
			id64, err := result.LastInsertId()
			if err == nil {
				id = int(id64)
			}
		}
	}

	if err != nil {
		logger.Log.WithFields(logrus.Fields{
			"level":  "error",
			"ts":     time.Now().Format(time.RFC3339Nano),
			"caller": "handlers.go:30",
			"msg":    fmt.Sprintf("Failed to insert task: %v", err),
		}).Error(err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	task.ID = id
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(task)
}

func UpdateTask(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := strconv.Atoi(vars["id"])
	if err != nil {
		logger.Log.WithFields(logrus.Fields{
			"level":  "error",
			"ts":     time.Now().Format(time.RFC3339Nano),
			"caller": "handlers.go:42",
			"msg":    fmt.Sprintf("Invalid task ID: %v", err),
		}).Error(err)
		http.Error(w, "Invalid task ID", http.StatusBadRequest)
		return
	}

	var task database.Task
	err = json.NewDecoder(r.Body).Decode(&task)
	if err != nil {
		logger.Log.WithFields(logrus.Fields{
			"level":  "error",
			"ts":     time.Now().Format(time.RFC3339Nano),
			"caller": "handlers.go:50",
			"msg":    fmt.Sprintf("Failed to decode task: %v", err),
		}).Error(err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	_, err = database.DB.Exec("UPDATE tasks SET name=$1, service=$2, time=$3, days_of_week=$4, is_recurring=$5, description=$6, hosts=$7 WHERE id=$8",
		task.Name, task.Service, task.Time, task.DaysOfWeek, task.IsRecurring, task.Description, task.Hosts, id)
	if err != nil {
		logger.Log.WithFields(logrus.Fields{
			"level":  "error",
			"ts":     time.Now().Format(time.RFC3339Nano),
			"caller": "handlers.go:58",
			"msg":    fmt.Sprintf("Failed to update task: %v", err),
		}).Error(err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	task.ID = id
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(task)
}

func DeleteTask(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := strconv.Atoi(vars["id"])
	if err != nil {
		logger.Log.WithFields(logrus.Fields{
			"level":  "error",
			"ts":     time.Now().Format(time.RFC3339Nano),
			"caller": "handlers.go:71",
			"msg":    fmt.Sprintf("Invalid task ID: %v", err),
		}).Error(err)
		http.Error(w, "Invalid task ID", http.StatusBadRequest)
		return
	}

	_, err = database.DB.Exec("DELETE FROM tasks WHERE id=$1", id)
	if err != nil {
		logger.Log.WithFields(logrus.Fields{
			"level":  "error",
			"ts":     time.Now().Format(time.RFC3339Nano),
			"caller": "handlers.go:77",
			"msg":    fmt.Sprintf("Failed to delete task: %v", err),
		}).Error(err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func GetTasks(w http.ResponseWriter, r *http.Request) {
	rows, err := database.DB.Query("SELECT id, name, service, time, days_of_week, is_recurring, description, hosts FROM tasks")
	if err != nil {
		logger.Log.WithFields(logrus.Fields{
			"level":  "error",
			"ts":     time.Now().Format(time.RFC3339Nano),
			"caller": "handlers.go:86",
			"msg":    fmt.Sprintf("Failed to query tasks: %v", err),
		}).Error(err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var tasks []database.Task
	for rows.Next() {
		var task database.Task
		var description sql.NullString
		var hosts sql.NullString

		err := rows.Scan(&task.ID, &task.Name, &task.Service, &task.Time, &task.DaysOfWeek, &task.IsRecurring, &description, &hosts)
		if err != nil {
			logger.Log.WithFields(logrus.Fields{
				"level":  "error",
				"ts":     time.Now().Format(time.RFC3339Nano),
				"caller": "handlers.go:96",
				"msg":    fmt.Sprintf("Failed to scan task: %v", err),
			}).Error(err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		task.Description = description.String
		task.Hosts = hosts.String
		tasks = append(tasks, task)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(tasks)
}
