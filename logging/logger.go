package logging

import (
	"os"

	"github.com/sirupsen/logrus"
)

var Log *logrus.Logger

func BoostrapLogger() {
	Log = &logrus.Logger{
		Out:   nil,
		Hooks: nil,
		Formatter: &logrus.TextFormatter{
			DisableColors:    false,
			DisableQuote:     false,
			DisableTimestamp: false,
			FullTimestamp:    false,
			TimestampFormat:  "",
		},
		ReportCaller: false,
		Level:        logrus.DebugLevel,
		ExitFunc:     nil,
	}

	Log.SetReportCaller(true)
	Log.Out = os.Stdout
}
