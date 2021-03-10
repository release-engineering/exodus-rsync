package log

import (
	"encoding/json"
	"log/syslog"
	"strings"

	apexLog "github.com/apex/log"
)

type syslogHandler struct {
	writer *syslog.Writer
}

type writeFunc func(string) error

func newSyslogHandler() apexLog.Handler {
	out := syslogHandler{}

	writer, err := syslog.New(syslog.LOG_INFO, "")
	if err == nil {
		out.writer = writer
	}

	return &out
}

func (h *syslogHandler) writerForLevel(l apexLog.Level) writeFunc {
	if l >= apexLog.ErrorLevel {
		return h.writer.Err
	}
	if l == apexLog.WarnLevel {
		return h.writer.Warning
	}
	if l == apexLog.InfoLevel {
		return h.writer.Info
	}
	return h.writer.Debug
}

func (h *syslogHandler) HandleLog(e *apexLog.Entry) error {
	bld := strings.Builder{}
	bld.WriteString(e.Message + " ")

	enc := json.NewEncoder(&bld)
	enc.Encode(e.Fields)

	return h.writerForLevel(e.Level)(bld.String())
}
