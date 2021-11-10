package walk

import (
	"context"
	"crypto/sha256"
	"fmt"
	"hash"
	"io"
	"io/fs"
	"os"
	"runtime"

	"github.com/release-engineering/exodus-rsync/internal/log"
	"github.com/release-engineering/exodus-rsync/internal/syncutil"
)

// SyncItemHandler is a callback invoked on each sync item as discovered while
// walking the source tree for publish.
//
// If it returns an error, the walk process is stopped.
type SyncItemHandler func(item SyncItem) error

type walkItem struct {
	SrcPath string
	Entry   fs.DirEntry
	Error   error
}

// SyncItem contains information on a single item (file) to be included in sync.
type SyncItem struct {
	SrcPath string
	Key     string
	LinkTo  string
	Info    fs.FileInfo
}

type syncItemPrivate struct {
	SyncItem
	Error error
}

func fileHash(path string, hasher hash.Hash) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	if _, err := io.Copy(hasher, file); err != nil {
		return "", err
	}

	return fmt.Sprintf("%x", hasher.Sum(nil)), nil
}

func fillItem(ctx context.Context, c chan<- syncItemPrivate, w walkItem, links bool) error {
	logger := log.FromContext(ctx)

	if w.Error != nil {
		return w.Error
	}

	info, err := w.Entry.Info()
	if err != nil {
		return fmt.Errorf("get file info for %s: %w", w.SrcPath, err)
	}

	if info.Mode().IsDir() {
		// Nothing to do
		return nil
	}

	var (
		key    string
		linkTo string
	)

	if w.Entry.Type()&fs.ModeSymlink != 0 && links {
		linkTo, err = os.Readlink(w.SrcPath)
		if err != nil {
			return err
		}
	} else {
		key, err = fileHash(w.SrcPath, sha256.New())
		if err != nil {
			return fmt.Errorf("checksum %s: %w", w.SrcPath, err)
		}
	}

	item := syncItemPrivate{
		SyncItem{
			SrcPath: w.SrcPath,
			Key:     key,
			LinkTo:  linkTo,
			Info:    info,
		},
		nil,
	}

	logger.F("goroutines", runtime.NumGoroutine(), "item", item).Debug("send item")

	c <- item
	return nil
}

func fillItems(ctx context.Context, in <-chan walkItem, c chan<- syncItemPrivate, links bool) {
	logger := log.FromContext(ctx)

	for {
		select {

		case <-ctx.Done():
			logger.F("error", ctx.Err()).Debug("fillItems returning early")
			return

		case item, ok := <-in:
			if !ok {
				logger.Debug("fillItems completed normally")
				return
			}

			if err := fillItem(ctx, c, item, links); err != nil {
				c <- syncItemPrivate{Error: err}
			}
		}
	}
}

func getSyncItems(ctx context.Context, path string, exclude []string, include []string, onlyThese []string, links bool) <-chan syncItemPrivate {
	c := make(chan syncItemPrivate, 10)
	walkItemCh := make(chan walkItem, 10)

	go func() {
		err := walkDirWithLinks(ctx, path, exclude, include, onlyThese, links,
			func(path string, d fs.DirEntry, err error) error {
				if err != nil {
					return err
				}
				walkItemCh <- walkItem{SrcPath: path, Entry: d}
				return nil
			})

		if err != nil {
			walkItemCh <- walkItem{Error: err}
		}

		close(walkItemCh)
	}()

	go syncutil.RunWithGroup(20,
		func() {
			fillItems(ctx, walkItemCh, c, links)
		},
		func() {
			close(c)
		},
	)

	return c
}

// Walk will walk the directory tree at the given path and invoke a handler
// for every discovered item eligible for sync.
func Walk(ctx context.Context, path string, exclude []string, include []string, onlyThese []string, links bool, handler SyncItemHandler) error {
	logger := log.FromContext(ctx)

	for item := range getSyncItems(ctx, path, exclude, include, onlyThese, links) {
		logger.F("item", item).Debug("got item")

		if ctx.Err() != nil {
			return ctx.Err()
		}
		if item.Error != nil {
			return item.Error
		}
		if err := handler(item.SyncItem); err != nil {
			return err
		}
	}

	return ctx.Err()
}
