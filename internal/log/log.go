package log

import (
	"context"
	"os"

	"github.com/apex/log"
	apexLog "github.com/apex/log"
	"github.com/apex/log/handlers/cli"
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

type impl struct{}

// Package provides the default implementation of this package's interface.
var Package Interface = impl{}

// InfoLevel is appropriate for messages which should be
// visible by default to users of exodus-rsync.
const InfoLevel = apexLog.InfoLevel

// DebugLevel is appropriate for messages intended for the developers
// of exodus-rsync to diagnose issues.
const DebugLevel = apexLog.DebugLevel

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
//   logger.F("a", a, "b", b, "c", c).Info(...)
//
// ...is equivalent to the following more cumbersome:
//
//   logger.WithField("a", a).WithField("b", b).WithField("c", c).Info(...)
//
// ...or:
//
//   logger.WithFields(log.Fields{"a", a, "b", b, "c", c}).Info(...)
//
func (l *Logger) F(v ...interface{}) *apexLog.Entry {
	fields := apexLog.Fields{}
	for i := 0; i < len(v); i += 2 {
		fields[v[i].(string)] = v[i+1]
	}

	return l.WithFields(fields)
}

func (impl) NewLogger(args args.Config) *Logger {
	logger := Logger{}

	// TODO: use conf.Config for something?
	logger.Handler = cli.New(os.Stdout)
	logger.Level = log.InfoLevel
	if args.Verbose >= 1 {
		// TODO: think we need more loggers.
		logger.Level = log.DebugLevel
	}

	return &logger
}
