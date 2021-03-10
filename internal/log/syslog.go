package log

import (
	"encoding/json"
	"log/syslog"
	"strings"

	apexLog "github.com/apex/log"
)

type syslogEntry struct {
	message string
	level   apexLog.Level
}

type syslogHandler struct {
	writer  *syslog.Writer
	channel chan syslogEntry
}

type writeFunc func(string) error

func newSyslogHandler() apexLog.Handler {
	out := syslogHandler{}
	out.channel = make(chan syslogEntry, 100)

	writer, err := syslog.New(syslog.LOG_INFO, "")
	if err == nil {
		out.writer = writer
		go out.sender(out.channel)
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

func (h *syslogHandler) sender(entries <-chan syslogEntry) {
	for e := range entries {
		h.writerForLevel(e.level)(e.message)
	}
}

func (h *syslogHandler) HandleLog(e *apexLog.Entry) error {
	bld := strings.Builder{}
	bld.WriteString(e.Message + " ")

	enc := json.NewEncoder(&bld)
	enc.Encode(e.Fields)

	h.channel <- syslogEntry{bld.String(), e.Level}

	return nil
}
