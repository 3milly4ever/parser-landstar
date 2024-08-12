package log

import (
	"os"

	"github.com/sirupsen/logrus"
)

var Logger *logrus.Logger

func InitLogger() {
	Logger = logrus.New()

	// Set the output to a file
	file, err := os.OpenFile("logs/app.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err == nil {
		Logger.Out = file
	} else {
		Logger.Info("Failed to log to file, using default stderr")
	}

	// Set the log level (optional, default is Info)
	Logger.SetLevel(logrus.InfoLevel)

	// Set log format (optional)
	Logger.SetFormatter(&logrus.TextFormatter{
		FullTimestamp: true,
	})
}
