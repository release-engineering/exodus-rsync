package diag

import (
	"bytes"
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/release-engineering/exodus-rsync/internal/args"
	"github.com/release-engineering/exodus-rsync/internal/conf"
	"github.com/release-engineering/exodus-rsync/internal/gw"
	"github.com/release-engineering/exodus-rsync/internal/log"
	"github.com/release-engineering/exodus-rsync/internal/rsync"
)

//go:generate go run -modfile ../../go.tools.mod github.com/golang/mock/mockgen -package $GOPACKAGE -destination mock.go -source $GOFILE

// Interface defines the public interface of this package.
type Interface interface {
	// Run diagnostics.
	//
	// Diagnostics never fail, hence the lack of an error return. This
	// function is called purely for the side effect of generating
	// user-oriented logs.
	Run(context.Context, conf.Config, args.Config)
}

type impl struct{}

// Package provides the default implementation of this package's interface.
var Package Interface = impl{}

var ext = struct {
	gw    gw.Interface
	rsync rsync.Interface
}{
	gw.Package,
	rsync.Package,
}

func (impl) Run(ctx context.Context, cfg conf.Config, args args.Config) {
	logger := log.FromContext(ctx)

	logConfig(ctx, cfg)
	logCommand(ctx, cfg, args)
	logFilters(ctx, cfg, args)
	logSrctree(ctx, cfg, args)
	logGw(ctx, cfg)

	logger.Warn("=============== diagnostics: end ====================")
}

func logConfig(ctx context.Context, cfg conf.Config) {
	logger := log.FromContext(ctx)

	logger.Warn("=============== diagnostics: config =================")

	logger.F(
		"gwcert", cfg.GwCert(),
		"gwkey", cfg.GwKey(),
		"gwurl", cfg.GwURL(),
		"gwenv", cfg.GwEnv(),
		"gwpollinterval", cfg.GwPollInterval(),
		"gwbatchsize", cfg.GwBatchSize(),
		"gwmaxattempts", cfg.GwMaxAttempts(),
		"gwmaxbackoff", cfg.GwMaxBackoff(),
	).Warn("exodus-gw")

	logger.F(
		"loglevel", cfg.LogLevel(),
		"logger", cfg.Logger(),
		"verbosity", cfg.Verbosity(),
	).Warn("logging")

	logger.Debug("This is a DEBUG log.")
	logger.Info("This is an INFO log.")
	logger.Warn("This is a WARNING log.")
	logger.Error("This is an ERROR log.")
}

func logGw(ctx context.Context, cfg conf.Config) {
	logger := log.FromContext(ctx)

	logger.Warn("=============== diagnostics: exodus-gw ==============")

	client, err := ext.gw.NewDryRunClient(ctx, cfg)

	if err != nil {
		logger.F("error", err).Error("failed to create exodus-gw client")
		return
	}

	logger.Warn("exodus-gw new client: OK")

	creds, err := client.WhoAmI(ctx)

	if err != nil {
		logger.F("error", err).Error("exodus-gw request failed")
		return
	}

	logger.F("whoami", creds).Warn("exodus-gw request: OK")
}

func logCommand(ctx context.Context, cfg conf.Config, args args.Config) {
	logger := log.FromContext(ctx)

	logger.Warn("=============== diagnostics: command ================")

	envConfig, isEnv := cfg.(conf.EnvironmentConfig)

	prefix := "<no prefix matched in config>"
	strip := cfg.Strip()

	if isEnv {
		prefix = envConfig.Prefix()
	}

	logger.F("src", args.Src, "dest", args.Dest, "prefix", prefix,
		"strip", strip).Warn("paths")

	cmd, err := ext.rsync.Command(ctx, rsync.Arguments(ctx, args))
	if err != nil {
		logger.F("error", err).Error("Couldn't generate rysnc command")
		return
	}

	logger.F("mode", cfg.RsyncMode(), "path", cmd.Path, "args", cmd.Args).Warn("rsync")
}

func logSrctree(ctx context.Context, cfg conf.Config, args args.Config) {
	logger := log.FromContext(ctx)

	logger.Warn("=============== diagnostics: srctree ================")

	err := filepath.Walk(args.Src, func(path string, info fs.FileInfo, err error) error {
		name := ""
		time := time.Time{}
		size := int64(-1)
		dest := ""
		mode := fs.FileMode(0)

		if info != nil {
			name = info.Name()
			time = info.ModTime()
			size = info.Size()
			mode = info.Mode()

			// If no error yet and this is a symlink, we'll try to include the symlink dest
			// as well.
			if err == nil && (mode&fs.ModeSymlink) != 0 {
				dest, err = os.Readlink(path)
			}
		}

		logger.F("error", err, "path", path, "name", name, "time", time,
			"size", size, "mode", mode, "dest", dest).Warn("item")

		return nil
	})

	logger.F("error", err).Warn("completed walk of source tree")
}

func logFilters(ctx context.Context, cfg conf.Config, args args.Config) {
	logger := log.FromContext(ctx)

	logger.Warn("=============== diagnostics: filters ================")

	logger.F("exclude", args.Exclude, "include", args.Include,
		"filter", args.Filter, "filesfrom", args.FilesFrom).Warn("filter arguments")

	if args.FilesFrom != "" {
		content, err := os.ReadFile(args.FilesFrom)

		if err != nil {
			logger.F("error", err).Error("Can't read 'files-from' file")
			return
		}

		lines := bytes.Split(content, []byte("\n"))
		for _, line := range lines {
			logger.F("line", string(line)).Warn("files-from")
		}
	}

}
