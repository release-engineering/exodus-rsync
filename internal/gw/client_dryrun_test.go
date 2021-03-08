package gw

import (
	"context"
	"testing"

	"github.com/release-engineering/exodus-rsync/internal/args"
	"github.com/release-engineering/exodus-rsync/internal/conf"
	"github.com/release-engineering/exodus-rsync/internal/log"
	"github.com/release-engineering/exodus-rsync/internal/walk"
)

func TestDryRunUpload(t *testing.T) {
	ctx := context.Background()
	ctx = log.NewContext(ctx, log.Package.NewLogger(args.Config{}))

	c := client{dryRun: true}

	// It should do nothing, successfully, without even looking at the
	// sync item
	err := c.uploadBlob(ctx, walk.SyncItem{})
	if err != nil {
		t.Errorf("uploadBlob failed in dry-run mode, err = %v", err)
	}
}

func TestDryRunPublish(t *testing.T) {
	ctx := context.Background()
	ctx = log.NewContext(ctx, log.Package.NewLogger(args.Config{}))

	c := client{dryRun: true}

	// It should be able to create a publish
	p, err := c.NewPublish(ctx)
	if err != nil {
		t.Errorf("NewPublish failed in dry-run mode, err = %v", err)
	}

	// Publish object should "work" but not really do anything.
	t.Logf("Created publish %s", p.ID())

	err = p.AddItems(ctx, []ItemInput{})
	if err != nil {
		t.Errorf("AddItems failed in dry-run mode, err = %v", err)
	}

	err = p.Commit(ctx)
	if err != nil {
		t.Errorf("Commit failed in dry-run mode, err = %v", err)
	}
}

func TestNewDryRunClientOk(t *testing.T) {
	cfg := conf.Config{
		GwCert: "../../test/data/service.pem",
		GwKey:  "../../test/data/service-key.pem",
	}
	env := conf.Environment{Config: &cfg}

	clientIface, err := Package.NewDryRunClient(env)

	// Should have succeeded
	if clientIface == nil || err != nil {
		t.Errorf("unexpectedly failed to make client, client = %v, err = %v", clientIface, err)
	}

	// The client should be marked as dry-run
	client := clientIface.(*client)
	if !client.dryRun {
		t.Errorf("NewDryRunClient didn't create a client in dry-run mode!")
	}
}
