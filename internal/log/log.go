package log

import (
	"context"
	"fmt"
	"os"
	"strings"

	apexLog "github.com/apex/log"
	"github.com/apex/log/handlers/level"
	"github.com/apex/log/handlers/multi"
	"github.com/coreos/go-systemd/v22/journal"
	"github.com/release-engineering/exodus-rsync/internal/args"
)

// This package thinly wraps apex/log with some helpers to make logger
// usage a little less cumbersome.

//go:generate go run -modfile ../../go.tools.mod github.com/golang/mock/mockgen -package $GOPACKAGE -destination mock.go -source $GOFILE

// Interface defines the public interface of this package.
type Interface interface {
	// NewLogger will construct and return a logger appropriately configured to
	// serve as the primary logger throughout exodus-rsync.
	NewLogger(args.Config) *Logger
}

// ConfigProvider is an interface for any kind of object able to
// provide logger configuration.
type ConfigProvider interface {
	// Minimum log level.
	LogLevel() string

	// "journald" or "syslog" to force specific logging backend.
	Logger() string
}

type impl struct{}

// Package provides the default implementation of this package's interface.
var Package Interface = impl{}

// InfoLevel is appropriate for messages which should be
// visible by default to users of exodus-rsync.
const InfoLevel = apexLog.InfoLevel

// DebugLevel is appropriate for messages intended for the developers
// of exodus-rsync to diagnose issues.
const DebugLevel = apexLog.DebugLevel

// WarnLevel is appropriate for messages which might indicate a problem.
const WarnLevel = apexLog.WarnLevel

// NewContext returns a context containing the given logger, which can later
// be accessed via FromContext.
func NewContext(ctx context.Context, v apexLog.Interface) context.Context {
	return apexLog.NewContext(ctx, v)
}

// FromContext returns the logger within a context previously created via
// NewContext, or nil if unset.
//
// Throughout exodus-gw, this should be the primary method of obtaining a logger.
func FromContext(ctx context.Context) *Logger {
	out, castOk := apexLog.FromContext(ctx).(*Logger)
	if !castOk {
		return nil
	}
	return out
}

// Logger wraps an apex logger with additional utilities.
type Logger struct {
	apexLog.Logger
}

// F is shorthand for creating a log entry with multiple fields.
//
// This code:
//
//	logger.F("a", a, "b", b, "c", c).Info(...)
//
// ...is equivalent to the following more cumbersome:
//
//	logger.WithField("a", a).WithField("b", b).WithField("c", c).Info(...)
//
// ...or:
//
//	logger.WithFields(log.Fields{"a", a, "b", b, "c", c}).Info(...)
func (l *Logger) F(v ...interface{}) *apexLog.Entry {
	fields := apexLog.Fields{}
	for i := 0; i < len(v); i += 2 {
		fields[v[i].(string)] = v[i+1]
	}

	return l.WithFields(fields)
}

func (impl) NewLogger(args args.Config) *Logger {
	logger := Logger{}

	logLevel := WarnLevel
	if args.Verbose == 1 {
		logLevel = InfoLevel
	} else if args.Verbose >= 2 {
		logLevel = DebugLevel
	}

	handler, _ := newBaseHandler(os.Stdout)
	logger.Handler = level.New(handler, logLevel)

	return &logger
}

func loggerBackend(cfg ConfigProvider, haveJournal bool) func() (apexLog.Handler, error) {
	logger := cfg.Logger()
	if logger == "journald" {
		return newJournalHandler
	}
	if logger == "syslog" {
		return newSyslogHandler
	}
	if strings.HasPrefix(logger, "file:") {
		return func() (apexLog.Handler, error) {
			logPath := strings.Split(logger, ":")[1]
			f, err := os.OpenFile(logPath, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0600)
			if err != nil {
				return nil, err
			}
			return newBaseHandler(f)
		}
	}
	if haveJournal {
		return newJournalHandler
	}
	return newSyslogHandler
}

// StartPlatformLogger will enable (or not) the platform native logging,
// such as journald or syslog, according to the config.
func (l *Logger) StartPlatformLogger(cfg ConfigProvider) {
	logLevel := cfg.LogLevel()

	if logLevel == "none" {
		return
	} else if logLevel == "trace" {
		logLevel = "debug"
	}

	lvl, err := apexLog.ParseLevel(logLevel)
	if err != nil {
		l.Warnf("Invalid loglevel '%v' in config, defaulting to 'info'", logLevel)
		lvl = apexLog.InfoLevel
	}

	ctor := loggerBackend(cfg, journal.Enabled())
	handler, loggerErr := ctor()

	if loggerErr != nil {
		l.Errorf("Failed to initialize '%s' logger: %v", cfg.Logger(), loggerErr)
		return
	}

	// platform logger only logs messages at lvl and higher.
	handler = level.New(handler, lvl)

	// logger object writes to CLI *and* to platform logger.
	l.Handler = multi.New(
		l.Handler,
		handler,
	)
}

// Log is intended for use by the AWS SDK logger and is necessary for
// compatiblity with said package. Will log messages at debug level.
func (l *Logger) Log(v ...interface{}) {
	l.F("aws", 1).Debug(fmt.Sprint(v...))
}
