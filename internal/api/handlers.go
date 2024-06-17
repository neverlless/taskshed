package api

import (
	"encoding/json"
	"github.com/neverlless/taskshed/internal/database"
	"github.com/neverlless/taskshed/internal/logger"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
)

func CreateTask(w http.ResponseWriter, r *http.Request) {
	var task database.Task
	err := json.NewDecoder(r.Body).Decode(&task)
	if err != nil {
		logger.Log.WithFields(logrus.Fields{
			"module": "api",
			"line":   14,
		}).Errorf("Failed to decode task: %v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	result, err := database.DB.Exec("INSERT INTO tasks (name, service, time, days_of_week, is_recurring, description) VALUES (?, ?, ?, ?, ?, ?)",
		task.Name, task.Service, task.Time, task.DaysOfWeek, task.IsRecurring, task.Description)
	if err != nil {
		logger.Log.WithFields(logrus.Fields{
			"module": "api",
			"line":   23,
		}).Errorf("Failed to insert task: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	id, err := result.LastInsertId()
	if err != nil {
		logger.Log.WithFields(logrus.Fields{
			"module": "api",
			"line":   29,
		}).Errorf("Failed to get last insert id: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	task.ID = int(id)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(task)
}

func UpdateTask(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := strconv.Atoi(vars["id"])
	if err != nil {
		logger.Log.WithFields(logrus.Fields{
			"module": "api",
			"line":   42,
		}).Errorf("Invalid task ID: %v", err)
		http.Error(w, "Invalid task ID", http.StatusBadRequest)
		return
	}

	var task database.Task
	err = json.NewDecoder(r.Body).Decode(&task)
	if err != nil {
		logger.Log.WithFields(logrus.Fields{
			"module": "api",
			"line":   50,
		}).Errorf("Failed to decode task: %v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	_, err = database.DB.Exec("UPDATE tasks SET name=?, service=?, time=?, days_of_week=?, is_recurring=?, description=? WHERE id=?",
		task.Name, task.Service, task.Time, task.DaysOfWeek, task.IsRecurring, task.Description, id)
	if err != nil {
		logger.Log.WithFields(logrus.Fields{
			"module": "api",
			"line":   58,
		}).Errorf("Failed to update task: %v", err)
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
			"module": "api",
			"line":   71,
		}).Errorf("Invalid task ID: %v", err)
		http.Error(w, "Invalid task ID", http.StatusBadRequest)
		return
	}

	_, err = database.DB.Exec("DELETE FROM tasks WHERE id=?", id)
	if err != nil {
		logger.Log.WithFields(logrus.Fields{
			"module": "api",
			"line":   77,
		}).Errorf("Failed to delete task: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func GetTasks(w http.ResponseWriter, r *http.Request) {
	rows, err := database.DB.Query("SELECT id, name, service, time, days_of_week, is_recurring, description FROM tasks")
	if err != nil {
		logger.Log.WithFields(logrus.Fields{
			"module": "api",
			"line":   86,
		}).Errorf("Failed to query tasks: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var tasks []database.Task
	for rows.Next() {
		var task database.Task
		err := rows.Scan(&task.ID, &task.Name, &task.Service, &task.Time, &task.DaysOfWeek, &task.IsRecurring, &task.Description)
		if err != nil {
			logger.Log.WithFields(logrus.Fields{
				"module": "api",
				"line":   96,
			}).Errorf("Failed to scan task: %v", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		tasks = append(tasks, task)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(tasks)
}
