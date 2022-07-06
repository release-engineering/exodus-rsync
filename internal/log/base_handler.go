package log

import (
	"fmt"
	"io"
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
	mu     sync.Mutex
	Writer io.Writer
}

func newBaseHandler(w io.Writer) *baseHandler {
	return &baseHandler{
		Writer: w,
	}
}

func (h *baseHandler) HandleLog(e *apexLog.Entry) error {
	level := stringMap[e.Level]
	names := e.Fields.Names()

	h.mu.Lock()
	defer h.mu.Unlock()

	fmt.Fprintf(h.Writer, "%s: %-25s", level, e.Message)

	for _, name := range names {
		fmt.Fprintf(h.Writer, " %s=%v", name, e.Fields.Get(name))
	}

	fmt.Fprintln(h.Writer)

	return nil
}
