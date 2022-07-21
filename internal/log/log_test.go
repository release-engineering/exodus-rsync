package log

import (
	"fmt"
	"os"
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
	h1, _ := fn()
	handler1, _ := h1.(*syslogHandler)

	if handler1 == nil {
		t.Error("auto with haveJournal=false did not return syslog handler")
	}

	// auto with journald means journald
	fn = loggerBackend(&testcase{"", "auto"}, true)
	h2, _ := fn()
	handler2, _ := h2.(*journalHandler)

	if handler2 == nil {
		t.Error("auto with haveJournal=true did not return journald handler")
	}
}

func TestSyslogHandler(t *testing.T) {
	fn := loggerBackend(&testcase{"", "syslog"}, false)
	h, _ := fn()
	handler := h.(*syslogHandler)
	handler.test = true

	log := Package.NewLogger(args.Config{})
	log.Level = DebugLevel
	log.Handler = handler

	// should handle simple fields
	log.F("foo", "bar").Info("Hi")

	// and complex fields
	err := fmt.Errorf("Mistakes were made")
	log.F("error", err).Error("Something went wrong")

	e := handler.Entries
	assert.Equal(t, e[0], "Hi {\"foo\":\"bar\"}\n")
	assert.Equal(t, e[1], "Something went wrong {\"error\":\"Mistakes were made\"}\n")
}

func TestJournaldHandler(t *testing.T) {
	fn := loggerBackend(&testcase{"", "journald"}, false)
	h, _ := fn()
	handler := h.(*journalHandler)
	handler.test = true

	log := Package.NewLogger(args.Config{})
	log.Level = DebugLevel
	log.Handler = handler

	// should handle simple fields
	log.F("foo", "bar").Info("Hi")

	// and complex fields
	err := fmt.Errorf("Mistakes were made")
	log.F("error", err).Error("Something went wrong")

	e := handler.Entries
	assert.Equal(t, e[0], "Hi FOO=bar")
	assert.Equal(t, e[1], "Something went wrong ERROR=Mistakes were made")
}

func TestFileBaseHandler(t *testing.T) {
	file, _ := os.CreateTemp("", "tmpfile-")
	fn := loggerBackend(&testcase{"", "file:" + file.Name()}, false)
	h, _ := fn()
	handler := h.(*baseHandler)
	handler.test = true

	log := Package.NewLogger(args.Config{})
	log.Level = DebugLevel
	log.Handler = handler

	// should handle simple fields
	log.F("foo", "bar").Info("Hi")

	// and complex fields
	err := fmt.Errorf("Mistakes were made")
	log.F("error", err).Error("Something went wrong")

	e := handler.Entries
	assert.Equal(t, e[0], "Hi {\"foo\":\"bar\"}\n")
	assert.Equal(t, e[1], "Something went wrong {\"error\":\"Mistakes were made\"}\n")
}

func TestBadFileBaseHandler(t *testing.T) {
	dir := os.TempDir()
	name := dir + "tmpfile-not-permitted"
	f, _ := os.OpenFile(name, os.O_CREATE, 0600)

	// This is a case when a file is not allowed to open.
	tc := testcase{"info", "file:" + name}
	log := Package.NewLogger(args.Config{})
	log.Level = DebugLevel

	log.StartPlatformLogger(&tc)
	f.Close()
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
