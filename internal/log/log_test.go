package log

import (
	"testing"

	apexLog "github.com/apex/log"
	"github.com/apex/log/handlers/memory"
	"github.com/release-engineering/exodus-rsync/internal/args"
	"github.com/stretchr/testify/assert"
)

type testcase struct {
	loglevel string
	logger   string
}

func (tc *testcase) LogLevel() string {
	return tc.loglevel
}

func (tc *testcase) Logger() string {
	return tc.logger
}

func TestPlatformLoggers(t *testing.T) {
	cases := []testcase{
		{"info", "journald"},
		{"debug", "journald"},
		{"info", "syslog"},
		{"debug", "syslog"},
		{"none", "auto"},
		{"invalid", "auto"},
		{"trace", "auto"},
	}

	for _, tc := range cases {
		t.Run(tc.loglevel+" "+tc.logger, func(t *testing.T) {
			// Test that a logger can be created and used with the given config.
			//
			// Note that all we are really testing here is that StartPlatformLogger
			// and the installed handler functions don't crash.
			log := Package.NewLogger(args.Config{})
			log.Level = DebugLevel

			log.StartPlatformLogger(&tc)

			log.F("foo", "bar").Debug("debug")
			log.F("foo", "bar").Info("info")
			log.F("foo", "bar").Warn("warn")
			log.F("foo", "bar").Error("err")
		})
	}
}

func TestPlatformAutoLoggers(t *testing.T) {

	// auto without journald means syslog
	fn := loggerBackend(&testcase{"", "auto"}, false)
	handler1, _ := fn().(*syslogHandler)

	if handler1 == nil {
		t.Error("auto with haveJournal=false did not return syslog handler")
	}

	// auto with journald means journald
	fn = loggerBackend(&testcase{"", "auto"}, true)
	handler2, _ := fn().(*journalHandler)

	if handler2 == nil {
		t.Error("auto with haveJournal=true did not return journald handler")
	}
}

func TestLogFunc(t *testing.T) {
	// Ensure Log can be used and contains the "aws" field.
	h := memory.New()
	logger := Package.NewLogger(args.Config{})
	logger.Handler = h

	logger.Log("hello")

	e := h.Entries[0]
	assert.Equal(t, e.Message, "hello")
	assert.Equal(t, apexLog.Fields{"aws": 1}, e.Fields)
}
