package cmd

import (
	"context"

	"github.com/release-engineering/exodus-rsync/internal/args"
	"github.com/release-engineering/exodus-rsync/internal/conf"
	"github.com/release-engineering/exodus-rsync/internal/log"
)

func rsyncMain(ctx context.Context, cfg conf.Config, args args.Config) int {
	logger := log.FromContext(ctx)
	exitCode := 0

	// Just run rsync. In the successful case, since we're doing execve system
	// call, this will never return.
	if err := ext.rsync.Exec(ctx, cfg, args); err != nil {
		logger.WithField("error", err).Error("can't exec rsync")
		exitCode = 94
	}

	return exitCode
}
