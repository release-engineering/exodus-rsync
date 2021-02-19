package log

import (
	"context"

	apexLog "github.com/apex/log"
)

const InfoLevel = apexLog.InfoLevel
const DebugLevel = apexLog.DebugLevel

func NewContext(ctx context.Context, v apexLog.Interface) context.Context {
	return apexLog.NewContext(ctx, v)
}

func FromContext(ctx context.Context) *Logger {
	return apexLog.FromContext(ctx).(*Logger)
}

type Logger struct {
	apexLog.Logger
}

func (l *Logger) F(v ...interface{}) *apexLog.Entry {
	fields := apexLog.Fields{}
	for i := 0; i < len(v); i += 2 {
		fields[v[i].(string)] = v[i+1]
	}

	return l.WithFields(fields)
}
