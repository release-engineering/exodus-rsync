package gw

import (
	"context"
	"reflect"
	"testing"

	"github.com/release-engineering/exodus-rsync/internal/args"
	"github.com/release-engineering/exodus-rsync/internal/conf"
	"github.com/release-engineering/exodus-rsync/internal/log"
)

func TestClientPublish(t *testing.T) {
	cfg := conf.Config{
		GwURL:          "https://exodus-gw.example.com",
		GwCert:         "../../test/data/service.pem",
		GwKey:          "../../test/data/service-key.pem",
		GwPollInterval: 1,
	}
	env := conf.Environment{Config: &cfg, GwEnv: "env"}

	clientIface, err := Package.NewClient(env)
	if clientIface == nil {
		t.Errorf("failed to create client, err = %v", err)
	}

	ctx := context.Background()
	ctx = log.NewContext(ctx, log.Package.NewLogger(args.Config{}))

	gw := newFakeGw(t, clientIface.(*client))
	gw.createPublishIds = append(gw.createPublishIds, "abc-123-456")

	publish, err := clientIface.NewPublish(ctx)

	// It should be able to create a publish
	if err != nil {
		t.Errorf("Failed to create publish, err = %v", err)
	}

	// It should have an ID
	id := publish.ID()
	if id != "abc-123-456" {
		t.Errorf("got unexpected id %s", id)
	}

	// It should be able to add some items
	addItems := []ItemInput{
		{"/some/path", "1234"},
		{"/other/path", "223344"},
	}
	err = publish.AddItems(ctx, addItems)
	if err != nil {
		t.Errorf("failed to add items to publish, err = %v", err)
	}

	// Those items should have made it in
	gotItems := gw.publishes[publish.ID()].items
	if !reflect.DeepEqual(gotItems, addItems) {
		t.Errorf("publish state incorrect after adding items, have items: %v", gotItems)
	}

	// We should be able to commit the result
	err = publish.Commit(ctx)
	if err != nil {
		t.Errorf("unexpected error from commit: %v", err)
	}
}
