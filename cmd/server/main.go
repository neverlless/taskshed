package main

import (
	"github.com/neverlless/taskshed/internal/api"
	"github.com/neverlless/taskshed/internal/database"
	"github.com/neverlless/taskshed/internal/logger"
	"github.com/sirupsen/logrus"
	"net/http"
)

func main() {
	// Инициализация логирования
	logger.Init()

	// Инициализация базы данных
	err := database.Init()
	if err != nil {
		logger.Log.WithFields(logrus.Fields{
			"module": "main",
			"line":   15,
		}).Fatalf("Failed to initialize database: %v", err)
	}

	// Инициализация API маршрутов
	router := api.InitRoutes()

	// Запуск веб-сервера
	logger.Log.WithFields(logrus.Fields{
		"module": "main",
		"line":   20,
	}).Info("Server is running on http://localhost:8080")

	logger.Log.Fatal(http.ListenAndServe(":8080", router))
}
