package cmd

import (
	"context"

	"github.com/release-engineering/exodus-rsync/internal/args"
	"github.com/release-engineering/exodus-rsync/internal/conf"
	"github.com/release-engineering/exodus-rsync/internal/diag"
	"github.com/release-engineering/exodus-rsync/internal/gw"
	"github.com/release-engineering/exodus-rsync/internal/log"
	"github.com/release-engineering/exodus-rsync/internal/rsync"
)

var ext = struct {
	conf  conf.Interface
	rsync rsync.Interface
	gw    gw.Interface
	log   log.Interface
	diag  diag.Interface
}{
	conf.Package,
	rsync.Package,
	gw.Package,
	log.Package,
	diag.Package,
}

// This version should be written at build time, see Makefile.
var version string = "(unknown version)"

type mainFunc func(context.Context, conf.Config, args.Config) int

func invalidMain(ctx context.Context, cfg conf.Config, _ args.Config) int {
	logger := log.FromContext(ctx)

	logger.F("rsyncmode", cfg.RsyncMode()).Error("Invalid 'rsyncmode' in configuration")
	return 95
}

// Main is the top-level entry point to the exodus-rsync command.
func Main(rawArgs []string) int {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Before anything else, check for --server or --sender, which
	// indicate rsync itself is trying to do something.
	// If either are provided, pass through to real rsync.
	for _, arg := range rawArgs {
		if arg == "--server" || arg == "--sender" {
			logger := ext.log.NewLogger(args.Config{})
			ctx = log.NewContext(ctx, logger)
			return rsyncRaw(ctx, rawArgs)
		}
	}

	parsedArgs := args.Parse(rawArgs, version, nil)

	logger := ext.log.NewLogger(parsedArgs)

	ctx = log.NewContext(ctx, logger)

	cfg, err := ext.conf.Load(ctx, parsedArgs)
	if err != nil {
		if _, ok := err.(*conf.MissingConfigFile); ok {
			// Failed to find any config files, fallback to rsync
			logger.WithField("error", err).Debug("setting rsyncmode to 'rsync'")
			return rsyncMain(ctx, nil, parsedArgs)
		}
		logger.WithField("error", err).Error("can't load config")
		return 23
	}

	var env conf.Config = cfg.EnvironmentForDest(ctx, parsedArgs.Dest)
	var main mainFunc = invalidMain

	if env == nil || env.RsyncMode() == "rsync" {
		main = rsyncMain
	} else if env.RsyncMode() == "exodus" {
		main = exodusMain
	} else if env.RsyncMode() == "mixed" {
		main = mixedMain
	}

	if env == nil {
		env = cfg
	}

	logger.StartPlatformLogger(env)

	// We've now decided more or less what we're going to do.
	// In diagnostic mode, before proceeding we will *also* dump
	// a wealth of information about the current environment,
	// configuration and command, then proceed with publish
	// afterward.
	if env.Diag() {
		ext.diag.Run(ctx, env, parsedArgs)
	}

	return main(ctx, env, parsedArgs)
}
