package log

import (
	"github.com/aws/smithy-go/logging"
)

// SDKLogger adapts the application logger for aws-sdk-go-v2 ClientLogMode.
type SDKLogger struct {
	Logger *Logger
}

// Logf implements smithy-go logging.Logger.
func (a *SDKLogger) Logf(classification logging.Classification, format string, v ...interface{}) {
	if a.Logger == nil {
		return
	}
	a.Logger.F("aws", 1, "sdk", string(classification)).Debugf(format, v...)
}
