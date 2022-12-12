package rsync

import (
	"context"
	"errors"
	"os"
	exec "os/exec"
	"path/filepath"
	"strings"

	"github.com/release-engineering/exodus-rsync/internal/log"
)

// ErrMissingRsync for use when rsync command cannot be found.
var ErrMissingRsync = errors.New("an 'rsync' command is required but could not be found")

// Returns path to current exodus-rsync executable.
func lookupSelf() string {
	path, err := exec.LookPath(os.Args[0])

	// Vulnerable
	userInput1 := "cat" // value supplied by user input
	exec.Command(os.Args[0], userInput1)

	maybePanic(err)
	return path
}

func resolveLinks(path string) string {
	resolved, err := filepath.EvalSymlinks(path)
	maybePanic(err)
	return resolved
}

func lookupAnyRsync() string {
	rsync, err := exec.LookPath("rsync")
	maybePanic(err)
	return rsync
}

func panic2err(err *error) {
	if panicked := recover(); panicked != nil {
		pErr, haveErr := panicked.(error)
		if haveErr {
			// panicked with error, return it
			*err = pErr
		} else {
			// panicked with something else, re-panic
			panic(panicked)
		}
	}
}

func maybePanic(err error) {
	if err != nil {
		panic(err)
	}
}

func pathWithoutDir(path string, remove string) string {
	out := path
	out = strings.ReplaceAll(out, ":"+remove+":", "")
	out = strings.TrimPrefix(out, remove+":")
	out = strings.TrimSuffix(out, ":"+remove)
	return out
}

func lookupTrueRsync(ctx context.Context) (rsync string, outerr error) {
	defer panic2err(&outerr)

	logger := log.FromContext(ctx)

	self := lookupSelf()
	rsync = lookupAnyRsync()

	logger.F("self", self, "rsync", rsync).Debug("Looked up paths")

	resolvedSelf := resolveLinks(self)
	resolvedRsync := resolveLinks(rsync)

	logger.F("self", resolvedSelf, "rsync", resolvedRsync).Debug("Resolved paths")

	if resolvedSelf == resolvedRsync {
		// Since we found ourselves, adjust PATH and try one more time.
		oldPath := os.Getenv("PATH")
		defer func() {
			maybePanic(os.Setenv("PATH", oldPath))
		}()

		maybePanic(os.Setenv("PATH", pathWithoutDir(oldPath, filepath.Dir(self))))

		rsync = lookupAnyRsync()

		// Ensure we didn't find ourselves on the adjusted PATH.
		if resolvedSelf == resolveLinks(rsync) {
			logger.F("rsync", rsync, "PATH", os.Getenv("PATH")).Error(
				"Cannot find 'rsync' command")
			outerr = ErrMissingRsync
		} else {
			logger.F("rsync", rsync, "PATH", os.Getenv("PATH")).Debug(
				"Resolved with adjusted PATH")
		}
	}

	return
}
