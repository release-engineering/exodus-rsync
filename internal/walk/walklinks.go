package walk

import (
	"context"
	"fmt"
	fs "io/fs"
	"os"
	"path/filepath"
	"regexp"
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

// Determines if path matches pattern, striving for parity with rsync,
// ("Include/Exclude Pattern Rules", https://linux.die.net/man/1/rsync).
func matchPattern(path string, pattern string, isDir bool) (bool, error) {
	if strings.ContainsAny(pattern, "*?") {
		converted := ""
		chars := []rune(pattern)
		for i := 0; i < len(chars); i++ {
			char := string(chars[i])
			switch char {
			case `\`:
				i++
				converted += (char + string(chars[i]))
			case `*`:
				converted += `[^/]+`
			case `?`:
				converted += `[^/]`
			default:
				converted += char
			}
		}
		pattern = strings.Replace(converted, `[^/]+[^/]+`, `.*`, -1)
	}

	if strings.HasPrefix(pattern, "/") {
		pattern = `^` + pattern
	}
	if strings.HasSuffix(pattern, "/") && isDir {
		pattern = strings.TrimRight(pattern, "/") + `\z`
	}

	re, err := regexp.Compile(pattern)
	if err != nil {
		return false, err
	}

	return re.MatchString(path), nil
}

func contains(s []string, str string) bool {
	for _, v := range s {
		if v == str {
			return true
		}
	}
	return false
}

// Like filepath.WalkDir but resolves symlinks to directories.
func walkDirWithLinks(ctx context.Context, root string, exclude []string,
	onlyTheseFiles []string, fn fs.WalkDirFunc) error {
	logger := log.FromContext(ctx)

	var walkFunc fs.WalkDirFunc

	walkFunc = func(path string, d fs.DirEntry, err error) error {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		if len(onlyTheseFiles) > 0 && !contains(onlyTheseFiles, path) {
			logger.F("path", path).Debug("skipping; not included in --files-from file")
			return nil
		}

		for _, pattern := range exclude {
			isExcluded, err := matchPattern(path, pattern, d.IsDir())
			if err != nil {
				return fmt.Errorf("error processing --exclude `%s`: %w", pattern, err)
			}

			if isExcluded {
				logger.F("path", path, "exclude", pattern).Info("path excluded")
				if d.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
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
