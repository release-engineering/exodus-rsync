package log

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"sync"

	apexLog "github.com/apex/log"
)

var stringMap = [...]string{
	apexLog.DebugLevel: "DEBUG",
	apexLog.InfoLevel:  "INFO",
	apexLog.WarnLevel:  "WARN",
	apexLog.ErrorLevel: "ERROR",
	apexLog.FatalLevel: "FATAL",
}

type baseHandler struct {
	mutex   sync.Mutex
	test    bool
	Writer  io.Writer
	Entries []string
}

func newBaseHandler(w io.Writer) (*baseHandler, error) {
	return &baseHandler{
		Writer: w,
	}, nil
}

func baselFields(e *apexLog.Entry) map[string]string {
	out := make(map[string]string)

	for key := range e.Fields {
		val := fmt.Sprint(e.Fields[key])
		out[key] = val
	}

	return out
}

func (h *baseHandler) HandleLog(e *apexLog.Entry) error {
	bld := strings.Builder{}
	bld.WriteString(e.Message + " ")

	enc := json.NewEncoder(&bld)
	enc.Encode(baselFields(e))

	if h.test {
		h.mutex.Lock()
		defer h.mutex.Unlock()

		h.Entries = append(h.Entries, bld.String())
	}

	fmt.Fprintln(h.Writer, e.Timestamp.UTC().Format("2006-01-02 15:04:05")+" "+bld.String())

	return nil
}
