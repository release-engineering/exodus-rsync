package rsync

import (
	"context"
	"fmt"
	"os"
	"strings"
	"syscall"

	"github.com/release-engineering/exodus-rsync/internal/args"
	"github.com/release-engineering/exodus-rsync/internal/conf"
	"github.com/release-engineering/exodus-rsync/internal/log"
)

//go:generate go run -modfile ../../go.tools.mod github.com/golang/mock/mockgen -package $GOPACKAGE -destination mock.go -source $GOFILE

// Interface defines the public interface of this package.
type Interface interface {
	// Exec will prepare and execute an rsync command according to the configuration
	// and arguments passed into exodus-rsync.
	//
	// Note that the command is run using the execve syscall, meaning that it
	// *replaces* the current process. It never returns, unless an error occurs.
	Exec(context.Context, conf.Config, args.Config) error
}

type impl struct{}

// Package provides the default implementation of this package's interface.
var Package Interface = impl{}

// Externals which may be swapped out during tests.
var ext = struct {
	exec func(string, []string, []string) error
}{
	syscall.Exec,
}

func rsyncArguments(ctx context.Context, cfg conf.Config, args args.Config) []string {
	logger := log.FromContext(ctx)

	argv := []string{}

	if args.Recursive {
		argv = append(argv, "--recursive")
	}
	if args.Times {
		argv = append(argv, "--times")
	}
	if args.Delete {
		argv = append(argv, "--delete")
	}
	if args.KeepDirlinks {
		argv = append(argv, "--keep-dirlinks")
	}
	if args.OmitDirTimes {
		argv = append(argv, "--omit-dir-times")
	}
	if args.Compress {
		argv = append(argv, "--compress")
	}
	if args.ItemizeChanges {
		argv = append(argv, "--itemize-changes")
	}
	if args.Rsh != "" {
		argv = append(argv, "--rsh", args.Rsh)
	}
	if args.CopyLinks {
		argv = append(argv, "--copy-links")
	}
	if args.Stats {
		argv = append(argv, "--stats")
	}
	if args.Timeout != 0 {
		argv = append(argv, "--timeout", fmt.Sprint(args.Timeout))
	}
	if args.Archive {
		argv = append(argv, "--archive")
	}
	if args.Verbose != 0 {
		argv = append(argv, "-"+strings.Repeat("v", args.Verbose))
	}
	if args.IgnoreExisting {
		argv = append(argv, "--ignore-existing")
	}
	if args.Filter != "" {
		argv = append(argv, "--filter", fmt.Sprint(args.Filter))
	}

	argv = append(argv, args.Src, args.Dest)

	logger.F("argv", argv).Debug("prepared rsync command")

	return argv
}

func (impl) Exec(ctx context.Context, cfg conf.Config, args args.Config) error {
	return ext.exec(
		// TODO: look up path properly, ensure we don't look up ourselves
		"/usr/bin/rsync",
		rsyncArguments(ctx, cfg, args),
		os.Environ(),
	)
}
