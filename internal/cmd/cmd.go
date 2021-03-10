package cmd

import (
	"context"

	"github.com/release-engineering/exodus-rsync/internal/args"
	"github.com/release-engineering/exodus-rsync/internal/conf"
	"github.com/release-engineering/exodus-rsync/internal/gw"
	"github.com/release-engineering/exodus-rsync/internal/log"
	"github.com/release-engineering/exodus-rsync/internal/rsync"
)

var ext = struct {
	conf  conf.Interface
	rsync rsync.Interface
	gw    gw.Interface
	log   log.Interface
}{
	conf.Package,
	rsync.Package,
	gw.Package,
	log.Package,
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
	parsedArgs := args.Parse(rawArgs, version, nil)

	logger := ext.log.NewLogger(parsedArgs)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ctx = log.NewContext(ctx, logger)

	cfg, err := ext.conf.Load(ctx, parsedArgs)
	if err != nil {
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

	return main(ctx, env, parsedArgs)
}
