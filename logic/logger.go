package logic

import (
	"os"

	"github.com/sirupsen/logrus"
)

var log = logrus.New()

func init() {
	log.Formatter = &logrus.JSONFormatter{}
	log.SetLevel(logrus.WarnLevel)
}

// GetLogs reads all log files in the logs directory and returns their content.
func GatewayLogs() {
	log.Out = os.Stdout
	file, err := os.OpenFile("logs/gateway.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		log.Out = file
	} else {
		log.Info("Failed to open gateway log file: ", err)
	}
	defer file.Close()
}
