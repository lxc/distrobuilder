package shared

import (
	"github.com/sirupsen/logrus"
)

// GetLogger returns a new logger.
func GetLogger(debug bool) (*logrus.Logger, error) {
	logger := logrus.StandardLogger()

	formatter := logrus.TextFormatter{
		FullTimestamp: true,
	}

	logger.Formatter = &formatter

	if debug {
		logger.Level = logrus.DebugLevel
	}

	return logger, nil
}
