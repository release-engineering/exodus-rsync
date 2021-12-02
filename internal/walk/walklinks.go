package walk

import (
	"context"
	"fmt"
	fs "io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/release-engineering/exodus-rsync/internal/args"
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

func makeRegexp(pattern string) (*regexp.Regexp, error) {
	converted := ""
	chars := []rune(pattern)
	for i := 0; i < len(chars); i++ {
		char := string(chars[i])
		switch char {
		case `\`:
			// Escape; skip this and following wildcard character.
			i++
			next := string(chars[i])
			if strings.ContainsAny(next, "*?[]") {
				converted += (char + next)
			}
		case `.`:
			// Always treat as literal.
			converted += `\.`
		case `*`:
			// Matches any path component, but it stops at forward slashes (/).
			converted += `[^/]+`
		case `?`:
			// Matches any character except a forward slash (/).
			converted += `[^/]`
		default:
			converted += char
		}
	}

	pattern = strings.Replace(converted, `[^/]+[^/]+`, `.*`, -1)

	pattern = "^" + pattern + "$"

	return regexp.Compile(pattern)
}

// Determines if path matches pattern, striving for parity with rsync,
// ("Include/Exclude Pattern Rules", https://linux.die.net/man/1/rsync).
func matchPattern(path string, pattern string, isDir bool) (bool, error) {
	if strings.HasSuffix(pattern, "/") {
		// If pattern ends with a forward slash (/), only match a directory.
		if isDir {
			pattern = strings.TrimRight(pattern, "/")
		} else {
			return false, nil
		}
	}

	if strings.ContainsAny(pattern, "*?[]") {
		// Use regex for wildcard matching.
		re, err := makeRegexp(pattern)
		if err != nil {
			return false, err
		}

		components := strings.SplitAfter(path, "/")
		for c := range components {
			match := re.MatchString(components[c])
			if match {
				return match, nil
			}
		}
		return re.MatchString(path), nil
	}

	// Default to simple string matching.
	return strings.Contains(path, pattern), nil
}

func contains(s []string, str string) bool {
	for _, v := range s {
		if v == str {
			return true
		}
	}
	return false
}

func filter(logger *log.Logger, path string, exclude []string, include []string, isDir bool) error {
	filtErr := fmt.Errorf("filtered '%s'", path)

	for _, ex := range exclude {
		isExcluded, matchErr := matchPattern(path, ex, isDir)
		if matchErr != nil {
			return fmt.Errorf("could not process --exclude `%s`: %w", ex, matchErr)
		}

		if isExcluded {
			for _, in := range include {
				if in == "*/" {
					// Automatically include dirs, do not apply pattern otherwise.
					if isDir {
						return nil
					}
					continue
				}

				isIncluded, err := matchPattern(path, in, isDir)
				if err != nil {
					return fmt.Errorf("could not process --include `%s`: %w", in, err)
				}

				if isIncluded {
					logger.F("path", path, "include", in).Debug("path included")
					return nil
				}
			}

			logger.F("path", path, "exclude", ex).Debug("path excluded")

			if isDir {
				return fs.SkipDir
			}
			return filtErr
		}
	}
	return nil
}

// Like filepath.WalkDir but resolves symlinks to directories.
func walkDirWithLinks(ctx context.Context, args args.Config, onlyThese []string, fn fs.WalkDirFunc) error {
	logger := log.FromContext(ctx)

	var walkFunc fs.WalkDirFunc

	walkFunc = func(path string, d fs.DirEntry, err error) error {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err != nil {
			return fn(path, d, err)
		}

		if len(onlyThese) > 0 && !contains(onlyThese, path) {
			logger.F("path", path).Debug("skipping; not included in --files-from file")
			return nil
		}

		// The path filtered should be relative.
		filterPath := strings.TrimPrefix(filepath.Clean(path), filepath.Clean(args.Src+"/"))
		filterErr := filter(logger, filterPath, args.Excluded(), args.Included(), d.IsDir())
		if filterErr != nil {
			if strings.Contains(filterErr.Error(), fmt.Sprintf("filtered '%s'", filterPath)) {
				return nil
			}
			return filterErr
		}

		if d.Type()&fs.ModeSymlink != 0 && !args.Links {
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

	return filepath.WalkDir(args.Src, walkFunc)
}
