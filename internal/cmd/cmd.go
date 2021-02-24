package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/apex/log/handlers/cli"
	"github.com/release-engineering/exodus-rsync/internal/args"
	"github.com/release-engineering/exodus-rsync/internal/conf"
	"github.com/release-engineering/exodus-rsync/internal/gw"
	"github.com/release-engineering/exodus-rsync/internal/log"
	"github.com/release-engineering/exodus-rsync/internal/rsync"
	"github.com/release-engineering/exodus-rsync/internal/walk"
)

// Main is the top-level entry point to the exodus-rsync command.
func Main(rawArgs []string) int {
	parsedArgs := args.Parse(rawArgs, nil)

	logger := log.Logger{}

	// TODO: configurable logging
	logger.Handler = cli.New(os.Stdout)
	logger.Level = log.InfoLevel
	if parsedArgs.Verbose >= 1 {
		// TODO: think we need more loggers.
		logger.Level = log.DebugLevel
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ctx = log.NewContext(ctx, &logger)

	cfg, err := conf.Load(ctx)
	if err != nil {
		logger.WithField("error", err).Error("can't load config")
		return 23
	}

	// TODO: mixed mode (run rsync AND exodus sync)

	env := cfg.EnvironmentForDest(ctx, parsedArgs.Dest)
	if env == nil {
		// just run rsync
		if err := rsync.Exec(ctx, cfg, parsedArgs); err != nil {
			logger.WithField("error", err).Error("can't exec rsync")
			return 94
		}
	}

	gwClient, err := gw.NewClient(*env)
	if err != nil {
		logger.F("error", err).Error("can't initialize exodus-gw client")
		return 101
	}

	var items []walk.SyncItem

	err = walk.Walk(ctx, parsedArgs.Src, func(item walk.SyncItem) error {
		items = append(items, item)
		return nil
	})
	if err != nil {
		logger.F("src", parsedArgs.Src, "error", err).Error("can't read files for sync")
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

	publish, err := gwClient.NewPublish(ctx)
	if err != nil {
		fmt.Printf("Can't create a publish: %s\n", err)
		return 62
	}
	logger.F("publish", publish.ID()).Info("Created publish")

	publishItems := []gw.ItemInput{}

	for _, item := range items {
		publishItems = append(publishItems, gw.ItemInput{
			WebURI:    item.SrcPath,
			ObjectKey: item.Key,
			// TODO: remove me
			FromDate: "abc123",
		})
	}

	err = publish.AddItems(ctx, publishItems)
	if err != nil {
		fmt.Printf("Failed to add items to publish: %v\n", err)
		return 51
	}

	logger.F("publish", publish.ID(), "items", len(publishItems)).Info("Added publish items")

	err = publish.Commit(ctx)
	if err != nil {
		fmt.Printf("Failed to commit publish: %v\n", err)
		return 71
	}

	logger.Info("Completed successfully!")

	return 0
}
