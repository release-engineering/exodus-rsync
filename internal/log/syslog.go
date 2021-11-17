package log

import (
	"encoding/json"
	"fmt"
	"log/syslog"
	"strings"
	"sync"

	apexLog "github.com/apex/log"
)

type syslogHandler struct {
	writer  *syslog.Writer
	test    bool
	mutex   sync.Mutex
	Entries []string
}

type writeFunc func(string) error

func newSyslogHandler() apexLog.Handler {
	out := syslogHandler{}

	writer, err := syslog.New(syslog.LOG_INFO, "exodus-rsync")
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
	// We will not go any lower than INFO severity here because such
	// messages will often be filtered by syslog itself.
	return h.writer.Info
}

func syslogFields(e *apexLog.Entry) map[string]string {
	out := make(map[string]string)

	for key := range e.Fields {
		val := fmt.Sprint(e.Fields[key])
		out[key] = val
	}

	return out
}

func (h *syslogHandler) HandleLog(e *apexLog.Entry) error {
	bld := strings.Builder{}
	bld.WriteString(e.Message + " ")

	enc := json.NewEncoder(&bld)
	enc.Encode(syslogFields(e))

	if h.test {
		h.mutex.Lock()
		defer h.mutex.Unlock()

		h.Entries = append(h.Entries, bld.String())
	}

	return h.writerForLevel(e.Level)(bld.String())
}
