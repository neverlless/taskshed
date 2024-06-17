package database

import (
	"database/sql"
	_ "github.com/mattn/go-sqlite3"
	"github.com/neverlless/taskshed/internal/logger"
	"github.com/sirupsen/logrus"
)

var DB *sql.DB

func Init() error {
	var err error
	DB, err = sql.Open("sqlite3", "./taskshed.db")
	if err != nil {
		return err
	}

	// Создание таблицы tasks, если ее нет
	createTableQuery := `
    CREATE TABLE IF NOT EXISTS tasks (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        name TEXT NOT NULL,
        service TEXT NOT NULL,
        time TEXT NOT NULL,
        days_of_week TEXT NOT NULL,
        is_recurring BOOLEAN NOT NULL,
        description TEXT
    );
    `
	_, err = DB.Exec(createTableQuery)
	if err != nil {
		return err
	}

	logger.Log.WithFields(logrus.Fields{
		"module": "database",
		"line":   23,
	}).Info("Database initialized")
	return nil
}
