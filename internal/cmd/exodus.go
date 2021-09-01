package cmd

import (
	"context"
	"fmt"
	"path"
	"path/filepath"
	"strings"

	"github.com/release-engineering/exodus-rsync/internal/args"
	"github.com/release-engineering/exodus-rsync/internal/conf"
	"github.com/release-engineering/exodus-rsync/internal/gw"
	"github.com/release-engineering/exodus-rsync/internal/log"
	"github.com/release-engineering/exodus-rsync/internal/walk"
)

func webURI(srcPath string, srcTree string, destTree string) string {
	cleanSrcPath := path.Clean(srcPath)
	cleanSrcTree := path.Clean(srcTree)
	relPath := strings.TrimPrefix(cleanSrcPath, cleanSrcTree+"/")

	// Presence of trailing slash changes the behavior when assembling
	// destination paths, see "man rsync" and search for "trailing".
	if srcTree != "." && !strings.HasSuffix(srcTree, "/") {
		srcBase := filepath.Base(srcTree)
		return path.Join(destTree, srcBase, relPath)
	}

	return path.Join(destTree, relPath)
}

func exodusMain(ctx context.Context, cfg conf.Config, args args.Config) int {
	logger := log.FromContext(ctx)

	clientCtor := ext.gw.NewClient
	if args.DryRun {
		clientCtor = ext.gw.NewDryRunClient
	}
	gwClient, err := clientCtor(ctx, cfg)
	if err != nil {
		logger.F("error", err).Error("can't initialize exodus-gw client")
		return 101
	}

	var items []walk.SyncItem

	err = walk.Walk(ctx, args.Src, args.Exclude, func(item walk.SyncItem) error {
		if args.IgnoreExisting {
			// This argument is not (properly) supported, so bail out.
			//
			// We only check the argument here (after we've found an item) because we want
			// the argument to be accepted if we're running over a directory tree with no
			// files.
			//
			// The story with this is that some tools use an approach somewhat like this
			// to implement a "remote mkdir":
			//
			//   mkdir empty
			//   rsync --ignore-existing empty host:/dest/some/dir/which/should/be/created
			//
			// Since directories don't actually exist in exodus and there is no need to
			// create a directory before writing to a particular path, this should be a
			// no-op which successfully does nothing.  But any *other* attempted usage of
			// --ignore-existing would be dangerous to ignore, as we can't actually deliver
			// the requested semantics, so make it an error.
			return fmt.Errorf("--ignore-existing is not supported")
		}
		items = append(items, item)
		return nil
	})
	if err != nil {
		logger.F("src", args.Src, "error", err).Error("can't read files for sync")
		return 73
	}

	uploadCount := 0
	existingCount := 0

	err = gwClient.EnsureUploaded(ctx, items,
		func(uploadedItem walk.SyncItem) error {
			uploadCount++
			return nil
		},
		func(existingItem walk.SyncItem) error {
			existingCount++
			return nil
		},
	)

	if err != nil {
		logger.F("error", err).Error("can't upload files")
		return 25
	}

	logger.F("uploaded", uploadCount, "existing", existingCount).Info("Completed uploads")

	var publish gw.Publish

	if args.Publish == "" {
		// No publish provided, then create a new one.
		publish, err = gwClient.NewPublish(ctx)
		if err != nil {
			logger.F("error", err).Error("can't create publish")
			return 62
		}
		logger.F("publish", publish.ID()).Info("Created publish")
	} else {
		publish = gwClient.GetPublish(args.Publish)
		logger.F("publish", publish.ID()).Info("Joining publish")
	}

	publishItems := []gw.ItemInput{}

	for _, item := range items {
		publishItems = append(publishItems, gw.ItemInput{
			WebURI:    webURI(item.SrcPath, args.Src, args.DestPath()),
			ObjectKey: item.Key,
		})
	}

	err = publish.AddItems(ctx, publishItems)
	if err != nil {
		logger.F("error", err).Error("can't add items to publish")
		return 51
	}

	logger.F("publish", publish.ID(), "items", len(publishItems)).Info("Added publish items")

	if args.Publish == "" {
		// We created the publish, then we should commit it.
		err = publish.Commit(ctx)
		if err != nil {
			logger.F("error", err).Error("can't commit publish")
			return 71
		}
	}

	msg := "Completed successfully!"
	if args.DryRun {
		msg = "Completed successfully (in dry-run mode - no changes written)"
	}
	logger.Info(msg)

	return 0

}
