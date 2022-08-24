package shared

import (
	"os"

	"github.com/sirupsen/logrus"
)

// GetLogger returns a new logger.
func GetLogger(debug bool) (*logrus.Logger, error) {
	logger := logrus.StandardLogger()

	logger.SetOutput(os.Stdout)

	formatter := logrus.TextFormatter{
		FullTimestamp: true,
		PadLevelText:  true,
	}

	logger.Formatter = &formatter

	if debug {
		logger.Level = logrus.DebugLevel
	}

	return logger, nil
}
