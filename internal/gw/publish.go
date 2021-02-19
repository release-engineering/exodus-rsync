package gw

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	"github.com/release-engineering/exodus-rsync/internal/log"
)

type Publish struct {
	client *Client
	raw    struct {
		ID    string
		Env   string
		State string
		Links map[string]string
	}
}

type ItemInput struct {
	WebUri    string `json:"web_uri"`
	ObjectKey string `json:"object_key"`
	FromDate  string `json:"from_date"`
}

func (c *Client) NewPublish(ctx context.Context) (Publish, error) {
	url := "/" + c.env.GwEnv + "/publish"

	out := Publish{}
	if err := c.doJSONRequest(ctx, "POST", url, nil, &out.raw); err != nil {
		return out, err
	}

	out.client = c

	return out, nil
}

func (p *Publish) ID() string {
	return p.raw.ID
}

func (p *Publish) AddItems(ctx context.Context, items []ItemInput) error {
	// TODO: break up items into batches as needed.

	c := p.client
	url, ok := p.raw.Links["self"]
	if !ok {
		return fmt.Errorf("publish object is missing 'self' link: %+v", p.raw)
	}

	body := bytes.Buffer{}
	enc := json.NewEncoder(&body)
	if err := enc.Encode(items); err != nil {
		return fmt.Errorf("encoding items as JSON: %w", err)
	}

	empty := struct{}{}
	return c.doJSONRequest(ctx, "PUT", url, &body, &empty)
}

func (p *Publish) Commit(ctx context.Context) error {
	var err error

	logger := log.FromContext(ctx)
	defer logger.F("publish", p.ID()).Trace("Committing publish").Stop(&err)

	c := p.client
	url, ok := p.raw.Links["commit"]
	if !ok {
		err = fmt.Errorf("publish not eligible for commit: %+v", p.raw)
		return err
	}

	task := Task{}
	if err := c.doJSONRequest(ctx, "POST", url, nil, &task.raw); err != nil {
		return err
	}

	task.client = c

	err = task.Await(ctx)
	return err
}
