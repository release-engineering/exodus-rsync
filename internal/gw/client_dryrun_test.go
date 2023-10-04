package gw

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/golang/mock/gomock"

	"github.com/release-engineering/exodus-rsync/internal/args"
	"github.com/release-engineering/exodus-rsync/internal/conf"
	"github.com/release-engineering/exodus-rsync/internal/log"
	"github.com/release-engineering/exodus-rsync/internal/walk"
)

func TestNewDryRunClientCertError(t *testing.T) {
	ctrl := gomock.NewController(t)
	cfg := conf.NewMockConfig(ctrl)

	cfg.EXPECT().GwCert().Return("cert-does-not-exist")
	cfg.EXPECT().GwKey().Return("key-does-not-exist")

	_, err := Package.NewDryRunClient(context.Background(), cfg)

	// Should have given us this error
	if !strings.Contains(fmt.Sprint(err), "can't load cert/key") {
		t.Error("did not get expected error, err =", err)
	}
}

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

	type publishGetter func(t *testing.T) Publish

	cases := []struct {
		name   string
		getter publishGetter
	}{
		{"new publish",
			func(t *testing.T) Publish {
				p, err := c.NewPublish(ctx)
				if err != nil {
					t.Fatalf("NewPublish failed in dry-run mode, err = %v", err)
				}
				return p
			},
		},

		{"get publish",
			func(t *testing.T) Publish {
				p, err := c.GetPublish(ctx, "whatever-id")
				if err != nil {
					t.Fatalf("GetPublish failed in dry-run mode, err = %v", err)
				}
				return p
			},
		},
	}

	for _, testcase := range cases {
		t.Run(testcase.name, func(t *testing.T) {
			p := testcase.getter(t)

			// Publish object should "work" but not really do anything.
			t.Logf("Got publish %s", p.ID())

			err := p.AddItems(ctx, []ItemInput{})
			if err != nil {
				t.Errorf("AddItems failed in dry-run mode, err = %v", err)
			}

			err = p.Commit(ctx, "")
			if err != nil {
				t.Errorf("Commit failed in dry-run mode, err = %v", err)
			}
		})
	}
}

func TestNewDryRunClientOk(t *testing.T) {
	cfg := testConfig(t)

	clientIface, err := Package.NewDryRunClient(context.Background(), cfg)

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
