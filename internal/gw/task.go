package gw

import (
	"context"
	"fmt"
	"time"

	"github.com/release-engineering/exodus-rsync/internal/log"
)

// Task represents a single task object within exodus-gw.
type Task struct {
	client *Client
	raw    struct {
		ID        string
		PublishID string
		State     string
		Links     map[string]string
	}
}

func (t *Task) refresh(ctx context.Context) error {
	logger := log.FromContext(ctx)

	url, ok := t.raw.Links["self"]
	if !ok {
		return fmt.Errorf("task object is missing 'self' link: %+v", *t)
	}

	logger.F("url", url).Debug("polling task")

	return t.client.doJSONRequest(ctx, "GET", url, nil, &t.raw)
}

// ID is the unique ID of this task.
func (t *Task) ID() string {
	return t.raw.ID
}

// Await will repeatedly refresh the state of this task from exodus-gw
// and return once the task has reached a terminal state.
//
// The return value will be nil if and only if the task succeeded.
func (t *Task) Await(ctx context.Context) error {
	logger := log.FromContext(ctx)

	for {
		if t.raw.State == "COMPLETE" {
			// succeeded
			logger.F("task", t.ID()).Info("Task completed")
			return nil
		}

		if t.raw.State == "FAILED" {
			logger.F("task", t.raw.ID).Info("Task failed")
			return fmt.Errorf("publish task %s failed", t.raw.ID)
		}

		// Not in a terminal state - query it again soon
		select {
		case <-ctx.Done():
			return ctx.Err()
		// TODO: make duration configurable
		case <-time.After(time.Second * 5):
		}

		if err := t.refresh(ctx); err != nil {
			return fmt.Errorf("polling task %v: %w", t.raw.ID, err)
		}
	}
}
