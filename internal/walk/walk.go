package walk

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"

	"github.com/release-engineering/exodus-rsync/internal/log"
	"github.com/release-engineering/exodus-rsync/internal/syncutil"
)

type SyncItemHandler func(item SyncItem) error

type walkItem struct {
	SrcPath string
	Entry   fs.DirEntry
}

// SyncItem contains information on a single item (file) to be included in sync.
type SyncItem struct {
	SrcPath string
	Key     string
	Info    fs.FileInfo
}

type syncItemPrivate struct {
	SyncItem
	Error error
}

func walkRawItems(ctx context.Context, path string, handler func(walkItem)) error {
	logger := log.FromContext(ctx)

	logger.F("path", path).Debug("start walking src tree")

	var walkFunc fs.WalkDirFunc = func(path string, d fs.DirEntry, err error) error {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		if err != nil {
			return err
		}

		handler(walkItem{SrcPath: path, Entry: d})
		return nil
	}

	return filepath.WalkDir(path, walkFunc)
}

func fileSha256Sum(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("open %s: %w", path, err)
	}
	defer file.Close()

	h := sha256.New()
	if _, err := io.Copy(h, file); err != nil {
		return "", err
	}

	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

func fillItem(ctx context.Context, c chan<- syncItemPrivate, w walkItem) error {
	logger := log.FromContext(ctx)

	info, err := w.Entry.Info()
	if err != nil {
		return fmt.Errorf("get file info for %s: %w", w.SrcPath, err)
	}

	if info.Mode().IsDir() {
		// Nothing to do
		return nil
	}

	key, err := fileSha256Sum(w.SrcPath)
	if err != nil {
		return fmt.Errorf("checksum %s: %w", w.SrcPath, err)
	}

	item := syncItemPrivate{
		SyncItem{
			SrcPath: w.SrcPath,
			Info:    info,
			Key:     key,
		},
		nil,
	}

	logger.F("goroutines", runtime.NumGoroutine(), "item", item).Debug("send item")

	c <- item
	return nil
}

func fillItems(ctx context.Context, in <-chan walkItem, c chan<- syncItemPrivate) {
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

			if err := fillItem(ctx, c, item); err != nil {
				c <- syncItemPrivate{Error: err}
			}
		}
	}
}

func getSyncItems(ctx context.Context, path string) <-chan syncItemPrivate {
	c := make(chan syncItemPrivate, 10)
	walkItemCh := make(chan walkItem, 10)

	go func() {
		err := walkRawItems(ctx, path, func(wi walkItem) {
			walkItemCh <- wi
		})

		if err != nil {
			c <- syncItemPrivate{Error: err}
		}

		close(walkItemCh)
	}()

	go syncutil.RunWithGroup(20,
		func() {
			fillItems(ctx, walkItemCh, c)
		},
		func() {
			close(c)
		},
	)

	return c
}

func Walk(ctx context.Context, path string, handler SyncItemHandler) error {
	logger := log.FromContext(ctx)

	for item := range getSyncItems(ctx, path) {
		logger.F("item", item).Debug("got item")

		if item.Error != nil {
			return item.Error
		}
		if err := handler(item.SyncItem); err != nil {
			return err
		}
	}

	return nil
}
