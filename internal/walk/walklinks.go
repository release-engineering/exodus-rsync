package walk

import (
	"context"
	"fmt"
	fs "io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/release-engineering/exodus-rsync/internal/log"
)

func pathRewriter(src string, dest string, fn fs.WalkDirFunc) fs.WalkDirFunc {
	return func(path string, d fs.DirEntry, err error) error {
		if strings.HasPrefix(path, src) {
			path = strings.Replace(path, src, dest, 1)
		}
		return fn(path, d, err)
	}
}

// Like filepath.WalkDir but resolves symlinks to directories.
func walkDirWithLinks(ctx context.Context, root string, fn fs.WalkDirFunc) error {
	logger := log.FromContext(ctx)

	var walkFunc fs.WalkDirFunc

	walkFunc = func(path string, d fs.DirEntry, err error) error {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		if err != nil {
			return fn(path, d, err)
		}

		if d.Type()&fs.ModeSymlink != 0 {
			var info fs.FileInfo

			resolved, err := filepath.EvalSymlinks(path)
			if err == nil {
				info, err = os.Stat(resolved)
			}

			if err != nil {
				return fn(path, d, fmt.Errorf("resolving link %s: %w", path, err))
			}

			if info.IsDir() {
				// Walk this entire directory too.
				logger.F("path", resolved).Debug("walking dir via link")

				// We need to call WalkDir on the target of the symlink, but we want
				// the callback function to receive the pre-resolution paths, so we
				// rewrite on the fly.
				thisWalker := pathRewriter(resolved, path, walkFunc)
				return filepath.WalkDir(resolved, thisWalker)
			}
		}

		// We are not looking at a symlink-to-dir, just call the real handler.
		return fn(path, d, err)
	}

	return filepath.WalkDir(root, walkFunc)
}
