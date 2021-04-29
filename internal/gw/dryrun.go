package gw

import (
	"context"

	"github.com/release-engineering/exodus-rsync/internal/conf"
)

type dryRunPublish struct{}

func (i impl) NewDryRunClient(ctx context.Context, cfg conf.Config) (Client, error) {
	clientIface, err := i.NewClient(ctx, cfg)
	if err != nil {
		return nil, err
	}

	clientIface.(*client).dryRun = true
	return clientIface, err
}

func (*dryRunPublish) ID() string {
	return "abcd1234"
}

func (*dryRunPublish) AddItems(ctx context.Context, _ []ItemInput) error {
	return ctx.Err()
}

func (*dryRunPublish) Commit(ctx context.Context) error {
	return ctx.Err()
}
