package cmd

import (
	"testing"

	"github.com/apex/log/handlers/memory"
	"github.com/apex/log/handlers/multi"
	"github.com/release-engineering/exodus-rsync/internal/args"
	"github.com/release-engineering/exodus-rsync/internal/log"

	apexLog "github.com/apex/log"
)

// Implementation of log.Interface which will return a logger with
// a memory Handler installed.
type memoryLoggerInterface struct {
	handler  memory.Handler
	delegate func(args.Config) *log.Logger
}

func (m *memoryLoggerInterface) NewLogger(args args.Config) *log.Logger {
	logger := m.delegate(args)

	// We expect logger to have some real handler installed.
	// We'll let it keep that handler, but also install our memory handler.
	logger.Handler = multi.New(
		&m.handler,
		logger.Handler,
	)

	return logger
}

func CaptureLogger(t *testing.T) *memory.Handler {
	oldLog := ext.log
	t.Cleanup(func() { ext.log = oldLog })

	memoryLogger := memoryLoggerInterface{
		delegate: oldLog.NewLogger,
	}

	ext.log = &memoryLogger

	return &memoryLogger.handler
}

func FindEntry(m *memory.Handler, msg string) *apexLog.Entry {
	found := make([]*apexLog.Entry, 0)

	for _, entry := range m.Entries {
		if entry.Message == msg {
			found = append(found, entry)
		}
	}

	if len(found) == 1 {
		return found[0]
	}

	return nil
}
