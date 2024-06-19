package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/neverlless/taskshed/internal/api"
	"github.com/neverlless/taskshed/internal/database"
	"github.com/neverlless/taskshed/internal/logger"

	"github.com/sirupsen/logrus"
)

func main() {
	// Comand line flags
	var (
		port       = flag.String("port", "8080", "Port to run the server on")
		dbType     = flag.String("db-type", getEnv("DB_TYPE", "sqlite"), "Database type (sqlite or postgres)")
		dbHost     = flag.String("db-host", getEnv("DB_HOST", "localhost"), "Database host")
		dbPort     = flag.String("db-port", getEnv("DB_PORT", "5432"), "Database port")
		dbUser     = flag.String("db-user", getEnv("DB_USER", "postgres"), "Database user")
		dbPassword = flag.String("db-password", getEnv("DB_PASSWORD", ""), "Database password")
		dbName     = flag.String("db-name", getEnv("DB_NAME", "taskshed"), "Database name")
	)
	flag.Parse()

	// Logger initialization
	logger.Init()

	// Database initialization
	var err error
	var isPostgres bool
	if *dbType == "postgres" {
		err = database.InitPostgres(*dbHost, *dbPort, *dbUser, *dbPassword, *dbName)
		isPostgres = true
	} else {
		err = database.InitSQLite()
		isPostgres = false
	}
	if err != nil {
		logger.Log.WithFields(logrus.Fields{
			"level":  "error",
			"ts":     time.Now().Format(time.RFC3339Nano),
			"caller": "main.go:43",
			"msg":    fmt.Sprintf("Failed to initialize database: %v", err),
		}).Fatal(err)
	}

	// Set database type
	database.IsPostgres = isPostgres

	// API initialization
	router := api.InitRoutes(true)

	// Server start
	addr := fmt.Sprintf(":%s", *port)
	logger.Log.WithFields(logrus.Fields{
		"level":  "info",
		"ts":     time.Now().Format(time.RFC3339Nano),
		"caller": "main.go:55",
		"msg":    fmt.Sprintf("Server is running on http://localhost%s", addr),
	}).Info("Server started")

	log.Fatal(http.ListenAndServe(addr, router))
}

func getEnv(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}
