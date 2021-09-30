package rsync

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"

	"github.com/release-engineering/exodus-rsync/internal/args"
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
	Exec(context.Context, args.Config) error

	// RawExec will execute an rsync command with no argument parsing or configuration,
	// simply passing the raw arguments through to real rsync, /usr/bin/rsync.
	RawExec(context.Context, []string) error

	// Command will prepare and return an os.exec Cmd struct for invoking rsync.
	//
	// Only Path and Args are filled in. Other elements such as stdout, stderr
	// can be set up by the caller prior to invoking the command.
	Command(context.Context, []string) *exec.Cmd
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

// Arguments converts the args.Config struct back into an argument vector.
func Arguments(ctx context.Context, args args.Config) []string {
	logger := log.FromContext(ctx)

	argv := []string{}

	if args.Verbose != 0 {
		argv = append(argv, "-"+strings.Repeat("v", args.Verbose))
	}
	if args.Archive {
		argv = append(argv, "--archive")
	}
	if args.Recursive {
		argv = append(argv, "--recursive")
	}
	if args.Relative {
		argv = append(argv, "--relative")
	}
	if args.Links {
		argv = append(argv, "--links")
	}
	if args.CopyLinks {
		argv = append(argv, "--copy-links")
	}
	if args.KeepDirlinks {
		argv = append(argv, "--keep-dirlinks")
	}
	if args.HardLinks {
		argv = append(argv, "--hard-links")
	}
	if args.Perms {
		argv = append(argv, "--perms")
	}
	if args.Executability {
		argv = append(argv, "--executability")
	}
	if args.Acls {
		argv = append(argv, "--acls")
	}
	if args.Xattrs {
		argv = append(argv, "--xattrs")
	}
	if args.Owner {
		argv = append(argv, "--owner")
	}
	if args.Group {
		argv = append(argv, "--group")
	}
	if args.Devices {
		argv = append(argv, "--devices")
	}
	if args.Specials {
		argv = append(argv, "--specials")
	}
	if args.Times {
		argv = append(argv, "--times")
	}
	if args.Atimes {
		argv = append(argv, "--atimes")
	}
	if args.Crtimes {
		argv = append(argv, "--crtimes")
	}
	if args.OmitDirTimes {
		argv = append(argv, "--omit-dir-times")
	}
	if args.Rsh != "" {
		argv = append(argv, "--rsh", args.Rsh)
	}
	if args.IgnoreExisting {
		argv = append(argv, "--ignore-existing")
	}
	if args.Delete {
		argv = append(argv, "--delete")
	}
	if args.PruneEmptyDirs {
		argv = append(argv, "--prune-empty-dirs")
	}
	if args.Timeout != 0 {
		argv = append(argv, "--timeout", fmt.Sprint(args.Timeout))
	}
	if args.Compress {
		argv = append(argv, "--compress")
	}
	for _, rule := range args.Filter {
		argv = append(argv, "--filter", fmt.Sprint(rule))
	}
	for _, ex := range args.Exclude {
		argv = append(argv, "--exclude", fmt.Sprint(ex))
	}
	for _, in := range args.Include {
		argv = append(argv, "--include", fmt.Sprint(in))
	}
	if args.FilesFrom != "" {
		argv = append(argv, "--files-from", fmt.Sprint(args.FilesFrom))
	}
	if args.Stats {
		argv = append(argv, "--stats")
	}
	if args.ItemizeChanges {
		argv = append(argv, "--itemize-changes")
	}

	argv = append(argv, args.Src, args.Dest)

	logger.F("argv", argv).Debug("prepared rsync command")

	return argv
}

func (i impl) Exec(ctx context.Context, args args.Config) error {
	cmd := i.Command(ctx, Arguments(ctx, args))
	return ext.exec(
		cmd.Path,
		cmd.Args,
		os.Environ(),
	)
}

func (i impl) RawExec(ctx context.Context, args []string) error {
	cmd := i.Command(ctx, args)
	return ext.exec(
		cmd.Path,
		cmd.Args,
		os.Environ(),
	)
}

func (impl) Command(ctx context.Context, args []string) *exec.Cmd {
	logger := log.FromContext(ctx)

	rsync, err := lookupTrueRsync(ctx)
	if err != nil {
		logger.F("error", err).Warn("Failed to look up rsync, fallback to /usr/bin/rsync")
		rsync = "/usr/bin/rsync"
	} else {
		logger.F("path", rsync).Debug("Located rsync")
	}

	return exec.CommandContext(ctx, rsync, args...)
}
