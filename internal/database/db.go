package database

import (
	"database/sql"
	"fmt"
	"time"

	_ "github.com/lib/pq"
	_ "github.com/mattn/go-sqlite3"
	"github.com/neverlless/taskshed/internal/logger"
	"github.com/sirupsen/logrus"
)

var DB *sql.DB
var IsPostgres bool

func InitSQLite() error {
	var err error
	DB, err = sql.Open("sqlite3", "./taskshed.db")
	if err != nil {
		return err
	}

	return createTables()
}

func InitPostgres(host, port, user, password, dbname string) error {
	var err error
	psqlInfo := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		host, port, user, password, dbname)
	DB, err = sql.Open("postgres", psqlInfo)
	if err != nil {
		return err
	}

	return createTables()
}

func createTables() error {
	var createTableQuery string
	if IsPostgres {
		createTableQuery = `
        CREATE TABLE IF NOT EXISTS tasks (
            id SERIAL PRIMARY KEY,
            name TEXT NOT NULL,
            service TEXT NOT NULL,
            time TEXT NOT NULL,
            days_of_week TEXT NOT NULL,
            is_recurring BOOLEAN NOT NULL,
            description TEXT,
            hosts TEXT
        );
        `
	} else {
		createTableQuery = `
        CREATE TABLE IF NOT EXISTS tasks (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            name TEXT NOT NULL,
            service TEXT NOT NULL,
            time TEXT NOT NULL,
            days_of_week TEXT NOT NULL,
            is_recurring BOOLEAN NOT NULL,
            description TEXT,
            hosts TEXT
        );
        `
	}

	_, err := DB.Exec(createTableQuery)
	if err != nil {
		logger.Log.WithFields(logrus.Fields{
			"level":  "error",
			"ts":     time.Now().Format(time.RFC3339Nano),
			"caller": "db.go:44",
			"msg":    fmt.Sprintf("Failed to create tables: %v", err),
		}).Error(err)
		return err
	}

	logger.Log.WithFields(logrus.Fields{
		"level":  "info",
		"ts":     time.Now().Format(time.RFC3339Nano),
		"caller": "db.go:50",
		"msg":    "Database initialized",
	}).Info("Database initialized")
	return nil
}
