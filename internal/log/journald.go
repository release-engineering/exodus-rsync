package log

import (
	"fmt"
	"strings"
	"sync"

	apexLog "github.com/apex/log"
	"github.com/coreos/go-systemd/v22/journal"
)

type journalHandler struct {
	test    bool
	mutex   sync.Mutex
	Entries []string
}

func newJournalHandler() apexLog.Handler {
	return &journalHandler{}
}

func priority(e *apexLog.Entry) journal.Priority {
	if e.Level >= apexLog.ErrorLevel {
		return journal.PriErr
	}
	if e.Level == apexLog.WarnLevel {
		return journal.PriWarning
	}
	if e.Level == apexLog.InfoLevel {
		return journal.PriInfo
	}
	return journal.PriDebug
}

func journalFields(e *apexLog.Entry) map[string]string {
	out := make(map[string]string)

	for key := range e.Fields {
		val := fmt.Sprint(e.Fields[key])
		key = strings.ToUpper(key)
		out[key] = val
	}

	return out
}

func (h *journalHandler) HandleLog(e *apexLog.Entry) error {
	msg := e.Message
	pri := priority(e)
	fields := journalFields(e)

	if h.test {
		h.mutex.Lock()
		defer h.mutex.Unlock()

		var sFields string
		for k, v := range fields {
			sFields += fmt.Sprintf("%s=%s", k, v)
		}
		h.Entries = append(h.Entries, msg+" "+sFields)
	}

	return journal.Send(msg, pri, fields)
}
