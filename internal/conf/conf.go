package conf

import (
	"context"

	"github.com/release-engineering/exodus-rsync/internal/args"
)

//go:generate go run -modfile ../../go.tools.mod github.com/golang/mock/mockgen -package $GOPACKAGE -destination mock.go -source $GOFILE

// Interface defines the public interface of this package.
type Interface interface {
	// Load will load and return configuration from the most appropriate
	// exodus-rsync config file.
	Load(context.Context, args.Config) (GlobalConfig, error)
}

type impl struct{}

// Package provides the default implementation of this package's interface.
var Package Interface = impl{}

// Config contains parsed content of an exodus-rsync configuration file.
type Config interface {
	// Path to certificate used to authenticate with exodus-gw.
	GwCert() string

	// Path to private key used to authenticate with exodus-gw.
	GwKey() string

	// Base URL of exodus-gw service in use.
	GwURL() string

	// exodus-gw environment in use (e.g. "live").
	GwEnv() string

	// How often to poll for task updates, in milliseconds.
	GwPollInterval() int

	// Max number of items to include in a single HTTP request to exodus-gw.
	GwBatchSize() int

	// Commit mode for publishes.
	GwCommit() string

	// Maximum attempts for any HTTP request to exodus-gw.
	GwMaxAttempts() int

	// Maximum backoff between retried HTTP requests, in milliseconds.
	GwMaxBackoff() int

	// Execution mode for rsync.
	RsyncMode() string

	// Minimum log level for platform logger.
	LogLevel() string

	// Specific logger backend (journald or syslog).
	Logger() string

	// Level of verbosity requested via CLI args.
	Verbosity() int

	// Diagnostics mode.
	Diag() bool

	// Strips this prefix from the destination path of exodus publish items.
	Strip() string

	// Number of threads used to upload files to the CDN.
	UploadThreads() int
}

// EnvironmentConfig provides configuration specific to one environment.
type EnvironmentConfig interface {
	Config

	Prefix() string
}

// GlobalConfig provides configuration applied to all environments.
type GlobalConfig interface {
	Config

	EnvironmentForDest(context.Context, string) EnvironmentConfig
}
