package gw

import (
	"context"
	"fmt"

	"github.com/release-engineering/exodus-rsync/internal/log"
)

type publish struct {
	client *client
	raw    struct {
		ID    string
		Env   string
		State string
		Links map[string]string
	}
}

// ItemInput is a single item accepted for publish by the AddItems method.
type ItemInput struct {
	WebURI    string `json:"web_uri"`
	ObjectKey string `json:"object_key"`
}

// NewPublish creates and returns a new publish object within exodus-gw.
func (c *client) NewPublish(ctx context.Context) (Publish, error) {
	url := "/" + c.env.GwEnv + "/publish"

	out := &publish{}
	if err := c.doJSONRequest(ctx, "POST", url, nil, &out.raw); err != nil {
		return out, err
	}

	out.client = c

	return out, nil
}

func (p *publish) ID() string {
	return p.raw.ID
}

// AddItems will add all of the specified items onto this publish.
// This may involve multiple requests to exodus-gw.
func (p *publish) AddItems(ctx context.Context, items []ItemInput) error {
	// TODO: break up items into batches as needed.

	c := p.client
	url, ok := p.raw.Links["self"]
	if !ok {
		return fmt.Errorf("publish object is missing 'self' link: %+v", p.raw)
	}

	empty := struct{}{}
	return c.doJSONRequest(ctx, "PUT", url, items, &empty)
}

// Commit will cause this publish object to become committed, making all of
// the included content available from the CDN.
//
// The commit operation within exodus-gw is asynchronous. This method will
// wait for the commit to complete fully and will return nil only if the
// commit has succeeded.
func (p *publish) Commit(ctx context.Context) error {
	var err error

	logger := log.FromContext(ctx)
	defer logger.F("publish", p.ID()).Trace("Committing publish").Stop(&err)

	c := p.client
	url, ok := p.raw.Links["commit"]
	if !ok {
		err = fmt.Errorf("publish not eligible for commit: %+v", p.raw)
		return err
	}

	task := task{}
	if err := c.doJSONRequest(ctx, "POST", url, nil, &task.raw); err != nil {
		return err
	}

	task.client = c

	err = task.Await(ctx)
	return err
}
