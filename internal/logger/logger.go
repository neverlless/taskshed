package logger

import (
	"github.com/sirupsen/logrus"
	"io"
	"os"
)

var Log *logrus.Logger

func Init() {
	Log = logrus.New()
	Log.SetFormatter(&logrus.TextFormatter{
		FullTimestamp: true,
		ForceColors:   true,
	})

	file, err := os.OpenFile("taskshed.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		Log.Fatal(err)
	}

	Log.SetOutput(io.MultiWriter(file, os.Stdout))

	Log.SetLevel(logrus.InfoLevel)
}
