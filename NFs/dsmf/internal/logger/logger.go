package logger

import (
	"os"
	"time"

	"github.com/sirupsen/logrus"
)

var Log = logrus.New()
var MainLog = Log.WithField("component", "DSMF")
var SBILog = Log.WithField("component", "DSMF-SBI")
var RPCLog = Log.WithField("component", "DSMF-RPC")
var ProcLog = Log.WithField("component", "DSMF-PROC")

func Configure(enable bool, level string, reportCaller bool) error {
	if enable {
		Log.SetOutput(os.Stderr)
	} else {
		Log.SetOutput(os.Stdout)
	}
	parsedLevel, err := logrus.ParseLevel(level)
	if err != nil {
		return err
	}
	Log.SetLevel(parsedLevel)
	Log.SetReportCaller(reportCaller)
	Log.SetFormatter(&logrus.TextFormatter{
		FullTimestamp:   true,
		TimestampFormat: time.RFC3339Nano,
	})
	return nil
}
