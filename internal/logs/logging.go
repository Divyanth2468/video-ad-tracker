package logs

import (
	"os"

	"github.com/sirupsen/logrus"
)

var Logger = logrus.New()

func InitLogger() {
	// Create or append to log file
	file, err := os.OpenFile("./internal/logs/app.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		// Fallback to stdout only if file cannot be opened
		Logger.SetOutput(os.Stdout)
		Logger.Warn("Failed to log to file, using default stderr")
	} else {
		Logger.SetOutput(file)
	}

	Logger.SetFormatter(&logrus.JSONFormatter{})
	Logger.SetLevel(logrus.InfoLevel)
	Logger.SetReportCaller(true)
}
