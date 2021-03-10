package log

import (
	"fmt"
	"strings"

	apexLog "github.com/apex/log"
	"github.com/coreos/go-systemd/v22/journal"
)

type journalHandler struct {
	channel chan journalEntry
}

type journalEntry struct {
	message  string
	priority journal.Priority
	vars     map[string]string
}

func newJournalHandler() apexLog.Handler {
	out := journalHandler{}
	out.channel = make(chan journalEntry, 100)
	go sender(out.channel)
	return &out
}

func sender(entries <-chan journalEntry) {
	for e := range entries {
		// It is possible for journal.Send to fail, but if so,
		// it's not clear that we could do anything useful here.
		journal.Send(e.message, e.priority, e.vars)
	}
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

func fields(e *apexLog.Entry) map[string]string {
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
	fields := fields(e)

	h.channel <- journalEntry{msg, pri, fields}

	return nil
}
